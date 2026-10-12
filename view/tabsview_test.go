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
	"github.com/marrasen/gunim/text"
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

// tabAt is the middle of tab i, in the window's space.
func tabAt(t *testing.T, win *Window, i int) geom.Point {
	t.Helper()
	r, ok := lastUI.Bounds(win.tabs)
	if !ok || i >= len(win.tabs.boxes) {
		t.Fatalf("the tab bar is not drawn, or has no tab %d", i)
	}
	return win.tabs.boxes[i].Center().Add(r.Min)
}

// click presses and lets go the primary button at p.
func click(p geom.Point, b gi.Button) {
	lastWindow.Input(gi.PointerDown{Pos: p, Button: b})
	lastWindow.Input(gi.PointerUp{Pos: p, Button: b})
	lastWindow.Frame(time.Second / 60)
}

// The bar shows with two tabs and names the panes in place of the title
// bar; with one it goes, and the pane's title comes back. Each way, what
// shows fades out before the title bar changes, and what comes fades in.
func TestTheTabBarShowsWithTwoTabs(t *testing.T) {
	win, _, publish := windowStage(t)
	b := win.tabs
	frames := func(n int) {
		for range n {
			lastWindow.Frame(time.Second / 60)
		}
	}
	publish(twoTabs())
	frames(2)
	if b.shown() || b.titleFade.Value() >= 1 {
		t.Fatalf("a frame after the second tab came, the bar shows %v and the title is %v there", b.shown(), b.titleFade.Value())
	}
	settle()
	if !b.shown() || win.bar.Subtitle != "" || win.bar.Title != "" || b.fade.Value() != 1 {
		t.Fatalf("with two tabs the bar shows %v, %v faded in, and the title bar says %q, %q", b.shown(), b.fade.Value(), win.bar.Title, win.bar.Subtitle)
	}
	if b.front != 1 {
		t.Fatalf("the tab in front is %d, want 1", b.front)
	}
	st := twoTabs()
	st.Panes, st.Tabs = st.Panes[:1], st.Tabs[:1]
	publish(st)
	frames(2)
	if !b.shown() || b.fade.Value() >= 1 || b.fade.Value() <= 0 || len(b.gone) != 1 {
		t.Fatalf("a frame after the second tab closed, the bar shows %v, %v faded, with %d tabs going", b.shown(), b.fade.Value(), len(b.gone))
	}
	frames(10)
	if b.shown() || win.bar.Title != app.ProgramName || b.titleFade.Value() >= 1 {
		t.Fatalf("once the bar has faded, it shows %v, and the title %q is %v there, want coming", b.shown(), win.bar.Title, b.titleFade.Value())
	}
	settle()
	if b.shown() || win.bar.Subtitle != "Jobs" || win.bar.Title != app.ProgramName || b.titleFade.Value() != 1 {
		t.Fatalf("with one tab the bar shows %v, the title bar says %q, %q, %v there", b.shown(), win.bar.Title, win.bar.Subtitle, b.titleFade.Value())
	}
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
	r, _ := lastUI.Bounds(win.tabs)
	click(win.tabs.crossOf(0).Center().Add(r.Min), gi.ButtonPrimary)
	if in, ok := nextIntent(t).(app.CloseTab); !ok || in.Group != 1 {
		t.Fatalf("a click on the × sent %#v", in)
	}
	click(win.tabs.plus.Center().Add(r.Min), gi.ButtonPrimary)
	if in, ok := nextIntent(t).(app.NewTab); !ok {
		t.Fatalf("a click on the + sent %#v", in)
	}
	// A right click offers a terminal or a file manager.
	click(win.tabs.plus.Center().Add(r.Min), gi.ButtonSecondary)
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
	lastWindow.Input(gi.PointerDown{Pos: at, Button: gi.ButtonPrimary})
	lastWindow.Input(gi.PointerMove{Pos: at.Add(geom.Pt(20, 30))})
	lastWindow.Frame(time.Second / 60)
	if win.tabs.carried != 2 {
		t.Fatalf("the tab did not lift off: %d is carried", win.tabs.carried)
	}
	win.tabs.Handle(gi.DragEnd{Out: true, At: geom.Pt(1200, 300)}, lastUI)
	in, ok := nextIntent(t).(app.TabToNewWindow)
	if !ok || in.Group != 2 || in.At != geom.Pt(1200, 300).Sub(win.tabs.grab) || in.Size != win.size {
		t.Fatalf("let go outside, the bar sent %#v", in)
	}
	if win.tabs.carried != 0 {
		t.Fatal("the tab is still carried")
	}
}

