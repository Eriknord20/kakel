package view

import (
	"testing"

	"github.com/marrasen/kakel/app"
	"github.com/marrasen/kakel/machines"
)

// Files on a server are called by the server and the folder; files on
// this computer, and other panes, by their title alone.
func TestFilesOnAServerAreTitledByTheServer(t *testing.T) {
	list := []machines.Info{{ID: "s1", Name: "web"}, {ID: "w1", Name: "skylake", Window: true}}
	for _, c := range []struct {
		p    app.Pane
		want string
	}{
		{app.Pane{Kind: app.KindFileManager, Machine: "s1", Title: "log"}, "web: log"},
		{app.Pane{Kind: app.KindFileManager, Title: `D:\`}, `D:\`},
		{app.Pane{Kind: app.KindTerminal, Machine: "s1", Title: "me@web: ~"}, "me@web: ~"},
		{app.Pane{Kind: app.KindTerminal, Machine: "w1", Title: "PowerShell 7"}, "PowerShell 7 on skylake"},
		{app.Pane{Kind: app.KindTerminal, Machine: "w1", Title: "skylake 2"}, "skylake 2"},
		{app.Pane{Kind: app.KindTerminal, Machine: "w1", On: "db", Title: "me@db: ~"}, "me@db: ~"},
		{app.Pane{Kind: app.KindTerminal, Machine: "w1", On: "db", Title: "Terminal 3"}, "Terminal 3 on db"},
	} {
		if got := titleIn(list, c.p); got != c.want {
			t.Errorf("titleIn(%+v) = %q, want %q", c.p, got, c.want)
		}
	}
}
