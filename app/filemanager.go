package app

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/marrasen/kakel/conf"
	"github.com/marrasen/kakel/jobs"
	"github.com/marrasen/kakel/machines"
	"github.com/marrasen/kakel/vfs"
	"github.com/marrasen/kakel/words"

	"github.com/marrasen/gunim/filemanager"
)

// The file manager: gunim's, in panes among kakel's terminals; see
// fmpanes.go. Its places are the machines, this computer's folders under
// This computer and each saved server under Machines.

// FileWindows makes the file manager's panes and tells them their places
// changed: gunim's filemanager.Hub.
type FileWindows interface {
	NewPane(o filemanager.Options, host filemanager.PaneHost) (*filemanager.Window, error)
	Refresh()
}

// Intents for the file manager.
type (
	// OpenFilesOn opens a file manager pane on Machine, at Path or at
	// home, in the window in front: beside the file manager in front, or
	// in a tab of its own with NewTab.
	OpenFilesOn struct {
		Machine machines.ID
		Path    string
		NewTab  bool
	}
	// OpenFileManager opens a window of its own holding a file manager
	// pane on Machine, at Path or at home.
	OpenFileManager struct {
		Machine machines.ID
		Path    string
	}
)

// serverFS is how a server is named as a file system in the file
// manager's places: this prefix and its ID.
const serverFS = "kakel:"

// gone marks a server place whose files the file manager had and lost,
// so a pane showing them sees the place as elsewhere, and a click on it
// connects again.
const gone = "\x00gone"

// filesOn opens a file manager pane on machine, at path, or when path is
// empty at the one folder saved for the machine, or else at home: beside
// the file manager pane in front, two side by side being the way to copy
// between them, and otherwise on a stage of its own.
func (a *app) filesOn(machine machines.ID, path string) error {
	at := Placement{}
	if a.kindOfPane(a.st.Focus) == KindFileManager {
		at.Beside = a.st.Focus
	}
	return a.filePaneOn(machine, path, at, false)
}

// filePaneOn opens a file manager pane on machine, at path or as filesOn
// says, where at says. A server is connected to first, quietly when
// quiet, and its files opened.
func (a *app) filePaneOn(machine machines.ID, path string, at Placement, quiet bool) error {
	if a.files == nil {
		return errNoFileManager
	}
	path = a.startFolder(machine, path)
	if machine == machines.Local {
		return a.newFilePane(machine, filemanager.LocalFS(), path, at)
	}
	failed := func(err error) {
		if err != nil {
			a.failed("Couldn't open the files on "+a.machines.Name(machine), err.Error())
		}
	}
	return a.withFilesHow(machine, func(f vfs.FS) {
		failed(a.newFilePane(machine, a.fmFor(machine, f), path, at))
	}, failed, quiet)
}

// startFolder is where files on machine open for path: path, or for ""
// the one folder saved for the machine, or else home, "".
func (a *app) startFolder(machine machines.ID, path string) string {
	if saved := a.savedFolders(machine); path == "" && len(saved) == 1 {
		// One folder saved for the machine is where its files open,
		// however they are asked for: it is the one the user wants.
		return saved[0]
	}
	return path
}

// openFolder opens a window of its own holding a file manager pane on
// this computer's folder dir, or home for "", as a folder opened anywhere
// asks once kakel opens them. A file opens its folder, with it selected.
func (a *app) openFolder(dir string) {
	if err := a.openFileManager(machines.Local, dir); err != nil {
		a.failed("Couldn't open the folder", err.Error())
	}
}

// openFileManager opens a window of its own holding a file manager pane
// on m, at path or at home. A server is connected to quietly first, as
// the window is the files' alone.
func (a *app) openFileManager(m machines.ID, path string) error {
	if a.files == nil {
		return errNoFileManager
	}
	a.newWindowAt(a.filesPlace(), func() {
		// Into this window, however long the server takes, and whichever
		// window is in front by then.
		w := a.cur
		at := Placement{}
		if m != machines.Local {
			// The dial and the opening of the files, under way, keep the
			// window meanwhile.
			failed := func(err error) {
				if err != nil {
					a.failed("Couldn't open the files on "+a.machines.Name(m), err.Error())
				}
			}
			failed(a.withFilesHow(m, func(f vfs.FS) {
				if !w.gone {
					a.front(w)
				}
				failed(a.newFilePane(m, a.fmFor(m, f), a.startFolder(m, path), at))
			}, failed, true))
			return
		}
		if err := a.filePaneOn(m, path, at, true); err != nil {
			a.failed("Couldn't open the files on "+a.machines.Name(m), err.Error())
		}
	})
	return nil
}

