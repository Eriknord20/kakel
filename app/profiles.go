package app

import (
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/marrasen/kakel/machines"
	"github.com/marrasen/kakel/remote"
	"github.com/marrasen/kakel/settings"
	shellfind "github.com/marrasen/kakel/shells"
)

// Profiles: shells the user describes, such as MSYS2, nushell or ssh
// to a box, by a command line, a folder and variables. Each joins the
// shells found here, so the palette, the split chooser, the launcher,
// the tray and the Shell setting offer it with the rest.
//
// A profile on a saved server, such as tmux or htop, is run there in
// place of the account's own shell, and the server's dialog may make
// one what its new terminal runs.

// Intents for profiles.
type (
	// SaveProfile puts Profile in place of the one with its ID, or adds
	// it as a new one when its ID is empty.
	SaveProfile struct{ Profile settings.Profile }
	// RemoveProfile removes the profile ID.
	RemoveProfile struct{ ID string }
)

// lookPath finds a profile's program. A variable, so a test can
// describe a machine.
var lookPath = exec.LookPath

// ProfileLine is a profile's command as its dialog shows it, which
// ParseProfile reads back as the same words.
func ProfileLine(p settings.Profile) string { return joinLine(p.Command) }

// ParseProfile is the profile a dialog describes, or why it cannot be
// saved. have are the profiles there are, and id is the profile's own,
// empty for a new one: saving gives it one. hostID is the saved server
// it runs on, empty for this computer, and its name must not repeat
// another's on the same machine.
//
// A program here is checked to be here but kept as written, so a
// program found on the PATH is found wherever it moves to. A server
// cannot be looked at from here, so its program is taken as written,
// and its folder is a path from / or from home, ~.
func ParseProfile(have []settings.Profile, id, hostID, name, line, folder, env string) (settings.Profile, error) {
	p := settings.Profile{ID: id, HostID: hostID, Name: strings.TrimSpace(name)}
	if p.Name == "" {
		return p, errors.New("enter a name")
	}
	p.Command = splitLine(line)
	if len(p.Command) == 0 || p.Command[0] == "" {
		return p, errors.New("enter a command line")
	}
	if hostID == "" {
		if _, err := lookPath(p.Command[0]); err != nil {
			return p, errors.New(p.Command[0] + " not found")
		}
	}
	if err := taken(have, p); err != nil {
		return p, err
	}
	if folder = strings.TrimSpace(folder); folder != "" {
		switch {
		case hostID == "":
			full, err := FolderHere(folder)
			if err != nil {
				return p, err
			}
			p.Folder = full
		case folder == "~" || strings.HasPrefix(folder, "~/") || strings.HasPrefix(folder, "/"):
			p.Folder = folder
		default:
			return p, errors.New("enter an absolute path")
		}
	}
	for i, l := range strings.Split(env, "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		key, value, ok := strings.Cut(l, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" || strings.ContainsFunc(key, unicode.IsSpace) {
			return p, fmt.Errorf("invalid environment line %d", i+1)
		}
		p.Env = append(p.Env, key+"="+value)
	}
	return p, nil
}

// taken is why p cannot be kept beside the profiles in have, nil when
// it can: another profile on its machine has its name, or here runs
// its command line. A pane here knows its profile only by the command
// it runs, as profileOf reads it, so two profiles running one command
// would both start as the first. A server's pane keeps its profile by
// id.
func taken(have []settings.Profile, p settings.Profile) error {
	for _, other := range have {
		switch {
		case other.ID == p.ID || other.HostID != p.HostID:
		case strings.EqualFold(other.Name, p.Name):
			return errors.New("name already used")
		case p.HostID == "" && len(p.Command) > 0 && slices.EqualFunc(withoutFolder(other.Command), withoutFolder(p.Command), strings.EqualFold):
			return errors.New("command line already used")
		}
	}
	return nil
}

// profileID is the ID a new profile called name is given: its name in
// lower case, with a dash for each run of anything but letters and
// digits, and a number on the end when another profile has it.
func profileID(name string, have []settings.Profile) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			dash = false
			continue
		}
		dash = true
	}
	slug := b.String()
	if slug == "" {
		slug = "profile"
	}
	taken := func(id string) bool {
		return slices.ContainsFunc(have, func(p settings.Profile) bool { return p.ID == id })
	}
	id := shellfind.ProfilePrefix + slug
	for n := 2; taken(id); n++ {
		id = shellfind.ProfilePrefix + slug + "-" + strconv.Itoa(n)
	}
	return id
}

