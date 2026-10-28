package view

import (
	"strings"
	"testing"
	"time"

	"github.com/marrasen/kakel/app"
	"github.com/marrasen/kakel/internal/sessiontest"
	"github.com/marrasen/kakel/screen"
	"github.com/marrasen/kakel/vt"

	"github.com/marrasen/gunim"
	"github.com/marrasen/gunim/filemanager"
	"github.com/marrasen/gunim/geom"
	gi "github.com/marrasen/gunim/input"
	"github.com/marrasen/gunim/paint"
	"github.com/marrasen/gunim/widget"
	"github.com/marrasen/gunim/widget/dock"
)

// twoTabs is a window with two tabs, p1's in front, and p2's.
func twoTabs() app.State {
	b1, b2 := &app.Box{Pane: "p1"}, &app.Box{Pane: "p2"}
	return app.State{
		Window: 1,
		Panes: []app.Pane{
			{ID: "p1", Title: "Jobs", Kind: app.KindJobs},
			{ID: "p2", Title: "Log", Kind: app.KindLog},
		},
		Stage: b1, Focus: "p1",
		Groups: map[string]*app.Box{"p1": b1, "p2": b2},
		Tabs:   []app.Tab{{Group: 1, Pane: "p1", Panes: 1}, {Group: 2, Pane: "p2", Panes: 1}},
	}
}

// settle runs the window a second, for the title bar to slide the tab
// bar into place.
func settle() {
	for range 60 {
		lastWindow.Frame(time.Second / 60)
	}
}

// drain empties the window's intents.
func drain() {
	settle()
	for len(lastWindow.Client().Intents()) > 0 {
		<-lastWindow.Client().Intents()
	}
}

// tabRect is where tab i stands, in the window's space.
func tabRect(t *testing.T, win *Window, i int) geom.Rect {
	t.Helper()
	r, ok := lastUI.Bounds(win.tabs.bar)
	if !ok || i >= len(win.tabs.tabs) {
		t.Fatalf("the tab bar is not drawn, or has no tab %d", i)
	}
	rc, ok := win.tabs.bar.TabRect(tabID(win.tabs.tabs[i].Group))
	if !ok {
		t.Fatalf("tab %d is not laid out", i)
	}
	return rc.Add(r.Min)
}

// tabAt is the middle of tab i, in the window's space.
func tabAt(t *testing.T, win *Window, i int) geom.Point {
	t.Helper()
	return tabRect(t, win, i).Center()
}

// plusAt is the middle of the bar's +, in the window's space.
func plusAt(t *testing.T, win *Window) geom.Point {
	t.Helper()
	r, ok := lastUI.Bounds(win.tabs.bar)
	if !ok {
		t.Fatal("the tab bar is not drawn")
	}
	return win.tabs.bar.PlusRect().Center().Add(r.Min)
}

// captionLeast is the room a bar in the title bar leaves after its +
// to move the window by, as gunim's has it.
const captionLeast = 64

// click presses and lets go the primary button at p.
func click(p geom.Point, b gi.Button) {
	lastWindow.Input(gi.PointerDown{Pos: p, Button: b})
	lastWindow.Input(gi.PointerUp{Pos: p, Button: b})
	lastWindow.Frame(time.Second / 60)
}

// The bar shows from the window's first tab and names the panes in
// place of the title bar: what shows fades out before the title bar
// changes, and what comes fades in. Its only tab stays on it, so a lone
// pane can be dragged by it, and the last but one closing shrinks away
// as any tab does.
func TestTheTabBarShowsWithOneTab(t *testing.T) {
	win, _, publish := windowStage(t)
	b := win.tabs
	frames := func(n int) {
		for range n {
			lastWindow.Frame(time.Second / 60)
		}
	}
	one := twoTabs()
	one.Panes, one.Tabs = one.Panes[:1], one.Tabs[:1]
	publish(one)
	frames(2)
	if b.shown() || b.titleFade.Value() >= 1 {
		t.Fatalf("a frame after the first tab came, the bar shows %v and the title is %v there", b.shown(), b.titleFade.Value())
	}
	settle()
	if !b.shown() || win.bar.Subtitle != "" || win.bar.Title != "" || b.fade.Value() != 1 || len(b.bar.Tabs()) != 1 {
		t.Fatalf("with one tab the bar shows %v with %d tabs, %v faded in, and the title bar says %q, %q", b.shown(), len(b.bar.Tabs()), b.fade.Value(), win.bar.Title, win.bar.Subtitle)
	}
	publish(twoTabs())
	settle()
	if !b.shown() || len(b.bar.Tabs()) != 2 || b.front != 1 || b.bar.Front() != "1" {
		t.Fatalf("with two tabs the bar shows %v with %d tabs, %d in front", b.shown(), len(b.bar.Tabs()), b.front)
	}
	publish(one)
	frames(2)
	if !b.shown() || b.fade.Value() != 1 || len(b.bar.Tabs()) != 1 {
		t.Fatalf("a frame after the second tab closed, the bar shows %v, %v faded, with %d tabs", b.shown(), b.fade.Value(), len(b.bar.Tabs()))
	}
	settle()
	if !b.shown() || len(b.bar.Tabs()) != 1 || win.bar.Subtitle != "" {
		t.Fatalf("back to one tab the bar shows %v with %d tabs, the title bar says %q", b.shown(), len(b.bar.Tabs()), win.bar.Subtitle)
	}
}

