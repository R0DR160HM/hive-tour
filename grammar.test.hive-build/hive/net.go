package hive

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type NetHttpRequest struct {
	Method  string
	Url     string
	Headers Table
	Body    string
}

type NetHttpResponse struct {
	Status  int
	Headers Table
	Body    string
}

type NetHttpError struct {
	Url     string
	Message string
}

func (e NetHttpError) Error() string {
	return "hive: http error for " + e.Url + ": " + e.Message
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

func NetHttpSend(req NetHttpRequest) Result[NetHttpResponse, NetHttpError] {
	fail := func(err error) Result[NetHttpResponse, NetHttpError] {
		return Err[NetHttpResponse, NetHttpError](NetHttpError{Url: req.Url, Message: err.Error()})
	}
	hreq, err := http.NewRequest(strings.ToUpper(req.Method), req.Url, strings.NewReader(req.Body))
	if err != nil {
		return fail(err)
	}
	for _, row := range req.Headers {
		if len(row) >= 2 {
			hreq.Header.Add(row[0], row[1])
		}
	}
	resp, err := httpClient.Do(hreq)
	if err != nil {
		return fail(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fail(err)
	}
	return Ok[NetHttpResponse, NetHttpError](NetHttpResponse{
		Status:  resp.StatusCode,
		Headers: headerTable(resp.Header),
		Body:    string(body),
	})
}

func NetHttpServe(port int, handler func(NetHttpRequest) NetHttpResponse) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		resp := handler(NetHttpRequest{
			Method:  r.Method,
			Url:     r.URL.String(),
			Headers: headerTable(r.Header),
			Body:    string(body),
		})
		for _, row := range resp.Headers {
			if len(row) >= 2 {
				w.Header().Add(row[0], row[1])
			}
		}
		status := resp.Status
		if status == 0 {
			status = 200
		}
		w.WriteHeader(status)
		io.WriteString(w, resp.Body)
	})
	if err := http.ListenAndServe(":"+strconv.Itoa(port), mux); err != nil {
		panic("hive: net.httpServe: " + err.Error())
	}
}

func headerTable(h http.Header) Table {
	table := Table{}
	for name, values := range h {
		for _, value := range values {
			table = append(table, []string{name, value})
		}
	}
	return table
}

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

const wsMaxPayload = 32 << 20

const (
	wsContinuation byte = 0x0
	wsText         byte = 0x1
	wsBinary       byte = 0x2
	wsCloseOp      byte = 0x8
	wsPing         byte = 0x9
	wsPong         byte = 0xA
)

type NetWsError struct {
	Reason  string
	Message string
}

func (e NetWsError) Error() string {
	return "hive: websocket " + e.Reason + ": " + e.Message
}

type NetWsConnection struct {
	ws *wsSocket
}

type wsSocket struct {
	conn net.Conn
	r    *bufio.Reader
	masking bool
	req NetHttpRequest
	mu     sync.Mutex
	closed bool

	frag   []byte
	fragOp byte
}

