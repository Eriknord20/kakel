package view

import (
	"math"
	"time"

	"github.com/marrasen/kakel/app"

	"github.com/marrasen/gunim"
	"github.com/marrasen/gunim/anim"
	"github.com/marrasen/gunim/geom"
	"github.com/marrasen/gunim/input"
	"github.com/marrasen/gunim/paint"
	"github.com/marrasen/gunim/text"
	"github.com/marrasen/gunim/widget"
)

// Dragging a pane out of the switcher, to another of kakel's windows or
// out of them all. The tile lifts off and follows the pointer as a
// image of the pane; a window it is over lights up, saying it takes
// it. Let go there, the pane moves in; let go over no window, it opens
// a window of its own where it was let go.

// dragSlop is how far the pointer goes, pressed on a tile, before the
// pane lifts off rather than being picked.
const dragSlop = 6

// moved reports whether p has gone further than dragSlop from from.
func moved(p, from geom.Point) bool {
	d := p.Sub(from)
	return math.Hypot(float64(d.X), float64(d.Y)) > dragSlop
}

// movesHere is what a window answers a pane dragged over it: a drop
// there moves the pane in.
const movesHere = "moves here"

// carry lifts the tile pressed off the switcher, to follow the pointer
// at at.
func (s *switcher) carry(at geom.Point, u *gunim.UI) {
	t := s.pressed
	s.carried = t
	s.dropped = false
	r := t.box.Value()
	s.grab = at.Sub(r.Min)
	t.fade.Animate(0.25, widget.Quick.Get(u.Theme()))
	g := &paneGhost{s: s, t: t, size: r.Size(), lit: anim.NewFloat(0), thin: anim.NewFloat(0)}
	g.Add(g.lit, g.thin)
	u.StartDrag(s, app.PaneDrag{Pane: t.id, Window: t.win, Grab: s.grab}, g, s.grab)
	u.Invalidate()
}

// dragEnded hears how the drag of a tile ended: let go over the
// switcher, which asked for the pane to go where it was let go; taken
// by another window, which has the pane now; let go outside every
// window, which opens one for it; or none of these, and the tile goes
// back to its place. Wherever the pane goes, the switcher shows it there
// once the program says it is.
func (s *switcher) dragEnded(e input.DragEnd, u *gunim.UI) {
	t := s.carried
	s.pressed, s.carried = nil, nil
	s.drop = overDrop{}
	s.lit.Animate(0, widget.Quick.Get(u.Theme()))
	for _, c := range s.cards {
		c.lit.Animate(0, widget.Quick.Get(u.Theme()))
	}
	if t == nil {
		return
	}
	switch {
	case s.dropped:
		// It goes from where it was let go to where it lands, once the
		// program says where that is.
		t.box.Jump(geom.Rect{Min: e.At.Sub(s.grab), Max: e.At.Sub(s.grab).Add(t.box.Value().Size().Point())})
		t.heldUntil = time.Now().Add(holdDrop)
	case e.Taken:
	case e.Out && s.alone(t) == "":
		// The pane's top left corner where the image's was.
		u.Send(s, app.PaneToNewWindow{Pane: t.id, At: e.At.Sub(s.grab), Size: s.windowSize(t)})
	}
	s.dropped = false
	t.fade.Animate(1, widget.Settle.Get(u.Theme()))
	u.Invalidate()
}

// holdDrop is how long a tile let go over the switcher waits where it
// was let go for the program to say where it went.
const holdDrop = 700 * time.Millisecond

// windowSize is the size of a window of its own for tile t's pane: as
// large as the window it is in, or as the switcher.
func (s *switcher) windowSize(t *tile) geom.Size {
	for _, ow := range s.wins {
		if ow.ID == t.win && ow.Size.W > 0 && ow.Size.H > 0 {
			return ow.Size
		}
	}
	return s.size
}

// alone says why tile t's pane cannot leave its window for one of its
// own: it is the window's only pane, which is where it is wanted
// already. It is "" when it can.
func (s *switcher) alone(t *tile) string {
	for _, o := range s.tiles {
		if o != t && o.win == t.win {
			return ""
		}
	}
	return "its window's only pane"
}

