package relay

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// The doors of a relay, after its secret.
const (
	// doorHost is where a served window registers its code and hears
	// of each window that wants it.
	doorHost = "host"
	// doorJoin, then the code, is where a window connecting comes in.
	doorJoin = "join"
	// doorAccept, then the ticket, is where a served window brings the
	// stream for one window connecting.
	doorAccept = "accept"
)

// tokenHeader carries a served window's token: kept with its code, so
// the relay can tell the same window coming back after its network
// dropped from another trying to take the code.
const tokenHeader = "Kakel-Token"

// Limits, so somebody given the relay's URL can tie up a little of it
// and no more. Everyone a link was ever sent to has the secret.
const (
	// mostHosts is how many windows may be registered at once, and
	// mostHostsFrom how many of them from one address.
	mostHosts     = 1000
	mostHostsFrom = 16
	// mostWaiting is how many windows may be waiting for one served
	// window to answer them.
	mostWaiting = 8
	// mostTickets is how many may be waiting on the relay as a whole.
	mostTickets = 512
	// mostMisses is how many codes one address may ask for that nobody
	// serves, in missWindow, before it is told to wait: a code is not
	// guessed by asking a few at a time.
	mostMisses = 30
	missWindow = time.Minute
	// mostMissing is how many addresses the relay counts misses for at
	// once; past that it forgets the counts that have run out.
	mostMissing = 10000
)

// answerWithin is how long a served window has to bring the stream for
// a window connecting. It is up and listening, so a few seconds is all
// it should take; this is for a slow proxy.
var answerWithin = 20 * time.Second

// knock is what the relay tells a served window about a window wanting
// it: the ticket to bring a stream with, and where it came from.
type knock struct {
	Ticket string `json:"knock"`
	From   string `json:"from,omitempty"`
}

// Server is a relay: an http.Handler joining served windows and the
// windows connecting to them.
type Server struct {
	secret string
	// Log is told what happened, a line at a time. A nil one is quiet.
	Log func(format string, args ...any)

	// within is answerWithin as it was when the relay was made.
	within time.Duration

	mu      sync.Mutex
	hosts   map[string]*host
	tickets map[string]*ticket
	// misses counts, by address, the codes asked for that nobody
	// serves.
	misses map[string]*misses
}

// misses are the codes one address asked for that nobody serves, since
// from.
type misses struct {
	from  time.Time
	count int
}

// host is a served window registered on the relay.
type host struct {
	code, token string
	// from is the address it registered from.
	from string
	ws   *websocket.Conn
	// knocks are the knocks waiting to be written to it, by the one
	// goroutine that writes them.
	knocks chan []byte
	// waiting counts the tickets made for it not yet answered.
	waiting int
	// gone is closed once it has left the relay.
	gone chan struct{}
}

// ticket is one window connecting, waiting for the stream the served
// window brings for it.
type ticket struct {
	host *host
	// brought is where the served window's stream arrives.
	brought chan *websocket.Conn
	// done is closed once the two are no longer joined, for the
	// handler holding the served window's stream.
	done chan struct{}
}

// NewServer makes a relay answering under secret.
func NewServer(secret string) *Server {
	return &Server{secret: secret, hosts: map[string]*host{}, tickets: map[string]*ticket{},
		misses: map[string]*misses{}, within: answerWithin}
}

// NewSecret makes a secret for a relay's URL.
func NewSecret() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:]))
}

// logf says what happened, when anybody is listening.
func (s *Server) logf(format string, args ...any) {
	if s.Log != nil {
		s.Log(format, args...)
	}
}

// ServeHTTP answers a request on any of the relay's doors. The path
// may have anything before /k/, for a relay behind a proxy that hands
// it a folder of its own.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	at := strings.LastIndex(r.URL.Path, secretPath)
	if at < 0 {
		http.NotFound(w, r)
		return
	}
	secret, door, _ := strings.Cut(r.URL.Path[at+len(secretPath):], "/")
	// The same answer as a path that is not there at all, so the relay
	// says nothing about itself to someone without the secret.
	if !same(secret, s.secret) {
		http.NotFound(w, r)
		return
	}
	door, arg, _ := strings.Cut(door, "/")
	if door != doorHost && door != doorJoin && door != doorAccept {
		http.NotFound(w, r)
		return
	}
	// Turned away before anything is done for it, as a proxy in front
	// that does not pass WebSockets on would have the served window
	// bring a stream for every try.
	if !upgrading(r) {
		http.Error(w, "the relay was reached without a WebSocket upgrade: a proxy in front of it has to pass the Upgrade and Connection headers on", http.StatusUpgradeRequired)
		return
	}
	switch door {
	case doorHost:
		s.serveHost(w, r)
	case doorJoin:
		s.serveJoin(w, r, arg)
	case doorAccept:
		s.serveAccept(w, r, arg)
	}
}

