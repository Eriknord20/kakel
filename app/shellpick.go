package app

import (
	"slices"
	"strings"

	"github.com/marrasen/kakel/machines"
	"github.com/marrasen/kakel/session"
	"github.com/marrasen/kakel/settings"
	shellfind "github.com/marrasen/kakel/shells"
)

// Choosing the shell: the shells on this machine are
// found as the window opens, and where there is more than one, as on
// Windows with its Command Prompt, its two PowerShells and its WSL
// distributions, the palette opens a terminal with any of them and
// keeps one as the shell new terminals start.

// ShellChoice is a shell on this machine, as the palette offers it.
type ShellChoice struct {
	ID, Title string
	// Folder is where Windows reaches a WSL distribution's files, empty
	// for any other shell.
	Folder string
}

// Intents for the shell.
type (
	// OpenShellNamed opens a terminal with the shell ID, once.
	OpenShellNamed struct{ ID string }
	// OpenDefaultShell opens a terminal with this machine's default
	// shell, and goes back to it for new terminals.
	OpenDefaultShell struct{}
	// PickShell keeps the shell ID as the one new terminals start, or
	// with ID empty, goes back to the default.
	PickShell struct{ ID string }
)

// findShells finds this machine's shells. A variable, so a test can
// describe a machine.
var findShells = shellfind.Find

// scanShells finds the shells in the background, and publishes them.
func (a *app) scanShells() {
	go func() {
		found, err := findShells()
		a.events <- func() {
			if err != nil {
				a.failed("Couldn't list the shells here", err.Error())
			}
			a.detected, a.scanned, a.looked = found, err == nil, true
			a.mergeShells()
			a.ownShell = ""
			if argv, err := session.DefaultShell(); err == nil {
				if sh, ok := shellfind.Running(found, argv); ok {
					a.ownShell = sh.ID
				}
			}
			// Panes that opened before the shells were known, such as
			// the first, are named again: "Command Prompt", not the
			// path its program calls itself by.
			for _, p := range a.st.Panes {
				if raw := a.programTitle[p.ID]; raw != "" && p.Machine == machines.Local {
					a.retitle(p.ID, raw)
				}
			}
			// A kept shell found gone is said now: the first pane may
			// have tried it before the shells were looked for.
			if a.scanned && a.settings != nil {
				if id, chosen := a.settings.Shell(); chosen && a.shellCommand(id) == nil {
					a.sayShellGone(id)
				}
			}
		}
	}()
}

// defaultShell is the ID of the shell a new terminal here starts: the
// one kept, or the user's own; "" where neither is among the shells
// found.
func (a *app) defaultShell() string {
	if a.st.ChosenShell != "" {
		return a.st.ChosenShell
	}
	return a.ownShell
}

// shellHere is the name of the shell a new terminal from the pane in
// front runs, as likeHere picks it: the one that pane runs, or else the
// default. It is "" for a pane on a server, or a shell not found.
func (a *app) shellHere() string {
	id := a.here()
	if a.machineOf(id) != "" {
		return ""
	}
	want := a.defaultShell()
	if _, cmd := a.commands[id]; !cmd {
		if s, ok := shellfind.Running(a.found, a.argvs[id]); ok && len(a.argvs[id]) > 0 {
			want = s.ID
		}
	}
	for _, s := range a.st.Shells {
		if s.ID == want {
			return s.Title
		}
	}
	return ""
}

// shellsDefaultFirst are the shells here, the one a new terminal starts
// first.
func (a *app) shellsDefaultFirst() []ShellChoice {
	out := slices.Clone(a.st.Shells)
	if i := slices.IndexFunc(out, func(s ShellChoice) bool { return s.ID == a.defaultShell() }); i > 0 {
		def := out[i]
		out = slices.Insert(slices.Delete(out, i, i+1), 0, def)
	}
	return out
}

