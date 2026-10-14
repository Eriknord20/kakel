package view

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/marrasen/kakel/app"
	"github.com/marrasen/kakel/look"

	"github.com/marrasen/gunim"
	"github.com/marrasen/gunim/access"
	"github.com/marrasen/gunim/anim"
	"github.com/marrasen/gunim/geom"
	"github.com/marrasen/gunim/icon"
	"github.com/marrasen/gunim/input"
	"github.com/marrasen/gunim/paint"
	"github.com/marrasen/gunim/text"
	"github.com/marrasen/gunim/theme"
	"github.com/marrasen/gunim/widget"
)

// The tab bar, in the title bar after the menu button, while the window
// has two tabs or more. A click shows a tab; a middle click or its ×
// closes it; the + opens a terminal in a tab of its own. A tab dragged
// along the bar moves along it, onto another window's bar moves there,
// onto a pane joins it in a split, and let go outside every window
// opens a window of its own. The bar's empty room moves the window, as
// the rest of the title bar does.

// tabBar shows the window's tabs.
type tabBar struct {
	anim.Group
	w     *Window
	tabs  []app.Tab
	front int
	// titles are the tabs' titles, shaped, and more how many other
	// panes each tab holds, shaped too. names are the titles as words,
	// and shorts what each tab says where its title does not fit, ""
	// for nothing shorter: a server's folder without the server.
	titles []text.Run
	more   []text.Run
	names  []string
	shorts []text.Run
	// cut says which tabs showed less than their title when last drawn,
	// and so show it all in a tooltip under the pointer.
	cut []bool
	tip widget.PartTip
	// boxes are where the tabs are, plus where the + is, and end where
	// the tabs and the + end.
	boxes []geom.Rect
	plus  geom.Rect
	// plusMenu is the menu a right click on the + opens, while it is.
	plusMenu *gunim.Popup
	end      float32
	// hot is the tab under the pointer, and pressed the one the pointer
	// went down on, at pressAt, on its × with onCross; -1 for none.
	hot, pressed int
	pressAt      geom.Point
	pressedCross bool
	// hotAt is where the pointer was last seen over the bar, and
	// crossHot says it was on the hot tab's ×. plusPressed says the
	// pointer went down on the +.
	hotAt       geom.Point
	crossHot    bool
	plusPressed bool
	// shaped are the titles and the text size the runs were shaped
	// from, to shape them again only when those change.
	shaped     []string
	shapedSize float32
	// carried is the group dragged out of the bar, 0 for none, grabbed
	// at grab from its tab's top left corner.
	carried int
	grab    geom.Point
	// landing is where a tab dragged over the bar would land: before the
	// tab at that index, len(tabs) for last, and -1 while none is over.
	landing int
	// spring is the group of the tab a drag is held over, which comes to
	// the front after springHold, so what is dragged can dock in it;
	// springRun numbers the wait for it, so one the drag moved on from
	// does nothing. sprungFrom is the tab in front before a drag brought
	// another forward, 0 for none, to go back to if the tab dragged is
	// let go where nothing takes it.
	spring, springRun, sprungFrom int
	// moves are how the tabs move, by group: boxes is where the layout
	// puts them, and a tab slides there, grows in as it opens, and
	// shrinks away as it closes, in gone. arriving are the groups new
	// on the bar, which grow in rather than appear; plusAt is where the
	// + is drawn, sliding after the tabs.
	moves    map[int]*tabMove
	arriving map[int]bool
	gone     []tabGone
	plusAt   *anim.Rect
	// laid says the title row is laid out for the tabs, which it is
	// from the second tab on, and stays while the last tab but one
	// fades away. fade is how far the tabs show, and titleFade how far
	// the window's title after the menu button does: the one fades out
	// before the row changes, and the other in after.
	laid            bool
	fade, titleFade *anim.Float
	// lit is how far the front tab's look has come from the tab that
	// was in front, was, to the one in front now: its fill glides from
	// litFrom, where it was drawn, and the titles' ink fades across.
	lit     *anim.Float
	litFrom geom.Rect
	was     int
	// dropped is the group whose drag ended in a drop, or out of the
	// window, until the tabs change for it: the tab in front changing
	// with them is the drag's doing, and shows no switch.
	dropped int
}

// tabMove is how a tab moves on the bar: its box, sliding to where the
// layout puts it, and how far it is open, 0 to 1, which its look fades
// with.
type tabMove struct {
	box  *anim.Rect
	open *anim.Float
}

// step advances the tab's motion, and reports whether it moves still.
func (m *tabMove) step(dt time.Duration) bool {
	moving := m.box.Step(dt)
	return m.open.Step(dt) || moving
}

// tabGone is a closed tab, drawn as it was while it shrinks away.
type tabGone struct {
	*tabMove
	tab         app.Tab
	title, more text.Run
	kind        string
}

// tabSlide is how the tabs slide, open and close.
var tabSlide = anim.Spring{Response: 0.28, Damping: 1}

func newTabBar(w *Window) *tabBar {
	b := &tabBar{w: w, hot: -1, pressed: -1, landing: -1, moves: map[int]*tabMove{}, arriving: map[int]bool{},
		fade: anim.NewFloat(1), titleFade: anim.NewFloat(1), lit: anim.NewFloat(1)}
	b.Add(b.fade, b.titleFade, b.lit)
	return b
}

// tabGlide is how the front tab's look moves to the tab picked: about
// as quickly as the stage clears for it.
var tabGlide = anim.Tween{Duration: 160 * time.Millisecond, Ease: anim.EaseOut}

