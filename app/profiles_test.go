package app

import (
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/marrasen/kakel/internal/sessiontest"
	"github.com/marrasen/kakel/settings"
	shellfind "github.com/marrasen/kakel/shells"
	"github.com/marrasen/kakel/shellsetup"
)

// A profile's dialog is checked for what would stop it opening: no
// name, a name another profile has, a command line that cannot be
// read, names no program here or is another profile's, a folder that
// is not there, and variables written otherwise than NAME=value.
func TestAProfileIsCheckedBeforeItIsSaved(t *testing.T) {
	was := lookPath
	lookPath = func(file string) (string, error) {
		if file == "bash" {
			return "/usr/bin/bash", nil
		}
		return "", exec.ErrNotFound
	}
	t.Cleanup(func() { lookPath = was })
	have := []settings.Profile{{ID: "profile:git-bash", Name: "Git Bash", Command: []string{"bash"}}}
	folder := t.TempDir()
	for _, c := range []struct {
		what                     string
		id, name, line, dir, env string
		wantErr                  string
	}{
		{"no name", "", "  ", "bash", "", "", "enter a name"},
		{"another's name", "", "git bash", "bash", "", "", "name already used"},
		{"no command", "", "Bash", "  ", "", "", "enter a command line"},
		{"no such program", "", "Nu", "nu -l", "", "", "nu not found"},
		{"no such folder", "", "Bash", "bash -l", filepath.Join(folder, "nowhere"), "", "there is no folder " + filepath.Join(folder, "nowhere")},
		{"a variable with no value", "", "Bash", "bash -l", "", "FOO=bar\nBAZ", "invalid environment line 2"},
		{"a variable with no name", "", "Bash", "bash -l", "", "=bar", "invalid environment line 1"},
		{"a name with a space", "", "Bash", "bash -l", "", "MY VAR=1", "invalid environment line 1"},
		{"another's command line", "", "Login Bash", "bash", "", "", "command line already used"},
		{"its own name", "profile:git-bash", "Git Bash", "bash -i", "", "", ""},
		{"its own command line", "profile:git-bash", "Git Bash", "bash", "", "", ""},
	} {
		_, err := ParseProfile(have, c.id, "", c.name, c.line, c.dir, c.env)
		got := ""
		if err != nil {
			got = err.Error()
		}
		if got != c.wantErr {
			t.Errorf("with %s, it said %q, want %q", c.what, got, c.wantErr)
		}
	}
	p, err := ParseProfile(have, "", "", " Login Bash ", `bash -l "-c" ""`, folder, "\n  FOO=bar baz \n\nWSLENV=FOO/u\n")
	if err != nil {
		t.Fatal(err)
	}
	want := settings.Profile{Name: "Login Bash", Command: []string{"bash", "-l", "-c", ""}, Folder: folder, Env: []string{"FOO=bar baz", "WSLENV=FOO/u"}}
	if p.ID != "" || p.Name != want.Name || !slices.Equal(p.Command, want.Command) || p.Folder != want.Folder || !slices.Equal(p.Env, want.Env) {
		t.Fatalf("read as %+v, want %+v", p, want)
	}
}

// A new profile's ID is its name as a command id reads it, made
// unique, so a key bound to it reads as the name does.
func TestAProfilesIDComesFromItsName(t *testing.T) {
	have := []settings.Profile{{ID: "profile:git-bash"}, {ID: "profile:git-bash-2"}}
	for _, c := range []struct{ name, want string }{
		{"Nushell", "profile:nushell"},
		{"Ubuntu 22.04 (root)", "profile:ubuntu-22-04-root"},
		{"  --SSH to box--  ", "profile:ssh-to-box"},
		{"Git Bash", "profile:git-bash-3"},
		{"++", "profile:profile"},
	} {
		if got := profileID(c.name, have); got != c.want {
			t.Errorf("%q is given %q, want %q", c.name, got, c.want)
		}
	}
}

