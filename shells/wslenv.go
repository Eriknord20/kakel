package shells

import (
	"strings"
)

// IsWSL reports whether a command line starts a WSL distribution.
func IsWSL(argv []string) bool {
	return len(argv) > 0 && strings.EqualFold(CommandBase(argv[0]), "wsl.exe")
}

// CarryIntoWSL is a WSLENV value that adds names to the ones already
// carried into a distribution. A variable Windows holds does not reach
// a WSL shell unless WSLENV names it.
//
// was is the WSLENV the window is running with, and a name already in
// it is left alone with whatever flags the user gave it. A name may
// bring flags of its own, as JAVA_HOME/p does, which are kept.
func CarryIntoWSL(was string, names ...string) string {
	held := map[string]bool{}
	for _, entry := range strings.Split(was, ":") {
		if name, _, _ := strings.Cut(entry, "/"); name != "" {
			held[name] = true
		}
	}
	out := was
	for _, entry := range names {
		name, flags, _ := strings.Cut(entry, "/")
		if held[name] {
			continue
		}
		if out != "" {
			out += ":"
		}
		// u carries it into WSL and not back out again, which is what a
		// name Windows set is for.
		out += name + "/" + flags + "u"
		held[name] = true
	}
	return out
}

// PathFlag is the WSLENV flag that has WSL translate value into the
// paths a distribution reads: l for a list of Windows paths, as
// C:\tools;D:\bin, p for one, and none for anything else, which goes
// in as it is.
func PathFlag(value string) string {
	paths := 0
	for _, part := range strings.Split(value, ";") {
		if part == "" {
			continue
		}
		if !windowsPath(part) {
			return ""
		}
		paths++
	}
	switch {
	case paths == 0:
		return ""
	case strings.Contains(value, ";"):
		return "l"
	}
	return "p"
}

// windowsPath reports whether s starts as an absolute Windows path
// does: a drive, as C:\ or C:/, or a share, as \\server.
func windowsPath(s string) bool {
	return strings.HasPrefix(s, `\\`) || UnixPath(s) != ""
}