// rowFadeOut and rowFadeIn are how the tabs and the window's title fade
// as the title row changes between them: the one going quickly, the
// other coming a little slower.
var (
	rowFadeOut = anim.Tween{Duration: 110 * time.Millisecond, Ease: anim.EaseInOut}
	rowFadeIn  = anim.Tween{Duration: 180 * time.Millisecond, Ease: anim.EaseInOut}
)

// The tabs' measures: the room each side of a title, the widest and
// narrowest tab, the room a tab's icon and × take, and the gap between
// tabs.
const (
	tabPad    = 10
	tabWidest = 220
	tabLeast  = 72
	tabIcon   = 16
	tabCross  = 16
	tabGap    = 2
	// tabTop is how far below the top of the bar a tab starts.
	tabTop = 4
	// tabPlus is the room the + takes, and captionLeast the room the
	// bar always leaves after it to move the window by.
	tabPlus      = 32
	captionLeast = 64
)

// shown reports whether the bar has the title row's room and shows,
// which it does with two tabs or more, and as the last but one fades.
func (b *tabBar) shown() bool { return b.laid }

// wanted reports whether the window has tabs to show: two or more.
func (b *tabBar) wanted() bool { return len(b.tabs) > 1 }

// show puts the window's tabs on the bar, and reports whether the tab
// in front changed in a way the window shows in motion, and from which
// side the tab picked comes, as switchOf says.
func (b *tabBar) show(tabs []app.Tab, focus string, u *gunim.UI) (from float32, switched bool) {
	front := 0
	for _, t := range tabs {
		// A group's panes share its arrangement.
		if g := b.w.groups[t.Pane]; g != nil && g == b.w.groups[focus] {
			front = t.Group
		}
	}
	titles := make([]string, 0, 3*len(tabs))
	for _, t := range tabs {
		more := ""
		if t.Panes > 1 {
			more = "+" + strconv.Itoa(t.Panes-1)
		}
		titles = append(titles, b.w.tabTitle(t), b.w.tabShort(t), more)
	}
	size := smallText.Get(u.Theme()) + 1
	if slices.Equal(tabs, b.tabs) && front == b.front && slices.Equal(titles, b.shaped) && size == b.shapedSize {
		return 0, false
	}
	if len(tabs) != len(b.tabs) {
		// What the pointer was on may be another tab now.
		b.hot, b.crossHot = -1, false
		if b.carried == 0 {
			b.pressed = -1
		}
	}
	from, switched = b.switchOf(tabs, front)
	if switched {
		b.litFrom, b.was = b.litAt(), b.front
		b.lit.Jump(0)
		b.lit.Animate(1, tabGlide)
	}
	if !slices.Equal(tabs, b.tabs) {
		b.dropped = 0
	}
	b.tabsMoved(tabs)
	b.tabs, b.front = tabs, front
	if !slices.Equal(titles, b.shaped) || size != b.shapedSize {
		b.shaped, b.shapedSize = titles, size
		b.titles, b.more, b.names, b.shorts = b.titles[:0], b.more[:0], b.names[:0], b.shorts[:0]
		for i := 0; i < len(titles); i += 3 {
			b.titles = append(b.titles, text.Default().Shape(titles[i], size))
			b.names = append(b.names, titles[i])
			b.shorts = append(b.shorts, text.Default().Shape(titles[i+1], size))
			b.more = append(b.more, text.Default().Shape(titles[i+2], size-1))
		}
		b.cut = make([]bool, len(tabs))
	}
	if !b.wanted() {
		// Going, nothing on it is pressed or lit; a tab carried away
		// still hears how its drag ends.
		b.hot, b.pressed, b.landing, b.crossHot, b.plusPressed = -1, -1, -1, false, false
	}
	u.Invalidate()
	return from, switched
}

// switchOf reports whether the tab in front changing to front, as the
// bar comes to show tabs, is a switch to show in motion, and from which
// side the tab picked comes: -1 the left, 1 the right, 0 neither. A
// window's first tabs are simply there, and so is a tab in front while
// a tab is dragged, or after one was dragged away.
func (b *tabBar) switchOf(tabs []app.Tab, front int) (float32, bool) {
	if front == b.front || front == 0 || b.front == 0 || len(b.tabs) == 0 || b.carried != 0 ||
		b.dropped != 0 && !slices.Equal(tabs, b.tabs) {
		return 0, false
	}
	// Where each was on the bar, or, new on it, where it is now.
	at := func(g int) int {
		of := func(t app.Tab) bool { return t.Group == g }
		if i := slices.IndexFunc(b.tabs, of); i >= 0 {
			return i
		}
		return slices.IndexFunc(tabs, of)
	}
	switch from, to := at(b.front), at(front); {
	case to < from:
		return -1, true
	case to > from:
		return 1, true
	}
	return 0, true
}

// litAt is where the front tab's fill is drawn: on its way from where
// it was while it glides, or on the tab in front. It is empty while no
// tab is in front.
func (b *tabBar) litAt() geom.Rect {
	to, ok := b.drawnOf(b.front)
	if !ok {
		return geom.Rect{}
	}
	if t := b.lit.Value(); t < 1 && !b.litFrom.Empty() {
		return anim.Mix(anim.RectCodec, b.litFrom, to, max(t, 0))
	}
	return to
}

// drawnOf is where the tab of group g is drawn, closing or not.
func (b *tabBar) drawnOf(g int) (geom.Rect, bool) {
	for i, t := range b.tabs {
		if t.Group == g && i < len(b.boxes) {
			r, _ := b.drawnAt(i)
			return r, true
		}
	}
	for _, gone := range b.gone {
		if gone.tab.Group == g {
			return gone.box.Value(), true
		}
	}
	return geom.Rect{}, false
}

