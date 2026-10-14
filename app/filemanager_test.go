package app

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/marrasen/gunim/single"
	"github.com/marrasen/kakel/machines"
	"github.com/marrasen/kakel/remote"
	"github.com/marrasen/kakel/settings"

	"github.com/marrasen/gunim"
	"github.com/marrasen/gunim/driver"
	"github.com/marrasen/gunim/filemanager"
	"github.com/marrasen/gunim/geom"
	"github.com/marrasen/gunim/gunimtest"
)

// fakeFiles stands in for the file manager's hub: it records the
// options of each pane it is asked for, and runs the pane on a hub of its
// own, with its settings in the test's folder.
type fakeFiles struct {
	t         *testing.T
	hub       *filemanager.Hub
	opened    []filemanager.Options
	refreshed int
}

func newFakeFiles(t *testing.T) *fakeFiles {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &fakeFiles{t: t, hub: filemanager.NewHub(ctx, nil)}
}

func (f *fakeFiles) NewPane(o filemanager.Options, host filemanager.PaneHost) (*filemanager.Window, error) {
	f.opened = append(f.opened, o)
	o.PrefsPath, o.Poll = filepath.Join(f.t.TempDir(), "files.json"), -1
	w, err := f.hub.NewPane(o, host)
	if err == nil {
		// Stopped before its folder is taken away, which Windows refuses
		// while it has a file there open.
		f.t.Cleanup(func() {
			w.Stop()
			select {
			case <-w.Done():
			case <-time.After(5 * time.Second):
			}
		})
	}
	return w, err
}

func (f *fakeFiles) Refresh() { f.refreshed++ }

// Files open in a file manager pane, beside the one in front, and the
// servers are the file manager's places, under Machines, with how they
// are doing.
func TestFilesOpenInAFileManagerPane(t *testing.T) {
	a, _ := agentApp(t)
	a.settings = mustSettings(t)
	files := newFakeFiles(t)
	a.files = files
	a.st.Saved = []remote.Host{{ID: "s1", Name: "web", Address: "web.example"}}

	panes := len(a.st.Panes)
	a.handle(OpenFilesOn{})
	if len(files.opened) != 1 || len(a.st.Panes) != panes+1 || files.opened[0].FS == nil {
		t.Fatalf("files opened %d file managers and %d panes", len(files.opened), len(a.st.Panes)-panes)
	}
	first := a.st.Panes[len(a.st.Panes)-1]
	if first.Kind != KindFileManager || a.st.Focus != first.ID || a.fmPanes[first.ID] == nil {
		t.Fatalf("the pane opened is %+v, with the keyboard on %q", first, a.st.Focus)
	}
	a.handle(OpenFilesOn{})
	second := a.st.Panes[len(a.st.Panes)-1]
	if a.groupOf[second.ID] != a.groupOf[first.ID] {
		t.Fatal("a second file manager pane didn't open beside the one in front")
	}
	places, _ := files.opened[0].Places()
	i := slices.IndexFunc(places, func(p filemanager.Place) bool { return p.Group == "Machines" })
	if i < 0 || places[i].Name != "web" || places[i].Note != "Not connected" || places[i].FS != serverFS+"s1" {
		t.Fatalf("the places are %+v", places)
	}
	// Closed, its file manager stops, and the pane goes.
	a.handle(ClosePane{Pane: second.ID})
	fw := a.fmPanes[second.ID].w
	select {
	case <-fw.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("closing the pane left its file manager running")
	}
	waitFor(t, a, "the pane goes", func() bool { return !a.has(second.ID) })
	if _, ok := a.fmPanes[second.ID]; ok {
		t.Fatal("a closed file manager pane is still kept")
	}
}

