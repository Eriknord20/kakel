package view

import (
	"slices"

	"github.com/marrasen/kakel/app"
	"github.com/marrasen/kakel/look"

	"github.com/marrasen/gunim"
	"github.com/marrasen/gunim/anim"
	"github.com/marrasen/gunim/geom"
	"github.com/marrasen/gunim/input"
	"github.com/marrasen/gunim/paint"
	"github.com/marrasen/gunim/text"
	"github.com/marrasen/gunim/widget"
)

// buttonBar is a strip along a pane: a line of text at the start, and
// the buttons that fit what the pane shows now at the end.
type buttonBar struct {
	label *widget.Label
	// shown are the buttons in the bar now.
	shown []gunim.Node
}

func newButtonBar() *buttonBar {
	b := &buttonBar{label: widget.NewLabel("")}
	b.label.Size, b.label.Color, b.label.MaxLines = smallText, look.Faint, 1
	return b
}

// set says text, beside buttons: those new to the bar arrive, and
// those it had and has no more go.
func (b *buttonBar) set(text string, u *gunim.UI, buttons ...*widget.Button) {
	b.label.Text = text
	want := make([]gunim.Node, len(buttons))
	for i, x := range buttons {
		want[i] = x
	}
	for _, n := range b.shown {
		if !slices.Contains(want, n) {
			u.Remove(n)
		}
	}
	for _, n := range want {
		if !slices.Contains(b.shown, n) {
			u.Insert(b, n)
		}
	}
	b.shown = want
}

// Children implements [gunim.Composite].
func (b *buttonBar) Children() []gunim.Node { return append([]gunim.Node{b.label}, b.shown...) }

// Layout implements [gunim.Node]: the label at the start, the buttons
// at the end, in the order they were given.
func (b *buttonBar) Layout(c gunim.Constraints, _ gunim.Frame, kids gunim.Children) geom.Size {
	const padX, gap, height = 12, 8, 44
	// The children arrive in the order they were inserted, which is
	// not the order asked for; each is placed by its place in shown.
	sizes := map[gunim.Node]geom.Size{}
	byNode := map[gunim.Node]gunim.Child{}
	for i := 1; i < kids.Len(); i++ {
		k := kids.At(i)
		byNode[k.Node()] = k
		sizes[k.Node()] = k.Layout(gunim.Constraints{Max: geom.Sz(c.Max.W, height)})
	}
	x := c.Max.W - padX
	for i := len(b.shown) - 1; i >= 0; i-- {
		k, ok := byNode[b.shown[i]]
		if !ok {
			continue
		}
		s := sizes[b.shown[i]]
		x -= s.W
		k.Place(geom.Pt(x, (height-s.H)/2))
		x -= gap
	}
	k := kids.At(0)
	s := k.Layout(gunim.Constraints{Max: geom.Sz(max(0, x-padX), height)})
	k.Place(geom.Pt(padX, (height-s.H)/2))
	return c.Constrain(geom.Sz(c.Max.W, height))
}

// Paint implements [gunim.Node].
func (b *buttonBar) Paint(p *paint.Painter, f gunim.Frame, box geom.Size, kids gunim.Children) {
	p.RRect(geom.Rect{Max: box.Point()}, 0, paint.Solid(widget.MenuFill.Get(f.Theme)))
	p.RRect(geom.Rect{Max: geom.Pt(box.W, 1)}, 0, paint.Solid(widget.MenuBorder.Get(f.Theme)))
	for k := range kids.All {
		k.Paint(p)
	}
}

// captioned is a pane under a line naming it: in a split, where the
// line is what the pane is dragged by, and in every pane of a window
// that shows pane titles.
type captioned struct {
	pane gunim.Node
	bar  *captionBar
}

func newCaptioned(w *Window, id string, pane gunim.Node) *captioned {
	l := widget.NewLabel("")
	l.Size, l.Color, l.MaxLines = smallText, look.Faint, 1
	return &captioned{pane: pane, bar: &captionBar{w: w, id: id, label: l}}
}

// captionHeight is the height of the line over a pane.
const captionHeight = 22

