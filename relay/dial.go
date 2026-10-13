package relay

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// dialWithin bounds reaching the relay and its answer: the connection,
// any proxy, TLS and the WebSocket handshake.
var dialWithin = 30 * time.Second

// Dialer is how this machine reaches a relay.
type Dialer struct {
	// Proxy picks the HTTP proxy for a request, as http.Transport's
	// does. A nil one goes straight to the relay.
	Proxy func(*http.Request) (*url.URL, error)

	// Dial makes the connection to the relay, or to the proxy when
	// there is one: through a jump server, say. A nil one dials from
	// this machine.
	Dial func(ctx context.Context, network, addr string) (net.Conn, error)

	// tls is what checks the relay's certificate, nil for the system's
	// roots; a test's relay has a certificate of its own.
	tls *tls.Config
}

// open opens a WebSocket to one of the relay's doors.
func (d Dialer) open(ctx context.Context, to string, header http.Header) (*websocket.Conn, error) {
	dial := d.Dial
	if dial == nil {
		nd := &net.Dialer{Timeout: dialWithin, KeepAlive: 30 * time.Second}
		dial = nd.DialContext
	}
	// One transport a stream: the connection is the WebSocket's once it
	// is open, and nothing is pooled.
	tr := &http.Transport{
		Proxy:               d.Proxy,
		DialContext:         dial,
		TLSHandshakeTimeout: dialWithin,
		TLSClientConfig:     d.tls,
		DisableKeepAlives:   true,
	}
	defer tr.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(ctx, dialWithin)
	defer cancel()
	ws, resp, err := websocket.Dial(ctx, to, &websocket.DialOptions{
		HTTPClient:      &http.Client{Transport: tr},
		HTTPHeader:      header,
		CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		if resp != nil && resp.StatusCode != http.StatusSwitchingProtocols {
			return nil, said(resp, err)
		}
		return nil, unsaid(err)
	}
	return ws, nil
}

// unsaid is why a request could not be made, without the URL it was
// made to: that holds the relay's secret, which is never shown.
func unsaid(err error) error {
	var ue *url.Error
	if !errors.As(err, &ue) {
		return err
	}
	why := ue.Err
	if strings.Contains(why.Error(), "Proxy Authentication Required") {
		return errors.New("the proxy wants a user name and password: give them in its URL, as http://user:password@proxy:8080")
	}
	return why
}

// reachErr says the relay could not be reached, and why.
func reachErr(r URL, err error) error {
	return fmt.Errorf("relay %s: %w", r.Host(), err)
}

// Join reaches the window served through a relay under link's code: the
// stream it returns is the window's, for the serve protocol.
func Join(ctx context.Context, link Link, d Dialer) (net.Conn, error) {
	ws, err := d.open(ctx, link.Relay.to(doorJoin+"/"+link.Code), nil)
	if err != nil {
		return nil, reachErr(link.Relay, err)
	}
	return stream(ws, linkAddr(link.Shown())), nil
}

// stream is a WebSocket as a connection, pinged while it is quiet, so
// a relay that has gone is noticed and a proxy sees it in use.
func stream(ws *websocket.Conn, far net.Addr) net.Conn {
	ctx, cancel := context.WithCancel(context.Background())
	return &wsConn{Conn: carried(ctx, ws), cancel: cancel, far: far}
}

// wsConn is a stream through a relay.
type wsConn struct {
	net.Conn
	cancel context.CancelFunc
	far    net.Addr
}

func (c *wsConn) Close() error {
	err := c.Conn.Close()
	c.cancel()
	return err
}

// RemoteAddr is the window at the other end, as the relay named it.
func (c *wsConn) RemoteAddr() net.Addr { return c.far }

// linkAddr is the other end of a stream through a relay.
type linkAddr string

func (linkAddr) Network() string  { return "relay" }
func (a linkAddr) String() string { return string(a) }

// NewToken makes the token a served window keeps with its code.
func NewToken() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// HostConfig is what serving through a relay needs.
type HostConfig struct {
	Relay URL
	// Code is what a window connecting gives, until SetCode, and Token
	// is what tells the relay this is the window the code is kept for.
	Code, Token string
	Dialer      Dialer
	// OnState is told each time the window's place on the relay
	// changes, under code: nil once it is registered, and why not
	// otherwise. It is called from the listener's goroutine, and may
	// arrive after SetCode for the code before, which is what code is
	// for.
	OnState func(code string, err error)
}

// tryAgainAfter is how long to wait before reaching the relay again,
// after each failure in a row.
var tryAgainAfter = []time.Duration{time.Second, 2 * time.Second, 5 * time.Second,
	10 * time.Second, 30 * time.Second, time.Minute}

// Listener is a window served through a relay: the streams of the
// windows connecting, one by one. It reaches the relay again by itself
// whenever it loses it.
type Listener struct {
	cfg    HostConfig
	ctx    context.Context
	cancel context.CancelFunc
	conns  chan net.Conn
	once   sync.Once

	// mu guards code, and current, the registration held now, which
	// SetCode ends so the next one is made under the new code.
	mu      sync.Mutex
	code    string
	current context.CancelFunc
	// bringing holds a place for each stream being brought.
	bringing chan struct{}
	// waits are tryAgainAfter as it was when it started.
	waits []time.Duration
}

// Listen registers with the relay and keeps registered until closed.
// It returns at once: how registering goes is told to OnState.
func Listen(cfg HostConfig) (*Listener, error) {
	switch {
	case cfg.Relay.IsZero():
		return nil, errors.New("no relay to serve through")
	case cfg.Token == "":
		return nil, errors.New("no token to register with")
	}
	code, err := ParseCode(cfg.Code)
	if err != nil {
		return nil, err
	}
	cfg.Code = code
	ctx, cancel := context.WithCancel(context.Background())
	l := &Listener{cfg: cfg, ctx: ctx, cancel: cancel, conns: make(chan net.Conn),
		waits: tryAgainAfter, code: code, bringing: make(chan struct{}, mostWaiting)}
	go l.stay()
	return l, nil
}

// Accept waits for the next window connecting.
func (l *Listener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.ctx.Done():
		return nil, net.ErrClosed
	}
}