// The tabs stand on the title bar's fill, as the rest of the title bar
// does: gunim's bar is Plain and draws none of its own.
func TestTheTabsStandOnTheTitleBarsFill(t *testing.T) {
	win, _, publish := windowStage(t)
	publish(twoTabs())
	settle()
	r, ok := lastUI.Bounds(win.tabs)
	if !ok || r.Empty() {
		t.Fatal("the tab bar is not drawn")
	}
	fill := widget.MenubarFill.Get(lastUI.Theme())
	for _, op := range lastWindow.Offscreen().Ops() {
		if rr, ok := op.(*paint.RRectOp); ok && rr.Fill.Solid == fill && rr.Rect.Size() == r.Size() {
			return
		}
	}
	t.Fatalf("nothing paints the title bar's fill behind the tabs, %v", r)
}

// A click shows a tab, a middle click closes it, and so does its ×;
// the + opens a terminal.
func TestTheTabsAnswerTheMouse(t *testing.T) {
	win, _, publish := windowStage(t)
	publish(twoTabs())
	drain()
	click(tabAt(t, win, 1), gi.ButtonPrimary)
	if in, ok := nextIntent(t).(app.ShowTab); !ok || in.Group != 2 {
		t.Fatalf("a click sent %#v", in)
	}
	click(tabAt(t, win, 1), gi.ButtonMiddle)
	if in, ok := nextIntent(t).(app.CloseTab); !ok || in.Group != 2 {
		t.Fatalf("a middle click sent %#v", in)
	}
	// The × shows on the tab in front, and on the one under the pointer.
	first := tabRect(t, win, 0)
	cross := geom.Pt(first.Max.X-dock.TabPadding.Default()-4, first.Center().Y)
	lastWindow.Input(gi.PointerMove{Pos: cross})
	click(cross, gi.ButtonPrimary)
	if in, ok := nextIntent(t).(app.CloseTab); !ok || in.Group != 1 {
		t.Fatalf("a click on the × sent %#v", in)
	}
	click(plusAt(t, win), gi.ButtonPrimary)
	if in, ok := nextIntent(t).(app.NewTab); !ok {
		t.Fatalf("a click on the + sent %#v", in)
	}
	// A right click offers a terminal or a file manager.
	click(plusAt(t, win), gi.ButtonSecondary)
	if win.tabs.plusMenu == nil {
		t.Fatal("a right click on the + opened no menu")
	}
}

// A tab dragged away lifts off, and let go outside every window asks
// for a window of its own, where the image was let go.
func TestATabLetGoOutsideAsksForAWindow(t *testing.T) {
	win, _, publish := windowStage(t)
	publish(twoTabs())
	drain()
	at := tabAt(t, win, 1)
	grab := at.Sub(tabRect(t, win, 1).Min)
	lastWindow.Input(gi.PointerDown{Pos: at, Button: gi.ButtonPrimary})
	lastWindow.Input(gi.PointerMove{Pos: at.Add(geom.Pt(20, 30))})
	lastWindow.Frame(time.Second / 60)
	if win.tabs.bar.Carried() != "2" {
		t.Fatalf("the tab did not lift off: %q is carried", win.tabs.bar.Carried())
	}
	win.tabs.bar.Handle(gi.DragEnd{Out: true, At: geom.Pt(1200, 300)}, lastUI)
	in, ok := nextIntent(t).(app.TabToNewWindow)
	if !ok || in.Group != 2 || in.At != geom.Pt(1200, 300).Sub(grab) || in.Size != win.size {
		t.Fatalf("let go outside, the bar sent %#v", in)
	}
	if win.tabs.bar.Carried() != "" {
		t.Fatal("the tab is still carried")
	}
}

