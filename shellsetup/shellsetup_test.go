package shellsetup

import (
	"strings"
	"testing"
)

// The shell an argv runs decides what is typed into it.
func TestWhichShellAnArgvRuns(t *testing.T) {
	for _, tc := range []struct {
		argv []string
		want Route
	}{
		{[]string{`C:\Windows\System32\cmd.exe`}, Cmd},
		{[]string{`C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`}, PowerShell},
		{[]string{`C:\Program Files\PowerShell\7\pwsh.exe`}, PowerShell},
		{[]string{`C:\Windows\System32\wsl.exe`, "-d", "Ubuntu"}, Posix},
		{[]string{"/bin/bash"}, Posix},
		{[]string{"/usr/bin/zsh"}, Posix},
		// A login shell on a machine at the far end, which sshd picks
		// and the protocol never names.
		{nil, Posix},
	} {
		if got := RouteFor(tc.argv); got != tc.want {
			t.Errorf("%v came out as route %d, want %d", tc.argv, got, tc.want)
		}
	}
}

// The lines are typed only into a shell they are written for: a
// profile running anything else would take them as its own input.
func TestOnlyAShellTheLinesAreWrittenForIsKnown(t *testing.T) {
	for _, tc := range []struct {
		argv []string
		want bool
	}{
		{[]string{`C:\Program Files\Git\bin\bash.exe`, "-i", "-l"}, true},
		{[]string{`C:\msys64\usr\bin\ZSH.EXE`}, true},
		{[]string{"/bin/sh"}, true},
		{[]string{"ksh"}, true},
		{[]string{"dash"}, true},
		{[]string{`C:\Windows\System32\cmd.exe`, "/k", "echo hi"}, true},
		{[]string{"powershell.exe", "-NoLogo"}, true},
		{[]string{"pwsh"}, true},
		{[]string{`C:\Windows\System32\wsl.exe`, "-d", "Ubuntu"}, true},
		{[]string{"wsl.exe", "-d", "Ubuntu", "--cd", "/home/x"}, true},
		{[]string{"wsl", "-u", "root", "-e", "bash", "-l"}, true},
		{[]string{"wsl", "-e", "nu"}, false},
		{[]string{"wsl", "--exec", "htop"}, false},
		{[]string{"wsl", "--", "htop"}, false},
		{[]string{"wsl", "-d", "Ubuntu", "htop"}, false},
		{[]string{"wsl.exe", "~", "-d", "Ubuntu"}, true},
		{[]string{"wsl", "-e", "bash", "-c", "make"}, false},
		// A shell handed something to run runs that, not what is typed.
		{[]string{"bash", "-c", "make test"}, false},
		{[]string{"bash", "-lc", "make"}, false},
		{[]string{"zsh", "deploy.sh"}, false},
		{[]string{"bash", "-o", "vi", "--login"}, true},
		{[]string{"bash", "-l", "-Cl"}, true},
		{[]string{"cmd.exe", "/c", "build.bat"}, false},
		{[]string{"cmd.exe", "/D", "/C", "dir"}, false},
		{[]string{"cmd.exe", "/k", "echo /c"}, true},
		{[]string{"pwsh", "-Command", "Get-Date"}, false},
		{[]string{"pwsh", "-c", "Get-Date"}, false},
		{[]string{"powershell.exe", "-NoProfile", "-File", "x.ps1"}, false},
		{[]string{"powershell.exe", "-ec", "ZQBjAGgAbwA="}, false},
		{[]string{"pwsh", "-NoExit", "-Command", "cd ~"}, true},
		{[]string{"pwsh", "-ExecutionPolicy", "Bypass", "-NoLogo"}, true},
		{[]string{"nu"}, false},
		{[]string{`C:\Python312\python.exe`}, false},
		{[]string{"ssh", "-t", "box"}, false},
		{[]string{"bashful"}, false},
		{nil, false},
	} {
		if got := Knows(tc.argv); got != tc.want {
			t.Errorf("%q is known: %v, want %v", tc.argv, got, tc.want)
		}
	}
}

