package app

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/marrasen/gunim"
	"github.com/marrasen/gunim/filemanager"

	"github.com/marrasen/kakel/machines"
)

// File manager panes: gunim's file manager in a pane, among terminals,
// split, docked and moved between windows like any pane. Its program
// half runs on in the background as the pane moves: each window it shows
// in gives it a place, the host, and it mounts its views there, under IDs
// of its own. The intents of those views go straight to it.

// KindFileManager is a file manager pane's kind.
const KindFileManager = "filemanager"

// FilePaneViews starts the IDs of the views of file manager pane id, as
// its intents come from them.
func FilePaneViews(id string) string { return "fm:" + id }

// FilePaneHost is the ID of the place a window gives file manager pane
// id, which its views mount under.
func FilePaneHost(id string) gunim.ID { return gunim.ID("fmhost:" + id) }

// FilePaneCommands are the file manager's commands kakel's menus offer
// for a file manager pane, which its own menus leave out: kakel's Edit
// menu, New File Manager Window and Close Pane.
var FilePaneCommands = []string{filemanager.CmdCut, filemanager.CmdCopy, filemanager.CmdPaste,
	filemanager.CmdSelectAll, filemanager.CmdNewWindow, filemanager.CmdCloseApp}

// errNoFileManager says this kakel has no file manager, as in a test.
var errNoFileManager = errors.New("the file manager can't open here")

// fmPane is a file manager pane: its file manager, and the window it was
// put in last, or nil.
type fmPane struct {
	w   *filemanager.Window
	win *ownWin
	// done says the file manager has stopped: closed by the user, as
	// with Ctrl+W, so the pane goes.
	done bool
	// git is the git status on its status bar.
	git gitWatch
}

// newFilePane opens a file manager pane on fsys, the files of machine, at
// path or at home, where at says. A file on this computer opens its
// folder, with the file selected.
func (a *app) newFilePane(machine machines.ID, fsys filemanager.FS, path string, at Placement) error {
	if a.files == nil {
		return errNoFileManager
	}
	pick := ""
	if machine == machines.Local {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			path, pick = filepath.Dir(path), filepath.Base(path)
		}
	}
	a.next++
	id := "p" + itoa(a.next)
	fw, err := a.files.NewPane(a.fileManagerOptions(fsys, path, pick), filemanager.PaneHost{
		ID: FilePaneViews(id),
		// Its menus join kakel's, as the window shows them.
		HostMenus: true,
		Title: func(fs, folder string) {
			// The newest is kept, and taken whenever the program gets to
			// it: the goroutines that hand it over may come in any order.
			a.fmTitleMu.Lock()
			if a.fmTitles == nil {
				a.fmTitles = map[string][2]string{}
			}
			a.fmTitles[id] = [2]string{fs, folder}
			a.fmTitleMu.Unlock()
			a.later(func() { a.retitleFilePane(id) })
		},
		Folder: func(fs, path string) {
			a.fmTitleMu.Lock()
			if a.fmFolders == nil {
				a.fmFolders = map[string][2]string{}
			}
			a.fmFolders[id] = [2]string{fs, path}
			a.fmTitleMu.Unlock()
			a.later(func() { a.filePaneFolder(id) })
		},
		Commands: FilePaneCommands,
		Open: func(o filemanager.Options) error {
			a.later(func() { a.openFilePaneLike(o) })
			return nil
		},
	})
	if err != nil {
		return err
	}
	// Named for its folder once the file manager has read it.
	title := "Files"
	if path != "" {
		title = fsys.Paths().Base(path)
	}
	a.addPane(a.paneOn(machine, Pane{ID: id, Title: title, Kind: KindFileManager}), nil, at)
	if a.fmPanes == nil {
		a.fmPanes = map[string]*fmPane{}
	}
	a.fmPanes[id] = &fmPane{w: fw}
	a.routeFilePanes()
	go func() {
		<-fw.Done()
		a.later(func() { a.filePaneEnded(id) })
	}()
	return nil
}

// later runs fn on the program's goroutine, unless kakel ends first. It
// never waits for the program, so a file manager may call it from its
// own goroutine while the program waits for that.
func (a *app) later(fn func()) {
	go func() {
		select {
		case a.events <- fn:
		case <-a.ctx.Done():
		}
	}()
}

// fileManagerOptions are the options a file manager opens with on fsys,
// at path or at home, with the name pick selected.
func (a *app) fileManagerOptions(fsys filemanager.FS, path, pick string) filemanager.Options {
	a.notePlaces()
	return filemanager.Options{
		FS: fsys, Dir: path, Select: pick, Name: ProgramName, PrefsPath: fileManagerPrefs(),
		Places: a.fileManagerPlaces, Visit: a.visitPlace, Favourites: a.favStore(),
		Transfer: a.transferFiles, FSName: a.fsName,
		// A zip's password can come from the secrets, and go into them.
		Password:  a.zipPassword,
		PlaceMenu: placeMenu, PlaceCommand: a.placeCommand,
		SystemFrame: a.st.SystemTitleBar,
		// What a file manager shows going wrong is kept in the Window Log
		// too, past the banner it is dismissed from.
		Log: func(line string) { log.Print(line) },
	}
}

