package hive

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	syslinkMagic = "HIVE-SL1"

	syslinkTickEvery = 15 * time.Second
	syslinkTickStep  = 5 * time.Second
	syslinkMaxFrame  = 8 << 20
	syslinkMaxOutbox = 64 << 20
)

func syslinkSilenceBudget() time.Duration {
	total := time.Duration(0)
	for wait := syslinkTickEvery; wait > 0; wait -= syslinkTickStep {
		total += wait
	}
	return total
}

type frameKind byte

const (
	kindCast frameKind = iota + 1
	kindRequest
	kindReply
	kindMonitor
	kindStop
	kindTick
	kindNoProc
	kindDown

	kindPong
)

type frame struct {
	kind    frameKind
	name    Atom
	id      uint64
	ref     uint64
	watcher uint64
	digest  uint32
	payload []byte
}

var (
	nodeMu sync.RWMutex
	selfEndpoint string
	sessions = map[string]*session{}

	dialed    = map[string]string{}
	watchesMu sync.Mutex
	watches   = map[string][]watcher{}
	monSeq    atomic.Uint64
)

func SyslinkNode() string {
	nodeMu.RLock()
	defer nodeMu.RUnlock()
	return selfEndpoint
}

func SyslinkPeers() []string {
	nodeMu.RLock()
	peers := make([]string, 0, len(sessions))
	for node := range sessions {
		peers = append(peers, node)
	}
	nodeMu.RUnlock()
	sort.Strings(peers)
	return peers
}

func bindHost(endpoint, port string) string {
	host := strings.Trim(strings.TrimSuffix(endpoint, ":"+port), "[]")
	if ip := net.ParseIP(host); host == "localhost" || (ip != nil && ip.IsLoopback()) {
		return host
	}
	return ""
}

func SyslinkListen(endpoint string) Result[string, SyslinkSyslinkError] {
	if _, err := clusterSecret(); err != nil {
		return syslinkErr[string]("NoKey", err.Error())
	}
	cert, err := ephemeralCert()
	if err != nil {
		return syslinkErr[string]("NoKey", "could not generate this node's key: "+err.Error())
	}
	port := endpoint
	if i := strings.LastIndex(endpoint, ":"); i >= 0 {
		port = endpoint[i+1:]
	}
	if _, err := strconv.Atoi(port); err != nil {
		return syslinkErr[string]("NoListener",
			"`"+endpoint+"` is not a host:port this node can listen on")
	}
	ln, err := tls.Listen("tcp", net.JoinHostPort(bindHost(endpoint, port), port), serverTLS(cert))
	if err != nil {
		return syslinkErr[string]("NoListener", "could not listen on port "+port+": "+err.Error())
	}

	if shown := ln.Addr().String(); strings.LastIndex(shown, ":") >= 0 {
		bound := shown[strings.LastIndex(shown, ":")+1:]
		endpoint = endpoint[:len(endpoint)-len(port)] + bound
	}
	nodeMu.Lock()
	selfEndpoint = endpoint
	nodeMu.Unlock()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go acceptSession(conn.(*tls.Conn))
		}
	}()
	return Ok[string, SyslinkSyslinkError](endpoint)
}

type session struct {
	node string
	conn *tls.Conn

	outbound bool

	mu     sync.Mutex
	cond   *sync.Cond
	out    [][]byte
	bytes  int
	closed bool

	lastSeen time.Time
}

func newSession(node string, conn *tls.Conn, outbound bool) *session {
	s := &session{node: node, conn: conn, outbound: outbound, lastSeen: time.Now()}
	s.cond = sync.NewCond(&s.mu)
	return s
}

func (s *session) enqueue(b []byte) bool {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return false
	}
	if s.bytes+len(b) > syslinkMaxOutbox {
		s.mu.Unlock()
		s.shutdown("the outbox passed " + strconv.Itoa(syslinkMaxOutbox) + " bytes")
		return false
	}
	s.out = append(s.out, b)
	s.bytes += len(b)
	s.mu.Unlock()
	s.cond.Signal()
	return true
}

func (s *session) drain() ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for len(s.out) == 0 && !s.closed {
		s.cond.Wait()
	}
	if len(s.out) == 0 {
		return nil, false
	}
	b := s.out[0]
	s.out = s.out[1:]
	s.bytes -= len(b)
	return b, true
}

