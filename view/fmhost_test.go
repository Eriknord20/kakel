package view

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/marrasen/gunim"
	"github.com/marrasen/gunim/filemanager"
	"github.com/marrasen/gunim/geom"
	gi "github.com/marrasen/gunim/input"
	"github.com/marrasen/gunim/widget/dock"

	"github.com/marrasen/kakel/app"
)

// filePane starts a file manager on a folder of its own, as kakel's
// program does for file manager pane id, and stops it as the test ends.
func filePane(t *testing.T, id string) *filemanager.Window {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "dir")
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	fw, err := filemanager.NewHub(ctx, nil).NewPane(
		filemanager.Options{Dir: dir, PrefsPath: filepath.Join(root, "files.json"), Poll: -1},
		filemanager.PaneHost{ID: app.FilePaneViews(id), Commands: app.FilePaneCommands, HostMenus: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		<-fw.Done()
	})
	return fw
}

// frames draws the last window made until cond holds, and fails the test
// after five seconds.
func framesUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("waited five seconds for this: %s", what)
		}
		lastWindow.Frame(time.Second / 60)
		time.Sleep(2 * time.Millisecond)
	}
}

// A file manager pane shows in the place its window gives it, keeps what
// it shows while its tab is not showing, and has the keyboard where the
// window gives it.
func TestAFileManagerPaneShowsInItsPlace(t *testing.T) {
	win, _, publish := windowStage(t)
	filemanager.RegisterViews(lastWindow)
	fw := filePane(t, "p1")
	both := []app.Pane{{ID: "p1", Title: "dir", Kind: app.KindFileManager}, {ID: "p2", Title: "Help", Kind: app.KindHelp}}
	publish(app.State{Panes: both, Stage: &app.Box{Pane: "p1"}, Focus: "p2"})
	fw.Attach(lastWindow.Client(), app.FilePaneHost("p1"))
	var listing gunim.Node
	framesUntil(t, "the file manager shows in its place", func() bool {
		listing = filemanager.FocusIn(lastUI, app.FilePaneViews("p1"))
		return listing != nil
	})
	h := win.fmHosts["p1"]
	if h == nil || lastUI.Presence(h) == gunim.Exiting {
		t.Fatal("the pane has no place on stage")
	}

	// Its tab hidden, it is parked, and keeps its views.
	publish(app.State{Panes: both, Stage: &app.Box{Pane: "p2"}, Focus: "p2"})
	for range 30 {
		lastWindow.Frame(time.Second / 60)
	}
	if filemanager.FocusIn(lastUI, app.FilePaneViews("p1")) != listing {
		t.Fatal("parked, the file manager lost its views")
	}

	// Back on stage with the keyboard, its listing has it.
	publish(app.State{Panes: both, Stage: &app.Box{Pane: "p1"}, Focus: "p1"})
	framesUntil(t, "the listing has the keyboard", func() bool { return lastUI.Focused() == listing })

	// Kakel's Edit menu works on its files, Cut too.
	for _, id := range []string{"edit.cut", "edit.copy", "edit.paste", "edit.selectAll"} {
		if !win.applies(id) {
			t.Fatalf("%s doesn't apply to a file manager pane", id)
		}
	}
	if !filemanager.Run(lastUI, app.FilePaneViews("p1"), fileCommands["edit.selectAll"]) || !win.run("edit.selectAll", lastUI) {
		t.Fatal("Select All didn't reach the file manager")
	}

	// Its menus are kakel's, while it is in front: Go of its own, and its
	// lines in File, Edit and View, ticked as it is.
	if win.menuAt("Go") < 0 {
		t.Fatal("no Go menu with a file manager in front")
	}
	hidden := func() (m, i int, on bool) {
		for m, bm := range win.layout {
			for i, it := range bm.items {
				if it.pane == filemanager.CmdHidden {
					return m, i, win.bar.Menus[m].Items[i].Checked
				}
			}
		}
		return -1, -1, false
	}
	m, i, on := hidden()
	if m < 0 || win.layout[m].title != "View" || on {
		t.Fatalf("Show hidden files is in menu %d, ticked %v", m, on)
	}
	win.bar.OnPick(m, i, lastUI)
	framesUntil(t, "the tick follows the pick", func() bool {
		// The program's part: the file manager's intents go to it.
		for len(lastWindow.Client().Intents()) > 0 {
			fw.Deliver(<-lastWindow.Client().Intents())
		}
		win.tickPaneMenus(lastUI)
		_, _, on := hidden()
		return on
	})
	for _, bm := range win.layout {
		for _, it := range bm.items {
			if it.pane == filemanager.CmdCopy || it.pane == filemanager.CmdCloseApp {
				t.Fatalf("the file manager's %q is in the menus beside kakel's own", it.pane)
			}
		}
	}

	// The help in front, the menus are kakel's alone again.
	publish(app.State{Panes: both, Stage: &app.Box{Pane: "p2"}, Focus: "p2"})
	if win.menuAt("Go") >= 0 {
		t.Fatal("the Go menu stayed with the help in front")
	}

	// Gone from the window, its place goes too.
	publish(app.State{Panes: both[1:], Stage: &app.Box{Pane: "p2"}, Focus: "p2"})
	for range 60 {
		lastWindow.Frame(time.Second / 60)
	}
	if _, ok := win.fmHosts["p1"]; ok || lastUI.Mounted(app.FilePaneHost("p1")) != nil {
		t.Fatal("the place of a pane that left stayed")
	}
}