// overDrop is where a pane carried over the switcher would go, let go
// there.
type overDrop struct {
	kind dropKind
	// beside, vertical and first say the pane it would join in a split,
	// and on which side, which is lit at lit.
	beside          string
	vertical, first bool
	lit             geom.Rect
	// card is the window's card it would move to.
	card *card
}

// dropKind is what a drop over the switcher does.
type dropKind uint8

// What a drop does.
const (
	// dropNone leaves the pane where it is.
	dropNone dropKind = iota
	// dropDock joins it with the pane under it, in a split.
	dropDock
	// dropTab moves it onto a tab of its own, in the window of the card
	// under it.
	dropTab
	// dropWindow opens a window of its own for it, where it is let go.
	dropWindow
)

// opensWindow is what the switcher answers a pane dragged over room
// with no window's card: a drop there opens one for it.
const opensWindow = "opens a window"

// dropAt is where pane d, carried to p, would go.
func (s *switcher) dropAt(p geom.Point, d app.PaneDrag) overDrop {
	carried := s.tileOf(d.Pane)
	if i := s.tileAt(p); i >= 0 {
		t := s.tiles[i]
		if t.id == d.Pane {
			return overDrop{}
		}
		r := t.box.Value()
		sz := r.Size()
		if sz.W <= 0 || sz.H <= 0 {
			return overDrop{}
		}
		fx, fy := (p.X-r.Min.X)/sz.W, (p.Y-r.Min.Y)/sz.H
		dk := overDrop{kind: dropDock, beside: t.id}
		switch min(fx, 1-fx, fy, 1-fy) {
		case fx:
			dk.first, dk.lit = true, geom.Rc(r.Min.X, r.Min.Y, sz.W/2, sz.H)
		case 1 - fx:
			dk.lit = geom.Rc(r.Min.X+sz.W/2, r.Min.Y, sz.W/2, sz.H)
		case fy:
			dk.vertical, dk.first, dk.lit = true, true, geom.Rc(r.Min.X, r.Min.Y, sz.W, sz.H/2)
		default:
			dk.vertical, dk.lit = true, geom.Rc(r.Min.X, r.Min.Y+sz.H/2, sz.W, sz.H/2)
		}
		return dk
	}
	if c := s.cardAt(p); c != nil {
		// Into another window, or out of its split onto a tab of its own
		// in its own.
		if carried == nil || c.win != carried.win || s.inSplit(carried) {
			return overDrop{kind: dropTab, card: c}
		}
		return overDrop{}
	}
	if carried == nil || s.alone(carried) != "" {
		return overDrop{}
	}
	// Where the window would open, as large as it would be.
	at := p.Sub(d.Grab)
	return overDrop{kind: dropWindow, lit: geom.Rect{Min: at, Max: at.Add(s.windowSize(carried).Point())}}
}

// tileOf returns pane id's tile, or nil.
func (s *switcher) tileOf(id string) *tile {
	for _, t := range s.tiles {
		if t.id == id {
			return t
		}
	}
	return nil
}

// inSplit reports whether tile t's pane shares its tab with another.
func (s *switcher) inSplit(t *tile) bool {
	for _, o := range s.tiles {
		if o != t && o.win == t.win && o.group == t.group {
			return true
		}
	}
	return false
}

// dropHere takes a pane dragged over the switcher, its own or another
// window's: it lights where it would go, and a drop sends it there.
func (s *switcher) dropHere(e input.Event, u *gunim.UI) bool {
	quick := widget.Quick.Get(u.Theme())
	switch e := e.(type) {
	case input.DragOver:
		d, ok := e.Data.(app.PaneDrag)
		if !ok || s.picked >= 0 {
			return false
		}
		s.showDrop(s.dropAt(e.Pos, d), u)
		switch s.drop.kind {
		case dropDock:
			u.AnswerDrag(docksHere)
		case dropTab:
			u.AnswerDrag(movesHere)
		case dropWindow:
			u.AnswerDrag(opensWindow)
		case dropNone:
		}
		return true
	case input.DragLeave:
		s.showDrop(overDrop{}, u)
		return false
	case input.Drop:
		d, ok := e.Data.(app.PaneDrag)
		if !ok || s.picked >= 0 {
			return false
		}
		drop := s.dropAt(e.Pos, d)
		s.showDrop(overDrop{}, u)
		s.lit.Animate(0, quick)
		switch drop.kind {
		case dropDock:
			u.Send(s, app.DockPane{Pane: d.Pane, Beside: drop.beside, Vertical: drop.vertical, First: drop.first})
		case dropTab:
			u.Send(s, app.PaneToTab{Pane: d.Pane, Window: drop.card.win})
		case dropWindow:
			size := s.size
			if t := s.tileOf(d.Pane); t != nil {
				size = s.windowSize(t)
			}
			u.Send(s, app.PaneToNewWindow{Pane: d.Pane, At: e.Pos.Sub(d.Grab), Size: size})
		case dropNone:
			return true
		}
		s.dropped = true
		return true
	}
	return false
}