// Children implements [gunim.Composite].
func (c *captioned) Children() []gunim.Node { return []gunim.Node{c.bar, c.pane} }

// Layout implements [gunim.Node]: the line along the top, and the pane
// in the rest.
func (c *captioned) Layout(cs gunim.Constraints, _ gunim.Frame, kids gunim.Children) geom.Size {
	for k := range kids.All {
		if k.Node() == c.bar {
			k.Layout(gunim.Tight(geom.Sz(cs.Max.W, min(captionHeight, cs.Max.H))))
			k.Place(geom.Point{})
			continue
		}
		k.Layout(gunim.Tight(geom.Sz(cs.Max.W, max(0, cs.Max.H-captionHeight))))
		k.Place(geom.Pt(0, captionHeight))
	}
	return cs.Max
}

// Paint implements [gunim.Node].
func (c *captioned) Paint(p *paint.Painter, _ gunim.Frame, _ geom.Size, kids gunim.Children) {
	for k := range kids.All {
		k.Paint(p)
	}
}

// captionBar is the line over a pane, naming it. A click on it gives
// the pane the keyboard, a middle click closes the pane, as it does a
// tab, and a drag carries the pane off: beside another pane, onto a tab
// of its own, into another window, or out of them all into a window of
// its own.
type captionBar struct {
	w     *Window
	id    string
	label *widget.Label
	// front says the pane has the keyboard, which its line shows.
	front bool
	// pressed says the primary button went down on the line, at
	// pressAt; carried that the pane is being dragged off, held at grab
	// in the image that follows the pointer.
	pressed, carried bool
	pressAt, grab    geom.Point
}

// captionGhost is how wide the image of a pane carried off by its line
// is at most.
const captionGhost = 220

// Children implements [gunim.Composite].
func (b *captionBar) Children() []gunim.Node { return []gunim.Node{b.label} }

// Layout implements [gunim.Node].
func (b *captionBar) Layout(cs gunim.Constraints, _ gunim.Frame, kids gunim.Children) geom.Size {
	const padX = 10
	l := kids.At(0)
	s := l.Layout(gunim.Constraints{Max: geom.Sz(max(0, cs.Max.W-2*padX), cs.Max.H)})
	l.Place(geom.Pt(padX, (cs.Max.H-s.H)/2))
	return cs.Max
}

// Paint implements [gunim.Node]: the line, with the pane that has the
// keyboard marked along its bottom.
func (b *captionBar) Paint(p *paint.Painter, f gunim.Frame, box geom.Size, kids gunim.Children) {
	p.RRect(geom.Rect{Max: box.Point()}, 0, paint.Solid(widget.MenuFill.Get(f.Theme)))
	if b.front {
		p.RRect(geom.Rc(0, box.H-2, box.W, 2), 0, paint.Solid(widget.Accent.Get(f.Theme)))
	}
	for k := range kids.All {
		k.Paint(p)
	}
}

// setFront marks the line of the pane with the keyboard.
func (b *captionBar) setFront(on bool) {
	b.front = on
	b.label.Color = look.Faint
	if on {
		b.label.Color = widget.Ink
	}
}

// Cursor implements [gunim.CursorShaper]: the line moves the pane.
func (b *captionBar) Cursor(geom.Point) input.Cursor { return input.CursorMove }

// Handle implements [gunim.Handler].
func (b *captionBar) Handle(e input.Event, u *gunim.UI) bool {
	switch e := e.(type) {
	case input.PointerDown:
		if e.Button == input.ButtonMiddle {
			u.Send(b.w, app.ClosePane{Pane: b.id})
			return true
		}
		if e.Button != input.ButtonPrimary {
			return false
		}
		b.pressed, b.pressAt = true, e.Pos
		return true
	case input.PointerMove:
		if b.pressed && !b.carried && moved(e.Pos, b.pressAt) {
			b.lift(u)
		}
		return b.pressed
	case input.PointerUp:
		if !b.pressed {
			return false
		}
		b.pressed = false
		if !b.carried {
			u.Send(b.w, app.FocusPane{Pane: b.id})
		}
		return true
	case input.DragEnd:
		b.dragEnded(e, u)
		return true
	}
	return false
}