// A tab dropped on the bar lands before the tab whose middle is past
// the pointer, or last.
func TestATabDroppedOnTheBarMovesThere(t *testing.T) {
	win, _, publish := windowStage(t)
	publish(twoTabs())
	drain()
	r, _ := lastUI.Bounds(win.tabs)
	first := win.tabs.boxes[0]
	lastWindow.Input(gi.Drop{Pos: geom.Pt(first.Min.X+4, first.Center().Y).Add(r.Min), Data: app.TabDrag{Group: 2, Window: 1}})
	lastWindow.Frame(time.Second / 60)
	if in, ok := nextIntent(t).(app.MoveTab); !ok || in.Group != 2 || in.Before != 1 {
		t.Fatalf("dropped before the first tab, the bar sent %#v", in)
	}
	lastWindow.Input(gi.Drop{Pos: geom.Pt(win.tabs.plus.Min.X, first.Center().Y).Add(r.Min), Data: app.TabDrag{Group: 9, Window: 3}})
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
	if !win.tabDrop(gi.DragOver{Pos: left, Data: app.TabDrag{Group: 2, Window: 1}}, lastUI) || win.dock.pane != "p1" || !win.dock.first || win.dock.vertical {
		t.Fatalf("over the pane's left edge, the dock is %+v", win.dock)
	}
	lastWindow.Input(gi.Drop{Pos: geom.Pt(r.Center().X, r.Max.Y-10), Data: app.TabDrag{Group: 2, Window: 1}})
	lastWindow.Frame(time.Second / 60)
	if in, ok := nextIntent(t).(app.DockTab); !ok || in.Group != 2 || in.Beside != "p1" || !in.Vertical || in.First {
		t.Fatalf("dropped at the pane's bottom edge, the window sent %#v", in)
	}
	if win.tabDrop(gi.DragOver{Pos: left, Data: app.TabDrag{Group: 1, Window: 1}}, lastUI) {
		t.Fatal("the window took a tab over its own pane")
	}
}