// showDrop lights where d says a pane let go would go.
func (s *switcher) showDrop(d overDrop, u *gunim.UI) {
	quick := widget.Quick.Get(u.Theme())
	lights := d.kind == dropDock || d.kind == dropWindow
	if lights {
		if s.drop.kind != d.kind || s.drop.beside != d.beside {
			s.lit.Jump(0)
		}
		s.lit.Animate(1, quick)
	} else {
		s.lit.Animate(0, quick)
	}
	for _, c := range s.cards {
		to := float32(0)
		if d.kind == dropTab && d.card == c {
			to = 1
		}
		c.lit.Animate(to, quick)
	}
	if lights || s.drop.kind != dropDock && s.drop.kind != dropWindow {
		s.drop = d
	} else {
		// What was lit fades where it was.
		s.drop.kind = dropNone
	}
	u.Invalidate()
}

// paintDrop lights the half of a pane a pane carried over it would
// take, or the window it would open in, where it would open.
func (s *switcher) paintDrop(p *paint.Painter, f gunim.Frame) {
	on := min(max(s.lit.Value(), 0), 1)
	if on < 0.01 || s.drop.lit.Empty() {
		return
	}
	c := switcherRing.Get(f.Theme)
	tint := c
	tint.A = uint8(0x44 * on)
	c.A = uint8(float32(c.A) * on)
	r := s.drop.lit.Inset(geom.Uniform(2))
	if s.drop.kind == dropWindow || s.drop.beside == "" {
		// A window to be: faint, rounded, named.
		tint.A = uint8(0x18 * on)
		p.RRect(r, 10, paint.Solid(tint))
		p.RRectStroke(r, 10, paint.Fill{}, paint.Stroke{Width: 2, Color: c})
		name := text.Default().Shape("New window", 14)
		ink := widget.Ink.Get(f.Theme)
		ink.A = uint8(float32(ink.A) * on)
		// In the middle of what of it shows.
		seen := r
		seen.Min = geom.Pt(max(seen.Min.X, 0), max(seen.Min.Y, 0))
		seen.Max = geom.Pt(min(seen.Max.X, s.size.W), min(seen.Max.Y, s.size.H))
		c := seen.Center()
		name.Paint(p, geom.Pt(c.X-name.Advance/2, c.Y-name.Height()/2), ink)
		return
	}
	p.RRect(r, 4, paint.Solid(tint))
	p.RRectStroke(r, 4, paint.Fill{}, paint.Stroke{Width: 2, Color: c})
}

// paneGhost is the image of a pane that follows the pointer while it
// is dragged, ringed while it is over a window that takes it.
type paneGhost struct {
	anim.Group
	s    *switcher
	t    *tile
	size geom.Size
	lit  *anim.Float
	// thin is how far it fades, over a place in the overview it would
	// go, so what lights there shows through it.
	thin *anim.Float
}

// Layout implements [gunim.Node].
func (g *paneGhost) Layout(gunim.Constraints, gunim.Frame, gunim.Children) geom.Size { return g.size }

// Paint implements [gunim.Node].
func (g *paneGhost) Paint(p *paint.Painter, f gunim.Frame, box geom.Size, _ gunim.Children) {
	g.s.paintPane(p, f, g.t, geom.Rect{Max: box.Point()}, 0.92-0.4*min(max(g.thin.Value(), 0), 1), 1, g.lit.Value())
}