// newFilePaneLike opens another file manager pane, in a tab of its own,
// like file manager pane id: on its file system, at the folder it shows.
// It reports false where id is no file manager pane.
func (a *app) newFilePaneLike(id string) bool {
	fp := a.fmPanes[id]
	if fp == nil || fp.done {
		return false
	}
	// As its own New window does: it hands its options, at its folder,
	// to openFilePaneLike.
	go fp.w.Deliver(gunim.Envelope{From: gunim.ID(FilePaneViews(id) + "/browser"),
		Intent: filemanager.Command{Name: filemanager.CmdNewWindow}})
	return true
}

// openFilePaneLike opens another file manager pane with o's file system
// and folder, as New window in one asks: on a stage of its own, in the
// window in front.
func (a *app) openFilePaneLike(o filemanager.Options) {
	fsys := o.FS
	if fsys == nil {
		fsys = filemanager.LocalFS()
	}
	if err := a.newFilePane(machineOfFS(fsys.ID()), fsys, o.Dir, Placement{}); err != nil {
		a.failed("Couldn't open the files", err.Error())
	}
}

// retitleFilePane names file manager pane id after the folder it shows
// last said, and files it under the machine whose files it shows, as a
// place can turn it to another's.
func (a *app) retitleFilePane(id string) {
	a.fmTitleMu.Lock()
	t, ok := a.fmTitles[id]
	a.fmTitleMu.Unlock()
	if !ok {
		return
	}
	fs, folder := t[0], t[1]
	for i := range a.st.Panes {
		p := &a.st.Panes[i]
		if p.ID != id {
			continue
		}
		if !p.Named && folder != "" {
			p.Title = folder
		}
		if m := machineOfFS(fs); m != a.filesKey(id) {
			delete(a.farHost, id)
			p.On = ""
			*p = a.paneOn(m, *p)
		}
	}
}

// filePaneEnded takes away file manager pane id, whose file manager has
// stopped, as when the user closed it.
func (a *app) filePaneEnded(id string) {
	fp := a.fmPanes[id]
	if fp == nil {
		return
	}
	fp.done = true
	a.closePane(id)
}

// closeFilePane closes file manager pane id as the user asks: its file
// manager first, which asks before it stops what is running in it, and
// the pane once it has. It reports false for a pane that is no file
// manager's, or whose file manager has stopped already.
func (a *app) closeFilePane(id string) bool {
	fp := a.fmPanes[id]
	if fp == nil || fp.done {
		return false
	}
	// In front, so the question about what runs in it is seen, for one
	// closed from elsewhere, as the Machines pane.
	a.focusRaised(id)
	fp.w.Close()
	return true
}

// dropFilePane forgets file manager pane id as it goes, stopping its file
// manager at once if it still runs, as when its window closes.
func (a *app) dropFilePane(id string) {
	fp := a.fmPanes[id]
	if fp == nil {
		return
	}
	if !fp.done {
		fp.w.Stop()
	}
	delete(a.fmPanes, id)
	a.fmTitleMu.Lock()
	delete(a.fmTitles, id)
	delete(a.fmFolders, id)
	a.fmTitleMu.Unlock()
	a.routeFilePanes()
}

// fileOpsIn counts the operations running in the file manager panes
// among ids, or in all of them for nil: copies, moves and deletes, which
// closing a pane stops.
func (a *app) fileOpsIn(ids []string) int {
	n := 0
	for id, fp := range a.fmPanes {
		if !fp.done && (ids == nil || slices.Contains(ids, id)) {
			n += fp.w.Running()
		}
	}
	return n
}

// routeFilePanes tells the windows' goroutines which file managers the
// intents of their views may be for.
func (a *app) routeFilePanes() {
	list := make([]*filemanager.Window, 0, len(a.fmPanes))
	for _, fp := range a.fmPanes {
		list = append(list, fp.w)
	}
	a.fmRoute.Store(&list)
}

// toFilePane hands env to the file manager pane it is for, and reports
// whether there was one. It runs on a window's goroutine.
func (a *app) toFilePane(env gunim.Envelope) bool {
	if !strings.HasPrefix(string(env.From), "fm:") {
		if _, failed := env.Intent.(gunim.CommandFailed); !failed {
			return false
		}
	}
	list := a.fmRoute.Load()
	if list == nil {
		return false
	}
	for _, fw := range *list {
		if fw.Deliver(env) {
			return true
		}
	}
	return false
}

// showFilePanes puts each file manager pane in the window it is in now,
// once the windows have been sent the state that gives it a place there,
// and gives the keyboard to one that came into a window with it.
func (a *app) showFilePanes() {
	for id, fp := range a.fmPanes {
		w := a.ownerOf(id)
		if w == nil || w.gone {
			if fp.win != nil {
				fp.w.Detach()
				fp.win = nil
			}
			continue
		}
		if fp.win == w {
			continue
		}
		fp.w.Attach(w.c, FilePaneHost(id))
		fp.win = w
		if a.focusIn(w) == id {
			fp.w.Focus()
		}
	}
}