// Profiles are offered after the shells found here, and editing one
// does not look for the shells again, which runs wsl.exe. A profile
// kept for new terminals before the shells are found is found with
// them, not said to be gone.
func TestProfilesFollowTheShellsFoundAndAnEditFindsNoneAgain(t *testing.T) {
	was, wasPath := findShells, lookPath
	looked := 0
	findShells = func() ([]shellfind.Shell, error) {
		looked++
		return []shellfind.Shell{{ID: "cmd", Title: "Command Prompt", Path: `C:\Windows\System32\cmd.exe`}}, nil
	}
	lookPath = func(file string) (string, error) { return "/usr/bin/" + file, nil }
	t.Cleanup(func() { findShells, lookPath = was, wasPath })
	a := fontApp(t)
	a.handle(SaveProfile{Profile: settings.Profile{Name: "Git Bash", Command: []string{"bash", "-l"}}})
	a.handle(PickShell{ID: "profile:git-bash"})
	if got := a.shellCommand("profile:git-bash"); !slices.Equal(got, []string{"bash", "-l"}) {
		t.Fatalf("before the shells are found, the profile starts %q", got)
	}
	if len(a.st.Shells) != 0 {
		t.Fatalf("before the shells are found, the menus offer %+v", a.st.Shells)
	}
	a.scanShells()
	waitFor(t, a, "the shells", func() bool { return len(a.st.Shells) > 0 })
	a.handle(SaveProfile{Profile: settings.Profile{Name: "Nushell", Command: []string{"nu"}}})
	a.handle(SaveProfile{Profile: settings.Profile{ID: "profile:git-bash", Name: "Bash", Command: []string{"bash", "-l"}}})
	var got []string
	for _, s := range a.st.Shells {
		got = append(got, s.ID+" "+s.Title)
	}
	want := []string{"cmd Command Prompt", "profile:git-bash Bash", "profile:nushell Nushell"}
	if !slices.Equal(got, want) || looked != 1 {
		t.Fatalf("after %d looks, the menus offer %q, want %q", looked, got, want)
	}
	if got := a.localShell(); !slices.Equal(got, []string{"bash", "-l"}) || len(a.st.Notices) != 0 {
		t.Fatalf("a new terminal starts %q, and the notices are %+v", got, a.st.Notices)
	}
}