func (s *session) shutdown(reason string) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.out = nil
	s.bytes = 0
	s.mu.Unlock()
	s.cond.Broadcast()
	s.conn.Close()

	nodeMu.Lock()
	if sessions[s.node] == s {
		delete(sessions, s.node)
	}
	for typed, canonical := range dialed {
		if canonical == s.node {
			delete(dialed, typed)
		}
	}
	nodeMu.Unlock()

	failCallsToNode(s.node, "the node at "+s.node+" is unreachable: "+reason)
	nodeDown(s.node)
	fmt.Fprintln(os.Stderr, "hive: the node at " + s.node + " is down (" + reason + ")")
}

func (s *session) writer() {
	w := bufio.NewWriter(s.conn)
	for {
		b, ok := s.drain()
		if !ok {
			return
		}
		if _, err := w.Write(b); err != nil {
			s.shutdown("write failed: " + err.Error())
			return
		}

		s.mu.Lock()
		more := len(s.out) > 0
		s.mu.Unlock()
		if !more {
			if err := w.Flush(); err != nil {
				s.shutdown("flush failed: " + err.Error())
				return
			}
		}
	}
}

func (s *session) reader() {
	r := bufio.NewReader(s.conn)
	quiet := syslinkSilenceBudget() + syslinkTickStep
	for {
		s.conn.SetReadDeadline(time.Now().Add(quiet))
		f, err := readFrame(r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				s.shutdown("the peer hung up")
			} else {
				s.shutdown("read failed: " + err.Error())
			}
			return
		}
		s.mu.Lock()
		s.lastSeen = time.Now()
		s.mu.Unlock()
		switch f.kind {
		case kindTick:
			s.enqueue(writeFrame(frame{kind: kindPong}))
		case kindPong:

		default:
			routeFrame(s.node, f)
		}
	}
}

func (s *session) ticker() {
	wait := syslinkTickEvery

	probed := time.Now()
	s.enqueue(writeFrame(frame{kind: kindTick}))
	for {
		time.Sleep(wait)
		s.mu.Lock()
		closed, answered := s.closed, s.lastSeen.After(probed)
		s.mu.Unlock()
		if closed {
			return
		}
		if answered {
			wait = syslinkTickEvery
		} else {
			wait -= syslinkTickStep
			if wait <= 0 {
				s.shutdown("it answered no health check in " + syslinkSilenceBudget().String())
				return
			}
		}
		probed = time.Now()
		s.enqueue(writeFrame(frame{kind: kindTick}))
	}
}

func (s *session) start() {
	go s.writer()
	go s.reader()
	go s.ticker()
}

func sessionFor(endpoint string) (*session, *SyslinkSyslinkError) {
	if endpoint == "" {
		return nil, &SyslinkSyslinkError{Reason: "NoPeer", Message: "the address does not say where its node is"}
	}
	nodeMu.RLock()
	canonical, known := dialed[endpoint]
	s, ok := sessions[endpoint]
	if !ok && known {
		s, ok = sessions[canonical]
	}
	nodeMu.RUnlock()
	if ok {
		return s, nil
	}
	if SyslinkNode() == "" {
		return nil, &SyslinkSyslinkError{
			Reason:  "NoListener",
			Message: "this node is not listening yet — call hive.syslink.listen before addressing another node",
		}
	}
	return dialSession(endpoint)
}

func dialSession(endpoint string) (*session, *SyslinkSyslinkError) {
	cert, err := ephemeralCert()
	if err != nil {
		return nil, &SyslinkSyslinkError{Reason: "NoKey", Message: err.Error()}
	}
	raw, err := tls.Dial("tcp", endpoint, clientTLS(cert))
	if err != nil {
		return nil, &SyslinkSyslinkError{
			Reason:  "Unreachable",
			Message: "could not reach the node at " + endpoint + ": " + err.Error(),
		}
	}

	advertised, err := handshakeInitiator(raw)
	if err != nil {
		raw.Close()
		return nil, &SyslinkSyslinkError{Reason: "Unreachable", Message: err.Error()}
	}
	nodeMu.Lock()
	dialed[endpoint] = advertised
	nodeMu.Unlock()
	return install(advertised, raw, true)
}

func install(node string, conn *tls.Conn, outbound bool) (*session, *SyslinkSyslinkError) {
	keep := preferOutbound(node) == outbound

	nodeMu.Lock()
	existing, clash := sessions[node]
	if clash {
		if !keep {
			nodeMu.Unlock()
			conn.Close()
			return existing, nil
		}
		delete(sessions, node)
	}
	s := newSession(node, conn, outbound)
	sessions[node] = s
	nodeMu.Unlock()

	if clash {

		existing.closeQuietly()
	}
	s.start()
	return s, nil
}

func preferOutbound(node string) bool {
	return SyslinkNode() < node
}