// A tab dropped on the bar lands before the tab whose middle is past
// the pointer, or last.
func TestATabDroppedOnTheBarMovesThere(t *testing.T) {
	win, _, publish := windowStage(t)
	publish(twoTabs())
	drain()
	first := tabRect(t, win, 0)
	lastWindow.Input(gi.Drop{Pos: geom.Pt(first.Min.X+4, first.Center().Y), Data: dock.Drag{ID: "2", Win: 1}})
	lastWindow.Frame(time.Second / 60)
	if in, ok := nextIntent(t).(app.MoveTab); !ok || in.Group != 2 || in.Before != 1 {
		t.Fatalf("dropped before the first tab, the bar sent %#v", in)
	}
	lastWindow.Input(gi.Drop{Pos: geom.Pt(plusAt(t, win).X-10, first.Center().Y), Data: dock.Drag{ID: "9", Win: 3}})
	lastWindow.Frame(time.Second / 60)
	if in, ok := nextIntent(t).(app.MoveTab); !ok || in.Group != 9 || in.Before != 0 {
		t.Fatalf("dropped after the last tab, the bar sent %#v", in)
	}
}

// A tab dropped on a pane joins it in a split on the side it is
// nearest; dropped on its own pane, nothing happens.
func TestATabDroppedOnAPaneDocks(t *testing.T) {
	win, _, publish := windowStage(t)
	publish(twoTabs())
	drain()
	r, ok := lastUI.Bounds(win.paneNode("p1"))
	if !ok {
		t.Fatal("the pane is not drawn")
	}
	left := geom.Pt(r.Min.X+10, r.Center().Y)
	if !win.tabDrop(gi.DragOver{Pos: left, Data: dock.Drag{ID: "2", Win: 1}}, lastUI) || win.dock.pane != "p1" || !win.dock.first || win.dock.vertical {
		t.Fatalf("over the pane's left edge, the dock is %+v", win.dock)
	}
	lastWindow.Input(gi.Drop{Pos: geom.Pt(r.Center().X, r.Max.Y-10), Data: dock.Drag{ID: "2", Win: 1}})
	lastWindow.Frame(time.Second / 60)
	if in, ok := nextIntent(t).(app.DockTab); !ok || in.Group != 2 || in.Beside != "p1" || !in.Vertical || in.First {
		t.Fatalf("dropped at the pane's bottom edge, the window sent %#v", in)
	}
	if win.tabDrop(gi.DragOver{Pos: left, Data: dock.Drag{ID: "1", Win: 1}}, lastUI) {
		t.Fatal("the window took a tab over its own pane")
	}
}

// A tab carried away while its window drops to one tab still hears how
// the drag ends, and the bar works when the tab comes back.
func TestTheBarWorksAfterLosingTheTabMidDrag(t *testing.T) {
	win, _, publish := windowStage(t)
	publish(twoTabs())
	drain()
	at := tabAt(t, win, 1)
	lastWindow.Input(gi.PointerDown{Pos: at, Button: gi.ButtonPrimary})
	lastWindow.Input(gi.PointerMove{Pos: at.Add(geom.Pt(20, 30))})
	lastWindow.Frame(time.Second / 60)
	one := twoTabs()
	one.Panes, one.Tabs = one.Panes[:1], one.Tabs[:1]
	publish(one)
	lastWindow.Input(gi.PointerUp{Pos: at.Add(geom.Pt(20, 30)), Button: gi.ButtonPrimary})
	lastWindow.Frame(time.Second / 60)
	if win.tabs.bar.Carried() != "" {
		t.Fatalf("its tab gone, the bar still carries %q", win.tabs.bar.Carried())
	}
	publish(twoTabs())
	drain()
	click(tabAt(t, win, 1), gi.ButtonPrimary)
	if in := nextIntent(t); in != (app.ShowTab{Group: 2}) {
		t.Fatalf("the tab back, a click sent %#v", in)
	}
}