// A profile another kakel saved since this one last read the file is
// seen as this one saves, so the command line it runs is not taken
// twice, which would start both profiles' panes as the first.
func TestASaveSeesTheProfilesAnotherKakelSaved(t *testing.T) {
	was := lookPath
	lookPath = func(file string) (string, error) { return "/usr/bin/" + file, nil }
	t.Cleanup(func() { lookPath = was })
	a := fontApp(t)
	path := filepath.Join(t.TempDir(), "settings.json")
	here, err := settings.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	there, err := settings.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	a.settings = here
	if err := there.PutProfile(settings.Profile{ID: "profile:nushell", Name: "Nushell", Command: []string{"nu"}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := a.saveProfile(settings.Profile{Name: "Nu", Command: []string{"nu"}}); err == nil || err.Error() != "command line already used" {
		t.Fatalf("a second profile running nu was saved, %v", err)
	}
	if err := a.saveProfile(settings.Profile{Name: "Nushell", Command: []string{"nu", "-l"}}); err == nil || err.Error() != "name already used" {
		t.Fatalf("a second profile called Nushell was saved, %v", err)
	}
	again, err := settings.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.Profiles(); len(got) != 1 {
		t.Fatalf("the file holds %+v, want Nushell alone", got)
	}
}

// Removing the profile new terminals start takes them back to the
// default shell, and says nothing: it was removed on purpose.
func TestRemovingTheKeptProfileGoesBackToTheDefault(t *testing.T) {
	a := fontApp(t)
	a.handle(SaveProfile{Profile: settings.Profile{Name: "Nushell", Command: []string{"nu"}}})
	a.handle(PickShell{ID: "profile:nushell"})
	a.handle(RemoveProfile{ID: "profile:nushell"})
	if id, chosen := a.settings.Shell(); chosen || a.st.ChosenShell != "" {
		t.Fatalf("removed, the kept shell is %q, %v, and the state's %q", id, chosen, a.st.ChosenShell)
	}
	if argv := a.localShell(); argv != nil || len(a.st.Notices) != 0 {
		t.Fatalf("a new terminal starts %q, and the notices are %+v", argv, a.st.Notices)
	}
	if len(a.st.Profiles) != 0 || a.shellCommand("profile:nushell") != nil {
		t.Fatalf("removed, the profiles are %+v", a.st.Profiles)
	}
}

// A kept profile whose program has gone is said to be, by its name,
// and new terminals start the default shell instead of failing.
func TestAKeptProfileWhoseProgramHasGoneFallsBack(t *testing.T) {
	was := lookPath
	lookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	t.Cleanup(func() { lookPath = was })
	a := fontApp(t)
	a.handle(SaveProfile{Profile: settings.Profile{Name: "Nushell", Command: []string{"nu"}}})
	a.handle(PickShell{ID: "profile:nushell"})
	if argv := a.localShell(); argv != nil {
		t.Fatalf("a new terminal starts %q", argv)
	}
	if len(a.st.Notices) != 1 || a.st.Notices[0].Title != "Shell not found" || !strings.HasPrefix(a.st.Notices[0].Body, "Nushell is no longer installed.") {
		t.Fatalf("the notices are %+v", a.st.Notices)
	}
}

// A profile's folder is where its terminal starts in place of the
// start folder, where a terminal started by default would go; a
// profile with no folder, or one whose folder is gone, starts there.
func TestAProfileStartsInItsFolder(t *testing.T) {
	a := fontApp(t)
	start, own := t.TempDir(), t.TempDir()
	a.handle(SaveThisComputer{StartFolder: start})
	for _, c := range []struct{ folder, want string }{
		{own, own},
		{"", start},
		{filepath.Join(own, "gone"), start},
	} {
		if got := a.startDir("", c.folder); got != c.want {
			t.Errorf("a profile in %q starts in %s, want %s", c.folder, got, c.want)
		}
	}
	a.handle(SaveProfile{Profile: settings.Profile{Name: "Nushell", Command: []string{"nu"}, Folder: own}})
	if got := a.keptFolder(); got != "" {
		t.Fatalf("with no shell kept, a new window starts in the folder %q", got)
	}
	a.handle(PickShell{ID: "profile:nushell"})
	if got := a.keptFolder(); got != own {
		t.Fatalf("with the profile kept, a new window starts in the folder %q, want %s", got, own)
	}
	if got := a.startDir("", a.keptFolder()); got != own {
		t.Fatalf("with the profile kept, a new window starts in %s, want %s", got, own)
	}
}

// A profile's variables come after what the terminal is called, so one
// can name it otherwise, and a WSL profile carries each into the
// distribution, adding to the WSLENV it sets or the window has.
func TestAProfilesVariablesReachItsShell(t *testing.T) {
	t.Setenv("WSLENV", "USERPROFILE/p")
	wsl := []string{`C:\Windows\System32\wsl.exe`, "-d", "Ubuntu"}
	for _, c := range []struct {
		argv []string
		set  []string
		want []string
	}{
		{[]string{"nu"}, []string{"FOO=bar", "TERM_PROGRAM=WezTerm"},
			[]string{"FOO=bar", "TERM_PROGRAM=WezTerm"}},
		{wsl, []string{"FOO=bar"},
			[]string{"FOO=bar", "WSLENV=USERPROFILE/p:TERM_PROGRAM/u:TERM_PROGRAM_VERSION/u:FOO/u"}},
		{wsl, []string{"WSLENV=FOO/l", "FOO=/x"},
			[]string{"FOO=/x", "WSLENV=FOO/l:TERM_PROGRAM/u:TERM_PROGRAM_VERSION/u"}},
		{wsl, nil,
			[]string{"WSLENV=USERPROFILE/p:TERM_PROGRAM/u:TERM_PROGRAM_VERSION/u"}},
		// A Windows path is translated, one path with p and a list with
		// l, and PATH stays out, as WSL adds the Windows one itself.
		{wsl, []string{`JAVA_HOME=C:\jdk`, `TOOLS=C:\a;D:\b`, `PATH=C:\tools`},
			[]string{`JAVA_HOME=C:\jdk`, `TOOLS=C:\a;D:\b`, `PATH=C:\tools`,
				"WSLENV=USERPROFILE/p:TERM_PROGRAM/u:TERM_PROGRAM_VERSION/u:JAVA_HOME/pu:TOOLS/lu"}},
	} {
		got := paneEnv(c.argv, "", c.set)
		if len(got) < 2 || !strings.HasPrefix(got[0], "TERM_PROGRAM=") || !slices.Equal(got[2:], c.want) {
			t.Errorf("%q with %q is started with %q, want the terminal's name and then %q", c.argv, c.set, got, c.want)
		}
	}
}

// A profile's value may name a variable there already, written as this
// system's shell writes one, so a profile can add to PATH rather than
// take its place. A name not set stays as it is written, as cmd leaves
// it, and a doubled sign writes one, so any value can be written.
func TestAProfilesValuesNameTheVariablesThere(t *testing.T) {
	t.Setenv("KAKEL_TOOLS", `C:\tools`)
	cases := []struct{ value, want string }{
		{"$KAKEL_TOOLS:/bin", `C:\tools:/bin`},
		{"${KAKEL_TOOLS}x", `C:\toolsx`},
		{"plain", "plain"},
		{"pa$$word", "pa$word"},
		{"$$KAKEL_TOOLS", "$KAKEL_TOOLS"},
		{"$KAKEL_NOT_SET and ${KAKEL_NOT_SET}", "$KAKEL_NOT_SET and ${KAKEL_NOT_SET}"},
		{"cost: 5$, $1, ${}, ${KAKEL_TOOLS", "cost: 5$, $1, ${}, ${KAKEL_TOOLS"},
	}
	if runtime.GOOS == "windows" {
		cases = []struct{ value, want string }{
			{`D:\bin;%KAKEL_TOOLS%`, `D:\bin;C:\tools`},
			{"%kakel_tools%", `C:\tools`},
			{"%KAKEL_NOT_SET%;%KAKEL_TOOLS%", `%KAKEL_NOT_SET%;C:\tools`},
			{"100% of %KAKEL_TOOLS%", `100% of C:\tools`},
			{"%%KAKEL_TOOLS%%", "%KAKEL_TOOLS%"},
			{"100%% and 5%", "100% and 5%"},
			{"$KAKEL_TOOLS", "$KAKEL_TOOLS"},
		}
	}
	for _, c := range cases {
		if got := expandVars(c.value); got != c.want {
			t.Errorf("%q reads as %q, want %q", c.value, got, c.want)
		}
	}
	if got := paneEnv([]string{"nu"}, "", []string{"P=" + cases[0].value}); got[len(got)-1] != "P="+cases[0].want {
		t.Errorf("a profile setting %q starts its shell with %q", cases[0].value, got)
	}
}

// A profile running a program the shell setup is not written for is
// not taught, and every other shell is as it was: a profile running a
// shell, a shell found here, and the user's own.
func TestOnlyAProfileRunningAKnownShellIsTaught(t *testing.T) {
	a := fontApp(t)
	a.st.ShellSetup = true
	for _, p := range []settings.Profile{
		{Name: "Git Bash", Command: []string{`C:\Program Files\Git\bin\bash.exe`, "-i", "-l"}},
		{Name: "Nushell", Command: []string{"nu"}},
		{Name: "Box", Command: []string{"ssh", "-t", "box"}},
	} {
		a.handle(SaveProfile{Profile: p})
	}
	for _, c := range []struct {
		argv []string
		want bool
	}{
		{[]string{`C:\Program Files\Git\bin\bash.exe`, "-i", "-l"}, true},
		{[]string{"nu"}, false},
		{[]string{"ssh", "-t", "box"}, false},
		{[]string{"nu", "-l"}, true},
		{[]string{`C:\Windows\System32\cmd.exe`}, true},
		{nil, true},
	} {
		if got := a.teaches(c.argv); got != c.want {
			t.Errorf("%q is taught: %v, want %v", c.argv, got, c.want)
		}
	}
	a.st.ShellSetup = false
	if a.teaches(nil) {
		t.Error("with Shell Setup off, the user's own shell is taught")
	}
}

// A pane here running a profile's program the shell setup is not
// written for is of no kind a paste is coloured for, as a server's is
// not: its lines are its own language, not slips into the wrong shell.
func TestAPaneRunningAnUnknownProgramIsOfNoShellKind(t *testing.T) {
	a := fontApp(t)
	a.handle(SaveProfile{Profile: settings.Profile{Name: "Nushell", Command: []string{"nu"}}})
	a.handle(SaveProfile{Profile: settings.Profile{Name: "Git Bash", Command: []string{"bash", "-l"}}})
	for _, c := range []struct {
		argv []string
		kind shellsetup.Route
	}{
		{[]string{"nu"}, shellsetup.NoRoute},
		{[]string{"bash", "-l"}, shellsetup.Posix},
		{[]string{`C:\Windows\System32\cmd.exe`}, shellsetup.Cmd},
	} {
		a.argvs["p1"] = c.argv
		if got := a.shellKindOf(Pane{ID: "p1"}); got != c.kind {
			t.Errorf("a pane running %q is of kind %v, want %v", c.argv, got, c.kind)
		}
	}
}

// The profiles here are offered even when looking for the shells
// fails, as the shells it did find are.
func TestProfilesAreOfferedWhenTheShellsCannotBeListed(t *testing.T) {
	was := findShells
	findShells = func() ([]shellfind.Shell, error) { return nil, errors.New("wsl.exe did not answer") }
	t.Cleanup(func() { findShells = was })
	a := fontApp(t)
	a.handle(SaveProfile{Profile: settings.Profile{Name: "Nushell", Command: []string{"nu"}}})
	a.scanShells()
	waitFor(t, a, "the search for shells to answer", func() bool { return a.looked })
	a.handle(SaveProfile{Profile: settings.Profile{Name: "MSYS2", Command: []string{"bash", "--login", "-i"}}})
	var got []string
	for _, s := range a.st.Shells {
		got = append(got, s.ID)
	}
	if want := []string{"profile:nushell", "profile:msys2"}; !slices.Equal(got, want) {
		t.Fatalf("with the shells not listed, the menus offer %q, want %q", got, want)
	}
}

// A profile's shell is started with the profile's variables.
func TestAProfilesShellHasItsVariables(t *testing.T) {
	a, _ := agentApp(t)
	set, err := settingsIn(t)
	if err != nil {
		t.Fatal(err)
	}
	a.settings = set
	a.handle(SaveProfile{Profile: settings.Profile{Name: "Plain", Command: plainShell(), Env: []string{"KAKEL_PROFILE=plain"}}})
	a.handle(OpenShellNamed{ID: "profile:plain"})
	if len(a.st.Panes) != 2 {
		t.Fatalf("opened, the panes are %+v; notices %+v", a.st.Panes, a.st.Notices)
	}
	echoed(t, a, a.st.Panes[1].ID, echoVar("profile=", "KAKEL_PROFILE"), "profile=plain")
}

// A pane is known by its profile when the profile's command ends in the
// --cd a WSL shell starts in, though a pane is started again without it.
func TestAProfileEndingInAFolderIsKnownStartedAgain(t *testing.T) {
	a := fontApp(t)
	a.handle(SaveProfile{Profile: settings.Profile{Name: "Ubuntu home", Command: []string{"wsl.exe", "-d", "Ubuntu", "--cd", "~"}}})
	for _, argv := range [][]string{
		{"wsl.exe", "-d", "Ubuntu", "--cd", "~"},
		{"wsl.exe", "-d", "Ubuntu"},
		{"wsl.exe", "-d", "Ubuntu", "--cd", "/home/x"},
	} {
		if p, ok := a.profileOf(argv); !ok || p.Name != "Ubuntu home" {
			t.Errorf("%q runs the profile %+v, %v", argv, p, ok)
		}
	}
}

// A profile naming its program as found on the PATH is named by the
// profile, as the path the program titles its pane with is the one
// found.
func TestAProfilesPaneIsNamedByTheProfile(t *testing.T) {
	ssh := `C:\Windows\System32\OpenSSH\ssh.exe`
	was := lookPath
	lookPath = func(file string) (string, error) {
		if file == "ssh" {
			return ssh, nil
		}
		return "", exec.ErrNotFound
	}
	t.Cleanup(func() { lookPath = was })
	a := fontApp(t)
	a.handle(SaveProfile{Profile: settings.Profile{Name: "Box", Command: []string{"ssh", "box"}}})
	a.addPane(Pane{ID: "p1", Title: "Terminal 1"}, nil, Placement{})
	a.argvs["p1"] = []string{"ssh", "box"}
	for said, want := range map[string]string{
		ssh:            "Box",
		ssh + " - top": "Box - top",
		"box: ~":       "box: ~",
	} {
		a.retitle("p1", said)
		if got := a.titleOf("p1"); got != want {
			t.Errorf("titled %q, the pane is called %q, want %q", said, got, want)
		}
	}
}

// A server's profile is checked against the profiles on that server
// alone, and nothing on it is looked for here: its program is taken as
// written, and its folder is a path from / or from home.
func TestAServerProfileIsCheckedOnItsServer(t *testing.T) {
	was := lookPath
	lookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	t.Cleanup(func() { lookPath = was })
	have := []settings.Profile{
		{ID: "profile:git-bash", Name: "Git Bash", Command: []string{"bash"}},
		{ID: "profile:tmux", Name: "tmux", HostID: "srv", Command: []string{"tmux", "new", "-A"}},
		{ID: "profile:top", Name: "top", HostID: "box", Command: []string{"top"}},
	}
	for _, c := range []struct {
		what                     string
		id, name, line, dir, env string
		wantErr                  string
	}{
		{"another's name on the server", "", "TMUX", "htop", "", "", "name already used"},
		{"another's command line on the server", "", "Main", "tmux new -A", "~/src", "FOO=bar", ""},
		{"this computer's name and command line", "", "Git Bash", "bash", "", "", ""},
		{"another server's name and command line", "", "top", "top", "", "", ""},
		{"a program not here", "", "Nu", "nu -l", "", "", ""},
		{"home", "", "htop", "htop", "~", "", ""},
		{"a folder at home", "", "htop", "htop", "~/src", "", ""},
		{"a folder from /", "", "htop", "htop", "/srv/www", "", ""},
		{"a folder from where it starts", "", "htop", "htop", "src", "", "enter an absolute path"},
		{"another account's home", "", "htop", "htop", "~bob/src", "", "enter an absolute path"},
		{"a variable with no name", "", "htop", "htop", "", "=bar", "invalid environment line 1"},
		{"its own name", "profile:tmux", "tmux", "tmux new -A", "", "", ""},
	} {
		_, err := ParseProfile(have, c.id, "srv", c.name, c.line, c.dir, c.env)
		got := ""
		if err != nil {
			got = err.Error()
		}
		if got != c.wantErr {
			t.Errorf("with %s, it said %q, want %q", c.what, got, c.wantErr)
		}
	}
	p, err := ParseProfile(have, "", "srv", "Main", "tmux new -A -s main", " ~/src ", "FOO=bar")
	if err != nil {
		t.Fatal(err)
	}
	if p.HostID != "srv" || p.Folder != "~/src" || !slices.Equal(p.Command, []string{"tmux", "new", "-A", "-s", "main"}) || !slices.Equal(p.Env, []string{"FOO=bar"}) {
		t.Fatalf("read as %+v", p)
	}
}

// A server's profile is none of this computer's shells: not offered
// with them, not started here, and not what a pane here is known by.
func TestAServerProfileIsNoShellHere(t *testing.T) {
	a := fontApp(t)
	a.handle(SaveProfile{Profile: settings.Profile{Name: "tmux", HostID: "srv", Command: []string{"tmux"}, Folder: t.TempDir()}})
	a.detected, a.looked = []shellfind.Shell{{ID: "cmd", Title: "Command Prompt", Path: `C:\Windows\System32\cmd.exe`}}, true
	a.mergeShells()
	if len(a.st.Shells) != 1 || a.st.Shells[0].ID != "cmd" || a.shellCommand("profile:tmux") != nil {
		t.Fatalf("the shells here are %+v", a.st.Shells)
	}
	if p, ok := a.profileOf([]string{"tmux"}); ok {
		t.Fatalf("a pane here running tmux runs %+v", p)
	}
	a.handle(PickShell{ID: "profile:tmux"})
	if got := a.keptFolder(); got != "" {
		t.Fatalf("a new window here starts in the server's folder %q", got)
	}
}

// A server's profile runs its command in place of the account's shell,
// through env when it sets variables, in its folder: one at home is
// given from there, as the shell starts at home and reads no ~ in
// quotes. Anything but one of the server's own profiles runs the
// account's shell.
func TestAServerProfilesShellConfig(t *testing.T) {
	a := fontApp(t)
	a.st.Profiles = []settings.Profile{
		{ID: "profile:htop", HostID: "srv", Command: []string{"htop"}},
		{ID: "profile:tmux", HostID: "srv", Command: []string{"tmux", "new"}, Env: []string{"FOO=bar", "A=b c"}, Folder: "~/src"},
		{ID: "profile:home", HostID: "srv", Command: []string{"zsh", "-l"}, Folder: "~"},
		{ID: "profile:www", HostID: "srv", Command: []string{"bash"}, Folder: "/srv/www"},
		{ID: "profile:top", HostID: "box", Command: []string{"top"}},
		{ID: "profile:nu", Command: []string{"nu"}},
	}
	for _, c := range []struct {
		id   string
		argv []string
		dir  string
	}{
		{"profile:htop", []string{"htop"}, ""},
		{"profile:tmux", []string{"env", "FOO=bar", "A=b c", "tmux", "new"}, "src"},
		{"profile:home", []string{"zsh", "-l"}, ""},
		{"profile:www", []string{"bash"}, "/srv/www"},
		{"profile:top", nil, ""},
		{"profile:nu", nil, ""},
		{loginShell, nil, ""},
		{"", nil, ""},
	} {
		cfg := a.farConfig("srv", c.id, 80, 24)
		if !slices.Equal(cfg.Command, c.argv) || cfg.Dir != c.dir || cfg.Cols != 80 || cfg.Rows != 24 {
			t.Errorf("%q on srv starts as %+v, want %q in %q", c.id, cfg, c.argv, c.dir)
		}
	}
}

// serverWithProfiles is dialApp with settings, and two profiles on the
// server: tmux, in a folder at home and with a variable, and htop.
func serverWithProfiles(t *testing.T) (a *app, answering func()) {
	t.Helper()
	a, answering = dialApp(t)
	set, err := settingsIn(t)
	if err != nil {
		t.Fatal(err)
	}
	a.settings = set
	for _, p := range []settings.Profile{
		{Name: "tmux", HostID: "srv", Command: []string{"tmux", "new", "-A", "-s", "main"}, Folder: "~/src", Env: []string{"FOO=bar"}},
		{Name: "htop", HostID: "srv", Command: []string{"htop"}},
	} {
		a.handle(SaveProfile{Profile: p})
	}
	if len(a.st.Profiles) != 2 {
		t.Fatalf("saved, the profiles are %+v; notices %+v", a.st.Profiles, a.st.Notices)
	}
	return a, answering
}

// What the test server says it ran for each profile.
const (
	ranTmux = `RAN cd -- 'src' && 'env' 'FOO=bar' 'tmux' 'new' '-A' '-s' 'main'`
	ranHtop = `RAN 'htop'`
)

// paneSays reports whether pane i is open and its terminal shows s.
func paneSays(a *app, i int, s string) bool {
	if len(a.st.Panes) <= i || a.st.Panes[i].Kind == KindLog {
		return false
	}
	tm := a.terminal(a.st.Panes[i].ID)
	return tm != nil && strings.Contains(tm.AllText(), s)
}

// A server's profile opened while the server is not connected connects
// to it first, and then runs.
func TestAServerProfileOpensOnceItsServerIsConnected(t *testing.T) {
	a, answering := serverWithProfiles(t)
	a.handle(OpenShellNamed{ID: "profile:tmux"})
	waitFor(t, a, "the profile on the server", func() bool { answering(); return oneShell(a) && paneSays(a, 0, ranTmux) })
	if p := a.st.Panes[0]; p.Machine != "srv" || a.farRuns[p.ID] != "profile:tmux" || len(a.nextFar) != 0 {
		t.Fatalf("the pane is %+v, running %q, with %v asked for next", p, a.farRuns[p.ID], a.nextFar)
	}
}

// A server's new terminal runs the profile its dialog picks, and its
// login shell when that is asked for. A terminal opened from a pane
// there, and a pane started again, run what that pane runs, and not
// the server's default.
func TestAServersNewTerminalRunsItsProfile(t *testing.T) {
	a, answering := serverWithProfiles(t)
	h, _ := a.book.Lookup("srv")
	a.handle(SaveServer{Host: h, Under: "srv", Shell: "profile:htop"})
	if a.st.ServerShells["srv"] != "profile:htop" || a.settings.ServerShell("srv") != "profile:htop" {
		t.Fatalf("saved, the server runs %v", a.st.ServerShells)
	}
	a.handle(OpenOn{Machine: "srv"})
	waitFor(t, a, "the default profile", func() bool { answering(); return oneShell(a) && paneSays(a, 0, ranHtop) })
	a.handle(OpenOn{Machine: "srv", Login: true})
	waitFor(t, a, "the login shell", func() bool { return paneSays(a, 1, "READY") })
	login := a.st.Panes[1].ID
	a.handle(OpenShellNamed{ID: "profile:tmux"})
	waitFor(t, a, "the other profile", func() bool { return paneSays(a, 2, ranTmux) })
	tmux := a.st.Panes[2].ID

	a.focus(login)
	a.handle(SplitPane{})
	waitFor(t, a, "a split of the login shell", func() bool { return paneSays(a, 3, "READY") })
	a.focus(tmux)
	a.handle(SplitPane{})
	waitFor(t, a, "a split of the profile", func() bool { return paneSays(a, 4, ranTmux) })
	if len(a.nextFar) != 0 {
		t.Fatalf("opened, %v is asked for next", a.nextFar)
	}

	waitFor(t, a, "the profile to end", func() bool { return a.terminal(tmux).Exited() })
	if err := a.startAgain(tmux); err != nil {
		t.Fatal(err)
	}
	// The second run starts where the first left the cursor, so its line
	// wraps.
	waitFor(t, a, "the profile again", func() bool {
		return strings.Count(strings.Join(strings.Fields(a.terminal(tmux).AllText()), " "), ranTmux) == 2
	})
}

// A split asked to run a server's profile opens on that server and runs
// it.
func TestASplitRunsAServersProfile(t *testing.T) {
	a, answering := serverWithProfiles(t)
	a.handle(OpenOn{Machine: "srv", Login: true})
	waitFor(t, a, "the login shell", func() bool { answering(); return oneShell(a) && paneSays(a, 0, "READY") })
	a.handle(SplitPane{Shell: "profile:tmux"})
	waitFor(t, a, "the profile in the split", func() bool {
		return len(a.st.Panes) == 2 && a.st.Panes[1].Machine == "srv" && paneSays(a, 1, ranTmux)
	})
}

// A profile running a program the shell setup is not written for is
// not taught, on a server that teaches its shells, and its pane takes
// no shell's kind.
func TestAServerProfileRunningAnUnknownProgramIsNotTaught(t *testing.T) {
	a, _ := dialApp(t)
	h, _ := a.book.Lookup("srv")
	h.Setup = true
	if err := a.book.Put(h, "srv"); err != nil {
		t.Fatal(err)
	}
	a.st.Saved = a.book.Hosts()
	a.st.Profiles = []settings.Profile{
		{ID: "profile:htop", HostID: "srv", Command: []string{"htop"}},
		{ID: "profile:bash", HostID: "srv", Command: []string{"bash", "-l"}},
	}
	for _, c := range []struct {
		run    string
		taught bool
		kind   shellsetup.Route
	}{
		{"", true, shellsetup.Posix},
		{"profile:bash", true, shellsetup.Posix},
		{"profile:htop", false, shellsetup.NoRoute},
	} {
		profile, _ := a.farProfile("srv", c.run)
		sess := sessiontest.New()
		a.teachFar("srv", sess, profile.Command)
		if taught := sess.Sent() != ""; taught != c.taught {
			t.Errorf("running %q, the shell is taught: %v", c.run, taught)
		}
		a.farRuns["p1"] = c.run
		if got := a.shellKindOf(Pane{ID: "p1", Machine: "srv"}); got != c.kind {
			t.Errorf("running %q, the pane's shell is of kind %v, want %v", c.run, got, c.kind)
		}
	}
}

// A server's profiles are offered in the launcher and the tray, the
// one its new terminal runs first, and the tray's menu changes with
// them.
func TestAServersProfilesAreOfferedOnIt(t *testing.T) {
	a, _ := serverWithProfiles(t)
	h, _ := a.book.Lookup("srv")
	a.handle(SaveServer{Host: h, Under: "srv", Shell: "profile:htop"})
	ms := a.launchMachines()
	i := slices.IndexFunc(ms, func(m LaunchMachine) bool { return m.ID == "srv" })
	var actions []string
	for _, x := range ms[i].Actions {
		actions = append(actions, x.ID)
	}
	if want := []string{"terminal", "shell:profile:htop", "shell:profile:tmux", "files"}; !slices.Equal(actions, want) {
		t.Fatalf("the launcher offers %v on the server, want %v", actions, want)
	}
	things := map[string]LaunchThing{}
	for _, th := range a.launchThings(ms) {
		things[th.Title] = th
	}
	if th := things["htop on srv"]; th.Action != "shell:profile:htop" || th.Machine != "srv" || th.Note != "default" {
		t.Fatalf("the default profile is offered as %+v", th)
	}
	if th := things["tmux on srv"]; th.Action != "shell:profile:tmux" || th.Note != "" {
		t.Fatalf("the other profile is offered as %+v", th)
	}
	items, _ := a.trayMenu(1)
	var tray []string
	for _, it := range items {
		if it.Title == "srv" {
			for _, sub := range it.Items {
				tray = append(tray, sub.Title)
			}
		}
	}
	if want := []string{"Terminal", "Files", "htop", "tmux", "Connection Log"}; !slices.Equal(tray, want) {
		t.Fatalf("the tray offers %v on the server, want %v", tray, want)
	}
	sig := a.traySig()
	a.handle(RemoveProfile{ID: "profile:tmux"})
	if a.traySig() == sig {
		t.Fatal("a profile removed, the tray's menu is not made again")
	}
}

// Removing the profile a server's new terminal runs takes it back to
// the login shell, and removing the server removes its profiles, and
// no others.
func TestRemovingAServerRemovesItsProfiles(t *testing.T) {
	a, _ := serverWithProfiles(t)
	a.handle(SaveProfile{Profile: settings.Profile{Name: "Nushell", Command: []string{"nu"}}})
	h, _ := a.book.Lookup("srv")
	a.handle(SaveServer{Host: h, Under: "srv", Shell: "profile:tmux"})
	a.handle(RemoveProfile{ID: "profile:tmux"})
	if len(a.st.ServerShells) != 0 || a.settings.ServerShell("srv") != "" {
		t.Fatalf("its profile removed, the server runs %v", a.st.ServerShells)
	}
	a.handle(SaveServer{Host: h, Under: "srv", Shell: "profile:htop"})
	a.handle(RemoveServer{ID: "srv"})
	if len(a.st.Profiles) != 1 || a.st.Profiles[0].ID != "profile:nushell" || len(a.settings.Profiles()) != 1 {
		t.Fatalf("the server removed, the profiles are %+v", a.st.Profiles)
	}
	if len(a.st.ServerShells) != 0 || a.settings.ServerShell("srv") != "" {
		t.Fatalf("the server removed, it runs %v", a.st.ServerShells)
	}
}