func (s *session) closeQuietly() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.out = nil
	s.bytes = 0
	s.mu.Unlock()
	s.cond.Broadcast()
	s.conn.Close()
}

func acceptSession(conn *tls.Conn) {
	peer, err := handshakeResponder(conn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hive: rejected a connection: " + err.Error())

		reject := append([]byte(syslinkMagic), 0, 0)
		conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		conn.Write(reject)
		conn.Close()
		return
	}
	install(peer, conn, false)
}

func sendFrame(endpoint string, f frame) *SyslinkSyslinkError {
	s, err := sessionFor(endpoint)
	if err != nil {
		return err
	}
	if !s.enqueue(writeFrame(f)) {
		return &SyslinkSyslinkError{Reason: "Unreachable", Message: "the node at " + endpoint + " is unreachable"}
	}
	return nil
}

func routeFrame(from string, f frame) {
	switch f.kind {
	case kindCast, kindRequest:
		dest := SyslinkAddress{Name: f.name, Id: f.id}
		b, ok := resolveLocal(dest)
		if !ok {
			if f.kind == kindRequest {
				sendFrame(from, frame{
					kind:    kindNoProc,
					ref:     f.ref,
					payload: []byte("NoProc|no service is registered here under that name"),
				})
			}
			return
		}
		ref := uint64(0)
		origin := ""
		if f.kind == kindRequest {
			ref = f.ref
			origin = from
		}
		b.post(delivery{raw: f.payload, digest: f.digest, ref: ref, origin: origin})
	case kindReply:
		deliverAnswer(f.ref, answer{raw: f.payload})
	case kindNoProc:
		reason, message := "NoProc", string(f.payload)
		if i := strings.Index(message, "|"); i >= 0 {
			reason, message = message[:i], message[i+1:]
		}
		deliverAnswer(f.ref, answer{err: &SyslinkSyslinkError{Reason: reason, Message: message}})
	case kindMonitor:
		registerRemoteWatcher(
			SyslinkAddress{Name: f.name, Id: f.id},
			from, f.watcher, f.ref, f.payload, f.digest)
	case kindDown:

		forgetRemoteWatch(from, f.ref)
		if b, ok := boxes.Load(f.id); ok {
			b.(*mailbox).post(delivery{raw: f.payload, digest: f.digest, origin: ""})
		}
	case kindStop:
		if b, ok := resolveLocal(SyslinkAddress{Name: f.name, Id: f.id}); ok {
			b.die("killed")
		}
	}
}

func rememberRemoteWatch(node string, w watcher) {
	watchesMu.Lock()
	watches[node] = append(watches[node], w)
	watchesMu.Unlock()
}

func forgetRemoteWatch(node string, mon uint64) {
	watchesMu.Lock()
	defer watchesMu.Unlock()
	kept := watches[node][:0]
	for _, w := range watches[node] {
		if w.mon != mon {
			kept = append(kept, w)
		}
	}
	if len(kept) == 0 {
		delete(watches, node)
		return
	}
	watches[node] = kept
}

func nodeDown(node string) {
	watchesMu.Lock()
	ws := watches[node]
	delete(watches, node)
	watchesMu.Unlock()
	for _, w := range ws {
		notifyWatcher(w)
	}
}

func writeFrame(f frame) []byte {
	name := ""
	if f.name != atomNone {
		name = f.name.String()
	}
	body := make([]byte, 0, 32+len(name)+len(f.payload))
	body = append(body, byte(f.kind))
	body = binary.BigEndian.AppendUint64(body, f.ref)
	body = binary.BigEndian.AppendUint64(body, f.id)
	body = binary.BigEndian.AppendUint64(body, f.watcher)
	body = binary.BigEndian.AppendUint32(body, f.digest)
	body = binary.BigEndian.AppendUint16(body, uint16(len(name)))
	body = append(body, name...)
	body = binary.BigEndian.AppendUint32(body, uint32(len(f.payload)))
	body = append(body, f.payload...)

	out := binary.BigEndian.AppendUint32(make([]byte, 0, 4+len(body)), uint32(len(body)))
	return append(out, body...)
}