// localShell is the command a new terminal here starts, when the pane
// in front runs no shell to start another of (see likeHere): the shell
// kept, or nil for the user's own.
func (a *app) localShell() []string {
	if a.settings == nil {
		return nil
	}
	id, chosen := a.settings.Shell()
	if !chosen {
		return nil
	}
	argv := a.shellCommand(id)
	// A profile is offered whether or not its program is still here, so
	// whether it is is asked as it starts.
	if argv != nil && strings.HasPrefix(id, shellfind.ProfilePrefix) {
		if _, err := lookPath(argv[0]); err != nil {
			argv = nil
		}
	}
	if argv == nil {
		// The default shell opens instead.
		a.sayShellGone(id)
	}
	return argv
}

// sayShellGone says, once for each shell kept, that it is no longer on
// this machine, and that new terminals start the default shell.
func (a *app) sayShellGone(id string) {
	if a.shellGoneSaid {
		return
	}
	a.shellGoneSaid = true
	name := strings.TrimPrefix(id, "wsl:")
	if i := slices.IndexFunc(a.st.Profiles, func(p settings.Profile) bool { return p.ID == id }); i >= 0 {
		name = a.st.Profiles[i].Name
	}
	a.failed("Shell not found", name+" is no longer installed. New terminals start the default shell. Choose another in the palette, under Start … in New Terminals.")
}

// shellCommand is the command that starts shell id, or nil when this
// machine has no such shell. Before the shells have been looked for,
// a shell is taken as it is named; after, only one that was found.
func (a *app) shellCommand(id string) []string {
	i := slices.IndexFunc(a.found, func(s shellfind.Shell) bool { return s.ID == id })
	if i < 0 {
		if s, ok := shellfind.Named(id); ok && !a.scanned {
			return s.Command("")
		}
		return nil
	}
	return a.found[i].Command("")
}

// pickShell keeps a shell for new terminals.
func (a *app) pickShell(id string) error {
	if a.settings == nil {
		return nil
	}
	a.st.ChosenShell = id
	// A new pick that goes too is said again.
	a.shellGoneSaid = false
	if id == "" {
		return a.settings.ForgetShell()
	}
	return a.settings.PutShell(id)
}

// likeHere has the next terminal here start the shell the focused pane
// runs, in its folder: another Command Prompt from a Command Prompt, the
// same WSL distribution from a WSL shell. A pane on a server has the
// next terminal there run what it runs (see likeFar). A shell picked
// for it already, and one running a command rather than a shell leave
// it as it is.
func (a *app) likeHere() {
	id := a.here()
	if a.machineOf(id) != "" {
		a.likeFar(id)
		return
	}
	if a.nextShell != nil {
		return
	}
	if _, ok := a.commands[id]; ok {
		return
	}
	argv := a.argvs[id]
	if len(argv) == 0 {
		return
	}
	if s, ok := shellfind.Running(a.found, argv); ok {
		a.nextShell = s.Command(a.dirHere())
	}
}

// shellTitle is the title a local pane's program gave, with the
// program's own path, which the Command Prompt and PowerShell name
// themselves by, given as the shell's name: "Command Prompt", and
// "Command Prompt - ping host" while that runs. What Windows puts in
// front for a program run as administrator stays in front.
func (a *app) shellTitle(id, title string) string {
	if a.machineOf(id) != "" {
		return title
	}
	s, ok := shellfind.Running(a.found, a.localArgv(id))
	if !ok || s.Path == "" {
		return title
	}
	// A profile may name its program as found on the PATH, while the
	// title is the path it was started from.
	path := s.Path
	if !strings.ContainsAny(path, `/\`) {
		if found, err := lookPath(path); err == nil {
			path = found
		}
	}
	admin, rest := "", title
	if r, cut := strings.CutPrefix(title, "Administrator: "); cut {
		admin, rest = "Administrator: ", r
	}
	switch {
	case strings.EqualFold(rest, path):
		return admin + s.Title
	case len(rest) > len(path) && strings.EqualFold(rest[:len(path)], path) && strings.HasPrefix(rest[len(path):], " - "):
		return admin + s.Title + rest[len(path):]
	}
	return title
}

// withoutFolder is argv without the folder a WSL shell was told to start
// in, which is where the pane started, not where it is now: starting it
// again goes to the folder the shell says then, or its home.
func withoutFolder(argv []string) []string {
	if n := len(argv); n >= 2 && argv[n-2] == "--cd" {
		return argv[: n-2 : n-2]
	}
	return argv
}
