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

// A tab short of room for "web: log" says "log", the folder; but one
// short of room for "PowerShell 7 on skylake" keeps its whole title,
// faded where cut, so the machine's name still starts to show.
func TestATabsShortTitleDropsOnlyAMachineInFront(t *testing.T) {
	w := &Window{
		machineList: []machines.Info{{ID: "s1", Name: "web"}, {ID: "w1", Name: "skylake", Window: true}},
		panes: []app.Pane{
			{ID: "f", Kind: app.KindFileManager, Machine: "s1", Title: "log"},
			{ID: "t", Kind: app.KindTerminal, Machine: "w1", Title: "PowerShell 7"},
		},
	}
	if got := w.tabShort(app.Tab{Pane: "f"}); got != "log" {
		t.Errorf("files on a server are short for %q, want %q", got, "log")
	}
	if got := w.tabShort(app.Tab{Pane: "t"}); got != "" {
		t.Errorf("a terminal through a window is short for %q, want nothing shorter", got)
	}
}
