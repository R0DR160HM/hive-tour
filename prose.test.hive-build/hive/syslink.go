package hive

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

const atomNone Atom = -1

type SyslinkSyslinkError struct {
	Reason  string
	Message string
}

func (e SyslinkSyslinkError) String() string { return e.Reason + ": " + e.Message }

func syslinkErr[T any](reason, message string) Result[T, SyslinkSyslinkError] {
	return Err[T, SyslinkSyslinkError](SyslinkSyslinkError{Reason: reason, Message: message})
}

func SyslinkEncoder[T any](enc func(T) string) func(T) []byte {
	return func(v T) []byte { return []byte(enc(v)) }
}

func SyslinkDecoder[T any](dec func(JsonValue, string) (T, *JsonJsonError)) func([]byte) (any, error) {
	return func(b []byte) (any, error) {
		r := JsonParse(string(b), dec)
		if r.IsError() {
			return nil, errors.New(jsonWhy(r.Err()))
		}
		return r.Ok(), nil
	}
}

func SyslinkReplyDecoder[T any](dec func(JsonValue, string) (T, *JsonJsonError)) func([]byte) (T, error) {
	return func(b []byte) (T, error) {
		r := JsonParse(string(b), dec)
		if r.IsError() {
			var zero T
			return zero, errors.New(jsonWhy(r.Err()))
		}
		return r.Ok(), nil
	}
}

func JsonEncodeAddress(a SyslinkAddress) string {
	where := a.Endpoint
	if where == "" {

		where = SyslinkNode()
	}
	name := ""
	if a.Name != atomNone {
		name = a.Name.String()
	}
	return JsonEncodeStr(where + "|" + name + "|" + strconv.FormatUint(a.Id, 10))
}

func JsonAddress(v JsonValue, path string) (SyslinkAddress, *JsonJsonError) {
	text, jerr := JsonStr(v, path)
	if jerr != nil {
		return SyslinkAddress{}, jerr
	}
	parts := strings.Split(text, "|")
	if len(parts) != 3 {
		return SyslinkAddress{}, &JsonJsonError{Path: path, Expected: "a syslink address", Found: JsonEncodeStr(text)}
	}
	addr := SyslinkAddress{Endpoint: parts[0], Name: atomNone}
	if parts[1] != "" {
		addr.Name = atomByName(parts[1])
	}
	if id, err := strconv.ParseUint(parts[2], 10, 64); err == nil {
		addr.Id = id
	}

	if b, ok := resolveLocal(addr); ok {
		addr.box = b
	}
	return addr, nil
}

func jsonWhy(e JsonJsonError) string {
	where := e.Path
	if where == "" {
		where = "the message"
	}
	return "expected " + e.Expected + " at " + where + ", found " + e.Found
}

type SyslinkAddress struct {

	Endpoint string
	Name     Atom
	Id       uint64
	box      *mailbox
}

func (a SyslinkAddress) String() string {
	where := a.Endpoint
	if where == "" {
		where = SyslinkNode()
	}
	if where == "" {
		where = "this node"
	}
	if a.Name != atomNone {
		return "<" + where + "/" + a.Name.String() + ">"
	}
	return "<" + where + "#" + strconv.FormatUint(a.Id, 10) + ">"
}

type syslinkTurn struct {
	self  *mailbox
	drawn bool
}

func (t syslinkTurn) address() SyslinkAddress {
	return SyslinkAddress{Endpoint: SyslinkNode(), Name: t.self.name, Id: t.self.id, box: t.self}
}

type delivery struct {
	value  any
	raw    []byte
	digest uint32
	ref    uint64

	origin string
	drawn  bool
}

type watcher struct {
	box     *mailbox
	value   any
	node    string
	id      uint64
	mon     uint64
	payload []byte
	digest  uint32
}

type mailbox struct {
	id   uint64
	name Atom

	mu     sync.Mutex
	cond   *sync.Cond
	queue  []delivery
	closed bool

	decode func([]byte) (any, error)
	digest uint32

	watchers []watcher
}

var (
	boxSeq atomic.Uint64
	boxes  sync.Map

	registryOnce sync.Once
	registry     []atomic.Pointer[mailbox]
)

func registrySlots() []atomic.Pointer[mailbox] {
	registryOnce.Do(func() {
		registry = make([]atomic.Pointer[mailbox], len(atomNames))
	})
	return registry
}

func newMailbox(decode func([]byte) (any, error), digest uint32) *mailbox {
	b := &mailbox{id: boxSeq.Add(1), name: atomNone, decode: decode, digest: digest}
	b.cond = sync.NewCond(&b.mu)
	boxes.Store(b.id, b)
	return b
}