func (s *wsSocket) readFrame() (byte, bool, []byte, error) {
	var head [2]byte
	if _, err := io.ReadFull(s.r, head[:]); err != nil {
		return 0, false, nil, err
	}
	fin := head[0]&0x80 != 0
	opcode := head[0] & 0x0F
	masked := head[1]&0x80 != 0
	length := uint64(head[1] & 0x7F)
	switch length {
	case 126:
		var ext [2]byte
		if _, err := io.ReadFull(s.r, ext[:]); err != nil {
			return 0, false, nil, err
		}
		length = uint64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err := io.ReadFull(s.r, ext[:]); err != nil {
			return 0, false, nil, err
		}
		length = binary.BigEndian.Uint64(ext[:])
	}
	if length > wsMaxPayload {
		return 0, false, nil, errors.New("the peer announced a frame larger than the 32MiB limit")
	}
	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(s.r, mask[:]); err != nil {
			return 0, false, nil, err
		}
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(s.r, payload); err != nil {
		return 0, false, nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return opcode, fin, payload, nil
}

func (s *wsSocket) writeFrame(opcode byte, payload []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("the connection is closed")
	}
	maskBit := byte(0)
	if s.masking {
		maskBit = 0x80
	}
	n := len(payload)
	frame := []byte{0x80 | opcode}
	switch {
	case n < 126:
		frame = append(frame, maskBit|byte(n))
	case n <= 0xFFFF:
		frame = append(frame, maskBit|126)
		var ext [2]byte
		binary.BigEndian.PutUint16(ext[:], uint16(n))
		frame = append(frame, ext[:]...)
	default:
		frame = append(frame, maskBit|127)
		var ext [8]byte
		binary.BigEndian.PutUint64(ext[:], uint64(n))
		frame = append(frame, ext[:]...)
	}
	if s.masking {
		var mask [4]byte
		if _, err := rand.Read(mask[:]); err != nil {
			return err
		}
		frame = append(frame, mask[:]...)
		for i := 0; i < n; i++ {
			frame = append(frame, payload[i]^mask[i%4])
		}
	} else {

		both := net.Buffers{frame, payload}
		_, err := both.WriteTo(s.conn)
		return err
	}
	_, err := s.conn.Write(frame)
	return err
}

func (s *wsSocket) close() {
	s.mu.Lock()
	already := s.closed
	s.closed = true
	s.mu.Unlock()
	if !already {
		_ = s.conn.Close()
	}
}

func (s *wsSocket) receive() (string, *NetWsError) {
	for {
		opcode, fin, payload, err := s.readFrame()
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				s.close()
				return "", &NetWsError{Reason: "Closed", Message: "the peer closed the connection"}
			}
			return "", &NetWsError{Reason: "Receive", Message: err.Error()}
		}
		switch opcode {
		case wsPing:
			if err := s.writeFrame(wsPong, payload); err != nil {
				return "", &NetWsError{Reason: "Send", Message: err.Error()}
			}
		case wsPong:

		case wsCloseOp:
			_ = s.writeFrame(wsCloseOp, payload)
			s.close()
			return "", &NetWsError{Reason: "Closed", Message: "the peer closed the connection"}
		case wsText, wsBinary:
			if fin {
				return string(payload), nil
			}
			s.frag = append([]byte{}, payload...)
			s.fragOp = opcode
		case wsContinuation:
			if s.fragOp == 0 {
				return "", &NetWsError{
					Reason:  "Protocol",
					Message: "a continuation frame arrived with no message to continue",
				}
			}
			s.frag = append(s.frag, payload...)
			if fin {
				message := string(s.frag)
				s.frag, s.fragOp = nil, 0
				return message, nil
			}
		default:
			return "", &NetWsError{
				Reason:  "Protocol",
				Message: "unknown opcode " + strconv.Itoa(int(opcode)),
			}
		}
	}
}