// litOf is how far tab g has the front tab's look, 0 to 1: coming on
// the tab in front, going on the one that was.
func (b *tabBar) litOf(g int) float32 {
	t := min(max(b.lit.Value(), 0), 1)
	switch g {
	case b.front:
		return t
	case b.was:
		return 1 - t
	}
	return 0
}

// tabsMoved notes what changes on the bar as it comes to show tabs: a
// tab gone shrinks away where it was, and one new grows in where it
// lands. A bar coming to show, or going, moves nothing but the tab new
// on it: its first tabs, as the window opens, are simply there.
func (b *tabBar) tabsMoved(tabs []app.Tab) {
	in := func(tabs []app.Tab, g int) bool {
		return slices.ContainsFunc(tabs, func(t app.Tab) bool { return t.Group == g })
	}
	if len(tabs) < 2 && len(b.tabs) < 2 {
		// Hidden, it shows nothing moving. The last tab but one closing
		// shrinks away as any tab does, as the bar fades.
		clear(b.moves)
		clear(b.arriving)
		b.gone, b.plusAt = nil, nil
		return
	}
	for i, t := range b.tabs {
		m := b.moves[t.Group]
		if m == nil || in(tabs, t.Group) {
			continue
		}
		delete(b.moves, t.Group)
		g := tabGone{tabMove: m, tab: t, kind: b.w.tabKind(t)}
		if i < len(b.titles) {
			g.title, g.more = b.titles[i], b.more[i]
		}
		r := m.box.Value()
		m.box.Animate(geom.Rc(r.Min.X, r.Min.Y, 0, r.Size().H), tabSlide)
		m.open.Animate(0, tabSlide)
		b.gone = append(b.gone, g)
	}
	if len(b.tabs) > 0 {
		for _, t := range tabs {
			if !in(b.tabs, t.Group) {
				b.arriving[t.Group] = true
			}
		}
	}
}

// Step implements [gunim.Animator]: the tabs sliding, opening and
// closing.
func (b *tabBar) Step(dt time.Duration) bool {
	moving := b.Group.Step(dt)
	for _, m := range b.moves {
		if m.step(dt) {
			moving = true
		}
	}
	if b.plusAt != nil && b.plusAt.Step(dt) {
		moving = true
	}
	kept := b.gone[:0]
	for _, g := range b.gone {
		if g.step(dt) {
			moving = true
			kept = append(kept, g)
		}
	}
	clear(b.gone[len(kept):])
	b.gone = kept
	return moving
}

// moveTabs starts each tab toward where the layout put it, a tab new
// on the bar from nothing where it lands, and reports whether any is
// moving.
func (b *tabBar) moveTabs() bool {
	moving := false
	for i, t := range b.tabs {
		r := b.boxes[i]
		m := b.moves[t.Group]
		if m == nil {
			m = &tabMove{box: anim.NewRect(r), open: anim.NewFloat(1)}
			if b.arriving[t.Group] {
				m.box.Jump(geom.Rc(r.Min.X, r.Min.Y, 0, r.Size().H))
				m.open.Jump(0)
			}
			b.moves[t.Group] = m
		}
		delete(b.arriving, t.Group)
		m.box.Animate(r, tabSlide)
		m.open.Animate(1, tabSlide)
		moving = moving || m.box.Active() || m.open.Active()
	}
	if b.plusAt == nil {
		b.plusAt = anim.NewRect(b.plus)
	}
	b.plusAt.Animate(b.plus, tabSlide)
	return moving || b.plusAt.Active() || len(b.gone) > 0
}

// drawnAt is where tab i is drawn, and how far it is open.
func (b *tabBar) drawnAt(i int) (geom.Rect, float32) {
	if m := b.moves[b.tabs[i].Group]; m != nil {
		return m.box.Value(), m.open.Value()
	}
	return b.boxes[i], 1
}

// tabTitle is what tab t says: the title of the pane in it that last
// had the keyboard.
func (w *Window) tabTitle(t app.Tab) string {
	for _, p := range w.panes {
		if p.ID == t.Pane {
			return w.titleOf(p)
		}
	}
	return ""
}

// tabShort is what tab t says where its title does not fit: the pane's
// own title, where the tab's names the machine before it, as "web:
// logs" does, and "" where nothing is shorter. A title naming the
// machine after, as "PowerShell on XPS25" does, has none: faded where
// it is cut, it still shows where the machine's name starts, as the
// pane's own title alone would not.
func (w *Window) tabShort(t app.Tab) string {
	for _, p := range w.panes {
		if p.ID == t.Pane {
			if full := w.titleOf(p); p.Title != "" && p.Title != full && strings.HasSuffix(full, p.Title) {
				return p.Title
			}
			return ""
		}
	}
	return ""
}

// fitTitle is what tab i says in room: its title where it fits, or
// else its short title where it has one. cut says that is less than all
// of the title; what passes the room fades out as it is drawn.
func (b *tabBar) fitTitle(i int, room float32) (run text.Run, cut bool) {
	full := b.titles[i]
	switch {
	case full.Advance <= room:
		return full, false
	case i < len(b.shorts) && b.shorts[i].Advance > 0:
		return b.shorts[i], true
	}
	return full, true
}

// tipOf is the tooltip for the pointer at pos: the whole title of the
// tab there where the tab shows less of it, and "" anywhere else.
func (b *tabBar) tipOf(pos geom.Point) string {
	i := b.tabAt(pos)
	if i < 0 || b.carried != 0 || i >= len(b.cut) || !b.cut[i] || i >= len(b.names) {
		return ""
	}
	return b.names[i]
}

// tabKind is the kind of the pane tab t shows, for its icon.
func (w *Window) tabKind(t app.Tab) string {
	if t.Panes > 1 {
		return "split"
	}
	return w.paneIcon(t.Pane, w.kindOf(t.Pane))
}

