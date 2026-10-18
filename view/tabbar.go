package view

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/marrasen/kakel/app"
	"github.com/marrasen/kakel/look"

	"github.com/marrasen/gunim"
	"github.com/marrasen/gunim/anim"
	"github.com/marrasen/gunim/geom"
	"github.com/marrasen/gunim/icon"
	"github.com/marrasen/gunim/input"
	"github.com/marrasen/gunim/paint"
	"github.com/marrasen/gunim/text"
	"github.com/marrasen/gunim/theme"
	"github.com/marrasen/gunim/widget"
	"github.com/marrasen/gunim/widget/dock"
)

// The tab bar, in the title bar after the menu button, while the window
// has a tab. It is gunim's dock.Bar, on its own: a click shows a tab; a
// middle click or its × closes it; the + opens a terminal in a tab of
// its own, and its right click offers a file manager too. A tab dragged
// along the bar moves along it, onto another window's bar moves there,
// onto a pane joins it in a split, and let go outside every window
// opens a window of its own. A pane dragged onto the bar lands on a tab
// of its own. The bar's empty room moves the window, as the rest of the
// title bar does. Here the bar fades in and out with the title row, and
// says when the tab in front changed in a way the stage shows in
// motion.

// tabBar shows the window's tabs, on a dock.Bar it holds.
type tabBar struct {
	anim.Group
	w   *Window
	bar *dock.Bar
	// tabs are the window's tabs, and front the group of the one in
	// front.
	tabs  []app.Tab
	front int
	// plusMenu is the menu a right click on the + opens, while it is.
	plusMenu *gunim.Popup
	// laid says the title row is laid out for the tabs, which it is
	// from the first tab on, and stays while the last tab fades away.
	// fade is how far the tabs show, and titleFade how far the window's
	// title after the menu button does: the one fades out before the
	// row changes, and the other in after.
	laid            bool
	fade, titleFade *anim.Float
}

// The tabs' measures, for a tab dragged out of a pane's caption: the
// room each side of a title, its icon, and how far below the top of
// the bar a tab starts.
const (
	tabPad  = 10
	tabIcon = 16
	tabTop  = 4
	// springHold is how long a drag is held over a tab before the tab
	// comes to the front, as gunim's bar has it.
	springHold = 500 * time.Millisecond
)

// rowFadeOut and rowFadeIn are how the tabs and the window's title fade
// as the title row changes between them: the one going quickly, the
// other coming a little slower.
var (
	rowFadeOut = anim.Tween{Duration: 110 * time.Millisecond, Ease: anim.EaseInOut}
	rowFadeIn  = anim.Tween{Duration: 180 * time.Millisecond, Ease: anim.EaseInOut}
)

func newTabBar(w *Window) *tabBar {
	b := &tabBar{w: w, bar: dock.NewBar("", w.winID), fade: anim.NewFloat(1), titleFade: anim.NewFloat(1)}
	b.Add(b.fade, b.titleFade)
	b.bar.Plus, b.bar.Plain, b.bar.Caption = true, true, true
	b.bar.OnChange = b.change
	b.bar.OnPlusMenu = func(at geom.Rect, u *gunim.UI) gunim.Intent {
		b.openPlusMenu(at, u)
		return nil
	}
	b.bar.Takes = func(data any) bool {
		_, ok := data.(app.PaneDrag)
		return ok && b.shown()
	}
	b.bar.OnDrop = func(d dock.Dropped, u *gunim.UI) gunim.Intent {
		// Out of its split, or out of another window, onto a tab of its
		// own where it was let go.
		if pd, ok := d.Data.(app.PaneDrag); ok {
			u.Send(b.w, app.PaneToTab{Pane: pd.Pane, Window: b.w.winID, Bar: true, Before: b.before(d.Index)})
		}
		return nil
	}
	return b
}

// tabID is the ID a tab of group g has on the bar, and groupOf the
// group of the tab of ID id, 0 for none.
func tabID(g int) string { return strconv.Itoa(g) }

func groupOf(id string) int {
	g, _ := strconv.Atoi(id)
	return g
}

// before is the group of the tab at place i on the bar, 0 past the
// last, for a tab to land before.
func (b *tabBar) before(i int) int {
	if i >= 0 && i < len(b.tabs) {
		return b.tabs[i].Group
	}
	return 0
}

// change says what the user asked the bar for, as the program's
// intents, from the window.
func (b *tabBar) change(c dock.Change, u *gunim.UI) gunim.Intent {
	var in gunim.Intent
	switch c := c.(type) {
	case dock.Show:
		in = app.ShowTab{Group: groupOf(c.ID)}
	case dock.Close:
		in = app.CloseTab{Group: groupOf(c.ID)}
	case dock.New:
		// Like the tab in front: a file manager where one is.
		in = app.NewTab{}
	case dock.Move:
		in = app.MoveTab{Group: groupOf(c.ID), Before: b.before(c.To.Index)}
	case dock.Detach:
		// The tab's top left corner where the image's was, and the
		// window as large as this one.
		in = app.TabToNewWindow{Group: groupOf(c.ID), At: c.At, Size: b.w.size}
	}
	if in != nil {
		u.Send(b.w, in)
	}
	return nil
}

