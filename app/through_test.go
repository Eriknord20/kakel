package app

import (
	"net"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/marrasen/kakel/internal/sshtest"
	"github.com/marrasen/kakel/machines"
	"github.com/marrasen/kakel/relay"
	"github.com/marrasen/kakel/remote"
)

// withBastion gives a a saved server, bastion, which is the test SSH
// server and forwards anywhere, and says how to answer its sign-in.
func withBastion(t *testing.T, a *app) (*sshtest.Server, machines.ID, func()) {
	t.Helper()
	s := sshtest.New(t)
	host, port := s.Host()
	book, err := remote.LoadBook(filepath.Join(t.TempDir(), "servers.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := book.Put(remote.Host{Name: "bastion", Address: host, Port: port, User: "tester"}, ""); err != nil {
		t.Fatal(err)
	}
	h, _ := book.Lookup("bastion")
	a.book = book
	a.st.Saved = book.Hosts()
	t.Cleanup(a.hangUp)
	return s, machines.ID(h.ID), func() {
		for _, q := range a.st.Asks {
			ans := AskAnswered{ID: q.ID, Yes: true}
			if len(q.Prompts) > 0 {
				ans.Answers = []string{sshtest.Password}
			}
			a.handle(ans)
		}
	}
}

func TestAWindowIsReachedThroughASavedServer(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	a, _ := agentApp(t)
	dir := t.TempDir()
	a.serving.hostKey, a.serving.allowed = filepath.Join(dir, "host_key"), filepath.Join(dir, "authorized_keys")
	b, keyFile := clientOf(t, a)
	s, bastion, answering := withBastion(t, b)
	addr := a.st.Serving.Addr

	b.handle(ConnectWindow{Addr: addr, KeyFile: keyFile, Via: bastion})
	pumpBoth(t, a, b, "a terminal on the window", func() bool { answering(); return oneShell(b) })
	if !slices.Contains(s.Forwarded(), addr) {
		t.Fatalf("the bastion carried %v, not %s", s.Forwarded(), addr)
	}
	if b.machines.Get(bastion).Conn == nil {
		t.Fatal("the bastion is not connected")
	}
}

func TestARelayIsReachedThroughASavedServer(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	a, _ := agentApp(t)
	dir := t.TempDir()
	a.serving.hostKey, a.serving.allowed = filepath.Join(dir, "host_key"), filepath.Join(dir, "authorized_keys")
	b, keyFile := clientOf(t, a)
	if err := a.stopServing(); err != nil {
		t.Fatal(err)
	}
	url := testRelay(t)
	u, _ := relay.ParseURL(url)

	// The served end through its own bastion.
	sa, abastion, aanswering := withBastion(t, a)
	a.handle(StartServing{NoPort: true, Relay: true, RelayURL: url, RelayVia: abastion})
	t.Cleanup(func() { _ = a.stopServing() })
	waitFor(t, a, "the relay to have it", func() bool { aanswering(); return a.st.Serving.Reached })
	if !slices.Contains(sa.Forwarded(), u.Host()) {
		t.Fatalf("the served end's bastion carried %v, not the relay at %s", sa.Forwarded(), u.Host())
	}

	// The connecting end through another.
	sb, bbastion, banswering := withBastion(t, b)
	b.handle(ConnectWindow{Addr: a.st.Serving.Link, KeyFile: keyFile, Via: bbastion})
	pumpBoth(t, a, b, "a terminal on the window", func() bool { aanswering(); banswering(); return oneShell(b) })
	if !slices.Contains(sb.Forwarded(), u.Host()) {
		t.Fatalf("the connecting end's bastion carried %v, not the relay at %s", sb.Forwarded(), u.Host())
	}
}

func TestAServerIsReachedThroughAKakelWindow(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	a, _ := agentApp(t)
	dir := t.TempDir()
	a.serving.hostKey, a.serving.allowed = filepath.Join(dir, "host_key"), filepath.Join(dir, "authorized_keys")
	b, keyFile := clientOf(t, a)
	s := sshtest.New(t)
	host, port := s.Host()
	wHost, wPort, err := net.SplitHostPort(a.st.Serving.Addr)
	if err != nil {
		t.Fatal(err)
	}
	n, _ := strconv.Atoi(wPort)
	book, err := remote.LoadBook(filepath.Join(t.TempDir(), "servers.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := book.Put(remote.Host{Name: "desk", Address: wHost, Port: n, Window: true, Identities: []string{keyFile}}, ""); err != nil {
		t.Fatal(err)
	}
	desk, _ := book.Lookup("desk")
	if err := book.Put(remote.Host{Name: "inner", Address: host, Port: port, User: "tester", Via: desk.ID}, ""); err != nil {
		t.Fatal(err)
	}
	inner, _ := book.Lookup("inner")
	b.book = book
	b.st.Saved = book.Hosts()
	t.Cleanup(b.hangUp)
	answering := func() {
		for _, q := range b.st.Asks {
			ans := AskAnswered{ID: q.ID, Yes: true}
			if len(q.Prompts) > 0 {
				ans.Answers = []string{sshtest.Password}
			}
			b.handle(ans)
		}
	}

	b.handle(ConnectTo{Server: machines.ID(inner.ID)})
	pumpBoth(t, a, b, "the server behind the window", func() bool {
		answering()
		return b.machines.Get(machines.ID(inner.ID)).Conn != nil
	})
	if b.machines.Get(machines.ID(desk.ID)).Window == nil {
		t.Fatal("the window it goes through is not connected")
	}
	pumpBoth(t, a, b, "a terminal on the server", func() bool { return oneShell(b) })
	if err := b.machines.SavedOtherwise(machines.ID(inner.ID)); err != nil {
		t.Fatalf("the route is not the one saved: %v", err)
	}
}
