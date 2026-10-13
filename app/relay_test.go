package app

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marrasen/kakel/relay"
)

// testRelay is a relay on a port of the test's, and its URL with the
// secret.
func testRelay(t *testing.T) string {
	t.Helper()
	secret := relay.NewSecret()
	ts := httptest.NewServer(relay.NewServer(secret))
	t.Cleanup(ts.Close)
	return ts.URL + "/k/" + secret
}

func TestAWindowIsServedThroughARelay(t *testing.T) {
	a, _ := agentApp(t)
	dir := t.TempDir()
	a.serving.hostKey, a.serving.allowed = filepath.Join(dir, "host_key"), filepath.Join(dir, "authorized_keys")
	clientOf(t, a)
	if err := a.stopServing(); err != nil {
		t.Fatal(err)
	}
	url := testRelay(t)
	a.handle(StartServing{NoPort: true, Relay: true, RelayURL: url})
	if !a.st.Serving.On || a.st.Serving.Addr != "" {
		t.Fatalf("served %+v, notices %+v", a.st.Serving, a.st.Notices)
	}
	t.Cleanup(func() { _ = a.stopServing() })
	waitFor(t, a, "the relay to have it", func() bool { return a.st.Serving.Reached })
	first := a.st.Serving.Code
	if _, err := relay.ParseCode(first); err != nil {
		t.Fatalf("the code is %q: %v", first, err)
	}
	if l, err := relay.ParseLink(a.st.Serving.Link); err != nil || l.Code != first || l.Relay.String() != url {
		t.Fatalf("the link is %q", a.st.Serving.Link)
	}

	a.handle(NewRelayCode{})
	if a.st.Serving.Code == first || a.st.Serving.Reached {
		t.Fatalf("after a new code: %+v", a.st.Serving)
	}
	waitFor(t, a, "the relay to have the new code", func() bool { return a.st.Serving.Reached })
}

func TestServingNowhereIsRefused(t *testing.T) {
	a, _ := agentApp(t)
	if err := a.startServing(StartServing{NoPort: true}); err == nil {
		t.Fatal("served on no port and no relay")
	}
	if err := a.startServing(StartServing{NoPort: true, Relay: true, RelayURL: "relay.example.com"}); err == nil {
		t.Fatal("served through a relay URL with no secret")
	}
}

func TestAWindowConnectsThroughARelay(t *testing.T) {
	a, _ := agentApp(t)
	dir := t.TempDir()
	a.serving.hostKey, a.serving.allowed = filepath.Join(dir, "host_key"), filepath.Join(dir, "authorized_keys")
	b, keyFile := clientOf(t, a)
	if err := a.stopServing(); err != nil {
		t.Fatal(err)
	}
	a.handle(StartServing{NoPort: true, Relay: true, RelayURL: testRelay(t)})
	t.Cleanup(func() { _ = a.stopServing() })
	waitFor(t, a, "the relay to have it", func() bool { return a.st.Serving.Reached })
	link, err := relay.ParseLink(a.st.Serving.Link)
	if err != nil {
		t.Fatal(err)
	}

	// Typed as the two fields give it: the relay, and the code in
	// lower case with no dash.
	typed := link.Relay.String() + "#" + strings.ToLower(strings.ReplaceAll(link.Code, "-", ""))
	b.handle(ConnectWindow{Addr: typed, KeyFile: keyFile})
	pumpBoth(t, a, b, "the question about the host key", func() bool { return len(b.st.Asks) > 0 })
	if q := b.st.Asks[0].Text; !strings.Contains(q, link.Shown()) || strings.Contains(q, "/k/") {
		t.Fatalf("asked %q", q)
	}
	b.handle(AskAnswered{ID: b.st.Asks[0].ID, Yes: true})
	pumpBoth(t, a, b, "a terminal on the window", func() bool { return oneShell(b) })
	if got := b.machines.Name(b.st.Panes[0].Machine); got != link.Shown() {
		t.Fatalf("the window is called %q, want %q", got, link.Shown())
	}
	pumpBoth(t, a, b, "the first window to see it", func() bool { return len(a.st.Serving.Clients) == 1 })
	if from := a.st.Serving.Clients[0].From; !strings.Contains(from, "through ") {
		t.Fatalf("the client came from %q", from)
	}

	// Nothing names it by the relay's secret.
	for _, m := range b.machines.Infos() {
		if strings.Contains(m.Name, "/k/") || strings.Contains(m.Target, "/k/") {
			t.Fatalf("a machine shows the secret: %+v", m)
		}
	}
	// Saved work on it finds it again by the name it is shown by.
	if id, ok := b.machines.Find(b.keptAs(b.st.Panes[0].Machine)); !ok || id != b.st.Panes[0].Machine {
		t.Fatalf("kept as %q, found %q, %v", b.keptAs(b.st.Panes[0].Machine), id, ok)
	}

	// Typed again, it is the same window, not a second heading.
	if got := b.windowAt(serveAddr(link.String())); got != b.st.Panes[0].Machine {
		t.Fatalf("the link is known as %q, want %q", got, b.st.Panes[0].Machine)
	}
}

func TestTheProxyForRelaysIsCheckedAndKept(t *testing.T) {
	a, _ := agentApp(t)
	a.settings = mustSettings(t)
	a.handle(SetRelayProxy{Proxy: "ftp://nope:21"})
	if a.st.Serving.Proxy != "" {
		t.Fatalf("a bad proxy was kept: %q", a.st.Serving.Proxy)
	}
	a.handle(SetRelayProxy{Proxy: "http://proxy.example.com:8080"})
	if a.st.Serving.Proxy != "http://proxy.example.com:8080" {
		t.Fatalf("the proxy is %q", a.st.Serving.Proxy)
	}
	pick := a.relayDialer("").Proxy
	r := httptest.NewRequest("GET", "https://relay.example.com/k/x", nil)
	if u, err := pick(r); err != nil || u == nil || u.Host != "proxy.example.com:8080" {
		t.Fatalf("a relay is reached through %v, %v", u, err)
	}
	a.handle(SetRelayProxy{Proxy: relay.NoProxy})
	if a.relayDialer("").Proxy != nil {
		t.Fatal("none still goes through a proxy")
	}
}
