package app

import (
	"net/http/httptest"
	"path/filepath"
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