// paneIcon is the icon pane id shows, kind being what it would show
// anyway: a terminal in another kakel window, watched here, shows that
// window's mark, so it is not taken for one on this computer.
func (w *Window) paneIcon(id, kind string) string {
	if kind != app.KindTerminal {
		return kind
	}
	for _, p := range w.panes {
		if p.ID == id && slices.ContainsFunc(w.remoteWindows, func(rw app.RemoteWindow) bool { return rw.Name == p.Machine }) {
			return "window"
		}
	}
	return kind
}

// Layout implements [gunim.Node]: the tabs side by side, as wide as
// their titles within bounds, narrower all alike when the room runs
// short, then the +.
func (b *tabBar) Layout(c gunim.Constraints, f gunim.Frame, _ gunim.Children) geom.Size {
	h := widget.MenubarHeight.Get(f.Theme)
	if c.Max.H > 0 {
		h = min(h, c.Max.H)
	}
	b.boxes = b.boxes[:0]
	b.plus, b.end = geom.Rect{}, 0
	if !b.shown() {
		return c.Constrain(geom.Sz(0, h))
	}
	want := make([]float32, len(b.tabs))
	total := float32(0)
	for i := range b.tabs {
		w := 2*tabPad + tabIcon + 6 + b.titles[i].Advance + tabCross + 4
		if b.more[i].Advance > 0 {
			w += b.more[i].Advance + 6
		}
		want[i] = min(max(w, tabLeast), tabWidest)
		total += want[i] + tabGap
	}
	room := c.Max.W - tabPlus - captionLeast
	if c.Max.W > 0 && total > room {
		// Short of room: each tab as wide as the room allows, alike,
		// leaving the + and room to move the window by.
		each := max(room/float32(len(b.tabs))-tabGap, 1)
		for i := range want {
			want[i] = min(want[i], each)
		}
	}
	x := float32(0)
	for i := range b.tabs {
		b.boxes = append(b.boxes, geom.Rc(x, tabTop, want[i], h-tabTop))
		x += want[i] + tabGap
	}
	b.plus = geom.Rc(x+2, (h-24)/2, 24, 24)
	b.end = x + tabPlus
	if b.moveTabs() {
		f.RedrawAt(f.Now)
	}
	return c.Constrain(geom.Sz(max(c.Max.W, b.end), h))
}

// Paint implements [gunim.Node].
func (b *tabBar) Paint(p *paint.Painter, f gunim.Frame, box geom.Size, _ gunim.Children) {
	if !b.shown() {
		return
	}
	th := f.Theme
	if fade := min(max(b.fade.Value(), 0), 1); fade < 1 {
		defer p.Layer(paint.LayerOpts{Bounds: geom.Rect{Max: box.Point()}, Opacity: fade})()
	}
	p.RRect(geom.Rect{Max: box.Point()}, 0, paint.Solid(widget.MenubarFill.Get(th)))
	for _, g := range b.gone {
		b.paintTab(p, th, g.tab, g.title, g.more, g.kind, g.box.Value(), g.open.Value(), tabLook{at: -1})
	}
	// The front tab's fill, under the tabs, where it glides between
	// them, as faint as the tab is while it opens.
	if r := b.litAt(); r.Size().W > 0 {
		fill := look.RowActive.Get(th)
		if i := slices.IndexFunc(b.tabs, func(t app.Tab) bool { return t.Group == b.front }); i >= 0 {
			_, open := b.drawnAt(i)
			fill.A = uint8(float32(fill.A) * min(max(open, 0), 1))
		}
		p.RRect(r, look.RowRadius.Get(th), paint.Solid(fill))
	}
	for i, t := range b.tabs {
		r, open := b.drawnAt(i)
		b.paintTab(p, th, t, b.titles[i], b.more[i], b.w.tabKind(t), r, open, tabLook{
			at:       i,
			front:    t.Group == b.front,
			lit:      b.litOf(t.Group),
			hot:      i == b.hot && b.carried == 0,
			lifted:   t.Group == b.carried,
			cross:    b.crossShown(i),
			crossLit: i == b.hot && b.crossHot,
		})
	}
	if b.landing >= 0 && len(b.boxes) > 0 && b.landing <= len(b.boxes) {
		x := b.boxes[len(b.boxes)-1].Max.X + tabGap/2
		if b.landing < len(b.boxes) {
			x = b.boxes[b.landing].Min.X - tabGap/2
		}
		p.RRect(geom.Rc(x-1.5, 6, 3, box.H-10), 1.5, paint.Solid(switcherRing.Get(th)))
	}
	plus := b.plus
	if b.plusAt != nil {
		plus = b.plusAt.Value()
	}
	drawIcon(p, icon.Plus, plus.Inset(geom.Uniform(4)), look.Faint.Get(th), 1.5)
}

// tabLook is how a tab is drawn: in front, under the pointer, dragged
// away, with its × showing, and with the pointer on the ×. lit is how
// far its title has the front tab's ink, 0 to 1, as the tab in front
// changes.
type tabLook struct {
	// at is the tab's place in the bar, to fit its title to its room,
	// and -1 for one going, whose title is shown as it was.
	at                                  int
	front, hot, lifted, cross, crossLit bool
	lit                                 float32
}