func wsAcceptKey(key string) string {
	sum := sha1.Sum([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func wsAccept(w http.ResponseWriter, r *http.Request) (*wsSocket, error) {
	key := r.Header.Get("Sec-WebSocket-Key")
	upgrading := strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") &&
		strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
	if !upgrading || key == "" {
		http.Error(w, "expected a WebSocket upgrade request", http.StatusBadRequest)
		return nil, errors.New("not a WebSocket upgrade request")
	}
	if version := r.Header.Get("Sec-WebSocket-Version"); version != "13" {
		w.Header().Set("Sec-WebSocket-Version", "13")
		http.Error(w, "unsupported WebSocket version "+version, http.StatusBadRequest)
		return nil, errors.New("unsupported WebSocket version " + version)
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "this connection cannot be upgraded", http.StatusInternalServerError)
		return nil, errors.New("the connection cannot be hijacked")
	}
	body, _ := io.ReadAll(r.Body)
	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return nil, err
	}
	handshake := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + wsAcceptKey(key) + "\r\n\r\n"
	if _, err := rw.WriteString(handshake); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := rw.Flush(); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return &wsSocket{
		conn: conn,
		r:    rw.Reader,
		req: NetHttpRequest{
			Method:  r.Method,
			Url:     r.URL.String(),
			Headers: headerTable(r.Header),
			Body:    string(body),
		},
	}, nil
}

func NetWsConnect(rawURL string) Result[NetWsConnection, NetWsError] {
	fail := func(reason string, message string) Result[NetWsConnection, NetWsError] {
		return Err[NetWsConnection, NetWsError](NetWsError{Reason: reason, Message: message})
	}
	target, err := url.Parse(rawURL)
	if err != nil {
		return fail("Handshake", "cannot parse the URL: "+err.Error())
	}
	secure := false
	switch strings.ToLower(target.Scheme) {
	case "ws", "http":
	case "wss", "https":
		secure = true
	default:
		return fail("Handshake", "unsupported scheme "+strconv.Quote(target.Scheme)+"; expected ws:// or wss://")
	}
	address := target.Host
	if target.Port() == "" {
		if secure {
			address = net.JoinHostPort(address, "443")
		} else {
			address = net.JoinHostPort(address, "80")
		}
	}
	conn, err := net.DialTimeout("tcp", address, 30*time.Second)
	if err != nil {
		return fail("Handshake", err.Error())
	}
	if secure {
		secured := tls.Client(conn, &tls.Config{ServerName: target.Hostname()})
		if err := secured.Handshake(); err != nil {
			_ = conn.Close()
			return fail("Handshake", err.Error())
		}
		conn = secured
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		_ = conn.Close()
		return fail("Handshake", err.Error())
	}
	key := base64.StdEncoding.EncodeToString(nonce[:])
	path := target.RequestURI()
	if path == "" {
		path = "/"
	}
	request := "GET " + path + " HTTP/1.1\r\n" +
		"Host: " + target.Host + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + key + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	if _, err := io.WriteString(conn, request); err != nil {
		_ = conn.Close()
		return fail("Handshake", err.Error())
	}

	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		_ = conn.Close()
		return fail("Handshake", err.Error())
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		_ = conn.Close()
		return fail("Handshake", "the server answered "+strconv.Itoa(resp.StatusCode)+" instead of 101")
	}
	if resp.Header.Get("Sec-WebSocket-Accept") != wsAcceptKey(key) {
		_ = conn.Close()
		return fail("Handshake", "the server's Sec-WebSocket-Accept does not match the key we sent")
	}
	return Ok[NetWsConnection, NetWsError](NetWsConnection{ws: &wsSocket{
		conn:    conn,
		r:       reader,
		masking: true,
		req: NetHttpRequest{
			Method:  "GET",
			Url:     rawURL,
			Headers: headerTable(resp.Header),
		},
	}})
}

func NetWsSend(c NetWsConnection, message string) Result[int, NetWsError] {
	if c.ws == nil {
		return Err[int, NetWsError](NetWsError{Reason: "Closed", Message: "the connection is not open"})
	}
	if err := c.ws.writeFrame(wsText, []byte(message)); err != nil {
		return Err[int, NetWsError](NetWsError{Reason: "Send", Message: err.Error()})
	}
	return Ok[int, NetWsError](len(message))
}

func NetWsReceive(c NetWsConnection) Result[string, NetWsError] {
	if c.ws == nil {
		return Err[string, NetWsError](NetWsError{Reason: "Closed", Message: "the connection is not open"})
	}
	message, werr := c.ws.receive()
	if werr != nil {
		return Err[string, NetWsError](*werr)
	}
	return Ok[string, NetWsError](message)
}

func NetWsRequest(c NetWsConnection) NetHttpRequest {
	if c.ws == nil {
		return NetHttpRequest{Headers: Table{}}
	}
	return c.ws.req
}

func NetWsClose(c NetWsConnection) {
	if c.ws == nil {
		return
	}

	_ = c.ws.writeFrame(wsCloseOp, []byte{0x03, 0xE8})
	c.ws.close()
}

