package view

import (
	"testing"
	"time"

	"github.com/marrasen/gunim/geom"
	gi "github.com/marrasen/gunim/input"

	"github.com/marrasen/kakel/app"
)

// splitTabs is a window of two tabs: Jobs and Log side by side, and
// Help on its own.
func splitTabs() app.State {
	split := &app.Box{ID: "s1", Share: 0.5, A: &app.Box{Pane: "p1"}, B: &app.Box{Pane: "p2"}}
	help := &app.Box{Pane: "p3"}
	return app.State{
		Window: 1,
		Panes: []app.Pane{
			{ID: "p1", Title: "Jobs", Kind: app.KindJobs},
			{ID: "p2", Title: "Log", Kind: app.KindLog},
			{ID: "p3", Title: "Help", Kind: app.KindHelp},
		},
		Stage: split, Focus: "p1",
		Groups: map[string]*app.Box{"p1": split, "p2": split, "p3": help},
		Tabs:   []app.Tab{{Group: 1, Pane: "p1", Panes: 2}, {Group: 2, Pane: "p3", Panes: 1}},
	}
}

// The panes of a split each have a line naming them, the one with the
// keyboard marked; a pane alone has none, and nor does the Machines
// pane beside it.
func TestPanesInASplitAreNamed(t *testing.T) {
	win, _, publish := windowStage(t)
	publish(splitTabs())
	for _, id := range []string{"p1", "p2"} {
		c := win.captions[id]
		if c == nil || win.paneNode(id) != c {
			t.Fatalf("%s in a split has no line", id)
		}
		if c.bar.front != (id == "p1") {
			t.Fatalf("%s's line marked %v, with p1 having the keyboard", id, c.bar.front)
		}
	}
	if got := win.captions["p1"].bar.label.Text; got != "Jobs" {
		t.Fatalf("p1's line reads %q", got)
	}
	alone := splitTabs()
	alone.Stage, alone.Focus = alone.Groups["p3"], "p3"
	publish(alone)
	if _, ok := win.stage.shown.(*captioned); ok {
		t.Fatal("a pane alone on stage has a line")
	}
	beside := withServers(alone)
	publish(beside)
	if _, ok := win.paneNode("p3").(*captioned); ok {
		t.Fatal("a pane beside the Machines pane alone has a line")
	}
}

// A pane dragged by its line docks beside another, on the side it is
// let go nearest; let go above the stage it takes a tab of its own; let
// go over itself it stays.
func TestAPaneDraggedByItsLineGoesWhereItIsLetGo(t *testing.T) {
	win, _, publish := windowStage(t)
	publish(splitTabs())
	drain()
	r, ok := lastUI.Bounds(win.paneNode("p2"))
	if !ok {
		t.Fatal("p2 is not drawn")
	}
	p1 := app.PaneDrag{Pane: "p1", Window: 1}
	bottom := geom.Pt(r.Center().X, r.Max.Y-10)
	lastWindow.Input(gi.DragOver{Pos: bottom, Data: p1})
	lastWindow.Frame(time.Second / 60)
	if win.dock.pane != "p2" || !win.dock.vertical || win.dock.first {
		t.Fatalf("over p2's bottom edge, the dock is %+v", win.dock)
	}
	lastWindow.Input(gi.Drop{Pos: bottom, Data: p1})
	lastWindow.Frame(time.Second / 60)
	if in, ok := nextIntent(t).(app.DockPane); !ok || in.Pane != "p1" || in.Beside != "p2" || !in.Vertical || in.First {
		t.Fatalf("let go at p2's bottom edge, the window sent %#v", in)
	}

	own, _ := lastUI.Bounds(win.paneNode("p1"))
	if win.paneDrop(gi.Drop{Pos: own.Center(), Data: p1}, lastUI) {
		t.Fatal("let go over itself, the pane went somewhere")
	}

	stage, _ := lastUI.Bounds(win.stage)
	above := geom.Pt(stage.Max.X-20, stage.Min.Y-3)
	if !win.paneDrop(gi.DragOver{Pos: above, Data: p1}, lastUI) || !win.dock.toTab {
		t.Fatalf("above the stage, the dock is %+v", win.dock)
	}
	if !win.paneDrop(gi.Drop{Pos: above, Data: p1}, lastUI) {
		t.Fatal("let go above the stage, the window took nothing")
	}
	if in, ok := nextIntent(t).(app.PaneToTab); !ok || in.Pane != "p1" || !in.Bar || in.Window != 1 {
		t.Fatalf("let go above the stage, the window sent %#v", in)
	}
}

// A pane let go on the tab bar takes a tab of its own where it lands.
func TestAPaneLetGoOnTheBarLandsThere(t *testing.T) {
	win, _, publish := windowStage(t)
	publish(splitTabs())
	drain()
	first := tabAt(t, win, 0)
	lastWindow.Input(gi.Drop{Pos: first.Sub(geom.Pt(20, 0)), Data: app.PaneDrag{Pane: "p2", Window: 1}})
	lastWindow.Frame(time.Second / 60)
	if in, ok := nextIntent(t).(app.PaneToTab); !ok || in.Pane != "p2" || !in.Bar || in.Before != 1 {
		t.Fatalf("let go before the first tab, the bar sent %#v", in)
	}
}