func (b *mailbox) post(d delivery) {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		failDelivery(d, "Down", "the service is no longer running")
		return
	}
	b.queue = append(b.queue, d)
	b.mu.Unlock()
	b.cond.Signal()
}

func (b *mailbox) take() (delivery, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for len(b.queue) == 0 && !b.closed {
		b.cond.Wait()
	}
	if len(b.queue) == 0 {
		return delivery{}, false
	}
	d := b.queue[0]
	b.queue = b.queue[1:]
	return d, true
}

func (b *mailbox) value(d delivery) (any, error) {
	if d.raw == nil {
		return d.value, nil
	}
	if d.digest != 0 && b.digest != 0 && d.digest != b.digest {
		return nil, fmt.Errorf(
			"message type digest %08x does not match the mailbox's %08x — the sender was built from a different message type",
			d.digest, b.digest)
	}
	if b.decode == nil {
		return nil, fmt.Errorf("this service cannot decode messages from the wire")
	}
	return b.decode(d.raw)
}

func (b *mailbox) die(reason string) {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	ws := b.watchers
	b.watchers = nil
	if b.name != atomNone {
		slots := registrySlots()
		if int(b.name) >= 0 && int(b.name) < len(slots) {
			slots[b.name].CompareAndSwap(b, nil)
		}
	}

	orphans := b.queue
	b.queue = nil
	b.mu.Unlock()
	b.cond.Broadcast()

	for _, d := range orphans {
		failDelivery(d, "Down", "the service died before answering: "+reason)
	}
	boxes.Delete(b.id)
	failCallsTo(b.id, reason)
	for _, w := range ws {
		notifyWatcher(w)
	}
}

func failDelivery(d delivery, reason, message string) {
	if d.ref == 0 {
		return
	}
	if d.origin == "" {
		deliverAnswer(d.ref, answer{err: &SyslinkSyslinkError{Reason: reason, Message: message}})
		return
	}

	sendFrame(d.origin, frame{kind: kindNoProc, ref: d.ref, payload: []byte(reason + "|" + message)})
}

func notifyWatcher(w watcher) {
	if w.box != nil {
		w.box.post(delivery{value: w.value, origin: ""})
		return
	}
	sendFrame(w.node, frame{
		kind:    kindDown,
		name:    atomNone,
		id:      w.id,
		ref:     w.mon,
		digest:  w.digest,
		payload: w.payload,
	})
}

const SyslinkUnnamed = atomNone

func SyslinkSpawn[S any, M any](
	handler func(*S, M) M,
	name Atom,
	initial S,
	decode func([]byte) (any, error),
	digest uint32,
	encode func(M) []byte,
	clone func(M) M,
) Result[SyslinkAddress, SyslinkSyslinkError] {
	turn := func(state *S, msg M, _ syslinkTurn) M { return handler(state, msg) }
	return spawnService(starterOf(funcKey(handler)), turn, name, initial, decode, digest, encode, clone)
}

func SyslinkSpawnAddressed[S any, M any](
	handler func(*S, M, SyslinkAddress) M,
	name Atom,
	initial S,
	decode func([]byte) (any, error),
	digest uint32,
	encode func(M) []byte,
	clone func(M) M,
) Result[SyslinkAddress, SyslinkSyslinkError] {
	turn := func(state *S, msg M, t syslinkTurn) M { return handler(state, msg, t.address()) }
	return spawnService(starterOf(funcKey(handler)), turn, name, initial, decode, digest, encode, clone)
}

type syslinkStarter func(turn any, initial any) (any, func(SyslinkAddress))

var starters sync.Map

func funcKey[F any](f F) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&f))
}

func startsAs(key unsafe.Pointer, s syslinkStarter) {
	starters.Store(key, s)
}

func starterOf(key unsafe.Pointer) syslinkStarter {
	if s, ok := starters.Load(key); ok {
		return s.(syslinkStarter)
	}
	return nil
}