// favStore is where the file manager's windows keep their favourites:
// kakel's settings, one store for every window, so they share them. nil
// when there are no settings, and the windows keep them as the file
// manager does by itself.
func (a *app) favStore() filemanager.FavouriteStore {
	if a.settings == nil {
		return nil
	}
	if a.fmFavs == nil {
		a.fmFavs = &fmFavourites{a: a}
	}
	return a.fmFavs
}

// fmFor is m's files, open as f, as the file manager reads them: the
// same for every window, so one that lost them has them again.
func (a *app) fmFor(m machines.ID, f vfs.FS) *fmFS {
	fm := a.fmFiles[m]
	if fm == nil {
		if a.fmFiles == nil {
			a.fmFiles = map[machines.ID]*fmFS{}
		}
		fm = newFMFS(serverFS+string(m), f)
		// Its place goes to its home once that is known.
		fm.learned = func() { go func() { a.events <- a.notePlaces }() }
		a.fmFiles[m] = fm
	} else {
		fm.set(f)
	}
	if fm.homeDir() == "" {
		go func() { _, _ = fm.Home() }()
	}
	return fm
}

// fmFilesOf is m's files, open as f, as the file manager reads them.
func (a *app) fmFilesOf(m machines.ID, f vfs.FS) filemanager.FS {
	if m == machines.Local {
		return filemanager.LocalFS()
	}
	return a.fmFor(m, f)
}

// fmBack hands the file manager m's files, opened again, so a window
// that lost them has them again.
func (a *app) fmBack(m machines.ID, f vfs.FS) {
	if fm := a.fmFiles[m]; fm != nil {
		fm.set(f)
	}
}

// fmGone tells the file manager m's files are gone, as its connection
// ended.
func (a *app) fmGone(m machines.ID) {
	if fm := a.fmFiles[m]; fm != nil {
		fm.set(nil)
	}
}

// fileManagerPrefs is where the file manager keeps its settings: beside
// kakel's own, or "" for its default when kakel's place can't be found.
func fileManagerPrefs() string {
	dir, err := conf.Dir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "files.json")
}

// fileManagerPlaces are the file manager's places: this computer's
// folders, then the servers, as last noted. It runs off the program's
// goroutine.
func (a *app) fileManagerPlaces() ([]filemanager.Place, error) {
	out, err := filemanager.LocalPlaces()
	for i := range out {
		out[i].Group = "This computer"
	}
	if servers := a.serverPlaces.Load(); servers != nil {
		out = append(out, *servers...)
	}
	return out, err
}

// notePlaces notes the servers as the file manager lists them, and has
// its windows read their places again when that changed: a server
// saved, renamed, connected or disconnected.
func (a *app) notePlaces() {
	var places []filemanager.Place
	// As the machines are now: State's lists are filled in only on the
	// copy the windows are sent.
	dialing, connected := a.machines.Dialing(), a.machines.Connected()
	// Saved windows among them: a window's files are its machine's.
	windows := map[machines.ID]bool{}
	for _, m := range a.machines.Windows() {
		windows[m] = true
	}
	for _, h := range a.st.Saved {
		m := machines.ID(h.ID)
		note := "Not connected"
		switch {
		case slices.Contains(dialing, m):
			note = "Connecting…"
		case slices.Contains(connected, m) || windows[m]:
			note = "Connected"
		}
		p := filemanager.Place{Name: h.Name, Kind: "drive", Group: "Machines", Note: note, FS: serverFS + h.ID,
			Lit: slices.Contains(connected, m) || windows[m]}
		if fm := a.fmFiles[m]; fm != nil {
			if fm.live() {
				p.Path = fm.homeDir()
			} else {
				p.FS += gone
			}
		}
		places = append(places, p)
	}
	a.noteNames()
	if old := a.serverPlaces.Load(); old != nil && slices.Equal(*old, places) {
		return
	}
	a.serverPlaces.Store(&places)
	if a.files != nil && len(a.fmPanes) > 0 {
		a.files.Refresh()
	}
}

