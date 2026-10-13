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

// Limits, so a stranger who learnt the secret can tie up a little of
// the relay and no more.
const (
	// mostHosts is how many windows may be registered at once.
	mostHosts = 1000
	// mostWaiting is how many windows may be waiting for one served
	// window to answer them.
	mostWaiting = 8
	// mostTickets is how many may be waiting on the relay as a whole.
	mostTickets = 512
)

// answerWithin is how long a served window has to bring the stream for
// a window connecting. It is up and listening, so a few seconds is all
// it should take; this is for a slow proxy.
var answerWithin = 20 * time.Second

// pingEvery is how often the relay checks each side is still there,
// which also keeps a proxy from closing a connection that has gone
// quiet. Most proxies close one idle for a minute or more.
var pingEvery = 25 * time.Second

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
}

// host is a served window registered on the relay.
type host struct {
	code, token string
	ws          *websocket.Conn
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
		within: answerWithin}
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
	if subtle.ConstantTimeCompare([]byte(secret), []byte(s.secret)) != 1 {
		http.NotFound(w, r)
		return
	}
	door, arg, _ := strings.Cut(door, "/")
	switch door {
	case doorHost:
		s.serveHost(w, r)
	case doorJoin:
		s.serveJoin(w, r, arg)
	case doorAccept:
		s.serveAccept(w, r, arg)
	default:
		http.NotFound(w, r)
	}
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
	if why, status := s.mayRegister(code, token); why != "" {
		http.Error(w, why, status)
		return
	}
	ws, err := accept(w, r)
	if err != nil {
		return
	}
	h := &host{code: code, token: token, ws: ws, gone: make(chan struct{})}
	s.mu.Lock()
	old := s.hosts[code]
	switch {
	case old != nil && old.token != token:
		// Taken by another between the check and now.
		s.mu.Unlock()
		_ = ws.Close(websocket.StatusPolicyViolation, "that code is in use by another window")
		return
	case old == nil && len(s.hosts) >= mostHosts:
		s.mu.Unlock()
		_ = ws.Close(websocket.StatusTryAgainLater, "the relay has as many windows as it takes")
		return
	}
	s.hosts[code] = h
	s.mu.Unlock()
	if old != nil {
		// The same window, back before the relay noticed it had gone.
		_ = old.ws.Close(websocket.StatusNormalClosure, "the same window registered again")
	}
	from := clientAddr(r)
	s.logf("relay: %s registered from %s", code, from)

	ctx := ws.CloseRead(r.Context())
	go keepUp(ctx, ws)
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

// mayRegister says why a window may not register code with token now,
// and the status saying so, or "".
func (s *Server) mayRegister(code, token string) (string, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.hosts[code]
	switch {
	case old != nil && old.token != token:
		return "that code is in use by another window; make a new code", http.StatusConflict
	case old == nil && len(s.hosts) >= mostHosts:
		return "the relay has as many windows as it takes", http.StatusServiceUnavailable
	}
	return "", 0
}

// serveJoin joins a window connecting to the served window registered
// under code, once that window has brought a stream for it.
func (s *Server) serveJoin(w http.ResponseWriter, r *http.Request, code string) {
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

	from := clientAddr(r)
	line, _ := json.Marshal(knock{Ticket: id, From: from})
	wctx, cancel := context.WithTimeout(r.Context(), s.within)
	defer cancel()
	if err := h.ws.Write(wctx, websocket.MessageText, line); err != nil {
		http.Error(w, "the window served with code "+code+" could not be told", http.StatusBadGateway)
		return
	}
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
	s.mu.Lock()
	t := s.tickets[id]
	if t != nil {
		// One stream a ticket.
		delete(s.tickets, id)
	}
	s.mu.Unlock()
	if t == nil {
		http.Error(w, "no window is waiting for that", http.StatusNotFound)
		return
	}
	if r.Header.Get(tokenHeader) != t.host.token {
		http.Error(w, "that is another window's", http.StatusForbidden)
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
	na := websocket.NetConn(ctx, a, websocket.MessageBinary)
	nb := websocket.NetConn(ctx, b, websocket.MessageBinary)
	go keepUp(ctx, a)
	go keepUp(ctx, b)
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

// keepUp pings ws until ctx ends, and closes it when a ping goes
// unanswered: the other end has gone without saying so.
func keepUp(ctx context.Context, ws *websocket.Conn) {
	t := time.NewTicker(pingEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		pctx, cancel := context.WithTimeout(ctx, pingEvery)
		err := ws.Ping(pctx)
		cancel()
		if err != nil && ctx.Err() == nil {
			_ = ws.CloseNow()
			return
		}
	}
}

// clientAddr is where a request came from: the address before a proxy
// on this machine, which says so in X-Forwarded-For, and the address
// that connected otherwise. Only a proxy on this machine is believed,
// as anyone else can send the header.
func clientAddr(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			first, _, _ := strings.Cut(fwd, ",")
			if first = strings.TrimSpace(first); first != "" {
				return first
			}
		}
	}
	return host
}

// said turns a relay's refusal into a sentence: the text it answered
// with, or the status when it said nothing.
func said(resp *http.Response, err error) error {
	if resp == nil {
		return err
	}
	var text string
	if resp.Body != nil {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		text = strings.TrimSpace(string(b))
	}
	switch {
	case resp.StatusCode == http.StatusNotFound && text == "404 page not found":
		return errors.New("the relay does not know that URL: check its secret, the part after /k/")
	case text != "":
		return errors.New(text)
	case resp.StatusCode == http.StatusSwitchingProtocols:
		return err
	}
	return fmt.Errorf("the relay answered %s", resp.Status)
}