// However many tabs there are, the + stays on the bar and room is left
// after it to move the window by; a tab too narrow for its × closes by
// no click on its icon.
func TestManyTabsKeepTheBarUsable(t *testing.T) {
	win, _, publish := windowStage(t)
	st := twoTabs()
	st.Tabs = nil
	for i := range 30 {
		id := "p" + string(rune('a'+i))
		b := &app.Box{Pane: id}
		st.Panes = append(st.Panes, app.Pane{ID: id, Title: "rdp@somewhere: ~/src/kakel", Kind: app.KindJobs})
		st.Groups[id] = b
		st.Tabs = append(st.Tabs, app.Tab{Group: i + 10, Pane: id, Panes: 1})
	}
	publish(st)
	drain()
	r, _ := lastUI.Bounds(win.tabs.bar)
	if plus := win.tabs.bar.PlusRect(); plus.Max.X > r.Size().W {
		t.Fatalf("the + ends at %.0f on a bar %.0f wide", plus.Max.X, r.Size().W)
	}
	if got := win.tabs.bar.CaptionRects(r.Size()); len(got) != 1 || got[0].Size().W < captionLeast-1 {
		t.Fatalf("the bar leaves %v to move the window by", got)
	}
	icon := tabAt(t, win, 3)
	lastWindow.Input(gi.PointerMove{Pos: icon})
	click(icon, gi.ButtonPrimary)
	if in, ok := nextIntent(t).(app.ShowTab); !ok || in.Group != 13 {
		t.Fatalf("a click on a narrow tab sent %#v", in)
	}
}

// Out of the tree, the Machines pane's list keeps no frames coming for a
// row it had last, and Escape in the list goes back to the pane last
// worked in.
func TestTheServersListRestsOffStage(t *testing.T) {
	win, _, publish := windowStage(t)
	st := withServers(twoPanes("p2", nil))
	st.Focus = "ps"
	st.Working = "p2"
	publish(st)
	drain()
	if !win.listShown || lastUI.Focused() == nil {
		t.Fatal("on stage, the list is not shown or has no keyboard")
	}
	lastWindow.Input(gi.KeyPress{Key: gi.KeyEscape})
	lastWindow.Frame(time.Second / 60)
	if in := nextIntent(t); in != (app.FocusPane{Pane: "p2"}) {
		t.Fatalf("Escape sent %#v", in)
	}
	publish(twoPanes("p2", nil))
	if win.listShown || win.anyBreathing(time.Now()) {
		t.Fatal("off stage, the list is taken as shown")
	}
}

// In a narrow pane the Machines pane's field and Add stay inside it,
// and the title makes room for them.
func TestTheServersButtonsFitANarrowPane(t *testing.T) {
	win, _, publish := windowStageOf(t, geom.Sz(300, 400))
	publish(withServers(app.State{}))
	settle()
	pane, _ := lastUI.Bounds(win.serversView)
	for _, n := range []gunim.Node{win.serversView.search, win.serversView.add} {
		r, _ := lastUI.Bounds(n)
		if r.Min.X < pane.Min.X || r.Max.X > pane.Max.X || r.Empty() {
			t.Fatalf("%T is at %v in a pane at %v", n, r, pane)
		}
	}
	if win.serversView.headShown {
		t.Fatal("the title shows in a pane too narrow for it")
	}
}

// twoTermTabs is twoTabs with a terminal in each tab, and the sessions
// typed into, by pane.
func twoTermTabs(t *testing.T, sh *screen.Shells) (app.State, map[string]*sessiontest.Typed) {
	t.Helper()
	quiet := screen.Hooks{Output: func() {}, Title: func(string) {}, Exit: func() {}, Clipboard: func(string) {}}
	sessions := map[string]*sessiontest.Typed{}
	for _, id := range []string{"p1", "p2"} {
		s := sessiontest.New()
		sessions[id] = s
		sh.Set(id, screen.Open(s, vt.DefaultPalette(), quiet))
		t.Cleanup(func() { _ = sh.Get(id).T.Close() })
	}
	st := twoTabs()
	st.Panes = []app.Pane{{ID: "p1", Title: "Terminal 1"}, {ID: "p2", Title: "Terminal 2"}}
	return st, sessions
}

// inFront is st with the tab of pane id in front.
func inFront(st app.State, id string) app.State {
	st.Stage, st.Focus = st.Groups[id], id
	return st
}

