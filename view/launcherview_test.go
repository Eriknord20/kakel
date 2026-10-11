package view

import (
	"testing"
	"time"

	"github.com/marrasen/kakel/app"

	"github.com/marrasen/gunim"
	"github.com/marrasen/gunim/geom"
	"github.com/marrasen/gunim/gunimtest"
	gi "github.com/marrasen/gunim/input"
)

// launcherStage mounts the launcher on a window, with this computer and
// two servers, and returns it with its window.
func launcherStage(t *testing.T) (*Launcher, *gunim.Window) {
	t.Helper()
	w := gunimtest.New(t, LauncherSize, nil)
	var l *Launcher
	gunim.RegisterView(w, "launcher", func(app.LaunchState) *Launcher { l = NewLauncher(); return l },
		func(l *Launcher, st app.LaunchState, u *gunim.UI) { l.Update(st, u) })
	c := w.Client()
	if err := c.Mount(gunim.Root, "launcher", "launcher", app.LaunchState{}, app.LauncherTopic); err != nil {
		t.Fatal(err)
	}
	acts := []app.LaunchAction{{ID: "terminal", Title: "Terminal"}, {ID: "files", Title: "Files"}}
	st := app.LaunchState{Opened: 1, Machines: []app.LaunchMachine{
		{ID: "", Name: "This computer", Actions: acts},
		{ID: "a", Name: "prod-web", Actions: acts, Note: "connected"},
		{ID: "b", Name: "backup", Actions: acts, Default: 1},
	}}
	if err := c.Publish(app.LauncherTopic, st); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		w.Frame(time.Second / 60)
	}
	return l, w
}

// next is the launcher window's next intent.
func next(t *testing.T, w *gunim.Window) gunim.Intent {
	t.Helper()
	select {
	case env := <-w.Client().Intents():
		return env.Intent
	case <-time.After(time.Second):
		t.Fatal("the launcher sent nothing")
		return nil
	}
}

// Typing finds a machine, and Enter opens what was opened there last.
func TestTheLauncherFindsAMachineByItsName(t *testing.T) {
	_, w := launcherStage(t)
	for _, r := range "back" {
		w.Input(gi.TextInput{Text: string(r)})
	}
	w.Input(gi.KeyPress{Key: gi.KeyEnter})
	w.Frame(time.Second / 60)
	if in := next(t, w); in != (app.Launch{Machine: "b", Action: "files"}) {
		t.Fatalf("Enter sent %#v", in)
	}
}

// Tab lights the next of what can be opened on the machine lit, and
// Enter opens it; Escape closes the launcher.
func TestTabLightsTheNextChip(t *testing.T) {
	_, w := launcherStage(t)
	press := func(k gi.Key, mods gi.Mods) {
		w.Input(gi.KeyPress{Key: k, Mods: mods})
		w.Frame(time.Second / 60)
	}
	press(gi.KeyDown, 0)
	press(gi.KeyTab, 0)
	press(gi.KeyEnter, 0)
	if in := next(t, w); in != (app.Launch{Machine: "a", Action: "files"}) {
		t.Fatalf("Tab and Enter on the second machine sent %#v", in)
	}
	press(gi.KeyEscape, 0)
	if in := next(t, w); in != (app.CloseLauncher{}) {
		t.Fatalf("Escape sent %#v", in)
	}
}

// The chip lit at first is what Enter opens there, what was opened last;
// Tab goes on round from it, and Shift+Tab back.
func TestTheChipLitIsWhatEnterOpens(t *testing.T) {
	l, w := launcherStage(t)
	press := func(k gi.Key, mods gi.Mods) {
		w.Input(gi.KeyPress{Key: k, Mods: mods})
		w.Frame(time.Second / 60)
	}
	press(gi.KeyDown, 0)
	press(gi.KeyDown, 0)
	if l.chip != 1 {
		t.Fatalf("backup, last opened for its files, lights chip %d", l.chip)
	}
	press(gi.KeyTab, 0)
	if l.chip != 0 {
		t.Fatalf("Tab from the last chip lit %d, want the first", l.chip)
	}
	press(gi.KeyTab, gi.ModShift)
	press(gi.KeyEnter, 0)
	if in := next(t, w); in != (app.Launch{Machine: "b", Action: "files"}) {
		t.Fatalf("Shift+Tab back and Enter sent %#v", in)
	}
}

// A click on a chip of the line lit opens it.
func TestAClickOnAChipOpensIt(t *testing.T) {
	l, w := launcherStage(t)
	for range 30 {
		w.Frame(time.Second / 60)
	}
	if len(l.chipHits) != 2 {
		t.Fatalf("this computer shows %d chips, want 2", len(l.chipHits))
	}
	at := l.chipHits[1].Center()
	w.Input(gi.PointerDown{Pos: at, Button: gi.ButtonPrimary, Clicks: 1})
	w.Frame(time.Second / 60)
	if in := next(t, w); in != (app.Launch{Machine: "", Action: "files"}) {
		t.Fatalf("a click on Files sent %#v", in)
	}
}

// Typing lights the best of what it finds, wherever the light was.
func TestTypingLightsTheBestMatch(t *testing.T) {
	_, w := launcherStage(t)
	w.Input(gi.KeyPress{Key: gi.KeyDown})
	w.Input(gi.KeyPress{Key: gi.KeyDown})
	w.Input(gi.TextInput{Text: "p"})
	w.Input(gi.KeyPress{Key: gi.KeyEnter})
	w.Frame(time.Second / 60)
	if in := next(t, w); in != (app.Launch{Machine: "a", Action: "terminal"}) {
		t.Fatalf("Enter sent %#v", in)
	}
}