func readFrame(r *bufio.Reader) (frame, error) {
	var head [4]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return frame{}, err
	}
	size := binary.BigEndian.Uint32(head[:])
	if size > syslinkMaxFrame {
		return frame{}, fmt.Errorf("frame of %d bytes exceeds the %d byte limit", size, syslinkMaxFrame)
	}
	body := make([]byte, size)
	if _, err := io.ReadFull(r, body); err != nil {
		return frame{}, err
	}
	if len(body) < 31 {
		return frame{}, errors.New("truncated frame header")
	}
	f := frame{kind: frameKind(body[0])}
	f.ref = binary.BigEndian.Uint64(body[1:9])
	f.id = binary.BigEndian.Uint64(body[9:17])
	f.watcher = binary.BigEndian.Uint64(body[17:25])
	f.digest = binary.BigEndian.Uint32(body[25:29])
	nameLen := int(binary.BigEndian.Uint16(body[29:31]))
	rest := body[31:]
	if len(rest) < nameLen+4 {
		return frame{}, errors.New("truncated frame name")
	}
	f.name = atomNone
	if nameLen > 0 {
		f.name = atomByName(string(rest[:nameLen]))
	}
	rest = rest[nameLen:]
	payloadLen := int(binary.BigEndian.Uint32(rest[:4]))
	rest = rest[4:]
	if len(rest) < payloadLen {
		return frame{}, errors.New("truncated frame payload")
	}
	f.payload = rest[:payloadLen]
	return f, nil
}

var (
	atomIndexOnce sync.Once
	atomIndex     map[string]Atom
)

func atomByName(name string) Atom {
	atomIndexOnce.Do(func() {
		atomIndex = make(map[string]Atom, len(atomNames))
		for i, n := range atomNames {
			atomIndex[n] = Atom(i)
		}
	})
	if a, ok := atomIndex[name]; ok {
		return a
	}
	return atomNone
}

var (
	secretOnce sync.Once
	secretVal  Secret
	secretErr  error
	chosenMu   sync.RWMutex
	chosenKey  Secret
)

func clusterSecret() (Secret, error) {
	chosenMu.RLock()
	chosen := chosenKey
	chosenMu.RUnlock()
	if chosen.p != nil {
		return chosen, nil
	}
	secretOnce.Do(func() {
		if env := os.Getenv("HIVE_SYSLINK_KEY"); env != "" {
			secretVal, secretErr = keptKey([]byte(env))
			return
		}
		home, err := os.UserHomeDir()
		if err != nil {
			secretErr = errors.New("no HIVE_SYSLINK_KEY is set and this machine has no home directory to keep one in")
			return
		}
		dir := filepath.Join(home, ".hive")
		path := filepath.Join(dir, "syslink.key")
		if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
			secretVal, secretErr = keptKey(b)
			return
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			secretErr = errors.New("could not create " + dir + ": " + err.Error())
			return
		}
		fresh := make([]byte, 32)
		if _, err := rand.Read(fresh); err != nil {
			secretErr = err
			return
		}
		if err := os.WriteFile(path, fresh, 0o600); err != nil {
			secretErr = errors.New("could not write " + path + ": " + err.Error())
			return
		}
		secretVal, secretErr = keptKey(fresh)
	})
	return secretVal, secretErr
}

func keptKey(b []byte) (Secret, error) {
	s, err := hideBytes(b)
	if err != nil {
		return s, errors.New("the cluster key: " + err.Message)
	}
	return s, nil
}

func SyslinkSetKey(key Secret) {
	if len(key.bytes()) == 0 {
		panic("hive: syslink.setKey: an empty key proves nothing about who holds it")
	}
	chosenMu.Lock()
	chosenKey = key
	chosenMu.Unlock()
}

var (
	certOnce sync.Once
	certVal  tls.Certificate
	certErr  error
)

func ephemeralCert() (tls.Certificate, error) {
	certOnce.Do(func() {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			certErr = err
			return
		}
		serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
		if err != nil {
			certErr = err
			return
		}
		template := x509.Certificate{
			SerialNumber:          serial,
			Subject:               pkix.Name{CommonName: "hive-syslink"},
			NotBefore:             time.Now().Add(-time.Hour),
			NotAfter:              time.Now().Add(24 * 365 * time.Hour),
			KeyUsage:              x509.KeyUsageDigitalSignature,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
			BasicConstraintsValid: true,
		}
		der, err := x509.CreateCertificate(rand.Reader, &template, &template, pub, priv)
		if err != nil {
			certErr = err
			return
		}
		certVal = tls.Certificate{Certificate: [][]byte{der}, PrivateKey: priv}
	})
	return certVal, certErr
}

func serverTLS(cert tls.Certificate) *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
		MaxVersion:   tls.VersionTLS13,

		ClientAuth: tls.RequireAnyClientCert,
	}
}

func clientTLS(cert tls.Certificate) *tls.Config {
	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		MinVersion:         tls.VersionTLS13,
		MaxVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true,
	}
}