func spawnService[S any, M any](
	starter syslinkStarter,
	turn func(*S, M, syslinkTurn) M,
	name Atom,
	initial S,
	decode func([]byte) (any, error),
	digest uint32,
	encode func(M) []byte,
	clone func(M) M,
) Result[SyslinkAddress, SyslinkSyslinkError] {
	var started func(SyslinkAddress)
	if starter != nil {
		folded, then := starter(turn, initial)
		turn = folded.(func(*S, M, syslinkTurn) M)
		started = then
	}
	b := newMailbox(decode, digest)
	addr := SyslinkAddress{Name: atomNone, Id: b.id, box: b}
	if name != atomNone {
		b.name = name
		slots := registrySlots()
		if int(name) < 0 || int(name) >= len(slots) || !slots[name].CompareAndSwap(nil, b) {
			boxes.Delete(b.id)
			return syslinkErr[SyslinkAddress]("Taken", "the name "+name.String()+" is already registered on this node")
		}
		addr = SyslinkAddress{Endpoint: SyslinkNode(), Name: name, Id: b.id, box: b}
	}
	go runService(b, turn, initial, encode, clone)
	if started != nil {
		started(addr)
	}
	return Ok[SyslinkAddress, SyslinkSyslinkError](addr)
}

func runService[S any, M any](
	b *mailbox,
	turn func(*S, M, syslinkTurn) M,
	initial S,
	encode func(M) []byte,
	clone func(M) M,
) {
	state := initial
	reason := "normal"
	for {
		d, ok := b.take()
		if !ok {
			break
		}
		raw, err := b.value(d)
		if err != nil {
			fmt.Fprintln(os.Stderr, "hive: " + serviceLabel(b) + " dropped a message: " + err.Error())
			failDelivery(d, "Decode", serviceLabel(b)+" could not read the message: "+err.Error())
			continue
		}
		msg, fits := raw.(M)
		if !fits {
			fmt.Fprintln(os.Stderr, "hive: " + serviceLabel(b) + " dropped a message of an unexpected type")
			failDelivery(d, "Decode", serviceLabel(b)+" does not handle messages of that type")
			continue
		}
		out, crashed := serviceTurn(turn, &state, msg, syslinkTurn{self: b, drawn: d.drawn})
		if crashed != nil {
			reason = Show(crashed)
			fmt.Fprintln(os.Stderr, "hive: " + serviceLabel(b) + " crashed: " + reason)
			failDelivery(d, "Down", "the service crashed while handling the request: "+reason)
			break
		}
		replyTo(d, out, encode, clone)
	}
	b.die(reason)
}

func serviceTurn[S any, M any](
	turn func(*S, M, syslinkTurn) M,
	state *S,
	msg M,
	t syslinkTurn,
) (out M, crashed any) {
	defer func() {
		if r := recover(); r != nil {
			crashed = r
		}
	}()
	return turn(state, msg, t), nil
}

func replyTo[M any](d delivery, out M, encode func(M) []byte, clone func(M) M) {
	if d.ref == 0 {
		return
	}
	if d.origin != "" {
		sendFrame(d.origin, frame{kind: kindReply, ref: d.ref, payload: encode(out)})
		return
	}
	if syslinkStrict() {
		deliverAnswer(d.ref, answer{raw: encode(out)})
		return
	}
	deliverAnswer(d.ref, answer{value: clone(out)})
}

func serviceLabel(b *mailbox) string {
	if b.name != atomNone {
		return "service " + b.name.String()
	}
	return "service #" + strconv.FormatUint(b.id, 10)
}

func SyslinkKill(addr SyslinkAddress) {
	if b, ok := resolveLocal(addr); ok {
		b.die("killed")
		return
	}
	sendFrame(addr.Endpoint, frame{kind: kindStop, name: addr.Name, id: addr.Id})
}

func SyslinkAt(name Atom) SyslinkAddress {
	if b := lookupName(name); b != nil {
		return SyslinkAddress{Name: name, Id: b.id, box: b}
	}
	return SyslinkAddress{Name: name, Id: 0}
}

func SyslinkOn(endpoint string, name Atom) SyslinkAddress {
	if endpoint == "" || endpoint == SyslinkNode() {
		return SyslinkAt(name)
	}
	return SyslinkAddress{Endpoint: endpoint, Name: name, Id: 0}
}

func lookupName(name Atom) *mailbox {
	slots := registrySlots()
	if int(name) < 0 || int(name) >= len(slots) {
		return nil
	}
	return slots[name].Load()
}

func resolveLocal(addr SyslinkAddress) (*mailbox, bool) {
	if addr.Endpoint != "" && addr.Endpoint != SyslinkNode() {
		return nil, false
	}
	if addr.Name != atomNone {
		if b := lookupName(addr.Name); b != nil {
			return b, true
		}
		return nil, false
	}

	if addr.box != nil {
		return addr.box, true
	}
	if addr.Id != 0 {
		if b, ok := boxes.Load(addr.Id); ok {
			return b.(*mailbox), true
		}
	}
	return nil, false
}