// F5 and Ctrl+R list the folder of the file manager in front again,
// with the keyboard on nothing, as after a click on its tab.
func TestF5RefreshesTheFileManagerInFrontWithTheKeyboardElsewhere(t *testing.T) {
	_, _, publish := windowStage(t)
	filemanager.RegisterViews(lastWindow)
	fw := filePane(t, "p1")
	publish(app.State{Panes: []app.Pane{{ID: "p1", Title: "dir", Kind: app.KindFileManager}}, Stage: &app.Box{Pane: "p1"}, Focus: "p1"})
	fw.Attach(lastWindow.Client(), app.FilePaneHost("p1"))
	framesUntil(t, "the file manager shows", func() bool { return filemanager.FocusIn(lastUI, app.FilePaneViews("p1")) != nil })
	for _, k := range []gi.KeyPress{{Key: gi.KeyF5}, {Key: gi.KeyR, Mods: gi.ModControl}} {
		lastUI.Focus(nil)
		for len(lastWindow.Client().Intents()) > 0 {
			<-lastWindow.Client().Intents()
		}
		lastWindow.Input(k)
		lastWindow.Frame(time.Second / 60)
		refreshed := false
		for len(lastWindow.Client().Intents()) > 0 {
			if c, ok := (<-lastWindow.Client().Intents()).Intent.(filemanager.Command); ok && c.Name == filemanager.CmdRefresh {
				refreshed = true
			}
		}
		if !refreshed {
			t.Fatalf("%v with the keyboard on nothing did not refresh the file manager", k)
		}
	}
}

// A tab dropped on a file manager pane docks beside it, as on any pane:
// the pane takes dropped files alone.
func TestATabDroppedOnAFileManagerDocks(t *testing.T) {
	win, _, publish := windowStage(t)
	filemanager.RegisterViews(lastWindow)
	fw := filePane(t, "p1")
	b1, b2 := &app.Box{Pane: "p1"}, &app.Box{Pane: "p2"}
	publish(app.State{
		Window: 1,
		Panes:  []app.Pane{{ID: "p1", Title: "dir", Kind: app.KindFileManager}, {ID: "p2", Title: "Log", Kind: app.KindLog}},
		Stage:  b1, Focus: "p1",
		Groups: map[string]*app.Box{"p1": b1, "p2": b2},
		Tabs:   []app.Tab{{Group: 1, Pane: "p1", Panes: 1}, {Group: 2, Pane: "p2", Panes: 1}},
	})
	fw.Attach(lastWindow.Client(), app.FilePaneHost("p1"))
	framesUntil(t, "the file manager shows in its place", func() bool {
		return filemanager.FocusIn(lastUI, app.FilePaneViews("p1")) != nil
	})
	drain()
	r, ok := lastUI.Bounds(win.paneNode("p1"))
	if !ok {
		t.Fatal("the file manager pane is not drawn")
	}
	at := geom.Pt(r.Max.X-10, r.Center().Y)
	tab := dock.Drag{ID: "2", Win: 1}
	lastWindow.Input(gi.DragOver{Pos: at, Data: tab})
	lastWindow.Frame(time.Second / 60)
	lastWindow.Input(gi.Drop{Pos: at, Data: tab})
	lastWindow.Frame(time.Second / 60)
	for {
		if in, ok := nextIntent(t).(app.DockTab); ok {
			if in.Group != 2 || in.Beside != "p1" || in.Vertical || in.First {
				t.Fatalf("dropped at the pane's right edge, the window sent %#v", in)
			}
			return
		}
	}
}