// visitPlace turns file manager w to a place on another file system than
// the one it shows: this computer's, or a server's, connected to first.
// With newWindow, as with Ctrl held, the place opens in a pane of its
// own, on a stage of its own, and w stays as it is. It runs on a
// goroutine of its own.
func (a *app) visitPlace(w *filemanager.Window, fs, path string, newWindow bool) {
	if fs == "" {
		if !newWindow {
			w.Show(filemanager.LocalFS(), path)
			return
		}
		a.events <- func() {
			if err := a.newFilePane(machines.Local, filemanager.LocalFS(), path, Placement{}); err != nil {
				w.Notify("Couldn't open the files", words.UpperFirst(err.Error())+".", "warning")
			}
		}
		return
	}
	id, ok := strings.CutPrefix(fs, serverFS)
	if !ok {
		return
	}
	m := machines.ID(strings.TrimSuffix(id, gone))
	a.events <- func() {
		// Quietly: the window says what went wrong, not kakel's.
		failed := func(err error) {
			if err != nil {
				w.Notify("Couldn't open the files on "+a.machines.Name(m), words.UpperFirst(err.Error())+".", "warning")
			}
		}
		show := func(f vfs.FS) {
			if !newWindow {
				w.Show(a.fmFor(m, f), path)
				return
			}
			if err := a.newFilePane(m, a.fmFor(m, f), path, Placement{}); err != nil {
				failed(err)
			}
		}
		if err := a.withFilesHow(m, show, failed, true); err != nil {
			failed(err)
		}
	}
}

// transferFiles copies or moves items between machines, as file
// manager window w asks, as kakel's copy jobs: w shows how they go,
// stops them and asks whether to replace a file, and kakel's windows
// list them, quietly. It runs on a goroutine of its own, and returns
// once the items are across.
func (a *app) transferFiles(ctx context.Context, _ *filemanager.Window, t filemanager.Transfer, p *filemanager.TransferProgress) error {
	if len(t.Paths) == 0 || t.Into == "" {
		return nil
	}
	from, to := machineOfFS(t.FromFS), machineOfFS(t.ToFS)
	// on runs fn on the program's goroutine, unless the transfer or
	// kakel ends first.
	on := func(fn func()) bool {
		select {
		case a.events <- fn:
			return true
		case <-ctx.Done():
		case <-a.ctx.Done():
		}
		return false
	}
	type opened struct {
		ff, tf vfs.FS
		err    error
	}
	got := make(chan opened, 1)
	var once sync.Once
	answer := func(o opened) { once.Do(func() { got <- o }) }
	unreached := func(err error) {
		if err == nil {
			err = errors.New("a machine could not be reached")
		}
		answer(opened{err: err})
	}
	if !on(func() {
		err := a.withFilesHow(from, func(ff vfs.FS) {
			if err := a.withFilesHow(to, func(tf vfs.FS) { answer(opened{ff: ff, tf: tf}) }, unreached, true); err != nil {
				answer(opened{err: err})
			}
		}, unreached, true)
		if err != nil {
			answer(opened{err: err})
		}
	}) {
		return context.Cause(ctx)
	}
	var o opened
	select {
	case o = <-got:
	case <-ctx.Done():
		return ctx.Err()
	}
	if o.err != nil {
		return o.err
	}
	if t.Zip != "" {
		// A zip is the file manager's to make, through the files of
		// each machine as it reads them.
		ends := make(chan [2]filemanager.FS, 1)
		if !on(func() { ends <- [2]filemanager.FS{a.fmFilesOf(from, o.ff), a.fmFilesOf(to, o.tf)} }) {
			return context.Cause(ctx)
		}
		var fs [2]filemanager.FS
		select {
		case fs = <-ends:
		case <-ctx.Done():
			return ctx.Err()
		}
		return filemanager.ZipFiles(ctx, fs[0], t.Paths, fs[1], t.Into, t.Zip, t.Password, p)
	}
	kind, verb := jobs.Copy, "Copying"
	if t.Move {
		kind, verb = jobs.Move, "Moving"
	}
	// The items by the folder they are in, in the order they came: a job
	// for each.
	var ats []string
	names := map[string][]string{}
	for _, path := range t.Paths {
		at := vfs.Dir(o.ff, path)
		if _, ok := names[at]; !ok {
			ats = append(ats, at)
		}
		names[at] = append(names[at], vfs.Base(o.ff, path))
	}
	ask := &fmAsker{p: p}
	for _, at := range ats {
		op := jobs.Op{Kind: kind, From: o.ff, At: at, Names: names[at], To: o.tf, Into: t.Into}
		title := verb + " " + words.Count(len(names[at]), "item") + " to " + vfs.Base(o.tf, t.Into)
		started := make(chan *jobs.Job, 1)
		if !on(func() {
			if ctx.Err() != nil {
				// Stopped while this waited its turn: nothing starts.
				started <- nil
				return
			}
			started <- a.followAsking(op, title, from, to, ask, true)
		}) {
			return context.Cause(ctx)
		}
		var job *jobs.Job
		select {
		case job = <-started:
		case <-a.ctx.Done():
			return a.ctx.Err()
		}
		if job == nil {
			return ctx.Err()
		}
		if err := follow(ctx, job, p); err != nil {
			return err
		}
	}
	return nil
}