// A tab carried away while its window drops to one tab still hears how
// the drag ends, and the bar works when it shows again.
func TestTheBarWorksAfterHidingMidDrag(t *testing.T) {
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
	if win.tabs.carried != 0 || win.tabs.pressed != -1 {
		t.Fatalf("hidden, the bar still carries %d, pressed %d", win.tabs.carried, win.tabs.pressed)
	}
	publish(twoTabs())
	drain()
	click(tabAt(t, win, 1), gi.ButtonPrimary)
	if in := nextIntent(t); in != (app.ShowTab{Group: 2}) {
		t.Fatalf("shown again, a click sent %#v", in)
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
	r, _ := lastUI.Bounds(win.tabs)
	if win.tabs.plus.Max.X > r.Size().W {
		t.Fatalf("the + ends at %.0f on a bar %.0f wide", win.tabs.plus.Max.X, r.Size().W)
	}
	if got := win.tabs.CaptionRects(r.Size()); len(got) != 1 || got[0].Size().W < captionLeast-1 {
		t.Fatalf("the bar leaves %v to move the window by", got)
	}
	icon := win.tabs.boxes[3].Center().Add(r.Min)
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

// threeTabs is twoTabs with a third tab, p3's, last.
func threeTabs() app.State {
	st := twoTabs()
	b3 := &app.Box{Pane: "p3"}
	st.Panes = append(st.Panes, app.Pane{ID: "p3", Title: "Files", Kind: app.KindJobs})
	st.Groups["p3"] = b3
	st.Tabs = append(st.Tabs, app.Tab{Group: 3, Pane: "p3", Panes: 1})
	return st
}

// A tab opened grows in where it lands, and one closed shrinks away
// while the tabs after it slide into its place. The window's first
// tabs are simply there.
func TestTabsOpenAndCloseInMotion(t *testing.T) {
	win, _, publish := windowStage(t)
	publish(twoTabs())
	settle()
	b := win.tabs
	if r, open := b.drawnAt(1); r != b.boxes[1] || open != 1 {
		t.Fatalf("a first tab is drawn at %v, %v open, not where it is", r, open)
	}
	publish(threeTabs())
	lastWindow.Frame(time.Second / 60)
	lastWindow.Frame(time.Second / 60)
	r, open := b.drawnAt(2)
	if r.Size().W >= b.boxes[2].Size().W || open >= 1 || r.Min.X != b.boxes[2].Min.X {
		t.Fatalf("an opened tab starts at %v, %v open, want narrower than %v", r, open, b.boxes[2])
	}
	settle()
	if r, open := b.drawnAt(2); r != b.boxes[2] || open != 1 {
		t.Fatalf("settled, the opened tab is at %v, %v open, want %v", r, open, b.boxes[2])
	}
	// The middle tab closes: it shrinks where it was, and the last
	// slides left into its place.
	was := b.boxes[2]
	st := threeTabs()
	st.Panes = append(st.Panes[:1], st.Panes[2])
	st.Tabs = append(st.Tabs[:1], st.Tabs[2])
	delete(st.Groups, "p2")
	publish(st)
	lastWindow.Frame(time.Second / 60)
	lastWindow.Frame(time.Second / 60)
	if len(b.gone) != 1 || b.gone[0].tab.Group != 2 {
		t.Fatalf("closing, the bar draws %d gone tabs", len(b.gone))
	}
	if r, _ := b.drawnAt(1); r.Min.X <= b.boxes[1].Min.X || r.Min.X >= was.Min.X {
		t.Fatalf("the last tab is at %v, want between %v and %v", r, was, b.boxes[1])
	}
	settle()
	if len(b.gone) != 0 {
		t.Fatalf("settled, %d closed tabs are still drawn", len(b.gone))
	}
	if r, _ := b.drawnAt(1); r != b.boxes[1] {
		t.Fatalf("settled, the last tab is at %v, want %v", r, b.boxes[1])
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
	if s.cover.Value() != 0 || b.lit.Value() != 1 {
		t.Fatalf("the window's first tabs move: the stage is %v covered, the front tab %v lit", s.cover.Value(), b.lit.Value())
	}
	publish(inFront(st, "p2"))
	if b.front != 2 || s.from != 1 {
		t.Fatalf("switched, the tab in front is %d, coming from %v", b.front, s.from)
	}
	if c := s.cover.Value(); c <= 0 || c >= 1 || !s.cover.Active() {
		t.Fatalf("a moment after the switch, the stage is %v covered", c)
	}
	if l := b.lit.Value(); l <= 0 || l >= 1 {
		t.Fatalf("a moment after the switch, the front tab's look is %v of the way", l)
	}
	if r := b.litAt(); r.Min.X <= b.boxes[0].Min.X || r.Min.X >= b.boxes[1].Min.X {
		t.Fatalf("the front tab's fill is at %v, want between %v and %v", r, b.boxes[0], b.boxes[1])
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
	if s.cover.Value() != 0 || s.cover.Active() || b.lit.Value() != 1 || b.lit.Active() || s.Step(time.Second/60) {
		t.Fatalf("settled, the stage is %v covered and the front tab %v lit", s.cover.Value(), b.lit.Value())
	}
	if r := b.litAt(); r != b.boxes[1] {
		t.Fatalf("settled, the front tab's fill is at %v, want %v", r, b.boxes[1])
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
	if win.tabs.carried != 1 {
		t.Fatalf("the tab did not lift off: %d is carried", win.tabs.carried)
	}
	win.tabs.Handle(gi.DragEnd{Out: true, At: geom.Pt(1200, 300)}, lastUI)
	// The program moves the tab to a window of its own.
	st := inFront(twoTabs(), "p2")
	st.Panes, st.Tabs = st.Panes[1:], st.Tabs[1:]
	delete(st.Groups, "p1")
	publish(st)
	if win.stage.cover.Value() != 0 || win.tabs.lit.Value() != 1 {
		t.Fatalf("dragged out, the stage is %v covered and the front tab %v lit", win.stage.cover.Value(), win.tabs.lit.Value())
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

// A tab's title that does not fit gives way to its short one, a
// server's folder without the server, and that ends in an ellipsis
// where it does not fit either: the end of a title cut off said nothing,
// and "Skylake: " was all a server's tab showed.
func TestATabTitleThatDoesNotFitKeepsItsFolder(t *testing.T) {
	const size = 13
	shape := func(s string) text.Run { return text.Default().Shape(s, size) }
	b := &tabBar{shapedSize: size, titles: []text.Run{shape("Skylake: build")}, names: []string{"Skylake: build"}, shorts: []string{"build"}}
	if got := b.fitTitle(0, 500); got.Advance != shape("Skylake: build").Advance {
		t.Errorf("with room the tab says %v wide, want all of its title", got.Advance)
	}
	short := shape("build").Advance
	if got := b.fitTitle(0, short+2); got.Advance != short {
		t.Errorf("with room for the folder the tab says %v wide, want the folder's %v", got.Advance, short)
	}
	if got, least := b.fitTitle(0, short-4), shape("b…").Advance; got.Advance > short-4 || got.Advance < least {
		t.Errorf("with less room the tab says %v wide, want the folder cut with an ellipsis, %v to %v", got.Advance, least, short-4)
	}
	b.shorts[0] = ""
	if got := b.fitTitle(0, 60); got.Advance > 60 {
		t.Errorf("a title with nothing shorter is %v wide in 60, want it cut to fit", got.Advance)
	}
}
