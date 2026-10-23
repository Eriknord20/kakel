package shells

import (
	"os/exec"
	"reflect"
	"slices"
	"testing"
)

// gitMachine is a Windows machine with nothing on it but the files in have, and git.exe on the PATH
// at onPath when that is set.
func gitMachine(onPath string, env map[string]string, have ...string) probe {
	return probe{
		goos:   "windows",
		getenv: func(name string) string { return env[name] },
		lookPath: func(file string) (string, error) {
			if file == "git.exe" && onPath != "" {
				return onPath, nil
			}
			if slices.Contains(have, file) {
				return file, nil
			}
			return "", exec.ErrNotFound
		},
		distros: func() ([]string, error) { return nil, nil },
	}
}

// installed is the files of a Git for Windows install at root that the lookup asks for.
func installed(root string) []string {
	return []string{root + `\git-bash.exe`, root + `\bin\bash.exe`}
}

// Git Bash is found next to the git.exe on the PATH, and failing that in the usual install folders.
func TestGitBashIsFoundWhereGitIsInstalled(t *testing.T) {
	const (
		portable = `D:\Tools\Git`
		allUsers = `C:\Program Files\Git`
		x86      = `C:\Program Files (x86)\Git`
		oneUser  = `C:\Users\me\AppData\Local\Programs\Git`
		scoop    = `C:\Users\me\scoop\apps\git\current`
		cygwin   = `C:\cygwin64`
	)
	env := map[string]string{
		"ProgramFiles":      `C:\Program Files`,
		"ProgramFiles(x86)": `C:\Program Files (x86)`,
		"LOCALAPPDATA":      `C:\Users\me\AppData\Local`,
		"USERPROFILE":       `C:\Users\me`,
	}
	for _, c := range []struct {
		what   string
		onPath string
		have   []string
		want   string
	}{
		{"git.exe on the PATH from cmd", portable + `\cmd\git.exe`,
			append(installed(portable), installed(allUsers)...), portable},
		{"git.exe on the PATH from mingw64", portable + `\mingw64\bin\git.exe`, installed(portable), portable},
		{"git.exe on the PATH from mingw32", x86 + `\mingw32\bin\git.exe`, installed(x86), x86},
		{"git.exe on the PATH from clangarm64", portable + `\clangarm64\bin\git.exe`, installed(portable), portable},
		{"git.exe on the PATH from bin", portable + `\bin\git.exe`, installed(portable), portable},
		{"a git.exe that is not Git for Windows", `C:\Other\git.exe`, installed(allUsers), allUsers},
		{"Cygwin's git.exe first on the PATH", cygwin + `\bin\git.exe`,
			append([]string{cygwin + `\bin\bash.exe`}, installed(allUsers)...), allUsers},
		{"no git.exe on the PATH", "", installed(allUsers), allUsers},
		{"32-bit Git", "", installed(x86), x86},
		{"installed for one user", "", installed(oneUser), oneUser},
		{"installed with Scoop", "", installed(scoop), scoop},
		{"not installed", "", nil, ""},
		{"Cygwin alone", cygwin + `\bin\git.exe`, []string{cygwin + `\bin\bash.exe`}, ""},
		{"git.exe with no bash beside it", portable + `\cmd\git.exe`, []string{portable + `\git-bash.exe`}, ""},
	} {
		p := gitMachine(c.onPath, env, c.have...)
		got, ok := named(p, "gitbash")
		if c.want == "" {
			if ok {
				t.Errorf("when %s, Git Bash was found at %q", c.what, got.Path)
			}
			continue
		}
		want := Shell{ID: "gitbash", Title: "Git Bash", Path: c.want + `\bin\bash.exe`, Args: []string{"--login", "-i"},
			MSYS: true}
		if !ok || !reflect.DeepEqual(got, want) {
			t.Errorf("when %s, Git Bash is %+v, %v, want %+v", c.what, got, ok, want)
		}
		if list, _ := find(p); !reflect.DeepEqual(ids(list), []string{"gitbash"}) {
			t.Errorf("when %s the machine offers %v, want Git Bash", c.what, ids(list))
		}
	}
}

// A path Git Bash says is the Windows one, and a Windows path is typed into Git Bash its way.
func TestGitBashPathsTranslateBothWays(t *testing.T) {
	sh := Shell{ID: "gitbash", Path: `C:\Program Files\Git\bin\bash.exe`, MSYS: true}
	for _, c := range []struct{ msys, win string }{
		{"/c/Workspace", `C:\Workspace`},
		{"/g/workspace/kakel", `G:\workspace\kakel`},
		{"/d/a b", `D:\a b`},
		{"/c/", `C:\`},
		{"//nas/projects/a b", `\nas\projects\a b`},
	} {
		if got := sh.LocalPath(c.msys); got != c.win {
			t.Errorf("%q is %q to Windows, want %q", c.msys, got, c.win)
		}
		if got := sh.PanePath(c.win); got != c.msys {
			t.Errorf("%q is %q to Git Bash, want %q", c.win, got, c.msys)
		}
	}

	// Git's own folders are in the install.
	for _, c := range []struct{ msys, win string }{
		{"/usr/share", `C:\Program Files\Git\usr\share`},
		{"/etc", `C:\Program Files\Git\etc`},
		{"/", `C:\Program Files\Git\`},
		// The temporary folder is the user's, and a relative path is nowhere.
		{"/tmp", ""},
		{"/tmp/x", ""},
		{"relative", ""},
		{"", ""},
		{"//", ""},
	} {
		if got := sh.LocalPath(c.msys); got != c.win {
			t.Errorf("%q is %q to Windows, want %q", c.msys, got, c.win)
		}
	}

	// A path Git Bash has no name for is typed as it is.
	for _, p := range []string{`Workspace\kakel`, `\?\C:\Workspace`} {
		if got := sh.PanePath(p); got != p {
			t.Errorf("%q is %q to Git Bash, want it as it is", p, got)
		}
	}
}