// follow tells p how job goes until it ends, and stops it when ctx
// ends. It returns why the job stopped short.
func follow(ctx context.Context, job *jobs.Job, p *filemanager.TransferProgress) error {
	t := time.NewTicker(SampleEvery)
	defer t.Stop()
	report := func() jobs.Progress {
		pr := job.Progress()
		p.Report(pr.BytesDone, pr.Bytes, pr.FilesDone, pr.Files, pr.Current)
		return pr
	}
	for {
		select {
		case <-job.Done():
			pr := report()
			if why := jobs.Trouble(pr.Err); why != nil {
				return why
			}
			// Nil, or stopped as the user asked.
			return pr.Err
		case <-ctx.Done():
			// A job stuck in a read or a write may not hear it at once:
			// the window does not wait long for it, as kakel's own jobs
			// list it until it ends.
			job.Cancel()
			select {
			case <-job.Done():
			case <-time.After(stopWait):
			}
			return ctx.Err()
		case <-t.C:
			report()
		}
	}
}

// stopWait is how long a transfer the user stopped waits for its job to
// end before the window hears it has.
const stopWait = 2 * time.Second

// fmAsker asks about a name that is taken through the file manager
// window a transfer runs in. An answer for all is kept here: a job
// would replace a file with a folder on a Replace for all, which the
// window never offers, and asks again about each name to keep beside.
type fmAsker struct {
	p   *filemanager.TransferProgress
	mu  sync.Mutex
	all *filemanager.Choice
}

// Overwrite implements [jobs.Ask].
func (f *fmAsker) Overwrite(ctx context.Context, c jobs.Conflict) (jobs.Choice, error) {
	sameKind := c.Have.IsDir() == c.Want.IsDir()
	f.mu.Lock()
	all := f.all
	f.mu.Unlock()
	if all != nil && (*all != filemanager.ChoiceReplace || sameKind) {
		return f.choice(c, *all), nil
	}
	coming := filemanager.ItemFacts(c.Want.Size, c.Want.IsDir(), c.Want.Mod)
	choice, forAll, err := f.p.Clash(ctx, c.Path, coming, sameKind)
	if err != nil {
		// Stopped there, as the user asked: a stop, not a failure.
		return jobs.Choice{What: jobs.Stop}, nil
	}
	if forAll {
		f.mu.Lock()
		f.all = &choice
		f.mu.Unlock()
	}
	return f.choice(c, choice), nil
}

// choice is what the job does about c, as the window answered.
func (f *fmAsker) choice(c jobs.Conflict, choice filemanager.Choice) jobs.Choice {
	switch choice {
	case filemanager.ChoiceReplace:
		return jobs.Choice{What: jobs.Replace}
	case filemanager.ChoiceSkip:
		return jobs.Choice{What: jobs.Skip}
	}
	return jobs.Choice{What: jobs.Rename, Name: nameBeside(c.To, c.Path, c.Want.IsDir())}
}

