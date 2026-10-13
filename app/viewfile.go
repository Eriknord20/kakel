package app

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"slices"

	"github.com/marrasen/kakel/machines"
	"github.com/marrasen/kakel/vfs"
)

// Viewed is a file shown in the viewer over a window: its bytes, or
// their start with Cut set, to show at Line, or why it could not be
// read.
type Viewed struct {
	ID uint64
	// Name is the file's name, and Path and Where where it is: the path
	// on the machine Where names.
	Name, Path, Where string
	Data              []byte
	Cut               bool
	Line              int
	Err               string
	machine           machines.ID
	win               int
}

// ViewLink asks for a link clicked in a document shown in the viewer
// to open: a web or mail address alone, as a link in a pane does.
type ViewLink struct {
	ID  uint64
	URL string
}

// mostView is how much of a file the viewer reads; a larger one shows
// its start, and says so.
const mostView = 4 << 20

// mostHops is how many links in a row the viewer follows to the file.
const mostHops = 8

// viewFile shows the file at path on a machine's files in the viewer,
// over the window in front, at line when it is past zero. The file is
// read on a goroutine, and only a file: a device or a pipe could give
// without end, or never.
func (a *app) viewFile(machine machines.ID, f vfs.FS, path string, line int) {
	path = vfs.Spelled(f, path)
	// Asked again while it is read, as by a second click: read once.
	key := string(machine) + "\x00" + path
	if a.viewing[key] {
		return
	}
	if a.viewing == nil {
		a.viewing = map[string]bool{}
	}
	a.viewing[key] = true
	a.viewed++
	where := a.machines.Name(machine)
	if machine == machines.Local {
		where = "This computer"
	}
	v := Viewed{ID: a.viewed, Name: vfs.Base(f, path), Path: path, Where: where, Line: line,
		machine: machine, win: a.frontID()}
	go func() {
		v.Data, v.Cut, v.Err = readView(f, path)
		a.events <- func() {
			delete(a.viewing, key)
			if v.Err != "" {
				// In a toast, and kept in the log past it.
				a.failed("Couldn't show "+v.Name, v.Err)
				return
			}
			a.st.Views = append(a.st.Views, v)
			// A window has opened all but the newest by now; the bytes of
			// the rest go.
			if n := len(a.st.Views); n > 2 {
				a.st.Views = slices.Delete(a.st.Views, 0, n-2)
			}
		}
	}()
}

// readView reads up to mostView bytes of the file at path on f,
// following links to it, and says whether there was more, or why it
// could not.
func readView(f vfs.FS, path string) (data []byte, cut bool, why string) {
	at := path
	for hop := 0; ; hop++ {
		e, err := f.Stat(at)
		if err != nil {
			return nil, false, err.Error()
		}
		if e.Mode&fs.ModeSymlink == 0 {
			if !e.Mode.IsRegular() {
				return nil, false, fmt.Sprintf("%s is not a file that can be shown", path)
			}
			break
		}
		if hop == mostHops {
			return nil, false, fmt.Sprintf("%s is a link to a link, %d times over", path, mostHops)
		}
		if absolute(f, e.Link) {
			at = e.Link
		} else {
			at = vfs.Join(f, vfs.Dir(f, at), e.Link)
		}
	}
	r, err := f.Open(at)
	if err != nil {
		return nil, false, err.Error()
	}
	data, err = io.ReadAll(io.LimitReader(r, mostView+1))
	if err = errors.Join(err, r.Close()); err != nil {
		return nil, false, err.Error()
	}
	if len(data) > mostView {
		return data[:mostView], true, ""
	}
	return data, false, ""
}

// absolute reports whether p is a whole path on f: from its root, or a
// drive.
func absolute(f vfs.FS, p string) bool {
	if p == "" {
		return false
	}
	if p[0] == f.Sep() || p[0] == '/' {
		return true
	}
	return len(p) >= 2 && p[1] == ':' && (p[0] >= 'a' && p[0] <= 'z' || p[0] >= 'A' && p[0] <= 'Z')
}

// viewLink opens a link clicked in the document of viewer id, from the
// machine the document is on: a web address on its own loopback goes
// through a tunnel, as a link in a pane there does.
func (a *app) viewLink(in ViewLink) {
	i := slices.IndexFunc(a.st.Views, func(v Viewed) bool { return v.ID == in.ID })
	machine := machines.Local
	if i >= 0 {
		machine = a.st.Views[i].machine
	}
	if err := a.openLink(machine, in.URL); err != nil {
		a.failed("Couldn't open the link", err.Error())
	}
}