func bindingProof(secret Secret, role string, mine, theirs []byte) []byte {
	defer runtime.KeepAlive(secret)
	mac := hmac.New(sha256.New, secret.bytes())
	mac.Write([]byte("hive-syslink-v1|"))
	mac.Write([]byte(role))
	mac.Write([]byte("|"))
	mac.Write(mine)
	mac.Write(theirs)
	return mac.Sum(nil)
}

func certHashes(conn *tls.Conn) (mine, theirs []byte, err error) {
	cert, err := ephemeralCert()
	if err != nil {
		return nil, nil, err
	}
	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return nil, nil, errors.New("the peer presented no certificate")
	}
	m := sha256.Sum256(cert.Certificate[0])
	t := sha256.Sum256(state.PeerCertificates[0].Raw)
	return m[:], t[:], nil
}

func handshakeInitiator(conn *tls.Conn) (string, error) {
	if err := conn.Handshake(); err != nil {
		return "", errors.New("TLS handshake failed: " + err.Error())
	}
	secret, err := clusterSecret()
	if err != nil {
		return "", err
	}
	mine, theirs, err := certHashes(conn)
	if err != nil {
		return "", err
	}

	hello := []byte(syslinkMagic)
	hello = appendString(hello, SyslinkNode())
	hello = append(hello, bindingProof(secret, "initiator", mine, theirs)...)
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if _, err := conn.Write(hello); err != nil {
		return "", err
	}

	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	r := bufio.NewReader(conn)
	magic := make([]byte, len(syslinkMagic))
	if _, err := io.ReadFull(r, magic); err != nil {
		return "", err
	}
	if string(magic) != syslinkMagic {
		return "", errors.New("the peer does not speak syslink")
	}
	advertised, err := readString(r)
	if err != nil {
		return "", err
	}
	if advertised == "" {
		return "", errors.New(
			"the peer turned this connection away — its cluster key does not match this node's")
	}
	proof := make([]byte, sha256.Size)
	if _, err := io.ReadFull(r, proof); err != nil {
		return "", err
	}
	want := bindingProof(secret, "responder", theirs, mine)
	if !hmac.Equal(proof, want) {
		return "", errors.New("the peer could not prove it shares this cluster's key")
	}
	conn.SetReadDeadline(time.Time{})
	conn.SetWriteDeadline(time.Time{})
	return advertised, nil
}

func handshakeResponder(conn *tls.Conn) (string, error) {
	if err := conn.Handshake(); err != nil {
		return "", errors.New("TLS handshake failed: " + err.Error())
	}
	secret, err := clusterSecret()
	if err != nil {
		return "", err
	}
	mine, theirs, err := certHashes(conn)
	if err != nil {
		return "", err
	}

	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	r := bufio.NewReader(conn)
	magic := make([]byte, len(syslinkMagic))
	if _, err := io.ReadFull(r, magic); err != nil {
		return "", err
	}
	if string(magic) != syslinkMagic {
		return "", errors.New("the caller does not speak syslink")
	}
	advertised, err := readString(r)
	if err != nil {
		return "", err
	}
	proof := make([]byte, sha256.Size)
	if _, err := io.ReadFull(r, proof); err != nil {
		return "", err
	}
	want := bindingProof(secret, "initiator", theirs, mine)
	if !hmac.Equal(proof, want) {
		return "", errors.New("the caller could not prove it shares this cluster's key")
	}
	if advertised == "" {
		return "", errors.New("the caller did not say where it can be reached")
	}

	hello := []byte(syslinkMagic)
	hello = appendString(hello, SyslinkNode())
	hello = append(hello, bindingProof(secret, "responder", mine, theirs)...)
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if _, err := conn.Write(hello); err != nil {
		return "", err
	}
	conn.SetReadDeadline(time.Time{})
	conn.SetWriteDeadline(time.Time{})

	if r.Buffered() > 0 {
		buffered, _ := io.ReadAll(io.LimitReader(r, int64(r.Buffered())))
		return advertised, pushBack(conn, buffered)
	}
	return advertised, nil
}

func pushBack(conn *tls.Conn, buffered []byte) error {
	if len(bytes.TrimSpace(buffered)) == 0 {
		return nil
	}
	return errors.New("the caller sent frames before its handshake was acknowledged")
}

func appendString(dst []byte, s string) []byte {
	dst = binary.BigEndian.AppendUint16(dst, uint16(len(s)))
	return append(dst, s...)
}

func readString(r *bufio.Reader) (string, error) {
	var head [2]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return "", err
	}
	n := int(binary.BigEndian.Uint16(head[:]))
	b := make([]byte, n)
	if _, err := io.ReadFull(r, b); err != nil {
		return "", err
	}
	return string(b), nil
}
