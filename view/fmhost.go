package view

import (
	"github.com/marrasen/gunim"
	"github.com/marrasen/gunim/filemanager"
	"github.com/marrasen/gunim/geom"
	"github.com/marrasen/gunim/input"
	"github.com/marrasen/gunim/paint"
	"github.com/marrasen/gunim/widget/dock"

	"github.com/marrasen/kakel/app"
)

// fileCommands are the file manager's commands kakel's Edit menu runs
// in a file manager pane, by kakel's command, and which the pane's own
// menus leave to it.
var fileCommands = map[string]string{
	"edit.cut":       filemanager.CmdCut,
	"edit.copy":      filemanager.CmdCopy,
	"edit.paste":     filemanager.CmdPaste,
	"edit.selectAll": filemanager.CmdSelectAll,
}

// fmHost is the place a file manager pane has in the window: the file
// manager mounts its views in it, under the ID app.FilePaneHost gives.
// It stays in the window for as long as the pane is in it: on stage, or
// parked while its tab is not showing, so the file manager keeps what it
// shows.
type fmHost struct {
	w  *Window
	id string
}

// Layout implements [gunim.Node]: the file manager fills the place.
func (h *fmHost) Layout(c gunim.Constraints, _ gunim.Frame, kids gunim.Children) geom.Size {
	for k := range kids.All {
		k.Layout(gunim.Tight(c.Max))
		k.Place(geom.Point{})
	}
	return c.Max
}

// Paint implements [gunim.Node].
func (h *fmHost) Paint(p *paint.Painter, _ gunim.Frame, _ geom.Size, kids gunim.Children) {
	for k := range kids.All {
		k.Paint(p)
	}
}

// Handle implements [gunim.Handler]: the keyboard coming into the file
// manager, as by a click, makes its pane the one in front.
func (h *fmHost) Handle(e input.Event, u *gunim.UI) bool {
	switch e := e.(type) {
	case input.FocusEntered:
		h.w.entered(h.id, u)
	case input.Drop:
		// Files dropped where the file manager takes none, as on its
		// status bar, go nowhere: not to the terminal beside it. A tab
		// or a pane goes on to the window, to dock beside this one.
		switch e.Data.(type) {
		case dock.Drag, app.PaneDrag:
			return false
		}
		return true
	}
	return false
}

// parking holds the places of the file manager panes whose tabs are not
// showing. It draws nothing, so nothing in it takes the pointer.
type parking struct{ _ byte }

// Layout implements [gunim.Node]: what is parked keeps the size it would
// have on stage.
func (*parking) Layout(c gunim.Constraints, _ gunim.Frame, kids gunim.Children) geom.Size {
	for k := range kids.All {
		k.Layout(gunim.Tight(c.Max))
		k.Place(geom.Point{})
	}
	return geom.Size{}
}

// Paint implements [gunim.Node].
func (*parking) Paint(*paint.Painter, gunim.Frame, geom.Size, gunim.Children) {}

// placeFilePanes gives each file manager pane of st a place in the
// window before the stage is built, so the file manager can mount its
// views there once the program tells it to, and takes away the places of
// those that have gone.
func (w *Window) placeFilePanes(st app.State, u *gunim.UI) {
	if w.parked == nil {
		w.parked = &parking{}
		u.InsertAt(w.stage, 0, w.parked)
	}
	live := map[string]bool{}
	for _, p := range st.Panes {
		if p.Kind != app.KindFileManager {
			continue
		}
		live[p.ID] = true
		if _, ok := w.fmHosts[p.ID]; ok {
			continue
		}
		h := &fmHost{w: w, id: p.ID}
		w.fmHosts[p.ID] = h
		u.Insert(w.parked, h)
		if err := u.SetID(h, app.FilePaneHost(p.ID)); err != nil {
			w.failed("Couldn't show the file manager", err.Error(), u)
		}
	}
	for id, h := range w.fmHosts {
		if !live[id] {
			u.Remove(h)
			delete(w.fmHosts, id)
		}
	}
}

// parkFilePanes parks the places of the file manager panes the stage
// does not show, once it is built: on, the panes it shows.
func (w *Window) parkFilePanes(on map[string]bool, u *gunim.UI) {
	for id, h := range w.fmHosts {
		if !on[id] {
			u.Insert(w.parked, h)
		}
	}
}

// leavesOf adds the panes b shows to on.
func leavesOf(b *app.Box, on map[string]bool) map[string]bool {
	if b == nil {
		return on
	}
	if b.Pane != "" {
		on[b.Pane] = true
	}
	leavesOf(b.A, on)
	leavesOf(b.B, on)
	return on
}