func missingLocal(addr SyslinkAddress) (string, string) {
	if addr.Name != atomNone {
		return "NoProc", "no service is registered as #" + addr.Name.String() + " on this node"
	}
	return "Down", "the service is no longer running"
}

var strictOnce sync.Once
var strict bool

func syslinkStrict() bool {
	strictOnce.Do(func() {
		strict = os.Getenv("HIVE_SYSLINK_STRICT") != ""
	})
	return strict
}

func SyslinkPost(addr SyslinkAddress, msg any) {
	if b, ok := resolveLocal(addr); ok {
		b.post(delivery{value: msg, origin: ""})
	}
}

func syslinkPostDrawn(addr SyslinkAddress, msg any) {
	if b, ok := resolveLocal(addr); ok {
		b.post(delivery{value: msg, origin: "", drawn: true})
	}
}

func SyslinkSend[M any](addr SyslinkAddress, msg M, encode func(M) []byte, digest uint32) {
	if b, ok := resolveLocal(addr); ok {
		if syslinkStrict() {
			b.post(delivery{raw: encode(msg), digest: digest, origin: ""})
			return
		}
		b.post(delivery{value: msg, origin: ""})
		return
	}
	sendFrame(addr.Endpoint, frame{
		kind:    kindCast,
		name:    addr.Name,
		id:      addr.Id,
		digest:  digest,
		payload: encode(msg),
	})
}

type answer struct {
	value any
	raw   []byte
	err   *SyslinkSyslinkError
}

type pendingCall struct {
	reply  chan answer
	target uint64
	node   string
	born   time.Time
}

var (
	refSeq  atomic.Uint64
	pendMu  sync.Mutex
	pending = map[uint64]pendingCall{}
)

const syslinkRefLifetime = 10 * time.Minute

func awaitRef(target uint64, node string) (uint64, chan answer) {
	ref := refSeq.Add(1)
	ch := make(chan answer, 1)
	pendMu.Lock()
	pending[ref] = pendingCall{reply: ch, target: target, node: node, born: time.Now()}

	if len(pending) > 64 {
		cutoff := time.Now().Add(-syslinkRefLifetime)
		for old, p := range pending {
			if p.born.Before(cutoff) {
				delete(pending, old)
			}
		}
	}
	pendMu.Unlock()
	return ref, ch
}

func dropRef(ref uint64) {
	pendMu.Lock()
	delete(pending, ref)
	pendMu.Unlock()
}

func deliverAnswer(ref uint64, a answer) {
	pendMu.Lock()
	p, ok := pending[ref]
	delete(pending, ref)
	pendMu.Unlock()
	if ok {
		p.reply <- a
	}
}

func failCallsTo(target uint64, reason string) {
	pendMu.Lock()
	hit := []pendingCall{}
	for ref, p := range pending {
		if p.target == target {
			hit = append(hit, p)
			delete(pending, ref)
		}
	}
	pendMu.Unlock()
	for _, p := range hit {
		p.reply <- answer{err: &SyslinkSyslinkError{Reason: "Down", Message: "the service died before answering: " + reason}}
	}
}

func failCallsToNode(node string, reason string) {
	pendMu.Lock()
	hit := []pendingCall{}
	for ref, p := range pending {
		if p.node == node {
			hit = append(hit, p)
			delete(pending, ref)
		}
	}
	pendMu.Unlock()
	for _, p := range hit {
		p.reply <- answer{err: &SyslinkSyslinkError{Reason: "Unreachable", Message: reason}}
	}
}

const syslinkDefaultWait = 5000

type SyslinkPending[M any] struct {
	ref    uint64
	reply  chan answer
	decode func([]byte) (M, error)

	mu       sync.Mutex
	resolved bool
	result   Result[M, SyslinkSyslinkError]
}

func SyslinkSendAwaitable[M any](
	addr SyslinkAddress,
	msg M,
	encode func(M) []byte,
	digest uint32,
	decode func([]byte) (M, error),
) *SyslinkPending[M] {
	local, isLocal := resolveLocal(addr)

	target := uint64(0)
	node := ""
	if isLocal {
		target = local.id
	} else {
		node = addr.Endpoint
	}
	ref, ch := awaitRef(target, node)
	p := &SyslinkPending[M]{ref: ref, reply: ch, decode: decode}

	if isLocal {
		if syslinkStrict() {
			local.post(delivery{raw: encode(msg), digest: digest, ref: ref, origin: ""})
		} else {
			local.post(delivery{value: msg, ref: ref, origin: ""})
		}
		return p
	}
	if addr.Endpoint == "" || addr.Endpoint == SyslinkNode() {
		dropRef(ref)
		p.settle(syslinkErr[M](missingLocal(addr)))
		return p
	}
	if err := sendFrame(addr.Endpoint, frame{
		kind:    kindRequest,
		name:    addr.Name,
		id:      addr.Id,
		ref:     ref,
		digest:  digest,
		payload: encode(msg),
	}); err != nil {

		dropRef(ref)
		p.settle(syslinkErr[M](err.Reason, err.Message))
	}
	return p
}