// same compares two secrets in a time that says nothing of where they
// differ.
func same(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }

// upgrading reports whether r asks for a WebSocket.
func upgrading(r *http.Request) bool {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return false
	}
	for _, v := range r.Header.Values("Connection") {
		for t := range strings.SplitSeq(v, ",") {
			if strings.EqualFold(strings.TrimSpace(t), "upgrade") {
				return true
			}
		}
	}
	return false
}

// accept takes a WebSocket on a request: binary, uncompressed, as what
// it carries is encrypted and does not compress.
func accept(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	return websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
}

// serveHost registers a served window under its code, and tells it of
// each window connecting until it leaves.
func (s *Server) serveHost(w http.ResponseWriter, r *http.Request) {
	code, err := ParseCode(r.URL.Query().Get("code"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	token := r.Header.Get(tokenHeader)
	if len(token) < 16 {
		http.Error(w, "a served window gives a token with its code", http.StatusBadRequest)
		return
	}
	from := clientAddr(r)
	if why, status := s.mayRegister(code, token, from); why != "" {
		http.Error(w, why, status)
		return
	}
	ws, err := accept(w, r)
	if err != nil {
		return
	}
	h := &host{code: code, token: token, from: from, ws: ws, knocks: make(chan []byte, mostWaiting),
		gone: make(chan struct{})}
	s.mu.Lock()
	// Asked again: somebody may have taken it between the check and now.
	if why, _ := s.mayRegisterLocked(code, token, from); why != "" {
		s.mu.Unlock()
		_ = ws.Close(websocket.StatusPolicyViolation, why)
		return
	}
	old := s.hosts[code]
	s.hosts[code] = h
	s.mu.Unlock()
	if old != nil {
		// The same window, back before the relay noticed it had gone.
		// Not waited for: a connection that went dead takes a while to
		// close, and this one is served meanwhile.
		go func() { _ = old.ws.Close(websocket.StatusNormalClosure, "the same window registered again") }()
	}
	s.logf("relay: %s registered from %s", code, from)

	ctx := ws.CloseRead(r.Context())
	go keepUp(ctx, ws, nil)
	go s.tell(ctx, h)
	<-ctx.Done()
	s.mu.Lock()
	if s.hosts[code] == h {
		delete(s.hosts, code)
	}
	close(h.gone)
	s.mu.Unlock()
	_ = ws.CloseNow()
	s.logf("relay: %s left", code)
}

// tell writes h's knocks to it, one at a time, until ctx ends. Writing
// them here rather than from the handler of each window connecting means
// one that gives up part way through a write cannot close the served
// window's connection under it.
func (s *Server) tell(ctx context.Context, h *host) {
	for {
		select {
		case <-ctx.Done():
			return
		case line := <-h.knocks:
			wctx, cancel := context.WithTimeout(ctx, pingWait)
			err := h.ws.Write(wctx, websocket.MessageText, line)
			cancel()
			if err != nil {
				_ = h.ws.CloseNow()
				return
			}
		}
	}
}

// mayRegister says why a window may not register code with token from
// the address from now, and the status saying so, or "".
func (s *Server) mayRegister(code, token, from string) (string, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mayRegisterLocked(code, token, from)
}

// mayRegisterLocked is mayRegister, with s.mu held.
func (s *Server) mayRegisterLocked(code, token, from string) (string, int) {
	old := s.hosts[code]
	if old != nil {
		if !same(old.token, token) {
			return "that code is in use by another window; make a new code", http.StatusConflict
		}
		// Taking its own place back takes no more room.
		return "", 0
	}
	if len(s.hosts) >= mostHosts {
		return "the relay has as many windows as it takes", http.StatusServiceUnavailable
	}
	n := 0
	for _, h := range s.hosts {
		if h.from == from {
			n++
		}
	}
	if n >= mostHostsFrom {
		return fmt.Sprintf("the relay takes %d windows from one address", mostHostsFrom), http.StatusServiceUnavailable
	}
	return "", 0
}

// missed counts a code asked for from the address from that nobody
// serves.
func (s *Server) missed(from string, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.misses[from]
	if m == nil || now.Sub(m.from) > missWindow {
		if len(s.misses) >= mostMissing {
			for k, old := range s.misses {
				if now.Sub(old.from) > missWindow {
					delete(s.misses, k)
				}
			}
			if len(s.misses) >= mostMissing {
				// Too many to count: this one goes uncounted rather than
				// the relay growing without end.
				return
			}
		}
		m = &misses{from: now}
		s.misses[from] = m
	}
	m.count++
}

// missing reports whether the address from has asked for too many codes
// nobody serves to be answered now.
func (s *Server) missing(from string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.misses[from]
	return m != nil && now.Sub(m.from) <= missWindow && m.count >= mostMisses
}

// serveJoin joins a window connecting to the served window registered
// under code, once that window has brought a stream for it.
func (s *Server) serveJoin(w http.ResponseWriter, r *http.Request, code string) {
	from := clientAddr(r)
	if s.missing(from, time.Now()) {
		http.Error(w, "too many codes that nobody serves were asked for from here; wait a minute", http.StatusTooManyRequests)
		return
	}
	code, err := ParseCode(code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id := NewSecret()
	t := &ticket{brought: make(chan *websocket.Conn, 1), done: make(chan struct{})}
	s.mu.Lock()
	h := s.hosts[code]
	switch {
	case h == nil:
		s.mu.Unlock()
		s.missed(from, time.Now())
		http.Error(w, "no window is served with code "+code+" on this relay", http.StatusNotFound)
		return
	case h.waiting >= mostWaiting || len(s.tickets) >= mostTickets:
		s.mu.Unlock()
		http.Error(w, "too many windows are connecting to "+code+" at once; try again", http.StatusServiceUnavailable)
		return
	}
	t.host = h
	h.waiting++
	s.tickets[id] = t
	s.mu.Unlock()
	defer close(t.done)
	// Taken back however this ends: a stream brought late finds no
	// ticket and is turned away.
	defer func() {
		s.mu.Lock()
		delete(s.tickets, id)
		h.waiting--
		s.mu.Unlock()
	}()

	line, _ := json.Marshal(knock{Ticket: id, From: from})
	// Room for it: no more knocks wait than tickets do, and those are
	// held to mostWaiting.
	select {
	case h.knocks <- line:
	default:
		http.Error(w, "too many windows are connecting to "+code+" at once; try again", http.StatusServiceUnavailable)
		return
	}
	wctx, cancel := context.WithTimeout(r.Context(), s.within)
	defer cancel()
	var far *websocket.Conn
	select {
	case far = <-t.brought:
	case <-h.gone:
		http.Error(w, "the window served with code "+code+" left the relay", http.StatusBadGateway)
		return
	case <-wctx.Done():
		http.Error(w, "the window served with code "+code+" did not answer", http.StatusGatewayTimeout)
		return
	}
	near, err := accept(w, r)
	if err != nil {
		_ = far.Close(websocket.StatusGoingAway, "the window connecting went")
		return
	}
	s.logf("relay: %s joined from %s", code, from)
	join(r.Context(), near, far)
}

// serveAccept takes the stream a served window brings for a ticket, and
// holds it while it is joined.
func (s *Server) serveAccept(w http.ResponseWriter, r *http.Request, id string) {
	token := r.Header.Get(tokenHeader)
	s.mu.Lock()
	t := s.tickets[id]
	if t != nil && !same(token, t.host.token) {
		// Not the window it was made for: left for that one.
		t = nil
	}
	if t != nil {
		// One stream a ticket.
		delete(s.tickets, id)
	}
	s.mu.Unlock()
	if t == nil {
		http.Error(w, "no window is waiting for that", http.StatusNotFound)
		return
	}
	ws, err := accept(w, r)
	if err != nil {
		return
	}
	t.brought <- ws
	<-t.done
	// Closed here too, for one brought just as the window connecting
	// gave up waiting, which nothing joined.
	_ = ws.CloseNow()
}

// join carries one stream into the other until either ends.
func join(ctx context.Context, a, b *websocket.Conn) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	na, nb := carried(ctx, a), carried(ctx, b)
	var wg sync.WaitGroup
	carry := func(to, from net.Conn) {
		defer wg.Done()
		_, _ = io.Copy(to, from)
		cancel()
	}
	wg.Add(2)
	go carry(na, nb)
	go carry(nb, na)
	wg.Wait()
	_ = na.Close()
	_ = nb.Close()
}

// clientAddr is where a request came from: for a proxy on this machine,
// the address it says it was asked from, the last in X-Forwarded-For,
// and the address that connected otherwise. Only the last is the
// proxy's own word; the rest are whatever the caller sent.
func clientAddr(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if fwd := r.Header.Values("X-Forwarded-For"); len(fwd) > 0 {
			all := strings.Split(fwd[len(fwd)-1], ",")
			if last := strings.TrimSpace(all[len(all)-1]); last != "" {
				return last
			}
		}
	}
	return host
}

// said turns a relay's refusal into a sentence: the text it answered
// with, or the status when it said nothing a person can read.
func said(resp *http.Response, err error) error {
	if resp == nil {
		return err
	}
	var text string
	if resp.Body != nil {
		kind, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
		if kind == "text/plain" {
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			text = strings.TrimSpace(string(b))
		}
	}
	switch {
	case resp.StatusCode == http.StatusNotFound && (text == "404 page not found" || text == ""):
		return errors.New("the relay does not know that URL: check its secret, the part after /k/")
	case text != "":
		return errors.New(text)
	}
	return fmt.Errorf("the relay answered %s", resp.Status)
}