// paintTab draws tab t in r, open as far as open says. The front tab's
// fill is the bar's to draw, as it glides between tabs.
func (b *tabBar) paintTab(p *paint.Painter, th *theme.Live, t app.Tab, title, more text.Run, kind string, r geom.Rect, open float32, l tabLook) {
	if r.Size().W <= 0 || open <= 0 {
		return
	}
	if open < 1 {
		defer p.Layer(paint.LayerOpts{Bounds: r, Opacity: min(open, 1)})()
	}
	ink, faint := widget.Ink.Get(th), look.Faint.Get(th)
	radius := look.RowRadius.Get(th)
	if l.hot && !l.front {
		p.RRect(r, radius, paint.Solid(look.RowHover.Get(th)))
	}
	c := faint
	switch {
	case l.lit >= 1:
		c = ink
	case l.lit > 0:
		c = anim.Mix(anim.ColorCodec, faint, ink, l.lit)
	}
	if l.lifted {
		c.A /= 3
	}
	mid := r.Min.Y + r.Size().H/2
	if r.Size().W < tabPad+tabIcon+6 {
		// Too narrow for its title: its icon alone, in the middle,
		// where it fits.
		if r.Size().W >= tabIcon+4 {
			paintIcon(p, kind, geom.Rc(r.Center().X-tabIcon/2, mid-tabIcon/2, tabIcon, tabIcon), c)
		}
		return
	}
	paintIcon(p, kind, geom.Rc(r.Min.X+tabPad, mid-tabIcon/2, tabIcon, tabIcon), c)
	x := r.Min.X + tabPad + tabIcon + 6
	stop := r.Max.X - tabPad
	if l.cross {
		stop -= tabCross
	}
	if more.Advance > 0 {
		mc := faint
		mc.A = uint8(float32(mc.A) * 0.8)
		more.Paint(p, geom.Pt(stop-more.Advance, mid-more.Height()/2), mc)
		stop -= more.Advance + 6
	}
	if room := stop - x; room > 0 {
		if l.at >= 0 && l.at < len(b.titles) {
			var cut bool
			title, cut = b.fitTitle(l.at, room)
			if l.at < len(b.cut) {
				b.cut[l.at] = cut
			}
		}
		// What passes the room fades out, rather than stopping short at
		// its edge.
		title.PaintFaded(p, geom.Pt(x, mid-title.Height()/2), c, room, widget.TextFade.Get(th))
	}
	if l.cross {
		cc := faint
		if l.crossLit {
			cc = ink
		}
		paintCross(p, crossIn(r), cc)
	}
}

// crossOf is where tab i's × is.
func (b *tabBar) crossOf(i int) geom.Rect { return crossIn(b.boxes[i]) }

// crossIn is where the × of a tab in r is.
func crossIn(r geom.Rect) geom.Rect {
	const s = 10
	return geom.Rc(r.Max.X-tabPad-s+2, r.Min.Y+(r.Size().H-s)/2, s, s)
}

// crossShown reports whether tab i shows its ×: the tab in front, and
// the one under the pointer, while no tab is dragged, where the tab is
// wide enough for it beside its icon.
func (b *tabBar) crossShown(i int) bool {
	if i < 0 || i >= len(b.tabs) || i >= len(b.boxes) || b.carried != 0 || b.boxes[i].Size().W < tabLeast {
		return false
	}
	return b.tabs[i].Group == b.front || i == b.hot
}

// onCross reports whether p is on tab i's ×, where it shows.
func (b *tabBar) onCross(i int, p geom.Point) bool {
	return b.crossShown(i) && b.crossOf(i).Inset(geom.Uniform(-4)).Contains(p)
}

// tabAt returns the tab at x, y, or -1.
func (b *tabBar) tabAt(p geom.Point) int {
	for i, r := range b.boxes {
		if r.Contains(p) {
			return i
		}
	}
	return -1
}

// CaptionRects implements [gunim.Caption]: the room after the tabs and
// the + moves the window.
func (b *tabBar) CaptionRects(size geom.Size) []geom.Rect {
	if b.end >= size.W {
		return nil
	}
	return []geom.Rect{geom.Rc(b.end, 0, size.W-b.end, size.H)}
}