func (p *SyslinkPending[M]) settle(r Result[M, SyslinkSyslinkError]) {
	p.mu.Lock()
	p.resolved = true
	p.result = r
	p.mu.Unlock()
}

func SyslinkAwait[M any](p *SyslinkPending[M], ms int) Result[M, SyslinkSyslinkError] {
	if p == nil {
		return syslinkErr[M]("NoProc", "there is no request to wait for")
	}
	p.mu.Lock()
	if p.resolved {
		r := p.result
		p.mu.Unlock()
		return r
	}
	p.mu.Unlock()

	if ms <= 0 {
		ms = syslinkDefaultWait
	}
	timer := time.NewTimer(time.Duration(ms) * time.Millisecond)
	defer timer.Stop()

	select {
	case a := <-p.reply:
		r := p.interpret(a)
		p.settle(r)
		return r
	case <-timer.C:
		return syslinkErr[M]("Timeout", "no answer within "+strconv.Itoa(ms)+"ms")
	}
}

func (p *SyslinkPending[M]) interpret(a answer) Result[M, SyslinkSyslinkError] {
	if a.err != nil {
		return Err[M, SyslinkSyslinkError](*a.err)
	}
	if a.raw != nil {
		v, err := p.decode(a.raw)
		if err != nil {
			return syslinkErr[M]("Decode", "the answer did not match this service's message type: "+err.Error())
		}
		return Ok[M, SyslinkSyslinkError](v)
	}
	v, fits := a.value.(M)
	if !fits {
		return syslinkErr[M]("Decode", "the service answered with a value that is not one of its own messages")
	}
	return Ok[M, SyslinkSyslinkError](v)
}

func SyslinkAwaitAll[M any](ps []*SyslinkPending[M], ms int) []Result[M, SyslinkSyslinkError] {
	if ms <= 0 {
		ms = syslinkDefaultWait
	}
	deadline := time.Now().Add(time.Duration(ms) * time.Millisecond)
	out := make([]Result[M, SyslinkSyslinkError], len(ps))
	for i, p := range ps {
		left := int(time.Until(deadline) / time.Millisecond)
		if left <= 0 {
			left = 1
		}
		out[i] = SyslinkAwait(p, left)
	}
	return out
}

func SyslinkMonitor[M any](by SyslinkAddress, target SyslinkAddress, msg M, encode func(M) []byte, digest uint32) {
	me, ok := resolveLocal(by)
	if !ok {
		return
	}
	if b, ok := resolveLocal(target); ok {
		b.mu.Lock()
		if b.closed {
			b.mu.Unlock()
			me.post(delivery{value: msg, origin: ""})
			return
		}
		b.watchers = append(b.watchers, watcher{box: me, value: msg})
		b.mu.Unlock()
		return
	}
	mon := monSeq.Add(1)
	w := watcher{box: me, value: msg, mon: mon}
	rememberRemoteWatch(target.Endpoint, w)
	if err := sendFrame(target.Endpoint, frame{
		kind:    kindMonitor,
		name:    target.Name,
		id:      target.Id,
		watcher: me.id,
		ref:     mon,
		digest:  digest,
		payload: encode(msg),
	}); err != nil {

		forgetRemoteWatch(target.Endpoint, mon)
		notifyWatcher(w)
	}
}

func registerRemoteWatcher(target SyslinkAddress, from string, watcherID, mon uint64, payload []byte, digest uint32) {
	w := watcher{node: from, id: watcherID, mon: mon, payload: payload, digest: digest}
	b, ok := resolveLocal(target)
	if !ok {
		notifyWatcher(w)
		return
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		notifyWatcher(w)
		return
	}
	b.watchers = append(b.watchers, w)
	b.mu.Unlock()
}

func SyslinkDigest(signature string) uint32 {
	hash := uint32(2166136261)
	for _, b := range []byte(signature) {
		hash ^= uint32(b)
		hash *= 16777619
	}
	return hash
}
