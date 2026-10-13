package view

import (
	"strings"
	"testing"
	"time"

	"github.com/marrasen/gunim"
	"github.com/marrasen/gunim/widget"

	"github.com/marrasen/kakel/app"
	"github.com/marrasen/kakel/remote"
)

// texts are the words of every label under n.
func texts(n gunim.Node) []string {
	var out []string
	if l, ok := n.(*widget.Label); ok {
		out = append(out, l.Text)
	}
	if c, ok := n.(gunim.Composite); ok {
		for _, k := range c.Children() {
			out = append(out, texts(k)...)
		}
	}
	return out
}

func TestTunnelsWithSSHGivesThisWindowsCommands(t *testing.T) {
	win, _, publish := windowStage(t)
	publish(app.State{
		Saved:   []remote.Host{{ID: "s1", Name: "DB Server", Address: "db.example"}},
		Serving: app.Serving{On: true, Addr: "0.0.0.0:2299", Anywhere: true, Host: "desk", Fingerprint: "SHA256:abc"},
	})
	win.sshDialog(win.serving, lastUI)
	for range 3 {
		lastWindow.Frame(time.Second / 60)
	}
	all := strings.Join(texts(win.dialog.Body), "\n")
	for _, want := range []string{
		"ssh -p 2299 -N -L 8080:intranet:80 kakel@desk",
		"ssh -p 2299 -N -L 8080:intranet:80 db-server@desk",
		"ssh -J kakel@desk:2299 user@intranet",
		"SHA256:abc",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("the dialog does not say %q; it says:\n%s", want, all)
		}
	}
	if strings.Contains(all, "listens on this machine only") {
		t.Error("it says the port is this machine's only, served on all networks")
	}
}