// A pane's line, dragged, lifts the pane off; a click gives it the
// keyboard; let go outside every window, it asks for a window of its
// own, as large as this one.
func TestALineCarriesItsPaneOff(t *testing.T) {
	win, _, publish := windowStage(t)
	publish(splitTabs())
	drain()
	bar := win.captions["p2"].bar
	r, ok := lastUI.Bounds(bar)
	if !ok || r.Size().H != captionHeight {
		t.Fatalf("p2's line is at %v", r)
	}
	click(r.Center(), gi.ButtonPrimary)
	if in := nextIntent(t); in != (app.FocusPane{Pane: "p2"}) {
		t.Fatalf("a click on the line sent %#v", in)
	}
	lastWindow.Input(gi.PointerDown{Pos: r.Center(), Button: gi.ButtonPrimary})
	lastWindow.Input(gi.PointerMove{Pos: r.Center().Add(geom.Pt(30, 40))})
	lastWindow.Frame(time.Second / 60)
	if !bar.carried {
		t.Fatal("dragged, the line did not lift its pane off")
	}
	bar.Handle(gi.DragEnd{Out: true, At: geom.Pt(1200, 300)}, lastUI)
	in, ok := nextIntent(t).(app.PaneToNewWindow)
	if !ok || in.Pane != "p2" || in.At != geom.Pt(1200, 300).Sub(bar.grab) || in.Size != win.size {
		t.Fatalf("let go outside, the line sent %#v", in)
	}
	if bar.carried || bar.pressed {
		t.Fatal("let go, the pane is still carried")
	}
}

// A middle click on a pane's line closes the pane, as it closes a tab.
func TestAMiddleClickOnALineClosesItsPane(t *testing.T) {
	win, _, publish := windowStage(t)
	publish(splitTabs())
	drain()
	r, ok := lastUI.Bounds(win.captions["p2"].bar)
	if !ok {
		t.Fatal("p2 has no line")
	}
	click(r.Center(), gi.ButtonMiddle)
	if in := nextIntent(t); in != (app.ClosePane{Pane: "p2"}) {
		t.Fatalf("a middle click on the line sent %#v", in)
	}
}

// A pane or a tab dragged and held over another tab brings that tab to
// the front, so it can be docked beside a pane there; one only passing
// over leaves it be. A tab let go where nothing takes it puts back the
// tab that was in front.
func TestADragHeldOverATabOpensIt(t *testing.T) {
	win, _, publish := windowStage(t)
	publish(splitTabs())
	drain()
	// In the bar's own space, as the drag reaches it.
	sr, _ := win.tabs.bar.TabRect("2")
	second := sr.Center()
	over := func(at geom.Point, data any) {
		if !win.tabs.bar.Handle(gi.DragOver{Pos: at, Data: data}, lastUI) {
			win.tabs.bar.Handle(gi.DragLeave{}, lastUI)
		}
		lastWindow.Frame(time.Second / 60)
	}

	// Passing over: nothing.
	over(second, app.PaneDrag{Pane: "p2", Window: 1})
	lastWindow.Frame(springHold / 2)
	over(second.Add(geom.Pt(0, 200)), app.PaneDrag{Pane: "p2", Window: 1})
	lastWindow.Frame(springHold)
	lastWindow.Frame(time.Second / 60)
	if len(lastWindow.Client().Intents()) != 0 {
		t.Fatalf("a drag passing over a tab sent %#v", nextIntent(t))
	}

	// Held there: it comes to the front.
	over(second, app.PaneDrag{Pane: "p2", Window: 1})
	lastWindow.Frame(springHold)
	lastWindow.Frame(time.Second / 60)
	if in := nextIntent(t); in != (app.ShowTab{Group: 2}) {
		t.Fatalf("a drag held over the second tab sent %#v", in)
	}
	win.tabs.bar.Handle(gi.DragLeave{}, lastUI)

	// A tab carried there and given up: the first tab, in front before,
	// comes back.
	publish(splitTabs())
	drain()
	from := tabAt(t, win, 0)
	lastWindow.Input(gi.PointerMove{Pos: from})
	lastWindow.Input(gi.PointerDown{Pos: from, Button: gi.ButtonPrimary})
	lastWindow.Input(gi.PointerMove{Pos: from.Add(geom.Pt(0, 12))})
	lastWindow.Frame(time.Second / 60)
	lastWindow.Input(gi.PointerMove{Pos: tabAt(t, win, 1)})
	lastWindow.Frame(time.Second / 60)
	lastWindow.Frame(springHold)
	lastWindow.Frame(time.Second / 60)
	if in := nextIntent(t); in != (app.ShowTab{Group: 2}) {
		t.Fatalf("a tab held over the second tab sent %#v", in)
	}
	lastWindow.Input(gi.KeyPress{Key: gi.KeyEscape})
	lastWindow.Frame(time.Second / 60)
	if in := nextIntent(t); in != (app.ShowTab{Group: 1}) {
		t.Fatalf("given up where nothing took it, the bar sent %#v", in)
	}
}

// springHold is how long a drag is held over a tab before the tab comes
// to the front, as gunim's tab bar has it.
const springHold = 500 * time.Millisecond