// Every shell gets lines to type, and the pane ends up cleared: what
// is typed is echoed, and a pane that opens on a screenful of setup is
// worse than one that opens empty.
func TestEveryShellIsLeftWithACleanPane(t *testing.T) {
	for _, tc := range []struct {
		route Route
		clear string
	}{
		{Posix, "clear"},
		{PowerShell, "Clear-Host"},
		{Cmd, "cls"},
	} {
		lines := Lines(tc.route)
		if len(lines) == 0 {
			t.Errorf("route %d has nothing to type", tc.route)
			continue
		}
		if last := lines[len(lines)-1]; !strings.HasSuffix(last, tc.clear) {
			t.Errorf("route %d ends with %q, want it to end by clearing the pane", tc.route, last)
		}
	}
}

// Nothing is typed into a shell nobody named.
func TestNoRouteTypesNothing(t *testing.T) {
	if got := Typed(NoRoute); len(got) != 0 {
		t.Errorf("%q was typed into no shell at all", got)
	}
}

// What is typed ends every line with a return, because this goes in
// where the keyboard goes in.
func TestEachLineEndsWithAReturn(t *testing.T) {
	got := string(Typed(Cmd))

	if strings.Contains(got, "\n") {
		t.Errorf("%q holds a newline, which is not what the enter key sends", got)
	}
	if !strings.HasSuffix(got, "\r") {
		t.Errorf("%q does not end with a return", got)
	}
	if n := strings.Count(got, "\r"); n != len(Lines(Cmd)) {
		t.Errorf("%d returns for %d lines", n, len(Lines(Cmd)))
	}
}

// The line for bash and zsh says where the shell is and marks each
// command, and starts with a space so a shell set to ignore those
// keeps it out of the history.
func TestTheLineForBashAndZsh(t *testing.T) {
	lines := Lines(Posix)
	setup := lines[0]

	if !strings.HasPrefix(setup, " ") {
		t.Error("it does not start with a space, so it goes into the history")
	}
	for _, want := range []string{`]7;file://`, `]133;A`, `]133;C`, `]133;D`} {
		if !strings.Contains(setup, want) {
			t.Errorf("it does not send %s", want)
		}
	}
	for _, want := range []string{"ZSH_VERSION", "PROMPT_COMMAND", "precmd", "preexec"} {
		if !strings.Contains(setup, want) {
			t.Errorf("it does not mention %s, so it does not cover both shells", want)
		}
	}
	if strings.Contains(setup, "\n") {
		t.Error("it is more than one line, and only one line can be typed in")
	}
}

// The PowerShell line keeps the prompt that was already there, so a
// profile that sets one is not thrown away.
func TestThePowerShellLineKeepsTheOldPrompt(t *testing.T) {
	setup := Lines(PowerShell)[0]

	if !strings.Contains(setup, "$function:prompt") {
		t.Error("it does not take hold of the prompt that was there")
	}
	if !strings.Contains(setup, "& $global:GtPrompt") {
		t.Error("it does not call the prompt that was there")
	}
	for _, want := range []string{`]7;file://`, `]133;A`, `]133;B`, `]133;C`, `]133;D`} {
		if !strings.Contains(setup, want) {
			t.Errorf("it does not send %s", want)
		}
	}
}

// The Command Prompt says where it is as a plain path. It builds its
// prompt out of what cmd.exe substitutes, and none of that makes a
// URL, so OSC 7 is not open to it.
func TestTheCommandPromptSaysWhereItIsAsAPath(t *testing.T) {
	setup := Lines(Cmd)[0]

	if !strings.Contains(setup, "]9;9;$p") {
		t.Errorf("%q does not send the directory as OSC 9;9", setup)
	}
	if strings.Contains(setup, "file://") {
		t.Errorf("%q tries to send a URL, which cmd.exe cannot build", setup)
	}
}