// Handle implements [gunim.Handler].
//
// The ends of what began while the bar showed, a press let go and a
// drag ended, are heard while it is hidden too.
func (b *tabBar) Handle(e input.Event, u *gunim.UI) bool {
	// A title the tab cuts short shows whole under the pointer.
	switch e := e.(type) {
	case input.PointerMove:
		b.tip.Handle(e, u, b, b.tipOf(e.Pos))
	case input.PointerEnter:
		b.tip.Handle(e, u, b, b.tipOf(e.Pos))
	case input.PointerDown:
		b.tip.Handle(e, u, b, b.tipOf(e.Pos))
	case input.PointerLeave, input.Scroll:
		b.tip.Handle(e, u, b, "")
	}
	switch e := e.(type) {
	case input.PointerMove:
		if !b.shown() {
			return false
		}
		b.hotAt = e.Pos
		if b.pressed >= 0 && b.carried == 0 && !b.pressedCross && moved(e.Pos, b.pressAt) {
			b.lift(u)
			return true
		}
		// Over a tab, its × lights under the pointer.
		hot := b.tabAt(e.Pos)
		if hot != b.hot {
			b.hot = hot
			u.Invalidate()
		}
		if cross := b.onCross(hot, e.Pos); cross != b.crossHot {
			b.crossHot = cross
			u.Invalidate()
		}
		return hot >= 0
	case input.PointerLeave:
		if b.hot >= 0 {
			b.hot, b.crossHot = -1, false
			u.Invalidate()
		}
		return false
	case input.PointerDown:
		if !b.shown() {
			return false
		}
		i := b.tabAt(e.Pos)
		switch {
		case e.Button == input.ButtonMiddle && i >= 0:
			u.Send(b.w, app.CloseTab{Group: b.tabs[i].Group})
			return true
		case e.Button == input.ButtonSecondary && b.plus.Inset(geom.Uniform(-2)).Contains(e.Pos):
			b.openPlusMenu(u)
			return true
		case e.Button != input.ButtonPrimary:
			return false
		case i >= 0:
			b.pressed, b.pressAt = i, e.Pos
			b.pressedCross = b.onCross(i, e.Pos)
			return true
		case b.plus.Inset(geom.Uniform(-2)).Contains(e.Pos) && e.Button == input.ButtonPrimary:
			b.plusPressed = true
			return true
		}
		return false
	case input.PointerUp:
		if b.plusPressed {
			b.plusPressed = false
			if b.shown() && b.plus.Inset(geom.Uniform(-2)).Contains(e.Pos) {
				// Like the tab in front: a file manager where one is.
				u.Send(b.w, app.NewTab{})
			}
			return true
		}
		i := b.pressed
		b.pressed = -1
		if i < 0 || i >= len(b.tabs) || b.carried != 0 || b.tabAt(e.Pos) != i || !b.shown() {
			return i >= 0
		}
		if b.pressedCross {
			if b.onCross(i, e.Pos) {
				u.Send(b.w, app.CloseTab{Group: b.tabs[i].Group})
			}
			return true
		}
		u.Send(b.w, app.ShowTab{Group: b.tabs[i].Group})
		return true
	case input.DragEnd:
		b.dragEnded(e, u)
		return true
	case input.DragOver:
		if !b.takes(e.Data) {
			return false
		}
		b.landing = b.landingAt(e.Pos)
		b.holdOver(b.tabAt(e.Pos), u)
		u.AnswerDrag(movesHere)
		u.Invalidate()
		return true
	case input.DragLeave:
		b.landing = -1
		b.holdOver(-1, u)
		u.Invalidate()
		return false
	case input.Drop:
		if !b.takes(e.Data) {
			return false
		}
		at := b.landingAt(e.Pos)
		b.landing = -1
		b.holdOver(-1, u)
		b.sprungFrom = 0
		before := 0
		if at < len(b.tabs) {
			before = b.tabs[at].Group
		}
		switch d := e.Data.(type) {
		case app.TabDrag:
			u.Send(b.w, app.MoveTab{Group: d.Group, Before: before})
		case app.PaneDrag:
			// Out of its split, or out of another window, onto a tab of
			// its own where it was let go.
			u.Send(b.w, app.PaneToTab{Pane: d.Pane, Window: b.w.winID, Bar: true, Before: before})
		}
		u.Invalidate()
		return true
	}
	return false
}

// springHold is how long a drag is held over a tab before the tab comes
// to the front.
const springHold = 500 * time.Millisecond

// holdOver notes that a drag is over the tab at i, -1 for none: held
// there for springHold, the tab comes to the front, so what is dragged
// can be docked beside a pane in it.
func (b *tabBar) holdOver(i int, u *gunim.UI) {
	g := 0
	if i >= 0 && i < len(b.tabs) {
		g = b.tabs[i].Group
	}
	if g == b.spring {
		return
	}
	b.spring = g
	b.springRun++
	if g == 0 || g == b.front {
		return
	}
	run := b.springRun
	u.After(springHold, func(u *gunim.UI) {
		if b.springRun != run || b.spring != g || g == b.front {
			return
		}
		if b.sprungFrom == 0 {
			b.sprungFrom = b.front
		}
		u.Send(b.w, app.ShowTab{Group: g})
	})
}

// takes reports whether the bar takes what is dragged over it: a tab,
// or a pane, which lands on a tab of its own.
func (b *tabBar) takes(data any) bool {
	switch data.(type) {
	case app.TabDrag, app.PaneDrag:
		return b.shown()
	}
	return false
}

// landingAt is where a tab dragged to p would land: before the first
// tab whose middle is past p.
func (b *tabBar) landingAt(p geom.Point) int {
	for i, r := range b.boxes {
		if p.X < r.Min.X+r.Size().W/2 {
			return i
		}
	}
	return len(b.boxes)
}

// lift picks the tab pressed up, to follow the pointer.
func (b *tabBar) lift(u *gunim.UI) {
	i := b.pressed
	t := b.tabs[i]
	r := b.boxes[i]
	b.carried, b.sprungFrom = t.Group, 0
	b.grab = b.pressAt.Sub(r.Min)
	g := &tabGhost{title: b.titles[i], kind: b.w.tabKind(t), size: r.Size(), lit: anim.NewFloat(0)}
	g.Add(g.lit)
	u.StartDrag(b, app.TabDrag{Group: t.Group, Window: b.w.winID}, g, b.grab)
	u.Invalidate()
}

// dragEnded hears how the drag of a tab ended: taken, which the next
// state shows; let go outside every window, which opens one for it; or
// neither, and the tab stays where it was.
func (b *tabBar) dragEnded(e input.DragEnd, u *gunim.UI) {
	g := b.carried
	b.carried, b.pressed, b.landing = 0, -1, -1
	from := b.sprungFrom
	b.sprungFrom = 0
	b.holdOver(-1, u)
	if g == 0 {
		return
	}
	if from != 0 && !e.Taken && !e.Out {
		// Let go where nothing took it, after a tab held over came
		// forward: the tab in front before goes back.
		u.Send(b.w, app.ShowTab{Group: from})
	}
	if e.Out || e.Taken {
		b.dropped = g
	}
	if e.Out && !e.Taken {
		// The tab's top left corner where the image's was, and the
		// window as large as this one.
		u.Send(b.w, app.TabToNewWindow{Group: g, At: e.At.Sub(b.grab), Size: b.w.size})
	}
	u.Invalidate()
}