func NetWsServe(port int, handler func(NetWsConnection)) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		socket, err := wsAccept(w, r)
		if err != nil {

			return
		}
		defer socket.close()
		handler(NetWsConnection{ws: socket})
	})
	if err := http.ListenAndServe(":"+strconv.Itoa(port), mux); err != nil {
		panic("hive: net.wsServe: " + err.Error())
	}
}

type NetSocketError struct {
	Reason  string
	Message string
}

func (e NetSocketError) Error() string {
	return "hive: socket " + e.Reason + ": " + e.Message
}

type NetSocketConnection struct {
	sock *socket
}

type socket struct {
	conn net.Conn
	r    *bufio.Reader
}

func NetSocketConnect(host string, port int) Result[NetSocketConnection, NetSocketError] {
	address := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", address, 30*time.Second)
	if err != nil {
		return Err[NetSocketConnection, NetSocketError](NetSocketError{Reason: "Connect", Message: err.Error()})
	}
	return Ok[NetSocketConnection, NetSocketError](NetSocketConnection{
		sock: &socket{conn: conn, r: bufio.NewReader(conn)},
	})
}

func NetSocketSend(c NetSocketConnection, data string) Result[int, NetSocketError] {
	if c.sock == nil {
		return Err[int, NetSocketError](NetSocketError{Reason: "Closed", Message: "the connection is not open"})
	}
	n, err := c.sock.conn.Write([]byte(data))
	if err != nil {
		return Err[int, NetSocketError](NetSocketError{Reason: "Send", Message: err.Error()})
	}
	return Ok[int, NetSocketError](n)
}

func NetSocketReceive(c NetSocketConnection, limit int) Result[string, NetSocketError] {
	if c.sock == nil {
		return Err[string, NetSocketError](NetSocketError{Reason: "Closed", Message: "the connection is not open"})
	}
	if limit <= 0 {
		return Ok[string, NetSocketError]("")
	}
	buf := make([]byte, limit)
	n, err := c.sock.r.Read(buf)
	if n > 0 {
		return Ok[string, NetSocketError](string(buf[:n]))
	}
	if err == io.EOF {
		return Err[string, NetSocketError](NetSocketError{Reason: "Closed", Message: "the peer closed the connection"})
	}
	if err != nil {
		return Err[string, NetSocketError](NetSocketError{Reason: "Receive", Message: err.Error()})
	}
	return Ok[string, NetSocketError]("")
}

func NetSocketReceiveLine(c NetSocketConnection) Result[string, NetSocketError] {
	if c.sock == nil {
		return Err[string, NetSocketError](NetSocketError{Reason: "Closed", Message: "the connection is not open"})
	}
	line, err := c.sock.r.ReadString('\n')
	line = strings.TrimRight(line, "\n")
	line = strings.TrimRight(line, "\r")
	if err != nil {

		if line != "" {
			return Ok[string, NetSocketError](line)
		}
		if err == io.EOF {
			return Err[string, NetSocketError](NetSocketError{Reason: "Closed", Message: "the peer closed the connection"})
		}
		return Err[string, NetSocketError](NetSocketError{Reason: "Receive", Message: err.Error()})
	}
	return Ok[string, NetSocketError](line)
}

func NetSocketPeer(c NetSocketConnection) string {
	if c.sock == nil {
		return ""
	}
	return c.sock.conn.RemoteAddr().String()
}

func NetSocketClose(c NetSocketConnection) {
	if c.sock != nil {
		_ = c.sock.conn.Close()
	}
}

func NetSocketServe(port int, handler func(NetSocketConnection)) {
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		panic("hive: net.socketServe: " + err.Error())
	}
	for {
		conn, err := listener.Accept()
		if err != nil {

			continue
		}
		go func(c net.Conn) {
			defer c.Close()
			handler(NetSocketConnection{sock: &socket{conn: c, r: bufio.NewReader(c)}})
		}(conn)
	}
}

