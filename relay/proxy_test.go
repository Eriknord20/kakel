package relay

import (
	"bufio"
	"context"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestTheSystemsProxyIsReadAsWindowsWritesIt(t *testing.T) {
	p, err := parseSystem("http=proxy:8080;https=secure:8443;ftp=old:21", "intranet.example.com;*.corp;<local>")
	if err != nil {
		t.Fatal(err)
	}
	pick := func(raw string) string {
		r := httptest.NewRequest("GET", raw, nil)
		u, err := p.pick(r)
		if err != nil {
			t.Fatal(err)
		}
		if u == nil {
			return ""
		}
		return u.Host
	}
	for raw, want := range map[string]string{
		"https://relay.example.com/k/x":    "secure:8443",
		"http://relay.example.com/k/x":     "proxy:8080",
		"https://intranet.example.com/k/x": "",
		"https://relay.corp/k/x":           "",
		"https://relay/k/x":                "",
		"https://127.0.0.1/k/x":            "",
	} {
		if got := pick(raw); got != want {
			t.Errorf("%s goes through %q, want %q", raw, got, want)
		}
	}
	one, err := parseSystem("proxy:3128", "")
	if err != nil || one.https == nil || one.https.Host != "proxy:3128" {
		t.Fatalf("one proxy for all: %+v, %v", one, err)
	}
	if _, err := parseSystem("ftp=old:21", ""); err == nil {
		t.Error("a proxy for FTP alone was taken as one for the web")
	}
}

func TestAProxyIsReadAsTyped(t *testing.T) {
	for in, want := range map[string]string{
		"proxy:8080":                     "http://proxy:8080",
		"http://me:pw@proxy:8080":        "http://me:pw@proxy:8080",
		"socks5://127.0.0.1:1080":        "socks5://127.0.0.1:1080",
		"https://secure.example.com:443": "https://secure.example.com:443",
	} {
		u, err := ParseProxy(in)
		if err != nil || u.String() != want {
			t.Errorf("ParseProxy(%q) = %v, %v; want %s", in, u, err, want)
		}
	}
	for _, bad := range []string{"ftp://x:1", "http://"} {
		if _, err := ParseProxy(bad); err == nil {
			t.Errorf("ParseProxy(%q) took it", bad)
		}
	}
	if f, err := Proxy(NoProxy); err != nil || f != nil {
		t.Error("none still picks a proxy")
	}
}

// connectProxy is an HTTP proxy that takes CONNECT, from a user called
// me with the password pw, and counts the tunnels it made.
func connectProxy(t *testing.T) (string, *atomic.Int32) {
	t.Helper()
	var made atomic.Int32
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = c.Close() }()
				br := bufio.NewReader(c)
				req, err := http.ReadRequest(br)
				if err != nil || req.Method != http.MethodConnect {
					return
				}
				want := "Basic " + base64.StdEncoding.EncodeToString([]byte("me:pw"))
				if req.Header.Get("Proxy-Authorization") != want {
					_, _ = io.WriteString(c, "HTTP/1.1 407 Proxy Authentication Required\r\nContent-Length: 0\r\n\r\n")
					return
				}
				far, err := net.Dial("tcp", req.Host)
				if err != nil {
					_, _ = io.WriteString(c, "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n")
					return
				}
				defer func() { _ = far.Close() }()
				made.Add(1)
				_, _ = io.WriteString(c, "HTTP/1.1 200 Connection established\r\n\r\n")
				go func() { _, _ = io.Copy(far, br) }()
				_, _ = io.Copy(c, far)
			}()
		}
	}()
	return ln.Addr().String(), &made
}

func TestARelayIsReachedThroughAProxy(t *testing.T) {
	s := NewServer(NewSecret())
	ts := httptest.NewTLSServer(s)
	t.Cleanup(ts.Close)
	u, err := ParseURL(ts.URL + "/k/" + s.secret)
	if err != nil {
		t.Fatal(err)
	}
	at, made := connectProxy(t)
	pick, err := Proxy("http://me:pw@" + at)
	if err != nil {
		t.Fatal(err)
	}
	d := Dialer{Proxy: pick, tls: ts.Client().Transport.(*http.Transport).TLSClientConfig}

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
	go func() {
		if c, err := l.Accept(); err == nil {
			_, _ = c.Write([]byte("hi"))
		}
	}()
	c, err := Join(context.Background(), Link{Relay: u, Code: code}, d)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	hi := make([]byte, 2)
	if _, err := io.ReadFull(c, hi); err != nil || string(hi) != "hi" {
		t.Fatalf("read %q, %v", hi, err)
	}
	if n := made.Load(); n < 3 {
		t.Errorf("the proxy carried %d connections, want the registration, the join and the stream", n)
	}

	// A proxy that wants a password not given says so.
	bad, _ := Proxy("http://" + at)
	_, err = Join(context.Background(), Link{Relay: u, Code: code}, Dialer{Proxy: bad, tls: d.tls})
	if err == nil || !strings.Contains(err.Error(), "user name and password") || strings.Contains(err.Error(), "/k/") {
		t.Fatalf("got %v", err)
	}
}

func TestWhyARelayCannotBeReachedKeepsItsSecret(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	u, err := ParseURL("https://" + addr + "/k/thesecretitself")
	if err != nil {
		t.Fatal(err)
	}
	_, err = Join(context.Background(), Link{Relay: u, Code: "AAAA-BBBB"}, Dialer{})
	if err == nil || strings.Contains(err.Error(), "thesecretitself") {
		t.Fatalf("got %v", err)
	}
}
