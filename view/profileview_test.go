package view

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	gi "github.com/marrasen/gunim/input"
	"github.com/marrasen/gunim/widget"

	"github.com/marrasen/kakel/app"
	"github.com/marrasen/kakel/machines"
	"github.com/marrasen/kakel/remote"
	"github.com/marrasen/kakel/settings"
)

// gitBash is a profile whose program is in a folder with a space in
// its name, as Git Bash is under Program Files, starting in a folder
// of its own with two variables.
func gitBash(t *testing.T) settings.Profile {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "Program Files")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	bash := filepath.Join(bin, "bash.exe")
	if err := os.WriteFile(bash, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	return settings.Profile{
		ID:      "profile:git-bash",
		Name:    "Git Bash",
		Command: []string{bash, "-i", "-l"},
		Folder:  filepath.Clean(t.TempDir()),
		Env:     []string{"FOO=bar", "GREETING=hello there"},
	}
}

// profileFields are the profile dialog's name, command line and folder
// fields, and its environment.
func profileFields(t *testing.T, win *Window) (name, line, dir *widget.TextField, env *widget.TextArea) {
	t.Helper()
	var fields []*widget.TextField
	for _, f := range formOf(win.dialog.Body).Children() {
		switch f := f.(type) {
		case *widget.TextField:
			fields = append(fields, f)
		case *widget.TextArea:
			env = f
		}
	}
	if len(fields) != 3 || env == nil {
		t.Fatalf("the profile dialog has %d fields and environment %v", len(fields), env)
	}
	return fields[0], fields[1], fields[2], env
}

// A profile saved as it opened is the profile it was: the program's
// path keeps its space, and the ID, the folder and the variables stay.
func TestAProfileSavedAsItOpenedIsUnchanged(t *testing.T) {
	win, _, publish := windowStage(t)
	publish(app.State{})
	was := gitBash(t)
	have := []settings.Profile{was}
	win.profileDialog(have, &have[0], lastUI)
	if msg := win.dialog.Check(); msg != "" {
		t.Fatalf("the profile as it was is refused: %s", msg)
	}
	in, ok := win.dialog.OnAccept(lastUI).(app.SaveProfile)
	if !ok || in.Profile.ID != was.ID || in.Profile.Name != was.Name || in.Profile.Folder != was.Folder ||
		!slices.Equal(in.Profile.Command, was.Command) || !slices.Equal(in.Profile.Env, was.Env) {
		t.Fatalf("saved untouched, the profile is %+v, want %+v", in.Profile, was)
	}
}

// A new profile may not take the name of one there is, whatever its
// case, and is saved with no ID for the program to give it one.
func TestANewProfileTakesANameOfItsOwn(t *testing.T) {
	win, _, publish := windowStage(t)
	publish(app.State{})
	have := gitBash(t)
	win.profileDialog([]settings.Profile{have}, nil, lastUI)
	name, line, _, _ := profileFields(t, win)
	name.SetText("git bash", nil)
	line.SetText(`"`+have.Command[0]+`" -i`, nil)
	if msg := win.dialog.Check(); msg == "" {
		t.Fatal("a second profile called Git Bash was taken")
	}
	name.SetText("Git Bash, login", nil)
	if msg := win.dialog.Check(); msg != "" {
		t.Fatalf("a profile with a name of its own is refused: %s", msg)
	}
	in, ok := win.dialog.OnAccept(lastUI).(app.SaveProfile)
	if !ok || in.Profile.ID != "" || !slices.Equal(in.Profile.Command, []string{have.Command[0], "-i"}) {
		t.Fatalf("the new profile saved is %+v", in.Profile)
	}
}

