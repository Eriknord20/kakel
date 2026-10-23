package shells

import "strings"

// OwnPaths reports whether the shell names this machine's folders its own way rather than as Windows
// does: a WSL distribution and Git Bash.
func (s Shell) OwnPaths() bool { return s.Distro != "" || s.MSYS }

// LocalPath is a path the shell names, as Windows names it, or "" when Windows cannot reach it. A shell
// that names folders as Windows does gives the path as it is.
func (s Shell) LocalPath(p string) string {
	switch {
	case s.Distro != "":
		return WindowsPath(s.Distro, p)
	case s.MSYS:
		return s.msysLocal(p)
	}
	return p
}

// PanePath is a Windows path as the shell takes it, or the path as it is when the shell has no name
// for it.
func (s Shell) PanePath(win string) string {
	var p string
	switch {
	case s.Distro != "":
		p = UnixPath(win)
	case s.MSYS:
		p = MSYSPath(win)
	}
	if p == "" {
		return win
	}
	return p
}

// MSYSPath translates a path for Git Bash, so C:\Workspace is /c/Workspace and \nas\share is
// //nas/share. A relative path gives "".
func MSYSPath(win string) string {
	if rest, ok := strings.CutPrefix(win, `\`); ok && !strings.HasPrefix(rest, `?\`) && rest != "" {
		return "//" + strings.ReplaceAll(rest, `\`, "/")
	}
	unix := UnixPath(win)
	if unix == "" {
		return ""
	}
	return strings.TrimPrefix(unix, "/mnt")
}

// msysLocal translates a path Git Bash says into the Windows one: /c/Workspace is C:\Workspace,
// //nas/share is \nas\share, and /usr/bin is the bin folder of the install's usr. It gives "" for
// a relative path, and for /tmp, which Git Bash keeps in the user's temporary folder.
func (s Shell) msysLocal(unix string) string {
	if rest, ok := strings.CutPrefix(unix, "//"); ok {
		if rest == "" {
			return ""
		}
		return `\` + strings.ReplaceAll(rest, "/", `\`)
	}
	if !strings.HasPrefix(unix, "/") {
		return ""
	}
	if drive := DrivePath("/mnt" + unix); drive != "" {
		return drive
	}
	if unix == "/tmp" || strings.HasPrefix(unix, "/tmp/") {
		return ""
	}
	root := s.msysRoot()
	if root == "" {
		return ""
	}
	if unix == "/" {
		return root + `\`
	}
	return root + strings.ReplaceAll(unix, "/", `\`)
}

// msysRoot is the folder Git Bash calls /, which holds the bin folder its bash.exe is in.
func (s Shell) msysRoot() string {
	const tail = `\bin\bash.exe`
	if len(s.Path) <= len(tail) || !strings.EqualFold(s.Path[len(s.Path)-len(tail):], tail) {
		return ""
	}
	return s.Path[:len(s.Path)-len(tail)]
}