// lift carries the pane off, as an image of its line that follows the
// pointer.
func (b *captionBar) lift(u *gunim.UI) {
	b.carried = true
	size := geom.Sz(min(captionGhost, max(b.w.size.W/4, 120)), widget.MenubarHeight.Get(u.Theme())-6)
	b.grab = geom.Pt(min(b.pressAt.X, size.W-tabPad), size.H/2)
	title := ""
	for _, p := range b.w.panes {
		if p.ID == b.id {
			title = p.Title
		}
	}
	g := &tabGhost{title: text.Default().Shape(title, smallText.Get(u.Theme())), kind: b.w.paneIcon(b.id, b.w.kindOf(b.id)), size: size, lit: anim.NewFloat(0)}
	g.Add(g.lit)
	u.StartDrag(b, app.PaneDrag{Pane: b.id, Window: b.w.winID, Grab: b.grab}, g, b.grab)
	u.Invalidate()
}

// dragEnded hears how the drag of the pane ended: taken, which the next
// state shows; let go outside every window, which opens one for it,
// unless it is its window's only pane; or neither, and it stays.
func (b *captionBar) dragEnded(e input.DragEnd, u *gunim.UI) {
	carried := b.carried
	b.pressed, b.carried = false, false
	if !carried || e.Taken || !e.Out || len(b.w.panes) < 2 {
		return
	}
	// The window as large as this one, its top left corner where the
	// image's was.
	u.Send(b.w, app.PaneToNewWindow{Pane: b.id, At: e.At.Sub(b.grab), Size: b.w.size})
}

// The measures of a pane's line dragged out of its caption, drawn as a
// tab: the room each side of its title, its icon, and how far below the
// top of the title bar a tab starts.
const (
	tabPad  = 10
	tabIcon = 16
	tabTop  = 4
)

// tabGhost is the image of a pane's line dragged out of its caption, as
// a tab, ringed while it is over something that takes it.
type tabGhost struct {
	anim.Group
	title text.Run
	kind  string
	size  geom.Size
	lit   *anim.Float
}

// Layout implements [gunim.Node].
func (g *tabGhost) Layout(gunim.Constraints, gunim.Frame, gunim.Children) geom.Size { return g.size }

// Paint implements [gunim.Node].
func (g *tabGhost) Paint(p *paint.Painter, f gunim.Frame, box geom.Size, _ gunim.Children) {
	th := f.Theme
	r := geom.Rect{Max: box.Point()}
	p.RRect(r, 6, paint.Solid(look.SidebarFill.Get(th)))
	p.RRect(r, 6, paint.Solid(look.RowActive.Get(th)))
	if on := min(max(g.lit.Value(), 0), 1); on > 0.01 {
		c := switcherRing.Get(th)
		c.A = uint8(float32(c.A) * on)
		p.RRectStroke(r.Inset(geom.Uniform(1)), 6, paint.Fill{}, paint.Stroke{Width: 2, Color: c})
	}
	ink := widget.Ink.Get(th)
	mid := box.H / 2
	paintIcon(p, g.kind, geom.Rc(tabPad, mid-tabIcon/2, tabIcon, tabIcon), ink)
	x := float32(tabPad + tabIcon + 6)
	defer p.Layer(paint.LayerOpts{Bounds: geom.Rc(x, 0, box.W-x-tabPad, box.H), Opacity: 1, Clip: true})()
	g.title.Paint(p, geom.Pt(x, mid-g.title.Height()/2), ink)
}

// Handle implements [gunim.Handler]: the node under the pointer says
// whether it takes the tab.
func (g *tabGhost) Handle(e input.Event, u *gunim.UI) bool {
	if a, ok := e.(input.DragAnswer); ok {
		to := float32(0)
		if a.Answer != nil {
			to = 1
		}
		g.lit.Animate(to, widget.Quick.Get(u.Theme()))
		u.Invalidate()
		return true
	}
	return false
}