// tabGhost is the image of a tab that follows the pointer while it is
// dragged, ringed while it is over something that takes it.
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

// showTabs shows the window's tabs, and gives the bar the title bar's
// room while it shows: the menu button keeps its own, and "kakel" makes
// way. The change fades: what goes first, then the row changes, then
// what comes; see settleTitleRow.
//
// A switch to another tab shows in motion: the front tab's look glides
// to it, and what it holds comes on stage from its side.
func (w *Window) showTabs(st app.State, u *gunim.UI) {
	if from, switched := w.tabs.show(st.Tabs, st.Focus, u); switched {
		w.stage.comeIn(from)
	}
	w.settleTitleRow(u.Theme())
	u.Invalidate()
}

// settleTitleRow moves the title row toward showing the tabs, when the
// window has two or more, or else the window's title: it fades out
// what shows, changes the row once that has gone, and fades in what it
// changed to. Called as the tabs change and as the window lays out,
// each step happens once the one before has finished.
func (w *Window) settleTitleRow(th *theme.Live) {
	b := w.tabs
	if b.laid {
		// As wide as the menu button, in the theme on now.
		w.barBox.Width = widget.MenubarHeight.Get(th) + 12
	}
	switch want := b.wanted(); {
	case want == b.laid:
		// Staying, or coming back before it went: what shows comes in.
		if b.laid {
			b.fade.Animate(1, rowFadeIn)
		} else {
			b.titleFade.Animate(1, rowFadeIn)
		}
	case b.laid:
		b.fade.Animate(0, rowFadeOut)
		if !b.fade.Active() {
			w.layTitleRow(false, th)
		}
	default:
		b.titleFade.Animate(0, rowFadeOut)
		if !b.titleFade.Active() {
			w.layTitleRow(true, th)
		}
	}
}

// layTitleRow lays the title row out for the tabs, or for the window's
// title, and fades in what it was laid out for.
func (w *Window) layTitleRow(tabs bool, th *theme.Live) {
	b := w.tabs
	b.laid = tabs
	if tabs {
		w.barBox.Width = widget.MenubarHeight.Get(th) + 12
		w.bar.Title, w.bar.Subtitle = "", ""
		w.titleRow.Grow(w.barFade, 0).Grow(w.tabs, 1)
		b.fade.Jump(0)
		b.fade.Animate(1, rowFadeIn)
		return
	}
	w.barBox.Width = 0
	w.bar.Title, w.bar.Subtitle = app.ProgramName, w.paneTitle
	w.titleRow.Grow(w.tabs, 0).Grow(w.barFade, 1)
	clear(b.moves)
	clear(b.arriving)
	b.gone, b.plusAt = nil, nil
	b.titleFade.Jump(0)
	b.titleFade.Animate(1, rowFadeIn)
}

// titleFader draws the menu bar it holds with its button as it is, and
// the window's title after the button faded by the tab bar's
// titleFade, so the title can fade without the button.
type titleFader struct {
	child gunim.Node
	tabs  *tabBar
}

// Children implements [gunim.Composite].
func (t *titleFader) Children() []gunim.Node { return []gunim.Node{t.child} }

// Layout implements [gunim.Node].
func (t *titleFader) Layout(c gunim.Constraints, _ gunim.Frame, kids gunim.Children) geom.Size {
	k := kids.At(0)
	s := k.Layout(c)
	k.Place(geom.Point{})
	return s
}

// Paint implements [gunim.Node].
func (t *titleFader) Paint(p *paint.Painter, f gunim.Frame, box geom.Size, kids gunim.Children) {
	fade := min(max(t.tabs.titleFade.Value(), 0), 1)
	if fade >= 1 {
		kids.At(0).Paint(p)
		return
	}
	// The bar's fill stays as the title fades, or the room beside the
	// button, while the tabs show, would be the window's background.
	p.RRect(geom.Rect{Max: box.Point()}, 0, paint.Solid(widget.MenubarFill.Get(f.Theme)))
	// The compact menu's button is the bar's height and a little more,
	// 4 in from the bar's start.
	button := widget.MenubarHeight.Get(f.Theme) + 8
	func() {
		defer p.Layer(paint.LayerOpts{Bounds: geom.Rc(0, 0, button, box.H), Opacity: 1, Clip: true})()
		kids.At(0).Paint(p)
	}()
	if fade > 0 && box.W > button {
		defer p.Layer(paint.LayerOpts{Bounds: geom.Rc(button, 0, box.W-button, box.H), Opacity: fade, Clip: true})()
		kids.At(0).Paint(p)
	}
}

// tabDock is where a tab or a pane dragged over the window would go:
// beside pane, on the side it is nearest, or, with toTab, onto a tab of
// its own. What it would take is lit at lit. A zero one is nowhere.
type tabDock struct {
	pane            string
	vertical, first bool
	toTab           bool
	lit             geom.Rect
}

// docksHere is what a pane answers a tab dragged over it: a drop there
// joins it in a split.
const docksHere = "docks here"