// A switch to another tab shows in motion: the front tab's look glides
// to it, and what it holds comes in from its side. What is typed goes to
// it from the start, and once settled nothing moves.
func TestASwitchedTabComesIn(t *testing.T) {
	win, sh, publish := windowStage(t)
	st, sessions := twoTermTabs(t, sh)
	publish(st)
	settle()
	b, s := win.tabs, win.stage
	if s.cover.Value() != 0 {
		t.Fatalf("the window's first tabs move: the stage is %v covered", s.cover.Value())
	}
	publish(inFront(st, "p2"))
	if b.front != 2 || b.bar.Front() != "2" || s.from != 1 {
		t.Fatalf("switched, the tab in front is %d, coming from %v", b.front, s.from)
	}
	if c := s.cover.Value(); c <= 0 || c >= 1 || !s.cover.Active() {
		t.Fatalf("a moment after the switch, the stage is %v covered", c)
	}
	// Typed while it moves, it reaches the tab picked.
	lastWindow.Input(gi.TextInput{Text: "ls"})
	lastWindow.Frame(time.Second / 60)
	got := sessions["p2"].Sent()
	for end := time.Now().Add(time.Second); !strings.Contains(got, "ls") && time.Now().Before(end); got = sessions["p2"].Sent() {
		time.Sleep(5 * time.Millisecond)
	}
	if !strings.Contains(got, "ls") || strings.Contains(sessions["p1"].Sent(), "ls") {
		t.Fatalf("typed as the tab came in, p2 was sent %q and p1 %q", got, sessions["p1"].Sent())
	}
	settle()
	if s.cover.Value() != 0 || s.cover.Active() || s.Step(time.Second/60) || b.bar.Step(time.Second/60) {
		t.Fatalf("settled, the stage is %v covered, or the bar still moves", s.cover.Value())
	}
	// Back to the left, it comes from the left.
	publish(inFront(st, "p1"))
	if s.from != -1 || !s.cover.Active() {
		t.Fatalf("switched back, the stage comes from %v, moving %v", s.from, s.cover.Active())
	}
}

// The tab in front dragged out of the window leaves the next one in
// front with no motion: it is simply there.
func TestATabDraggedOutSwitchesStill(t *testing.T) {
	win, _, publish := windowStage(t)
	publish(twoTabs())
	drain()
	at := tabAt(t, win, 0)
	lastWindow.Input(gi.PointerDown{Pos: at, Button: gi.ButtonPrimary})
	lastWindow.Input(gi.PointerMove{Pos: at.Add(geom.Pt(20, 30))})
	lastWindow.Frame(time.Second / 60)
	if win.tabs.bar.Carried() != "1" {
		t.Fatalf("the tab did not lift off: %q is carried", win.tabs.bar.Carried())
	}
	win.tabs.bar.Handle(gi.DragEnd{Out: true, At: geom.Pt(1200, 300)}, lastUI)
	// The program moves the tab to a window of its own.
	st := inFront(twoTabs(), "p2")
	st.Panes, st.Tabs = st.Panes[1:], st.Tabs[1:]
	delete(st.Groups, "p1")
	publish(st)
	if win.stage.cover.Value() != 0 || win.stage.cover.Active() {
		t.Fatalf("dragged out, the stage is %v covered", win.stage.cover.Value())
	}
}

// A file manager pane switched to in its tab comes in, has the keyboard,
// and fills its place once settled; parked again as its tab goes
// behind, it keeps its views.
func TestAFileManagerTabComesIn(t *testing.T) {
	win, _, publish := windowStage(t)
	filemanager.RegisterViews(lastWindow)
	fw := filePane(t, "p1")
	st := twoTabs()
	st.Panes[0] = app.Pane{ID: "p1", Title: "dir", Kind: app.KindFileManager}
	st = inFront(st, "p2")
	publish(st)
	fw.Attach(lastWindow.Client(), app.FilePaneHost("p1"))
	var listing gunim.Node
	framesUntil(t, "the file manager has its views", func() bool {
		listing = filemanager.FocusIn(lastUI, app.FilePaneViews("p1"))
		return listing != nil
	})
	settle()
	publish(inFront(st, "p1"))
	if !win.stage.cover.Active() || win.stage.from != -1 {
		t.Fatal("the file manager's tab did not come in")
	}
	framesUntil(t, "the listing has the keyboard", func() bool { return lastUI.Focused() == listing })
	settle()
	stage, _ := lastUI.Bounds(win.stage)
	if r, ok := lastUI.Bounds(win.fmHosts["p1"]); !ok || r != stage || win.stage.cover.Value() != 0 {
		t.Fatalf("settled, the file manager is at %v on a stage at %v, %v covered", r, stage, win.stage.cover.Value())
	}
	publish(inFront(st, "p2"))
	settle()
	if filemanager.FocusIn(lastUI, app.FilePaneViews("p1")) != listing {
		t.Fatal("parked after its tab came in, the file manager lost its views")
	}
}