// Handle implements [gunim.Handler]: the window under the pointer says
// whether it takes the pane.
func (g *paneGhost) Handle(e input.Event, u *gunim.UI) bool {
	if a, ok := e.(input.DragAnswer); ok {
		to, thin := float32(0), float32(0)
		switch a.Answer.(type) {
		case widget.DropHint:
			// The tab bar's answer: the pane goes onto a tab of its own.
			to = 1
		}
		switch a.Answer {
		case movesHere:
			to = 1
		case docksHere, opensWindow:
			thin = 1
		}
		g.lit.Animate(to, widget.Quick.Get(u.Theme()))
		g.thin.Animate(thin, widget.Quick.Get(u.Theme()))
		u.Invalidate()
		return true
	}
	return false
}

// paneDrop takes a pane dragged over the window, by its line or out of
// another window's switcher. Over a pane on stage, the half it would
// take lights, and a drop docks it there. Over the room above the stage,
// a pane of this window's split would go onto a tab of its own. From
// another window, anywhere else, the window lights up, and a drop moves
// the pane in.
func (w *Window) paneDrop(e input.Event, u *gunim.UI) bool {
	switch e := e.(type) {
	case input.DragOver:
		d, ok := e.Data.(app.PaneDrag)
		if !ok {
			return false
		}
		w.dock = w.paneDockAt(e.Pos, d, u)
		switch {
		case w.dock.pane != "":
			u.AnswerDrag(docksHere)
			w.dropLit.Animate(0, widget.Quick.Get(u.Theme()))
		case w.dock.toTab:
			u.AnswerDrag(movesHere)
		case d.Window != w.winID:
			u.AnswerDrag(movesHere)
			w.dropLit.Animate(1, widget.Quick.Get(u.Theme()))
		default:
			u.Invalidate()
			return false
		}
		u.Invalidate()
		return true
	case input.DragLeave:
		w.dock = tabDock{}
		w.dropLit.Animate(0, widget.Quick.Get(u.Theme()))
		u.Invalidate()
		return false
	case input.Drop:
		d, ok := e.Data.(app.PaneDrag)
		if !ok {
			return false
		}
		dock := w.paneDockAt(e.Pos, d, u)
		w.dock = tabDock{}
		w.dropLit.Animate(0, widget.Settle.Get(u.Theme()))
		u.Invalidate()
		switch {
		case dock.pane != "":
			u.Send(w, app.DockPane{Pane: d.Pane, Beside: dock.pane, Vertical: dock.vertical, First: dock.first})
		case dock.toTab:
			u.Send(w, app.PaneToTab{Pane: d.Pane, Window: w.winID, Bar: true})
		case d.Window != w.winID:
			u.Send(w, app.PaneToWindow{Pane: d.Pane})
		default:
			return false
		}
		return true
	}
	return false
}

// paneDockAt is where pane d, dragged to p, would go: beside the pane on
// stage under p, or, above the stage, out of its split onto a tab of
// its own.
func (w *Window) paneDockAt(p geom.Point, d app.PaneDrag, u *gunim.UI) tabDock {
	if dk := w.dockOver(p, u, func(id string) bool { return id == d.Pane }); dk.pane != "" {
		return dk
	}
	r, ok := u.Bounds(w.stage)
	if !ok || p.Y >= r.Min.Y || d.Window != w.winID || len(boxLeaves(w.groups[d.Pane], nil)) < 2 {
		return tabDock{}
	}
	return tabDock{toTab: true, lit: geom.Rc(0, 0, w.size.W, r.Min.Y).Inset(geom.Insets{Left: -4, Right: -4, Top: -4})}
}

// paintDropLit rings the window, and tints it, while a pane from
// another window is over it.
func (w *Window) paintDropLit(p *paint.Painter, f gunim.Frame, box geom.Size) {
	on := min(max(w.dropLit.Value(), 0), 1)
	if on < 0.01 {
		return
	}
	c := switcherRing.Get(f.Theme)
	tint := c
	tint.A = uint8(0x22 * on)
	c.A = uint8(float32(c.A) * on)
	r := geom.Rect{Max: box.Point()}
	p.RRect(r, 0, paint.Solid(tint))
	p.RRectStroke(r.Inset(geom.Uniform(2)), 6, paint.Fill{}, paint.Stroke{Width: 3, Color: c})
}