// -files opens a folder in a window holding a file manager pane, as
// Windows asks once kakel opens folders: in the kakel running, handed
// over, and a drive's root read through the quote Windows' "%V\." keeps
// whole.
func TestAFolderOpenedAnywhereOpensInTheFileManager(t *testing.T) {
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"-files", "/home/me/."}, filepath.FromSlash("/home/me")},
		{[]string{"-files", ""}, ""},
		{[]string{"-files", `/mnt/"`}, filepath.FromSlash("/mnt")},
	} {
		o, err := ParseOptions(c.args)
		if err != nil {
			t.Fatal(err)
		}
		dir, set := o.OpensFolder()
		if !set || dir != c.want || o.StartsHidden() || o.StartsInTray() {
			t.Errorf("%q opens %q (%v), hidden %v, in the tray %v", c.args, dir, set, o.StartsHidden(), o.StartsInTray())
		}
	}
	if _, set := (Options{}).OpensFolder(); set {
		t.Fatal("no -files opens a folder")
	}

	a, _ := agentApp(t)
	a.settings = mustSettings(t)
	files := newFakeFiles(t)
	a.files = files
	// Counted on the goroutine a window opens on.
	var windows atomic.Int32
	a.openWindow = func(*gunim.Window, geom.Point, geom.Size, *driver.Placement) (gunim.Client, *gunim.Window, error) {
		windows.Add(1)
		w := gunimtest.New(t, geom.Sz(400, 300), nil)
		return w.Client(), w, nil
	}
	a.handover(single.Handover{Args: []string{"-files", "/srv/data/."}})
	waitFor(t, a, "a window opens", func() bool { return windows.Load() == 1 && len(files.opened) == 1 })
	p := a.st.Panes[len(a.st.Panes)-1]
	if files.opened[0].Dir != filepath.FromSlash("/srv/data") || p.Kind != KindFileManager || a.ownerOf(p.ID) != a.cur || len(a.panesIn(a.cur)) != 1 {
		t.Fatalf("handed a folder, kakel opened %+v in a window of %d panes", files.opened, len(a.panesIn(a.cur)))
	}

	// A file opens its folder, with the file selected.
	dir := t.TempDir()
	file := filepath.Join(dir, "download.zip")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	a.handover(single.Handover{Args: []string{"-files", file}})
	waitFor(t, a, "a second window opens", func() bool { return windows.Load() == 2 && len(files.opened) == 2 })
	if o := files.opened[1]; o.Dir != dir || o.Select != "download.zip" {
		t.Fatalf("handed a file, kakel opened %q selecting %q", o.Dir, o.Select)
	}
}

