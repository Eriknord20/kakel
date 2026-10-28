package app

import (
	"net"
	"os"
	"runtime"
	"strings"

	"github.com/marrasen/kakel/machines"

	"github.com/marrasen/kakel/internal/build"
	"github.com/marrasen/kakel/session"
	shellfind "github.com/marrasen/kakel/shells"
	"github.com/marrasen/kakel/shellsetup"
)

// What a shell here is started with: in the folder of the pane the user
// is in, told which terminal it runs in, and, with Shell Setup on,
// taught to say what it is doing, so paths it prints can be followed and
// an agent can tell a command's end.

// Intents for the shell's start.
type (
	// ToggleShellSetup turns the teaching of new shells here on or off.
	ToggleShellSetup struct{}
	// SetTermProgram sets what new shells here are told the terminal
	// is called, "" for kakel's own name.
	SetTermProgram struct{ Called string }
)

// KnownTerminals are names a user may give instead, for a program that
// shows images only in a terminal it knows.
var KnownTerminals = []string{"iTerm.app", "WezTerm", "vscode", "Apple_Terminal"}

// startLocalSession starts argv here, or the user's shell when it is
// nil, in dir, as a new shell is started. A shell, rather than one
// command, is taught when teaches says so.
func (a *app) startLocalSession(argv []string, dir string, cols, rows int, shell bool) (session.Session, error) {
	called := ""
	if a.settings != nil {
		called = a.settings.TermProgram()
	}
	profile, _ := a.profileOf(argv)
	if dir == "" {
		cwd, _ := os.Getwd()
		dir = a.startDir(cwd, profile.Folder)
	}
	sess, err := session.StartLocal(session.LocalConfig{
		Command: argv, Dir: dir, Env: paneEnv(argv, called, profile.Env), Cols: cols, Rows: rows,
	})
	if err != nil || !shell || !a.teaches(argv) {
		return sess, err
	}
	route := argv
	if len(route) == 0 {
		if got, err := session.DefaultShell(); err == nil {
			route = got
		}
	}
	if typed := shellsetup.Typed(shellsetup.RouteFor(route)); len(typed) > 0 {
		if _, err := sess.Write(typed); err != nil {
			a.failed("Couldn't set up the shell", err.Error())
		}
	}
	return sess, nil
}

// teaches reports whether a shell started with argv is taught to say
// what it is doing: with Shell Setup on, unless it is a profile's
// program the teaching is not written for.
func (a *app) teaches(argv []string) bool {
	if !a.st.ShellSetup {
		return false
	}
	_, isProfile := a.profileOf(argv)
	return !isProfile || shellsetup.Knows(argv)
}

// teachFar teaches a server's shell to say what it is doing, when its
// saved server says to. argv is the command of the profile it runs,
// nil for the account's own shell, and a profile's program the lines
// are not written for is left alone.
func (a *app) teachFar(machine machines.ID, sess session.Session, argv []string) {
	if len(argv) > 0 && !shellsetup.Knows(argv) {
		return
	}
	for _, h := range a.st.Saved {
		if machines.ID(h.ID) == machine && h.Setup {
			if typed := shellsetup.Typed(shellsetup.RouteFor(argv)); len(typed) > 0 {
				if _, err := sess.Write(typed); err != nil {
					a.failed("Couldn't set up the shell on "+a.machines.Name(machine), err.Error())
				}
			}
		}
	}
}

// paneEnv is what a shell here is told about the terminal it runs in,
// and then the variables its profile sets, each NAME=value, which come
// last so that one of them can name the terminal otherwise. A value may
// name the variables there already, as PATH=C:\tools;%PATH% does.
//
// A WSL shell is also told to carry each of them into the distribution,
// a Windows path translated, on top of the WSLENV the profile sets, or
// else the one the window runs with. PATH stays out: carried, it would
// take the place of the distribution's own, and WSL adds the Windows
// one to that by itself.
func paneEnv(argv []string, called string, set []string) []string {
	if called == "" {
		called = build.Name
	}
	env := []string{"TERM_PROGRAM=" + called, "TERM_PROGRAM_VERSION=" + build.Version()}
	wsl := shellfind.IsWSL(argv)
	carry, was := []string{"TERM_PROGRAM", "TERM_PROGRAM_VERSION"}, os.Getenv("WSLENV")
	for _, v := range set {
		name, value, _ := strings.Cut(v, "=")
		value = expandVars(value)
		switch {
		case wsl && strings.EqualFold(name, "WSLENV"):
			was = value
			continue
		case wsl && !strings.EqualFold(name, "PATH"):
			carry = append(carry, name+"/"+shellfind.PathFlag(value))
		}
		env = append(env, name+"="+value)
	}
	if !wsl {
		return env
	}
	return append(env, "WSLENV="+shellfind.CarryIntoWSL(was, carry...))
}