type NetNetError struct {
	Reason  string
	Message string
}

func (e NetNetError) Error() string {
	return "hive: net " + e.Reason + ": " + e.Message
}

func netFail[T any](reason, message string) Result[T, NetNetError] {
	return Err[T, NetNetError](NetNetError{Reason: reason, Message: message})
}

const netResolveTimeout = 10 * time.Second

func NetResolve(name string) Result[[]string, NetNetError] {
	if strings.TrimSpace(name) == "" {
		return netFail[[]string]("NotFound", "there is no name here to resolve")
	}
	ctx, cancel := context.WithTimeout(context.Background(), netResolveTimeout)
	defer cancel()
	addresses, err := net.DefaultResolver.LookupHost(ctx, name)
	if err != nil {
		var dns *net.DNSError
		if errors.As(err, &dns) && dns.IsNotFound {
			return netFail[[]string]("NotFound", "no address is registered for "+name)
		}
		return netFail[[]string]("Lookup", "could not resolve "+name+": "+err.Error())
	}
	if len(addresses) == 0 {
		return netFail[[]string]("NotFound", "no address is registered for "+name)
	}
	return Ok[[]string, NetNetError](addresses)
}

func NetLocalAddress() Result[string, NetNetError] {
	if probe, err := net.Dial("udp", "192.0.2.1:9"); err == nil {
		host, _, splitErr := net.SplitHostPort(probe.LocalAddr().String())
		probe.Close()
		if splitErr == nil {
			if ip := net.ParseIP(host); ip != nil && !ip.IsLoopback() && !ip.IsUnspecified() {
				return Ok[string, NetNetError](ip.String())
			}
		}
	}

	if address, ok := netInterfaceAddress(false); ok {
		return Ok[string, NetNetError](address)
	}
	if address, ok := netInterfaceAddress(true); ok {
		return Ok[string, NetNetError](address)
	}
	return netFail[string]("NoAddress",
		"this machine has no address a peer could reach it on — only loopback")
}

func netInterfaceAddress(v6 bool) (string, bool) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", false
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			cidr, ok := address.(*net.IPNet)
			if !ok || cidr.IP == nil {
				continue
			}
			if cidr.IP.IsLoopback() || cidr.IP.IsLinkLocalUnicast() {
				continue
			}
			if (cidr.IP.To4() == nil) != v6 {
				continue
			}
			return cidr.IP.String(), true
		}
	}
	return "", false
}

func NetUrlEncode(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if ('A' <= c && c <= 'Z') || ('a' <= c && c <= 'z') || ('0' <= c && c <= '9') || c == '-' || c == '.' || c == '_' || c == '~' {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&15])
	}
	return b.String()
}

type NetQueryParamsError struct {
	Path     string
	Expected string
	Found    string
}

func (e NetQueryParamsError) Error() string {
	return "hive: cannot decode query string at " + e.Path + ": expected " + e.Expected + ", found " + e.Found
}

func NetQueryParamsCodec() CodecCodec[NetQueryParamsError] {
	return CodecCodec[NetQueryParamsError]{}
}

type NetQueryPair struct {
	Key   string
	Value string
}

func NetQueryParse[T any](text string, dec func([]NetQueryPair) (T, *NetQueryParamsError)) Result[T, NetQueryParamsError] {
	pairs := []NetQueryPair{}
	for _, part := range strings.Split(strings.TrimPrefix(text, "?"), "&") {
		if part == "" {
			continue
		}
		rawKey, rawValue, _ := strings.Cut(part, "=")
		key, kerr := url.QueryUnescape(rawKey)
		value, verr := url.QueryUnescape(rawValue)
		if kerr != nil || verr != nil {
			return Err[T, NetQueryParamsError](NetQueryParamsError{Path: "$", Expected: "a query string", Found: strconv.Quote(part)})
		}
		pairs = append(pairs, NetQueryPair{Key: key, Value: value})
	}
	out, qerr := dec(pairs)
	if qerr != nil {
		return Err[T, NetQueryParamsError](*qerr)
	}
	return Ok[T, NetQueryParamsError](out)
}