// nameBeside is a name beside path on f that nothing has: "a (2).txt" for
// a.txt, and "v1.2 (2)" for a folder, counting on. One that can't be
// told free is given as it is, and the job says what is wrong with it.
func nameBeside(f vfs.FS, path string, dir bool) string {
	at, name := vfs.Dir(f, path), vfs.Base(f, path)
	stem, ext := name, ""
	if i := strings.LastIndexByte(name, '.'); i > 0 && !dir {
		stem, ext = name[:i], name[i:]
	}
	for n := 2; ; n++ {
		try := stem + " (" + strconv.Itoa(n) + ")" + ext
		// Stat reads a link as itself, so one going nowhere is taken.
		if _, err := f.Stat(vfs.Join(f, at, try)); err != nil || n > 999 {
			return try
		}
	}
}

// placeMenu is kakel's part of a server place's context menu in the
// file manager: Disconnect while it is connected, Connect while not.
func placeMenu(p filemanager.Place) []filemanager.PlaceItem {
	if !strings.HasPrefix(p.FS, serverFS) || p.Kind == "favourite" {
		return nil
	}
	if p.Lit {
		return []filemanager.PlaceItem{{Label: "Disconnect", ID: "disconnect"}}
	}
	return []filemanager.PlaceItem{{Label: "Connect", ID: "connect"}}
}

// placeCommand does what kakel's item id of place p asks, from file
// manager window w. It runs on a goroutine of its own.
func (a *app) placeCommand(w *filemanager.Window, p filemanager.Place, id string) {
	m := machineOfFS(p.FS)
	a.events <- func() {
		var err error
		switch id {
		case "disconnect":
			err = a.disconnectAsking(m)
		case "connect":
			err = a.dialAgainHow(m, true, func(err error) {
				if err != nil {
					w.Notify("Couldn't connect to "+a.machines.Name(m), words.UpperFirst(err.Error())+".", "warning")
				}
			})
		}
		if err != nil {
			w.Notify("Couldn't "+id+" "+a.machines.Name(m), words.UpperFirst(err.Error())+".", "warning")
		}
	}
}

// disconnectAsking closes the connection to m: at once when nothing is
// on it, and after asking when closing it ends terminals, tunnels or
// copies.
func (a *app) disconnectAsking(m machines.ID) error {
	on := a.onMachine(m)
	if len(on) == 0 {
		return a.disconnect(m)
	}
	called := a.machines.Name(m)
	a.askThen(a.ctx, Ask{
		Title: "Disconnect " + called + "?", Text: "Still open on it: " + listOf(on) + ".",
		Yes: "Disconnect", Danger: true,
	}, func(ans AskAnswered) {
		if !ans.Yes {
			return
		}
		if err := a.disconnect(m); err != nil {
			a.failed("Couldn't disconnect "+called, err.Error())
		}
	})
	return nil
}

// onMachine is what closing the connection to m would end: copies
// running to or from it, its panes, and its tunnels.
func (a *app) onMachine(m machines.ID) []string {
	var out []string
	copies := 0
	for _, r := range a.running {
		if !r.job.Progress().Done && (r.from.Of(m) || r.to.Of(m)) {
			copies++
		}
	}
	if copies > 0 {
		out = append(out, words.ManyOf(copies, "copy running", "copies running"))
	}
	panes := 0
	for _, p := range a.st.Panes {
		if !isToolKind(p.Kind) && a.filesKey(p.ID).Of(m) {
			panes++
		}
	}
	if panes > 0 {
		out = append(out, words.ManyOf(panes, "pane", "panes"))
	}
	tunnels := 0
	for _, t := range a.st.Tunnels {
		if t.Live && t.Machine.Of(m) {
			tunnels++
		}
	}
	if tunnels > 0 {
		out = append(out, words.ManyOf(tunnels, "tunnel", "tunnels"))
	}
	return out
}

// closeFileManager stops every file manager pane, as kakel ends.
func (a *app) closeFileManager() {
	for id := range a.fmPanes {
		a.dropFilePane(id)
	}
}