// A file manager pane moved to another window shows there once the
// windows are told, and the intents of its views go to it, not to
// kakel.
func TestAFileManagerPaneFollowsItsPaneToAnotherWindow(t *testing.T) {
	a, one, two := twoWindowApp(t)
	a.files = newFakeFiles(t)
	a.front(one)
	if err := a.filesOn(machines.Local, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	id := a.st.Focus
	a.publish()
	if fp := a.fmPanes[id]; fp == nil || fp.win != one {
		t.Fatal("the file manager pane isn't shown in its window")
	}
	a.moveToWindow(id, two)
	a.publish()
	if a.fmPanes[id].win != two {
		t.Fatal("the file manager didn't follow its pane to the other window")
	}
	from := gunim.ID(FilePaneViews(id) + "/browser")
	if !a.toFilePane(gunim.Envelope{From: from, Intent: filemanager.Command{Name: filemanager.CmdRefresh}}) {
		t.Fatal("an intent of the file manager's views went to kakel")
	}
	if a.toFilePane(gunim.Envelope{From: "window", Intent: NewTerminal{}}) {
		t.Fatal("an intent of kakel's window went to the file manager")
	}
}

// New Tab, as the tab bar's + asks, opens a file manager at the folder
// of the file manager in front, in a tab of its own, and a terminal
// where a terminal is in front.
func TestNewTabIsLikeTheTabInFront(t *testing.T) {
	a, _ := agentApp(t)
	a.settings = mustSettings(t)
	files := newFakeFiles(t)
	a.files = files
	term := a.st.Focus
	dir := t.TempDir()
	if err := a.filesOn(machines.Local, dir); err != nil {
		t.Fatal(err)
	}
	first := a.st.Focus
	a.handle(NewTab{})
	waitFor(t, a, "a second file manager opens", func() bool { return len(a.fmPanes) == 2 })
	second := a.st.Focus
	if second == first || a.kindOfPane(second) != KindFileManager || a.groupOf[second] == a.groupOf[first] {
		t.Fatalf("New Tab opened %q, beside %q", second, first)
	}
	if got := files.opened[len(files.opened)-1].Dir; got != dir {
		t.Fatalf("the new file manager opened at %q, want %q", got, dir)
	}
	a.focus(term)
	panes := len(a.st.Panes)
	a.handle(NewTab{})
	if len(a.st.Panes) != panes+1 || a.kindOfPane(a.st.Focus) != KindTerminal {
		t.Fatal("New Tab on a terminal didn't open a terminal")
	}
}

// A file manager window opens where the last one was as it closed, a
// step down and right of one still there, and keeps that place apart
// from the terminals' window.
func TestAFileManagerWindowOpensWhereTheLastOneClosed(t *testing.T) {
	a, _ := agentApp(t)
	a.settings = mustSettings(t)
	a.files = newFakeFiles(t)
	var mu sync.Mutex
	var asked []*driver.Placement
	a.openWindow = func(_ *gunim.Window, _ geom.Point, _ geom.Size, place *driver.Placement) (gunim.Client, *gunim.Window, error) {
		mu.Lock()
		asked = append(asked, place)
		mu.Unlock()
		w := gunimtest.New(t, geom.Sz(400, 300), nil)
		return w.Client(), w, nil
	}
	where := map[*ownWin]driver.Placement{}
	a.placed = func(w *ownWin) (driver.Placement, bool) {
		p, ok := where[w]
		return p, ok
	}
	opened := func() []*driver.Placement {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(asked)
	}
	open := func(n int) *ownWin {
		t.Helper()
		before := a.cur
		a.openFolder(t.TempDir())
		waitFor(t, a, "the window opens", func() bool {
			panes := a.panesIn(a.cur)
			return len(opened()) == n && a.cur != before && len(panes) == 1 && panes[0].Kind == KindFileManager
		})
		return a.cur
	}

	first := open(1)
	if opened()[0] != nil {
		t.Fatalf("with no place kept, the first opened at %v", opened()[0])
	}
	where[first] = driver.Placement{Bounds: geom.Rc(100, 120, 1000, 700)}
	a.closeWindow(first)
	if p, ok := a.settings.FilesWindow(); !ok || p != (settings.WindowPlace{X: 100, Y: 120, W: 1000, H: 700}) {
		t.Fatalf("the file manager window closed and %v (%v) was kept", p, ok)
	}
	if _, ok := a.settings.Window(); ok {
		t.Fatal("the file manager window was kept as the terminals' window")
	}

	second := open(2)
	if p := opened()[1]; p == nil || p.Bounds != geom.Rc(100, 120, 1000, 700) {
		t.Fatalf("the next opened at %v, not where the last closed", p)
	}
	where[second] = *opened()[1]
	open(3)
	if p := opened()[2]; p == nil || p.Bounds != geom.Rc(100+cascade, 120+cascade, 1000, 700) {
		t.Fatalf("one opened over another at %v, not a step down and right", p)
	}
}

// A file manager that opens another, as Disk usage's Show in Files does,
// opens it in a pane of its own with the item it names selected.
func TestAnotherFileManagerOpensWithItsItemSelected(t *testing.T) {
	a, _ := agentApp(t)
	a.settings = mustSettings(t)
	files := newFakeFiles(t)
	a.files = files
	dir := t.TempDir()
	panes := len(a.st.Panes)
	a.openFilePaneLike(filemanager.Options{FS: filemanager.LocalFS(), Dir: dir, Select: "notes.md"})
	if len(files.opened) != 1 || len(a.st.Panes) != panes+1 {
		t.Fatalf("it opened %d file managers and %d panes", len(files.opened), len(a.st.Panes)-panes)
	}
	if o := files.opened[0]; o.Dir != dir || o.Select != "notes.md" {
		t.Errorf("it opened %s with %q selected", o.Dir, o.Select)
	}
}