// saveProfile saves a profile, giving a new one its ID, and offers it
// with the shells.
func (a *app) saveProfile(p settings.Profile) error {
	if a.settings == nil {
		return errors.New("the settings could not be read, so nothing can be kept")
	}
	// The dialog checked p against the profiles this window last read,
	// and another kakel may have saved one since.
	err := a.settings.PutProfile(p, func(have []settings.Profile) (settings.Profile, error) {
		if err := taken(have, p); err != nil {
			return p, err
		}
		if p.ID == "" {
			p.ID = profileID(p.Name, have)
		}
		return p, nil
	})
	if err != nil {
		return err
	}
	a.st.Profiles = a.settings.Profiles()
	a.st.ServerShells = a.settings.ServerShells()
	a.mergeShells()
	return nil
}

// removeProfile removes a profile. New terminals go back to the
// default shell when it was the one they started, without the notice
// a shell found missing gets: it was removed on purpose. A server
// whose new terminal ran it goes back to the account's own shell.
func (a *app) removeProfile(id string) error {
	if a.settings == nil {
		return errors.New("the settings could not be read, so nothing can be kept")
	}
	if err := a.settings.DropProfile(id); err != nil {
		return err
	}
	a.st.Profiles = a.settings.Profiles()
	a.st.ServerShells = a.settings.ServerShells()
	a.mergeShells()
	if kept, chosen := a.settings.Shell(); chosen && kept == id {
		return a.pickShell("")
	}
	return nil
}

// mergeShells puts this computer's profiles after the shells detected
// here, in a.found, and offers them all once the shells have been
// looked for. Before that the menus offer the user's own shell, which
// the profiles alone would hide; a.found holds them already, so a
// profile kept for new terminals opens.
func (a *app) mergeShells() {
	a.found = slices.Clone(a.detected)
	for _, p := range a.st.Profiles {
		if p.HostID == "" {
			a.found = append(a.found, profileShell(p))
		}
	}
	if !a.looked {
		return
	}
	a.st.Shells = nil
	for _, s := range a.found {
		choice := ShellChoice{ID: s.ID, Title: s.Title}
		if s.Distro != "" {
			choice.Folder = shellfind.WSLRoot(s.Distro)
		}
		a.st.Shells = append(a.st.Shells, choice)
	}
}

// profileShell is a profile as one of the shells here.
func profileShell(p settings.Profile) shellfind.Shell {
	s := shellfind.Shell{ID: p.ID, Title: p.Name}
	if len(p.Command) > 0 {
		s.Path, s.Args = p.Command[0], p.Command[1:]
	}
	return s
}

// profileOf is this computer's profile argv runs, and whether it runs
// one. The --cd a WSL shell is started with is set aside on both
// sides: a pane is started again without it, and a profile's own
// command may end in one.
func (a *app) profileOf(argv []string) (settings.Profile, bool) {
	if len(argv) == 0 {
		return settings.Profile{}, false
	}
	argv = withoutFolder(argv)
	for _, p := range a.st.Profiles {
		if p.HostID == "" && slices.EqualFunc(withoutFolder(p.Command), argv, strings.EqualFold) {
			return p, true
		}
	}
	return settings.Profile{}, false
}

// keptFolder is the folder of the profile new terminals start, "" when
// they start another shell.
func (a *app) keptFolder() string {
	if a.settings == nil {
		return ""
	}
	id, chosen := a.settings.Shell()
	if !chosen {
		return ""
	}
	for _, p := range a.st.Profiles {
		if p.ID == id && p.HostID == "" {
			return p.Folder
		}
	}
	return ""
}

// loginShell, as what the next terminal on a server runs, is the
// account's own shell, in place of the profile the server runs by
// default.
const loginShell = "login"