func netQueryValues(q []NetQueryPair, key string) []string {
	out := []string{}
	for _, p := range q {
		if p.Key == key {
			out = append(out, p.Value)
		}
	}
	return out
}

func NetQueryHas(q []NetQueryPair, key string) bool {
	return len(netQueryValues(q, key)) > 0
}

func netQueryCount(n int) string {
	if n == 1 {
		return "1 value"
	}
	return strconv.Itoa(n) + " values"
}

func NetQueryOne[T any](q []NetQueryPair, key string, read func(string, string) (T, *NetQueryParamsError)) (T, *NetQueryParamsError) {
	var zero T
	path := "$." + key
	values := netQueryValues(q, key)
	if len(values) == 0 {
		return zero, &NetQueryParamsError{Path: path, Expected: "a value", Found: "nothing"}
	}
	if len(values) > 1 {
		return zero, &NetQueryParamsError{Path: path, Expected: "one value", Found: netQueryCount(len(values))}
	}
	return read(values[0], path)
}

func NetQueryVec[T any](q []NetQueryPair, key string, n int, read func(string, string) (T, *NetQueryParamsError)) ([]T, *NetQueryParamsError) {
	path := "$." + key
	values := netQueryValues(q, key)
	if n >= 0 && len(values) != n {
		return nil, &NetQueryParamsError{Path: path, Expected: netQueryCount(n), Found: netQueryCount(len(values))}
	}
	out := make([]T, 0, len(values))
	for i, v := range values {
		e, qerr := read(v, path+"["+strconv.Itoa(i)+"]")
		if qerr != nil {
			return nil, qerr
		}
		out = append(out, e)
	}
	return out, nil
}

func NetQueryStr(s string, path string) (string, *NetQueryParamsError) {
	return s, nil
}

func NetQueryInt(s string, path string) (int, *NetQueryParamsError) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, &NetQueryParamsError{Path: path, Expected: "Int", Found: strconv.Quote(s)}
	}
	return n, nil
}

func NetQueryFloat(s string, path string) (float64, *NetQueryParamsError) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, &NetQueryParamsError{Path: path, Expected: "Float", Found: strconv.Quote(s)}
	}
	return f, nil
}

func NetQueryBool(s string, path string) (bool, *NetQueryParamsError) {
	if s == "true" || s == "false" {
		return s == "true", nil
	}
	return false, &NetQueryParamsError{Path: path, Expected: "Bool", Found: strconv.Quote(s)}
}

func NetQueryAtom(s string, path string) (Atom, *NetQueryParamsError) {
	for i, name := range atomNames {
		if name == s {
			return Atom(i), nil
		}
	}
	return 0, &NetQueryParamsError{Path: path, Expected: "a known atom", Found: strconv.Quote(s)}
}

func NetQueryEncodeStr(s string) string { return s }

func NetQueryEncodeInt(i int) string { return strconv.Itoa(i) }

func NetQueryEncodeFloat(f float64) string { return strconv.FormatFloat(f, 'g', -1, 64) }

func NetQueryEncodeBool(b bool) string { return strconv.FormatBool(b) }

func NetQueryEncodeAtom(a Atom) string { return a.String() }

func NetQueryOf(key string, value string) []NetQueryPair {
	return []NetQueryPair{{Key: key, Value: value}}
}

func NetQueryEach[T any](key string, items []T, write func(T) string) []NetQueryPair {
	out := make([]NetQueryPair, 0, len(items))
	for _, item := range items {
		out = append(out, NetQueryPair{Key: key, Value: write(item)})
	}
	return out
}

func NetQueryJoin(groups ...[]NetQueryPair) string {
	parts := []string{}
	for _, group := range groups {
		for _, p := range group {
			parts = append(parts, NetUrlEncode(p.Key)+"="+NetUrlEncode(p.Value))
		}
	}
	return strings.Join(parts, "&")
}