// Edit Profile… and Remove Profile… take no clicks with no profile to
// act on, and the profile picked to remove is the one removed.
func TestTheProfilePickedIsTheOneRemoved(t *testing.T) {
	win, _, publish := windowStage(t)
	st := app.State{Panes: []app.Pane{{ID: "s", Title: "Settings", Kind: app.KindSettings}}, Stage: &app.Box{Pane: "s"}, Focus: "s"}
	publish(st)
	p := win.settings
	if p == nil {
		t.Fatal("no Settings pane")
	}
	if !p.editP.Disabled || !p.removeP.Disabled {
		t.Fatalf("with no profiles, Edit is disabled %v and Remove %v", p.editP.Disabled, p.removeP.Disabled)
	}
	st.Profiles = []settings.Profile{
		{ID: "profile:git-bash", Name: "Git Bash", Command: []string{"bash.exe"}},
		{ID: "profile:nu", Name: "Nushell", Command: []string{"nu.exe"}},
	}
	publish(st)
	if p.editP.Disabled || p.removeP.Disabled {
		t.Fatalf("with profiles, Edit is disabled %v and Remove %v", p.editP.Disabled, p.removeP.Disabled)
	}
	for len(lastWindow.Client().Intents()) > 0 {
		<-lastWindow.Client().Intents()
	}
	win.removeProfilePickerFrom(st.Profiles, lastUI)
	lastWindow.Frame(time.Second / 60)
	for _, k := range []gi.Key{gi.KeyDown, gi.KeyEnter} {
		lastWindow.Input(gi.KeyPress{Key: k, Time: time.Now()})
		lastWindow.Frame(time.Second / 60)
	}
	if in, ok := nextIntent(t).(app.RemoveProfile); !ok || in.ID != "profile:nu" {
		t.Fatalf("picking the second profile sent %#v", in)
	}
}

// dropdownIn is the drop-down labelled label in the open dialog's form,
// or nil.
func dropdownIn(win *Window, label string) *widget.Dropdown {
	for _, f := range formOf(win.dialog.Body).Children() {
		if d, ok := f.(*widget.Dropdown); ok && d.Label == label {
			return d
		}
	}
	return nil
}

// A profile made for a saved server is that server's: its program is
// not looked for here, and its folder is a path there. A kakel window
// takes no profile. Edited, a profile stays on its machine.
func TestAProfileIsMadeOnTheMachineChosen(t *testing.T) {
	win, _, publish := windowStage(t)
	publish(app.State{Saved: []remote.Host{{ID: "s1", Name: "prod", Address: "prod.example"}, {ID: "w1", Name: "desk", Address: "desk.example", Window: true}},
		Machines: []machines.Info{{ID: "s1", Name: "prod"}, {ID: "w1", Name: "desk", Window: true}}})
	win.profileDialog(nil, nil, lastUI)
	machine := dropdownIn(win, "Machine")
	if machine == nil || len(machine.Items()) != 2 || machine.Items()[1].Label != "prod" {
		t.Fatalf("the Machine drop-down is %+v", machine)
	}
	machine.SetSelected(1, nil)
	name, line, dir, env := profileFields(t, win)
	name.SetText("tmux", nil)
	line.SetText("kakel-test-not-here new -A -s main", nil)
	dir.SetText("src", nil)
	if msg := win.dialog.Check(); msg == "" {
		t.Fatal("a folder on a server not from / or ~ was taken")
	}
	dir.SetText("~/src", nil)
	env.SetText("FOO=bar", nil)
	if msg := win.dialog.Check(); msg != "" {
		t.Fatalf("a profile on prod is refused: %s", msg)
	}
	in, ok := win.dialog.OnAccept(lastUI).(app.SaveProfile)
	if !ok || in.Profile.HostID != "s1" || in.Profile.Folder != "~/src" || in.Profile.Command[0] != "kakel-test-not-here" {
		t.Fatalf("the profile saved is %+v", in.Profile)
	}
	have := []settings.Profile{{ID: "profile:tmux", HostID: "s1", Name: "tmux", Command: []string{"tmux"}}}
	win.profileDialog(have, &have[0], lastUI)
	if machine := dropdownIn(win, "Machine"); machine == nil || !machine.Disabled || machine.Selected() != 1 {
		t.Fatalf("editing a profile on prod, the Machine drop-down is %+v", machine)
	}
	if in, ok := win.dialog.OnAccept(lastUI).(app.SaveProfile); !ok || in.Profile.HostID != "s1" {
		t.Fatalf("saved untouched, the profile is %+v", in.Profile)
	}
}

