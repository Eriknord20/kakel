package app

import (
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/marrasen/kakel/internal/sshtest"
	"github.com/marrasen/kakel/machines"
	"github.com/marrasen/kakel/remote"
)

// plainServed is agentApp's window served on a port to one key, with a
// saved server, "DB Server", that is the test SSH server; and that key,
// for a plain ssh client to log in with.
func plainServed(t *testing.T) (*app, ssh.Signer, machines.ID) {
	t.Helper()
	t.Setenv("SSH_AUTH_SOCK", "")
	a, _ := agentApp(t)
	dir := t.TempDir()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	a.serving.hostKey, a.serving.allowed = filepath.Join(dir, "host_key"), filepath.Join(dir, "authorized_keys")
	line := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey()))) + " laptop\n"
	if err := os.WriteFile(a.serving.allowed, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	s := sshtest.New(t)
	host, port := s.Host()
	book, err := remote.LoadBook(filepath.Join(t.TempDir(), "servers.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := book.Put(remote.Host{Name: "DB Server", Address: host, Port: port, User: "tester"}, ""); err != nil {
		t.Fatal(err)
	}
	h, _ := book.Lookup("DB Server")
	a.book = book
	a.st.Saved = book.Hosts()
	t.Cleanup(a.hangUp)
	a.handle(StartServing{Port: "0"})
	if !a.st.Serving.On {
		t.Fatalf("not served: %+v", a.st.Notices)
	}
	t.Cleanup(func() { _ = a.stopServing() })
	return a, signer, machines.ID(h.ID)
}

// echo is a port on this machine that says back what it is sent.
func echo(t *testing.T) string {
	t.Helper()
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
			go func() { _, _ = io.Copy(c, c); _ = c.Close() }()
		}
	}()
	return ln.Addr().String()
}

// plainTunnel logs in to a's window as user, as plain ssh would, and
// opens a tunnel to addr, as ssh -L does, while pumping a's events. It
// returns what came back from sending "hi", or the error.
func plainTunnel(t *testing.T, a *app, signer ssh.Signer, user, addr string) (string, error) {
	t.Helper()
	var said string
	var err error
	asAgent(t, a, func() {
		var c *ssh.Client
		c, err = ssh.Dial("tcp", a.st.Serving.Addr, &ssh.ClientConfig{User: user,
			Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)}, HostKeyCallback: ssh.InsecureIgnoreHostKey()})
		if err != nil {
			return
		}
		defer func() { _ = c.Close() }()
		var nc net.Conn
		nc, err = c.Dial("tcp", addr)
		if err != nil {
			return
		}
		defer func() { _ = nc.Close() }()
		_, _ = nc.Write([]byte("hi"))
		b := make([]byte, 2)
		_, err = io.ReadFull(nc, b)
		said = string(b)
	})
	return said, err
}

func TestPlainSSHTunnelsGoOutFromThisMachineAsKakel(t *testing.T) {
	a, signer, _ := plainServed(t)
	if said, err := plainTunnel(t, a, signer, "kakel", echo(t)); err != nil || said != "hi" {
		t.Fatalf("got %q, %v", said, err)
	}
}

func TestPlainSSHTunnelsGoOutFromTheMachineNamed(t *testing.T) {
	a, signer, db := plainServed(t)
	to := echo(t)

	// Not connected, and signing in needs a password: refused, and
	// nothing asked on this screen.
	_, err := plainTunnel(t, a, signer, "db-server", to)
	if err == nil || !strings.Contains(err.Error(), "connect it here first") {
		t.Fatalf("got %v", err)
	}
	if len(a.st.Asks) > 0 {
		t.Fatalf("asked %+v", a.st.Asks)
	}

	// Connected here, it carries the tunnel.
	a.handle(ConnectTo{Server: db, Only: true})
	waitFor(t, a, "DB Server", func() bool {
		for _, q := range a.st.Asks {
			a.handle(AskAnswered{ID: q.ID, Yes: true, Answers: []string{sshtest.Password}})
		}
		return a.machines.Get(db).Conn != nil
	})
	if said, err := plainTunnel(t, a, signer, "db-server", to); err != nil || said != "hi" {
		t.Fatalf("got %q, %v", said, err)
	}

	// A name nothing is saved as is refused, saying what to use.
	if _, err := plainTunnel(t, a, signer, "nope", to); err == nil || !strings.Contains(err.Error(), "kakel@") {
		t.Fatalf("got %v", err)
	}
}

func TestAMachineNamedIsConnectedToWhenNothingNeedsAsking(t *testing.T) {
	a, signer, db := plainServed(t)
	// A key in ~/.ssh and the server's host key known: nothing to ask.
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeKey(t, filepath.Join(home, ".ssh", "id_ed25519"))
	h, _ := a.machines.Saved(db)
	line := knownhosts.Line([]string{knownhosts.Normalize(net.JoinHostPort(h.Address, strconv.Itoa(h.Port)))}, sshtestHostKey(t, h))
	if err := os.WriteFile(filepath.Join(home, ".ssh", "known_hosts"), []byte(line+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if said, err := plainTunnel(t, a, signer, "DB Server", echo(t)); err != nil || said != "hi" {
		t.Fatalf("got %q, %v", said, err)
	}
	if a.machines.Get(db).Conn == nil {
		t.Fatal("DB Server is not connected")
	}
	if len(a.st.Asks) > 0 {
		t.Fatalf("asked %+v", a.st.Asks)
	}
}

// sshtestHostKey is the host key of the test SSH server h is saved as.
func sshtestHostKey(t *testing.T, h remote.Host) ssh.PublicKey {
	t.Helper()
	var key ssh.PublicKey
	c, err := ssh.Dial("tcp", net.JoinHostPort(h.Address, strconv.Itoa(h.Port)), &ssh.ClientConfig{User: "x",
		HostKeyCallback: func(_ string, _ net.Addr, k ssh.PublicKey) error { key = k; return nil }})
	if err == nil {
		_ = c.Close()
	}
	if key == nil {
		t.Fatalf("no host key from %s: %v", h.Address, err)
	}
	return key
}