// Close leaves the relay. Streams already accepted are left to whoever
// took them.
func (l *Listener) Close() error {
	l.once.Do(l.cancel)
	return nil
}

// Addr is the window's code on the relay, as it is shown: without the
// relay's secret.
func (l *Listener) Addr() net.Addr { return linkAddr(l.Link().Shown()) }

// Link is the window's link: the relay with its secret, and the code.
func (l *Listener) Link() Link { return Link{Relay: l.cfg.Relay, Code: l.Code()} }

// Relay is the relay the window is served through.
func (l *Listener) Relay() URL { return l.cfg.Relay }

// Code is the code the window is served under.
func (l *Listener) Code() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.code
}

// SetCode serves the window under another code from now on. The
// windows connected under the old one stay connected; a window
// connecting under it is no longer let through.
func (l *Listener) SetCode(code string) error {
	code, err := ParseCode(code)
	if err != nil {
		return err
	}
	l.mu.Lock()
	l.code = code
	if l.current != nil {
		l.current()
	}
	l.mu.Unlock()
	return nil
}

// state tells OnState, when there is one, of the registration under
// code.
func (l *Listener) state(code string, err error) {
	if l.cfg.OnState != nil && l.ctx.Err() == nil {
		l.cfg.OnState(code, err)
	}
}

// stay registers, and registers again each time the relay is lost,
// waiting longer after each failure in a row.
func (l *Listener) stay() {
	tries := 0
	for l.ctx.Err() == nil {
		began := time.Now()
		code, err := l.registered()
		if l.ctx.Err() != nil {
			return
		}
		if errors.Is(err, errNewCode) {
			// Registered again at once, under the new code.
			tries = 0
			continue
		}
		// One that held for a while had the relay; losing it is a fresh
		// start, not one more failure in a row.
		if time.Since(began) > time.Minute {
			tries = 0
		}
		l.state(code, err)
		wait := l.waits[min(tries, len(l.waits)-1)]
		tries++
		select {
		case <-l.ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// registered registers once, and hears of windows connecting until the
// relay is lost, returning the code it registered and why it ended.
func (l *Listener) registered() (string, error) {
	ctx, cancel := context.WithCancel(l.ctx)
	defer cancel()
	l.mu.Lock()
	code := l.code
	l.current = cancel
	l.mu.Unlock()
	header := http.Header{tokenHeader: {l.cfg.Token}}
	q := url.Values{"code": {code}}
	ws, err := l.cfg.Dialer.open(ctx, l.cfg.Relay.to(doorHost)+"?"+q.Encode(), header)
	if err != nil {
		if l.moved(code) {
			return code, errNewCode
		}
		return code, reachErr(l.cfg.Relay, err)
	}
	defer func() { _ = ws.CloseNow() }()
	ws.SetReadLimit(4 << 10)
	go keepUp(ctx, ws, nil)
	l.state(code, nil)
	for {
		kind, line, err := ws.Read(ctx)
		if err != nil {
			if l.ctx.Err() != nil {
				return code, nil
			}
			if l.moved(code) {
				return code, errNewCode
			}
			if why := websocket.CloseStatus(err); why != -1 {
				var ce websocket.CloseError
				if errors.As(err, &ce) && ce.Reason != "" {
					return code, reachErr(l.cfg.Relay, errors.New(ce.Reason))
				}
			}
			return code, reachErr(l.cfg.Relay, errors.New("the connection to it was lost"))
		}
		var k knock
		if kind != websocket.MessageText || json.Unmarshal(line, &k) != nil || k.Ticket == "" {
			continue
		}
		// As many at once as the relay lets wait, and no more: past
		// that the relay is not one this window should trust.
		select {
		case l.bringing <- struct{}{}:
			go func() {
				defer func() { <-l.bringing }()
				l.bring(k)
			}()
		default:
		}
	}
}

// errNewCode ends a registration under a code given up for another.
var errNewCode = errors.New("relay: the code changed")

// moved reports whether the window is no longer served under code.
func (l *Listener) moved(code string) bool { return l.Code() != code }

// bring brings the stream for one window connecting, and hands it to
// Accept.
func (l *Listener) bring(k knock) {
	header := http.Header{tokenHeader: {l.cfg.Token}}
	ws, err := l.cfg.Dialer.open(l.ctx, l.cfg.Relay.to(doorAccept+"/"+url.PathEscape(k.Ticket)), header)
	if err != nil {
		return
	}
	from := k.From
	if from == "" {
		from = "somewhere"
	}
	c := stream(ws, linkAddr(from+" through "+l.cfg.Relay.Host()))
	select {
	case l.conns <- c:
	case <-l.ctx.Done():
		_ = c.Close()
	}
}
