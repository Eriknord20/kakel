package view

import (
	"sync"
	"testing"
	"time"

	gi "github.com/marrasen/gunim/input"

	"github.com/marrasen/gunim"
	"github.com/marrasen/gunim/widget"

	"github.com/marrasen/kakel/app"
	"github.com/marrasen/kakel/settings"
)

// The Settings pane shows the sounds, the rings and the switches as
// they are, a box ticked saves at once, and Settings opens the pane.
func TestTheSettingsPaneShowsAndSaves(t *testing.T) {
	win, _, publish := windowStage(t)
	st := app.State{Panes: []app.Pane{{ID: "s", Title: "Settings", Kind: app.KindSettings}}, Stage: &app.Box{Pane: "s"}, Focus: "s"}
	st.Sounds = app.Alerts{Bell: true}
	st.Rings = app.Alerts{Connected: true, Lost: true, Finished: true, Bell: true, Other: true}
	st.InTray, st.SystemTitleBar = true, true
	st.Update = app.Update{Installed: true, Autostart: true, Updates: "install"}
	st.Themes, st.Theme = []string{"Dark", "Light"}, "Light"
	st.FontSize = 17
	st.LauncherFiles = settings.LauncherFilesWindow
	publish(st)
	p := win.settings
	if p == nil {
		t.Fatal("no Settings pane")
	}
	if !p.tray.Checked() || !p.autostart.Checked() || !p.titleBar.Checked() || p.updates.Selected() != 1 || p.theme.Selected() != 1 || p.size.Value() != 17 {
		t.Fatalf("the pane shows tray %v, autostart %v, title bar %v, updates %d, theme %d, size %v",
			p.tray.Checked(), p.autostart.Checked(), p.titleBar.Checked(), p.updates.Selected(), p.theme.Selected(), p.size.Value())
	}
	if p.launcherFiles.Selected() != 2 {
		t.Fatalf("set to open files in a window, the pane shows choice %d", p.launcherFiles.Selected())
	}
	boxes := []*widget.Checkbox{p.iface}
	for i := range p.sounds {
		boxes = append(boxes, p.sounds[i], p.rings[i])
	}
	on := make([]bool, len(boxes))
	for i, b := range boxes {
		on[i] = b.Checked()
	}
	want := []bool{false, false, true, false, true, false, true, true, true, false, true}
	for i := range want {
		if on[i] != want[i] {
			t.Fatalf("the checkboxes show %v, want %v", on, want)
		}
	}
	for len(lastWindow.Client().Intents()) > 0 {
		<-lastWindow.Client().Intents()
	}
	p.tabs.SetSelected(3, lastUI)
	lastWindow.Frame(time.Second / 60)
	lastUI.Focus(p.sounds[1])
	lastWindow.Input(gi.KeyPress{Key: gi.KeySpace})
	lastWindow.Frame(time.Second / 60)
	in, ok := nextIntent(t).(app.SaveLook)
	if !ok || in.Sounds != (app.Alerts{Lost: true, Bell: true}) || in.Rings != st.Rings || !in.SystemTitleBar {
		t.Fatalf("ticking the sound of a lost connection sent %+v", in)
	}
	if !win.run("app.settings", lastUI) {
		t.Fatal("Settings was not taken")
	}
	if got := nextIntent(t); got != (app.ShowSettings{}) {
		t.Fatalf("Settings sent %#v", got)
	}
}

// cueRecorder writes down the cues a window plays.
type cueRecorder struct {
	mu   sync.Mutex
	cues []gunim.Cue
}

func (r *cueRecorder) PlayCue(c gunim.Cue, _ float32) {
	r.mu.Lock()
	r.cues = append(r.cues, c)
	r.mu.Unlock()
}

func (r *cueRecorder) take() []gunim.Cue {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.cues
	r.cues = nil
	return out
}

// Each event plays its sound only where the settings ask for it: a
// connection lost does, one made does not, and a bell in the pane in
// front does, where nothing else would say it rang.
func TestAnEventSoundsAsTheSettingsSay(t *testing.T) {
	_, _, publish := windowStage(t)
	rec := &cueRecorder{}
	lastWindow.SetCues(rec)
	st := twoPanes("p1", nil)
	st.Sounds = app.Alerts{Lost: true, Bell: true}
	publish(st)
	rec.take()

	st.Pings.Lost++
	st.Pings.Connected++
	publish(st)
	if got := rec.take(); len(got) != 1 || got[0] != gunim.CueDisconnected {
		t.Fatalf("a connection made and one lost played %v, want the lost one's alone", got)
	}
	st.Bells++
	publish(st)
	if got := rec.take(); len(got) != 1 || got[0] != gunim.CueBell {
		t.Fatalf("a bell in the pane in front played %v", got)
	}
	st.Sounds = app.Alerts{}
	st.Bells++
	st.Pings.Lost++
	publish(st)
	if got := rec.take(); len(got) != 0 {
		t.Fatalf("with every sound off, %v played", got)
	}
}
