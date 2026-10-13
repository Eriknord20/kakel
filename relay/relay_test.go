package relay

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// started is a relay on a port of the test's, and its URL.
func started(t *testing.T) (*Server, URL) {
	t.Helper()
	s := NewServer(NewSecret())
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	u, err := ParseURL(ts.URL + "/k/" + s.secret)
	if err != nil {
		t.Fatal(err)
	}
	return s, u
}

// states gathers what a listener says of its place on the relay.
type states struct {
	mu   sync.Mutex
	errs []error
	more chan struct{}
}

func newStates() *states { return &states{more: make(chan struct{}, 64)} }

func (s *states) tell(_ string, err error) {
	s.mu.Lock()
	s.errs = append(s.errs, err)
	s.mu.Unlock()
	s.more <- struct{}{}
}

// next waits for the next thing said.
func (s *states) next(t *testing.T) error {
	t.Helper()
	select {
	case <-s.more:
	case <-time.After(10 * time.Second):
		t.Fatal("the listener said nothing")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.errs[len(s.errs)-1]
}

// hosted serves through u under code, and waits until it is registered.
func hosted(t *testing.T, u URL, code, token string) (*Listener, *states) {
	t.Helper()
	st := newStates()
	l, err := Listen(HostConfig{Relay: u, Code: code, Token: token, OnState: st.tell})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	if err := st.next(t); err != nil {
		t.Fatalf("registering: %v", err)
	}
	return l, st
}

func TestAWindowConnectingReachesTheOneServedUnderItsCode(t *testing.T) {
	_, u := started(t)
	code := NewCode()
	l, _ := hosted(t, u, code, NewToken())

	type got struct {
		c   net.Conn
		err error
	}
	accepted := make(chan got, 1)
	go func() {
		c, err := l.Accept()
		accepted <- got{c, err}
	}()
	near, err := Join(context.Background(), Link{Relay: u, Code: code}, Dialer{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = near.Close() }()
	a := <-accepted
	if a.err != nil {
		t.Fatal(a.err)
	}
	far := a.c
	defer func() { _ = far.Close() }()

	// Both ways, and more than one message's worth.
	big := bytes.Repeat([]byte("kakel "), 50000)
	go func() { _, _ = near.Write(big) }()
	back := make([]byte, len(big))
	if _, err := io.ReadFull(far, back); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(back, big) {
		t.Fatal("what arrived is not what was sent")
	}
	go func() { _, _ = far.Write([]byte("hello")) }()
	hello := make([]byte, 5)
	if _, err := io.ReadFull(near, hello); err != nil || string(hello) != "hello" {
		t.Fatalf("read %q, %v", hello, err)
	}
	if !strings.Contains(far.RemoteAddr().String(), "127.0.0.1 through ") {
		t.Errorf("the served end says it came from %q", far.RemoteAddr())
	}
	if want := code + " on " + u.Host(); near.RemoteAddr().String() != want {
		t.Errorf("the connecting end says it reached %q, not %q", near.RemoteAddr(), want)
	}

	// One end closing ends the other.
	_ = near.Close()
	_ = far.SetReadDeadline(time.Now().Add(10 * time.Second))
	if _, err := far.Read(make([]byte, 1)); err == nil {
		t.Fatal("the served end read on after the other closed")
	}
}

func TestACodeNobodyServesIsSaidSo(t *testing.T) {
	_, u := started(t)
	_, err := Join(context.Background(), Link{Relay: u, Code: "AAAA-BBBB"}, Dialer{})
	if err == nil || !strings.Contains(err.Error(), "no window is served with code AAAA-BBBB") {
		t.Fatalf("got %v", err)
	}
}

func TestAWrongSecretIsSaidPlainly(t *testing.T) {
	s, u := started(t)
	wrong, err := ParseURL(strings.Replace(u.String(), s.secret, "nottheone12345678", 1))
	if err != nil {
		t.Fatal(err)
	}
	_, err = Join(context.Background(), Link{Relay: wrong, Code: "AAAA-BBBB"}, Dialer{})
	if err == nil || !strings.Contains(err.Error(), "check its secret") {
		t.Fatalf("got %v", err)
	}
	st := newStates()
	l, err := Listen(HostConfig{Relay: wrong, Code: "AAAA-BBBB", Token: NewToken(), OnState: st.tell})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	if err := st.next(t); err == nil || !strings.Contains(err.Error(), "check its secret") {
		t.Fatalf("registering said %v", err)
	}
}

func TestACodeHeldByAnotherWindowIsRefused(t *testing.T) {
	_, u := started(t)
	code := NewCode()
	hosted(t, u, code, NewToken())
	st := newStates()
	l, err := Listen(HostConfig{Relay: u, Code: code, Token: NewToken(), OnState: st.tell})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	if err := st.next(t); err == nil || !strings.Contains(err.Error(), "in use by another window") {
		t.Fatalf("got %v", err)
	}
}

func TestTheSameWindowComingBackTakesItsCodeBack(t *testing.T) {
	_, u := started(t)
	code, token := NewCode(), NewToken()
	_, first := hosted(t, u, code, token)
	second, _ := hosted(t, u, code, token)
	// The first is let go of, and says so as it tries again.
	if err := first.next(t); err == nil {
		t.Fatal("the first said it was registered")
	}
	go func() {
		if c, err := second.Accept(); err == nil {
			_ = c.Close()
		}
	}()
	c, err := Join(context.Background(), Link{Relay: u, Code: code}, Dialer{})
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Close()
}

func TestAServedWindowReachesTheRelayAgainOnItsOwn(t *testing.T) {
	old := tryAgainAfter
	tryAgainAfter = []time.Duration{50 * time.Millisecond}
	t.Cleanup(func() { tryAgainAfter = old })

	_, u := started(t)
	// The network, as a set of connections the test can cut.
	var mu sync.Mutex
	var made []net.Conn
	d := Dialer{Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
		c, err := (&net.Dialer{}).DialContext(ctx, network, addr)
		if err == nil {
			mu.Lock()
			made = append(made, c)
			mu.Unlock()
		}
		return c, err
	}}
	code := NewCode()
	st := newStates()
	l, err := Listen(HostConfig{Relay: u, Code: code, Token: NewToken(), Dialer: d, OnState: st.tell})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	if err := st.next(t); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	for _, c := range made {
		_ = c.Close()
	}
	mu.Unlock()
	if err := st.next(t); err == nil {
		t.Fatal("losing the relay said nothing wrong")
	}
	// The relay may still think the old one is there: the token is what
	// gets the code back all the same.
	if err := st.next(t); err != nil {
		t.Fatalf("registering again: %v", err)
	}
	go func() {
		if c, err := l.Accept(); err == nil {
			_ = c.Close()
		}
	}()
	c, err := Join(context.Background(), Link{Relay: u, Code: code}, Dialer{})
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Close()
}

func TestAServedWindowThatNeverAnswersIsGivenUpOn(t *testing.T) {
	old := answerWithin
	answerWithin = 200 * time.Millisecond
	_, u := started(t)
	answerWithin = old
	code := NewCode()
	// Registered by hand, reading nothing: the knocks go nowhere.
	ws, err := Dialer{}.open(context.Background(), u.to(doorHost)+"?code="+code,
		http.Header{tokenHeader: {NewToken()}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ws.CloseNow() }()
	_, err = Join(context.Background(), Link{Relay: u, Code: code}, Dialer{})
	if err == nil || !strings.Contains(err.Error(), "did not answer") {
		t.Fatalf("got %v", err)
	}
}

func TestCodesReadAsTyped(t *testing.T) {
	for in, want := range map[string]string{
		"abcd-efgh": "ABCD-EFGH", "ABCDEFGH": "ABCD-EFGH", " ab cd ef gh ": "ABCD-EFGH",
		"oo11-iill": "0011-1111",
	} {
		got, err := ParseCode(in)
		if err != nil || got != want {
			t.Errorf("ParseCode(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "ABC", "ABCD-EFGHJ", "ABCD-EFG!", "UUUU-UUUU"} {
		if _, err := ParseCode(bad); err == nil {
			t.Errorf("ParseCode(%q) took it", bad)
		}
	}
	for range 100 {
		c := NewCode()
		if got, err := ParseCode(c); err != nil || got != c {
			t.Fatalf("a new code %q reads as %q, %v", c, got, err)
		}
	}
}

func TestRelayURLsReadAsTyped(t *testing.T) {
	for in, want := range map[string]string{
		"relay.example.com/k/abc":                 "https://relay.example.com/k/abc",
		"https://Relay.Example.com/k/abc/":        "https://relay.example.com/k/abc",
		"wss://relay.example.com:8443/x/k/abc":    "https://relay.example.com:8443/x/k/abc",
		"http://127.0.0.1:8080/k/abc#ABCD-EFGH":   "http://127.0.0.1:8080/k/abc",
		"https://user:pw@relay.example.com/k/abc": "https://relay.example.com/k/abc",
	} {
		got, err := ParseURL(in)
		if err != nil || got.String() != want {
			t.Errorf("ParseURL(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "relay.example.com", "https://relay.example.com/k/", "ftp://x/k/abc", "https:///k/abc", "https://r/k/a/b"} {
		if _, err := ParseURL(bad); err == nil {
			t.Errorf("ParseURL(%q) took it", bad)
		}
	}
	l, err := ParseLink("relay.example.com/k/abc#abcd efgh")
	if err != nil {
		t.Fatal(err)
	}
	if l.String() != "https://relay.example.com/k/abc#ABCD-EFGH" || l.Shown() != "ABCD-EFGH on relay.example.com" {
		t.Errorf("got %q, shown %q", l, l.Shown())
	}
	if !IsLink(l.String()) || IsLink("host:2222") {
		t.Error("IsLink cannot tell a link from an address")
	}
	if l.KnownAs() != "[abcd-efgh.relay.example.com]:443" && l.KnownAs() != "abcd-efgh.relay.example.com:443" {
		t.Errorf("known as %q", l.KnownAs())
	}
}

func TestTheSecretIsKept(t *testing.T) {
	path := t.TempDir() + "/secret"
	a, err := keptSecret(path, false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := keptSecret(path, false)
	if err != nil || a != b {
		t.Fatalf("%q then %q, %v", a, b, err)
	}
	c, err := keptSecret(path, true)
	if err != nil || c == a {
		t.Fatalf("a new secret was %q, %v", c, err)
	}
}

func TestMainPrintsItsURL(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var out syncBuffer
	done := make(chan error, 1)
	go func() { done <- Main(ctx, []string{"-relay", "-listen", "127.0.0.1:0"}, t.TempDir(), &out) }()
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(out.String(), "/k/") {
		if time.Now().After(deadline) {
			t.Fatalf("printed %q", out.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

// syncBuffer is a buffer written by one goroutine and read by another.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func TestAProxyOnThisMachineIsBelievedOnlyForWhatItAdded(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:5555"
	r.Header.Set("X-Forwarded-For", "10.0.0.5, 203.0.113.9")
	if got := clientAddr(r); got != "203.0.113.9" {
		t.Errorf("behind a proxy here: %q", got)
	}
	r.RemoteAddr = "198.51.100.7:5555"
	if got := clientAddr(r); got != "198.51.100.7" {
		t.Errorf("from afar, with the header: %q", got)
	}
}

func TestAProxyThatDropsTheUpgradeIsSaidSo(t *testing.T) {
	_, u := started(t)
	resp, err := http.Get(u.to(doorJoin + "/AAAA-BBBB"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusUpgradeRequired || !strings.Contains(string(b), "Upgrade and Connection") {
		t.Fatalf("%s: %s", resp.Status, b)
	}
}

func TestGuessingCodesIsSlowedDown(t *testing.T) {
	_, u := started(t)
	var err error
	for i := 0; i <= mostMisses; i++ {
		_, err = Join(context.Background(), Link{Relay: u, Code: NewCode()}, Dialer{})
	}
	if err == nil || !strings.Contains(err.Error(), "wait a minute") {
		t.Fatalf("got %v", err)
	}
}

func TestOneAddressCannotTakeEveryPlace(t *testing.T) {
	s := NewServer(NewSecret())
	for range mostHostsFrom {
		s.hosts[NewCode()] = &host{from: "192.0.2.1"}
	}
	if why, _ := s.mayRegister(NewCode(), NewToken(), "192.0.2.1"); why == "" {
		t.Error("one more from the same address was let in")
	}
	if why, _ := s.mayRegister(NewCode(), NewToken(), "192.0.2.2"); why != "" {
		t.Errorf("another address was turned away: %s", why)
	}
}

func TestANewCodeIsTakenAtOnce(t *testing.T) {
	_, u := started(t)
	code := NewCode()
	l, st := hosted(t, u, code, NewToken())
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	fresh := NewCode()
	if err := l.SetCode(fresh); err != nil {
		t.Fatal(err)
	}
	if err := st.next(t); err != nil {
		t.Fatalf("registering under the new code: %v", err)
	}
	if l.Code() != fresh {
		t.Fatalf("the code is %s", l.Code())
	}
	c, err := Join(context.Background(), Link{Relay: u, Code: fresh}, Dialer{})
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Close()
	if _, err := Join(context.Background(), Link{Relay: u, Code: code}, Dialer{}); err == nil {
		t.Fatal("the old code still let a window through")
	}
}