// A server's dialog offers its profiles as the shell its new terminal
// runs, the one it runs chosen, and sends the one chosen. A server with
// no profiles has nothing to choose, and a window runs no shell of its
// own.
func TestTheServerDialogChoosesTheShellItsNewTerminalRuns(t *testing.T) {
	win, _, publish := windowStage(t)
	prod := remote.Host{ID: "s1", Name: "prod", Address: "prod.example"}
	bare := remote.Host{ID: "s2", Name: "bare", Address: "bare.example"}
	desk := remote.Host{ID: "w1", Name: "desk", Address: "desk.example", Window: true}
	publish(app.State{Saved: []remote.Host{prod, bare, desk},
		Profiles: []settings.Profile{
			{ID: "profile:git-bash", Name: "Git Bash", Command: []string{"bash.exe"}},
			{ID: "profile:tmux", HostID: "s1", Name: "tmux", Command: []string{"tmux"}},
			{ID: "profile:htop", HostID: "s1", Name: "htop", Command: []string{"htop"}},
			{ID: "profile:old", HostID: "w1", Name: "old", Command: []string{"old"}},
		},
		ServerShells: map[string]string{"s1": "profile:htop"}})
	win.serverForm(&prod, lastUI)
	shell := dropdownIn(win, "Shell")
	if shell == nil || shell.Disabled || shell.Selected() != 2 {
		t.Fatalf("prod's Shell drop-down is %+v", shell)
	}
	if in, ok := win.dialog.OnAccept(lastUI).(app.SaveServer); !ok || in.Shell != "profile:htop" {
		t.Fatalf("saved untouched, prod's shell is %q", in.Shell)
	}
	shell.SetSelected(0, nil)
	if in, ok := win.dialog.OnAccept(lastUI).(app.SaveServer); !ok || in.Shell != "" {
		t.Fatalf("with the login shell chosen, prod's shell is %q", in.Shell)
	}
	win.serverForm(&bare, lastUI)
	if shell := dropdownIn(win, "Shell"); shell != nil {
		t.Fatal("a server with no profiles offers a shell")
	}
	win.serverForm(&desk, lastUI)
	if shell := dropdownIn(win, "Shell"); shell == nil || !shell.Disabled {
		t.Fatalf("a window's Shell drop-down is %+v", shell)
	}
}

// A server's profiles are chips on its palette line after its
// terminal, the one it runs first, and lines of their own; either
// opens the profile.
func TestAServersProfilesOpenFromThePalette(t *testing.T) {
	win, _, publish := windowStage(t)
	publish(app.State{Saved: []remote.Host{{ID: "s1", Name: "prod", Address: "prod.example"}},
		Machines: []machines.Info{{ID: "s1", Name: "prod"}},
		Profiles: []settings.Profile{
			{ID: "profile:tmux", HostID: "s1", Name: "tmux", Command: []string{"tmux"}},
			{ID: "profile:htop", HostID: "s1", Name: "htop", Command: []string{"htop"}},
		},
		ServerShells: map[string]string{"s1": "profile:htop"}})
	at := slices.Index(win.paletteIDs, "machine:s1")
	if at < 0 || !slices.Equal(win.palette.Items[at].Chips, []string{"Terminal", "htop", "tmux", "Files", "Connect", "Edit", "Remove"}) {
		t.Fatalf("prod's line is %+v", win.palette.Items[max(at, 0)])
	}
	line := slices.IndexFunc(win.palette.Items, func(it widget.PaletteItem) bool { return it.Title == "tmux" })
	if line < 0 || win.palette.Items[line].Detail != "prod" {
		t.Fatal("the palette has no line for tmux on prod")
	}
	for len(lastWindow.Client().Intents()) > 0 {
		<-lastWindow.Client().Intents()
	}
	win.palette.OnChip(at, 2, lastUI)
	if in, ok := nextIntent(t).(app.OpenShellNamed); !ok || in.ID != "profile:tmux" {
		t.Fatalf("prod's tmux chip sent %#v", in)
	}
	if !win.runItem(win.paletteIDs[line], lastUI) {
		t.Fatalf("tmux's line names %q, which runs nothing", win.paletteIDs[line])
	}
	if in, ok := nextIntent(t).(app.OpenShellNamed); !ok || in.ID != "profile:tmux" {
		t.Fatalf("tmux's line sent %#v", in)
	}
}