// tabDrop takes a tab dragged over the window: onto a pane, where it
// joins it in a split on the side it is nearest, or from another
// window onto anywhere else, where it becomes a tab of this one.
func (w *Window) tabDrop(e input.Event, u *gunim.UI) bool {
	switch e := e.(type) {
	case input.DragOver:
		d, ok := e.Data.(app.TabDrag)
		if !ok {
			return false
		}
		w.dock = w.dockAt(e.Pos, d, u)
		switch {
		case w.dock.pane != "":
			u.AnswerDrag(docksHere)
			w.dropLit.Animate(0, widget.Quick.Get(u.Theme()))
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
		if w.dock.pane != "" {
			w.dock = tabDock{}
			u.Invalidate()
		}
		return false
	case input.Drop:
		d, ok := e.Data.(app.TabDrag)
		if !ok {
			return false
		}
		dock := w.dockAt(e.Pos, d, u)
		w.dock = tabDock{}
		w.dropLit.Animate(0, widget.Settle.Get(u.Theme()))
		u.Invalidate()
		switch {
		case dock.pane != "":
			u.Send(w, app.DockTab{Group: d.Group, Beside: dock.pane, Vertical: dock.vertical, First: dock.first})
		case d.Window != w.winID:
			u.Send(w, app.MoveTab{Group: d.Group})
		default:
			return false
		}
		return true
	}
	return false
}

// dockAt is where tab d, dragged to p, would join a split: beside the
// pane on stage under p, on the side of it p is nearest. Not beside a
// pane of the tab itself.
func (w *Window) dockAt(p geom.Point, d app.TabDrag, u *gunim.UI) tabDock {
	own := ""
	if d.Window == w.winID {
		for _, t := range w.tabs.tabs {
			if t.Group == d.Group {
				own = t.Pane
			}
		}
		if own == "" {
			// A tab of this window not on the bar: its only one.
			return tabDock{}
		}
	}
	return w.dockOver(p, u, func(id string) bool { return own != "" && w.groups[own] == w.groups[id] })
}

// dockOver is where something dragged to p would join a split: beside
// the pane on stage under p, on the side of it p is nearest. Not beside
// a pane mine says is the dragged thing's own.
func (w *Window) dockOver(p geom.Point, u *gunim.UI, mine func(id string) bool) tabDock {
	for _, id := range boxLeaves(w.stageBox, nil) {
		r, ok := u.Bounds(w.paneNode(id))
		if !ok || !r.Contains(p) {
			continue
		}
		if mine(id) {
			return tabDock{}
		}
		s := r.Size()
		if s.W <= 0 || s.H <= 0 {
			return tabDock{}
		}
		fx, fy := (p.X-r.Min.X)/s.W, (p.Y-r.Min.Y)/s.H
		dk := tabDock{pane: id}
		switch min(fx, 1-fx, fy, 1-fy) {
		case fx:
			dk.first, dk.lit = true, geom.Rc(r.Min.X, r.Min.Y, s.W/2, s.H)
		case 1 - fx:
			dk.lit = geom.Rc(r.Min.X+s.W/2, r.Min.Y, s.W/2, s.H)
		case fy:
			dk.vertical, dk.first, dk.lit = true, true, geom.Rc(r.Min.X, r.Min.Y, s.W, s.H/2)
		default:
			dk.vertical, dk.lit = true, geom.Rc(r.Min.X, r.Min.Y+s.H/2, s.W, s.H/2)
		}
		return dk
	}
	return tabDock{}
}

// paintDock lights the half of a pane a tab dragged over it would take.
func (w *Window) paintDock(p *paint.Painter, f gunim.Frame) {
	if w.dock.pane == "" && !w.dock.toTab {
		return
	}
	c := switcherRing.Get(f.Theme)
	tint := c
	tint.A = 0x33
	r := w.dock.lit.Inset(geom.Uniform(4))
	p.RRect(r, 6, paint.Solid(tint))
	p.RRectStroke(r, 6, paint.Fill{}, paint.Stroke{Width: 2, Color: c})
}

// Access implements [gunim.Accessible]: a tab list with a tab for each
// of the window's tabs, named as its title.
func (b *tabBar) Access() access.Info {
	info := access.Info{Role: access.RoleTabList}
	if !b.shown() {
		return info
	}
	for i, t := range b.tabs {
		name := b.w.tabTitle(t)
		if t.Panes > 1 {
			name += ", " + strconv.Itoa(t.Panes) + " panes"
		}
		part := access.Info{Role: access.RoleTab, Name: name, Actions: []string{access.ActionPress}}
		if t.Group == b.front {
			part.State = access.StateSelected
			info.Active = i + 1
		}
		if i < len(b.boxes) {
			part.Bounds = b.boxes[i]
		}
		info.Parts = append(info.Parts, part)
	}
	return info
}

// AccessAct implements [gunim.AccessActor]: pressing a tab shows it.
func (b *tabBar) AccessAct(r access.Request, u *gunim.UI) bool {
	if r.Action != access.ActionPress || r.Part < 0 || r.Part >= len(b.tabs) {
		return false
	}
	u.Send(b.w, app.ShowTab{Group: b.tabs[r.Part].Group})
	return true
}

// openPlusMenu offers what a new tab can hold, on the machine of the
// pane in front: a terminal, or a file manager.
func (b *tabBar) openPlusMenu(u *gunim.UI) {
	b.closePlusMenu()
	m := b.w.machineOf(b.w.focused)
	menu := widget.NewMenu([]widget.MenuItem{
		{Label: "New Terminal", Icon: icon.SquareTerminal},
		{Label: "New File Manager", Icon: icon.Folder},
	})
	menu.OnPick = func(i int, u *gunim.UI) gunim.Intent {
		b.closePlusMenu()
		switch i {
		case 0:
			return app.OpenOn{Machine: m}
		case 1:
			return app.OpenFilesOn{Machine: m, NewTab: true}
		}
		return nil
	}
	b.plusMenu = u.OpenPopup(b, menu, gunim.PopupOptions{
		Anchor:  b.plus,
		Max:     geom.Sz(280, 200),
		Dismiss: func(*gunim.UI) { b.closePlusMenu() },
	})
}

// closePlusMenu closes the +'s menu, if it is open.
func (b *tabBar) closePlusMenu() {
	if b.plusMenu != nil {
		b.plusMenu.Close()
		b.plusMenu = nil
	}
}