// expandVars is value with the variables it names filled in, written as
// this system's own shell writes them: %NAME% on Windows, $NAME or
// ${NAME} elsewhere. A name that is not set stays as it is written, as
// cmd leaves it, and a doubled sign, %% or $$, is one sign of its own.
func expandVars(value string) string {
	if runtime.GOOS == "windows" {
		return expandPercent(value)
	}
	return expandDollar(value)
}

// expandPercent fills in each %NAME% in value that is set.
func expandPercent(value string) string {
	var out strings.Builder
	for {
		start := strings.IndexByte(value, '%')
		if start < 0 {
			break
		}
		out.WriteString(value[:start])
		value = value[start+1:]
		switch end := strings.IndexByte(value, '%'); {
		case end == 0:
			out.WriteByte('%')
			value = value[1:]
			continue
		case end > 0:
			if got, ok := os.LookupEnv(value[:end]); ok {
				out.WriteString(got)
				value = value[end+1:]
				continue
			}
		}
		// Not a variable: this % is a letter, and the next may start one.
		out.WriteByte('%')
	}
	return out.String() + value
}

// expandDollar fills in each $NAME and ${NAME} in value that is set.
func expandDollar(value string) string {
	var out strings.Builder
	for {
		start := strings.IndexByte(value, '$')
		if start < 0 {
			break
		}
		out.WriteString(value[:start])
		value = value[start+1:]
		if strings.HasPrefix(value, "$") {
			out.WriteByte('$')
			value = value[1:]
			continue
		}
		name, written := dollarName(value)
		if got, ok := os.LookupEnv(name); ok && name != "" {
			out.WriteString(got)
			value = value[written:]
			continue
		}
		out.WriteByte('$')
	}
	return out.String() + value
}

// dollarName is the name s starts with, after a $, and how many bytes
// it is written with: NAME, or NAME in braces. It is "" when s starts
// with neither.
func dollarName(s string) (string, int) {
	braced := strings.HasPrefix(s, "{")
	if braced {
		s = s[1:]
	}
	n := 0
	for n < len(s) && (s[n] == '_' || 'a' <= s[n] && s[n] <= 'z' || 'A' <= s[n] && s[n] <= 'Z' || n > 0 && '0' <= s[n] && s[n] <= '9') {
		n++
	}
	switch {
	case !braced:
		return s[:n], n
	case n > 0 && n < len(s) && s[n] == '}':
		return s[:n], n + 2
	}
	return "", 0
}

// dirHere is the folder of the focused pane's shell, when it is on this
// machine and has said, for a new shell to start in.
func (a *app) dirHere() string {
	id := a.here()
	t := a.terminal(id)
	if t == nil || a.machineOf(id) != "" {
		return ""
	}
	dir, host := t.Dir()
	if !isThisMachine(host) {
		return ""
	}
	return a.localDir(id, dir)
}

// here is the pane a new shell is like: the focused one, or while a
// split's chooser has the keyboard, the pane it was split from.
func (a *app) here() string {
	if from, ok := a.choosers[a.st.Focus]; ok {
		return from
	}
	return a.st.Focus
}

// isThisMachine reports whether a host a shell named is this computer.
// By the short name, as a shell sends what hostname gives it, which may
// or may not carry the domain.
func isThisMachine(host string) bool {
	switch strings.ToLower(host) {
	case "", "localhost", "127.0.0.1", "::1":
		return true
	}
	name, err := os.Hostname()
	if err != nil {
		return false
	}
	return strings.EqualFold(shortHost(name), shortHost(host))
}

// shortHost is a host name up to its first dot: an address is whole.
func shortHost(host string) string {
	if net.ParseIP(host) != nil {
		return host
	}
	short, _, _ := strings.Cut(host, ".")
	return short
}

// localDir is a folder a pane says it is in, as this computer names
// it, or "" when it names none here. A WSL shell says a Linux path, and
// under the computer's own name, since WSL shares it: /mnt/d/src is
// D:\src, and its own files are on \\wsl.localhost. Git Bash says /d/src
// for D:\src. A folder that is not here, as a path from another system,
// gives "", so a new shell starts where it would have rather than
// failing to start.
func (a *app) localDir(pane, dir string) string {
	if pathSep == `\` && strings.HasPrefix(dir, "/") {
		if sh := a.shellOf(pane); sh.OwnPaths() {
			dir = sh.LocalPath(dir)
		} else {
			dir = shellfind.DrivePath(dir)
		}
	}
	if dir == "" {
		return ""
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return ""
	}
	return dir
}

// pathSep is this system's separator of folders in a path.
const pathSep = string(os.PathSeparator)