// The light and its chip stay on their machine as the machines come in
// another order, and a right click picks nothing.
func TestTheLauncherKeepsItsMachine(t *testing.T) {
	l, w := launcherStage(t)
	w.Input(gi.KeyPress{Key: gi.KeyDown})
	w.Frame(time.Second / 60)
	w.Input(gi.KeyPress{Key: gi.KeyTab})
	st := l.st
	st.Machines = []app.LaunchMachine{st.Machines[0], st.Machines[2], st.Machines[1]}
	if err := w.Client().Publish(app.LauncherTopic, st); err != nil {
		t.Fatal(err)
	}
	w.Frame(time.Second / 60)
	w.Input(gi.KeyPress{Key: gi.KeyEnter})
	w.Frame(time.Second / 60)
	if in := next(t, w); in != (app.Launch{Machine: "a", Action: "files"}) {
		t.Fatalf("Enter sent %#v", in)
	}
	w.Input(gi.PointerDown{Pos: geom.Pt(100, l.listTop+5), Button: gi.ButtonSecondary})
	w.Frame(time.Second / 60)
	select {
	case env := <-w.Client().Intents():
		t.Fatalf("a right click sent %#v", env.Intent)
	case <-time.After(50 * time.Millisecond):
	}
}

// Typing finds more than machines: a shell here by its name or a word
// for it, files on a machine, and Enter opens it.
func TestTheLauncherFindsShellsAndFiles(t *testing.T) {
	l, w := launcherStage(t)
	st := l.st
	st.Things = []app.LaunchThing{
		{Title: "Command Prompt", Note: "This computer", Also: []string{"cmd"}, Kind: "terminal", Action: "shell:cmd"},
		{Title: "Ubuntu (WSL)", Note: "This computer", Also: []string{"wsl:Ubuntu", "wsl"}, Kind: "terminal", Action: "shell:wsl:Ubuntu"},
		{Title: "Files on backup", Kind: "files", Machine: "b", Action: "files"},
	}
	if err := w.Client().Publish(app.LauncherTopic, st); err != nil {
		t.Fatal(err)
	}
	w.Frame(time.Second / 60)
	typeIn := func(s string) {
		for _, r := range s {
			w.Input(gi.TextInput{Text: string(r)})
		}
		w.Input(gi.KeyPress{Key: gi.KeyEnter})
		w.Frame(time.Second / 60)
	}
	typeIn("wsl")
	if in := next(t, w); in != (app.Launch{Action: "shell:wsl:Ubuntu"}) {
		t.Fatalf("wsl and Enter sent %#v", in)
	}
	l.field.SetText("", nil)
	typeIn("files on b")
	if in := next(t, w); in != (app.Launch{Machine: "b", Action: "files"}) {
		t.Fatalf("files on b and Enter sent %#v", in)
	}
}

// Right and Left move along the chips once the field has nothing more
// to move its caret through, and stop at the ends.
func TestRightAndLeftMoveAlongTheChips(t *testing.T) {
	l, w := launcherStage(t)
	press := func(k gi.Key) {
		w.Input(gi.KeyPress{Key: k})
		w.Frame(time.Second / 60)
	}
	press(gi.KeyDown)
	press(gi.KeyRight)
	press(gi.KeyRight)
	if l.chip != 1 {
		t.Fatalf("Right twice lit chip %d, want the last, 1", l.chip)
	}
	press(gi.KeyLeft)
	press(gi.KeyEnter)
	if in := next(t, w); in != (app.Launch{Machine: "a", Action: "terminal"}) {
		t.Fatalf("Right, Left and Enter sent %#v", in)
	}
}

// The light slides from chip to chip, over frames; and where the chips
// are too many to fit, as this computer's shells can be, they scroll to
// show the one lit.
func TestTheChipLightSlidesAndTheChipsScroll(t *testing.T) {
	l, w := launcherStage(t)
	st := l.st
	var many []app.LaunchAction
	for _, name := range []string{"PowerShell 7", "Windows PowerShell", "Command Prompt", "Ubuntu (WSL)", "Debian (WSL)", "Files"} {
		many = append(many, app.LaunchAction{ID: "x:" + name, Title: name})
	}
	st.Machines[0].Actions = many
	if err := w.Client().Publish(app.LauncherTopic, st); err != nil {
		t.Fatal(err)
	}
	frames := func(n int) {
		for range n {
			w.Frame(time.Second / 60)
		}
	}
	frames(30)
	from := l.chipAt.Value()
	w.Input(gi.KeyPress{Key: gi.KeyTab})
	// Painted toward the next chip, and a frame on its way.
	frames(2)
	to := l.chipAt.Target()
	if mid := l.chipAt.Value(); !(mid > from && mid < to) {
		t.Fatalf("a frame after Tab the light is at %v, want on its way from %v to %v", mid, from, to)
	}
	if l.chipOff.Value() != 0 {
		t.Fatalf("the first chips are scrolled by %v", l.chipOff.Value())
	}
	for range len(many) - 2 {
		w.Input(gi.KeyPress{Key: gi.KeyTab})
		frames(2)
	}
	frames(40)
	last := l.chipHits[len(l.chipHits)-1]
	if l.chipOff.Value() <= 0 || last.Size().W < 20 {
		t.Fatalf("with the last chip lit the chips are scrolled by %v, and it shows %v wide", l.chipOff.Value(), last.Size().W)
	}
}