// shown reports whether the bar has the title row's room and shows,
// which it does while the window has a tab, and as the last one fades.
func (b *tabBar) shown() bool { return b.laid }

// wanted reports whether the window has tabs to show. Its only tab
// shows too, so a lone pane can be dragged by it to another window.
func (b *tabBar) wanted() bool { return len(b.tabs) > 0 }

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
	from, switched = b.switchOf(tabs, front)
	b.tabs, b.front = tabs, front
	b.bar.Win = b.w.winID
	ts := make([]dock.Tab, len(tabs))
	for i, t := range tabs {
		more := ""
		if t.Panes > 1 {
			more = "+" + strconv.Itoa(t.Panes-1)
		}
		ic, ok := kindIcons[b.w.tabKind(t)]
		if !ok {
			ic = icon.SquareTerminal
		}
		ts[i] = dock.Tab{ID: tabID(t.Group), Face: dock.Face{Title: b.w.tabTitle(t), Short: b.w.tabShort(t), More: more, Icon: ic, Closable: true}}
	}
	fid := ""
	if front != 0 {
		fid = tabID(front)
	}
	b.bar.SetTabs(ts, fid, u)
	u.Invalidate()
	return from, switched
}

// switchOf reports whether the tab in front changing to front, as the
// bar comes to show tabs, is a switch to show in motion, and from which
// side the tab picked comes: -1 the left, 1 the right, 0 neither. A
// window's first tabs are simply there, and so is a tab in front while
// a tab is dragged, or after one was dragged away.
func (b *tabBar) switchOf(tabs []app.Tab, front int) (float32, bool) {
	if front == b.front || front == 0 || b.front == 0 || len(b.tabs) == 0 || b.bar.Carried() != "" ||
		b.bar.Dropped() != "" && !slices.Equal(tabs, b.tabs) {
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

// Children implements [gunim.Composite].
func (b *tabBar) Children() []gunim.Node { return []gunim.Node{b.bar} }

// Layout implements [gunim.Node]: the bar, as tall as the title bar,
// while the title row is laid out for it; else nothing.
func (b *tabBar) Layout(c gunim.Constraints, f gunim.Frame, kids gunim.Children) geom.Size {
	h := widget.MenubarHeight.Get(f.Theme)
	if c.Max.H > 0 {
		h = min(h, c.Max.H)
	}
	if !b.shown() {
		return c.Constrain(geom.Sz(0, h))
	}
	b.bar.DetachSize = b.w.size
	k := kids.At(0)
	k.Layout(gunim.Tight(geom.Sz(c.Max.W, h)))
	k.Place(geom.Point{})
	return c.Constrain(geom.Sz(c.Max.W, h))
}

// Paint implements [gunim.Node]: the bar, as far as it has faded in.
func (b *tabBar) Paint(p *paint.Painter, _ gunim.Frame, box geom.Size, kids gunim.Children) {
	if !b.shown() {
		return
	}
	if fade := min(max(b.fade.Value(), 0), 1); fade < 1 {
		defer p.Layer(paint.LayerOpts{Bounds: geom.Rect{Max: box.Point()}, Opacity: fade})()
	}
	kids.At(0).Paint(p)
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
// window has any, or else the window's title: it fades out
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
		d, ok := e.Data.(dock.Drag)
		if !ok {
			return false
		}
		w.dock = w.dockAt(e.Pos, d, u)
		switch {
		case w.dock.pane != "":
			u.AnswerDrag(docksHere)
			w.dropLit.Animate(0, widget.Quick.Get(u.Theme()))
		case d.Win != w.winID:
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
		d, ok := e.Data.(dock.Drag)
		if !ok {
			return false
		}
		dk := w.dockAt(e.Pos, d, u)
		w.dock = tabDock{}
		w.dropLit.Animate(0, widget.Settle.Get(u.Theme()))
		u.Invalidate()
		switch {
		case dk.pane != "":
			u.Send(w, app.DockTab{Group: groupOf(d.ID), Beside: dk.pane, Vertical: dk.vertical, First: dk.first})
		case d.Win != w.winID:
			u.Send(w, app.MoveTab{Group: groupOf(d.ID)})
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
func (w *Window) dockAt(p geom.Point, d dock.Drag, u *gunim.UI) tabDock {
	own := ""
	if d.Win == w.winID {
		for _, t := range w.tabs.tabs {
			if t.Group == groupOf(d.ID) {
				own = t.Pane
			}
		}
		if own == "" {
			// A tab of this window not on its bar yet.
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

// openPlusMenu offers what a new tab can hold, on the machine of the
// pane in front: a terminal, or a file manager; at is where the + is,
// on the bar.
func (b *tabBar) openPlusMenu(at geom.Rect, u *gunim.UI) {
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
	b.plusMenu = u.OpenPopup(b.bar, menu, gunim.PopupOptions{
		Anchor:  at,
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