// farRun is what the next terminal on machine runs: what was asked for
// it, once, or else the profile the server runs by default.
func (a *app) farRun(machine machines.ID) string {
	run, asked := a.nextFar[machine]
	delete(a.nextFar, machine)
	if !asked {
		run = a.st.ServerShells[string(machine)]
	}
	return run
}

// farProfile is the profile id on the saved server machine, and
// whether that is one of its profiles.
func (a *app) farProfile(machine machines.ID, id string) (settings.Profile, bool) {
	i := slices.IndexFunc(a.st.Profiles, func(p settings.Profile) bool { return p.ID == id && p.HostID == string(machine) })
	if i < 0 {
		return settings.Profile{}, false
	}
	return a.st.Profiles[i], true
}

// serverProfile is the profile id when it is on a saved server.
func (a *app) serverProfile(id string) (settings.Profile, bool) {
	i := slices.IndexFunc(a.st.Profiles, func(p settings.Profile) bool { return p.ID == id && p.HostID != "" })
	if i < 0 {
		return settings.Profile{}, false
	}
	return a.st.Profiles[i], true
}

// runNextOn has the next terminal on p's server run p, and is that
// server.
func (a *app) runNextOn(p settings.Profile) machines.ID {
	m := machines.ID(p.HostID)
	a.nextFar[m] = p.ID
	return m
}

// farConfig is how a shell on the saved server machine starts, at cols
// by rows, running profileID when that is one of the server's
// profiles: its command, in its folder.
func (a *app) farConfig(machine machines.ID, profileID string, cols, rows int) remote.ShellConfig {
	cfg := a.shellConfig(machine, cols, rows)
	p, ok := a.farProfile(machine, profileID)
	if !ok {
		return cfg
	}
	cfg.Command = p.Command
	if len(p.Env) > 0 {
		// sshd passes on only the variables its AcceptEnv names, so
		// they go through env, which every POSIX server has.
		cfg.Command = slices.Concat([]string{"env"}, p.Env, p.Command)
	}
	// The folder reaches cd in quotes, which keep ~ from being read as
	// home; a command over SSH starts at home, so the rest of the path
	// is enough.
	switch {
	case p.Folder == "~":
	case strings.HasPrefix(p.Folder, "~/"):
		cfg.Dir = strings.TrimPrefix(p.Folder, "~/")
	default:
		cfg.Dir = p.Folder
	}
	return cfg
}

// likeFar has the next terminal on the server of pane id run what the
// pane runs: its profile, or else the account's own shell, which the
// server's default profile would otherwise take the place of. A
// command, a pane of anything but a terminal, and a terminal asked for
// already leave it as it is.
func (a *app) likeFar(id string) {
	m := a.filesKey(id)
	if _, ok := a.commands[id]; ok || a.kindOfPane(id) != KindTerminal {
		return
	}
	if _, asked := a.nextFar[m]; asked {
		return
	}
	run := a.farRuns[id]
	if run == "" {
		run = loginShell
	}
	a.nextFar[m] = run
}

// profilesOn are the profiles on the saved server m, the one its new
// terminal runs first; none on this computer or a kakel window.
func (a *app) profilesOn(m machines.ID) []settings.Profile {
	if h, ok := a.machines.Saved(m); !ok || h.Window {
		return nil
	}
	var out []settings.Profile
	for _, p := range a.st.Profiles {
		if p.HostID == string(m) {
			out = append(out, p)
		}
	}
	if i := slices.IndexFunc(out, func(p settings.Profile) bool { return p.ID == a.st.ServerShells[string(m)] }); i > 0 {
		def := out[i]
		out = slices.Insert(slices.Delete(out, i, i+1), 0, def)
	}
	return out
}

// dropProfilesOn removes the profiles on the saved server m, as the
// server is removed, and which of them its new terminal ran. called is
// the server's name, for a failure to say.
func (a *app) dropProfilesOn(m machines.ID, called string) {
	if a.settings == nil || !slices.ContainsFunc(a.st.Profiles, func(p settings.Profile) bool { return p.HostID == string(m) }) {
		return
	}
	if err := a.settings.DropProfilesOn(string(m)); err != nil {
		a.failed("Couldn't remove the profiles on "+called, err.Error())
		return
	}
	a.st.Profiles = a.settings.Profiles()
	a.st.ServerShells = a.settings.ServerShells()
}
