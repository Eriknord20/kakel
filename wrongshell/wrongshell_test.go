package wrongshell

import (
	"strings"
	"testing"

	"github.com/marrasen/kakel/shellsetup"
)

const (
	ps   = shellsetup.PowerShell
	cmd  = shellsetup.Cmd
	bash = shellsetup.Posix
)

// Each line written for another shell is found where the other shell's
// way starts, as long as that way runs, with a message that names the
// shell it belongs to and says what to write instead.
func TestALineForAnotherShellIsFound(t *testing.T) {
	for _, c := range []struct {
		to         shellsetup.Route
		line, slip string
		says       string
	}{
		{ps, "export PATH=$PATH:/opt/bin", "export", "bash: write $env:"},
		{ps, "FOO=bar ./run.sh", "FOO", "bash: write $NAME"},
		{ps, "docker run --rm \\", "\\", "bash: end the line with `"},
		{ps, "make 2>/dev/null", "/dev/null", "bash: write $null"},
		{ps, "if [ -f x ]; then", "[", "bash: write if"},
		{ps, "fi", "fi", "bash: write if"},
		{ps, "for f in *.txt; do", "for", "bash: write foreach"},
		{ps, "cat <<EOF", "<<EOF", "bash: write @'"},
		{ps, "rm -rf build", "rm -rf", "bash: write Remove-Item"},
		{ps, "ls -la", "ls -la", "bash: write ls -Force"},
		{ps, "echo %USERPROFILE%", "%USERPROFILE%", "Command Prompt: write $env:NAME"},
		{ps, "set JAVA_HOME=C:\\jdk", "set", "Command Prompt: write $env:NAME ="},
		{ps, "@echo off", "@echo off", "Command Prompt: leave it out"},
		{ps, "REM build it", "REM", "Command Prompt: write #"},
		{cmd, "$env:PATH += ';C:\\bin'", "$env:PATH", "PowerShell: write %NAME%"},
		{cmd, "$env:GOOS = 'windows'", "$env:GOOS", "PowerShell: write set NAME=value"},
		{bash, "$env:GOOS = 'linux'", "$env:GOOS", "PowerShell: write export"},
		{cmd, "$name = 'x'", "$name", "PowerShell: write set"},
		{cmd, "Get-ChildItem -Recurse", "Get-ChildItem", "PowerShell: run it"},
		{cmd, "dotnet build `", "`", "PowerShell: end the line with ^"},
		{cmd, "export A=1", "export", "bash: write set"},
		{cmd, "echo $HOME", "$HOME", "bash: write %NAME%"},
		{cmd, "# a comment", "#", "bash: write REM"},
		{cmd, "ls -la", "ls", "bash: write dir"},
		{bash, "$env:Path", "$env:Path", "PowerShell: write $NAME"},
		{bash, "Write-Host hi", "Write-Host", "PowerShell: run it"},
		{bash, "$x = 5", "$x =", "PowerShell: write name=value"},
		{bash, "echo %PATH%", "%PATH%", "Command Prompt: write $NAME"},
		{bash, "set A=1", "set", "Command Prompt: write NAME=value"},
		{bash, "cls", "cls", "Command Prompt: write clear"},
		{bash, "dir /s", "dir /s", "Command Prompt: write ls"},
		{bash, "make ^", "^", "Command Prompt: end the line with \\"},
	} {
		got := Find("ok\n"+c.line, c.to)
		if len(got) != 1 {
			t.Errorf("%q to %v: found %v, want one slip", c.line, c.to, got)
			continue
		}
		s := got[0]
		r := []rune(c.line)
		if s.Line != 2 || s.Col < 1 || s.End > len(r)+1 || string(r[s.Col-1:s.End-1]) != c.slip {
			t.Errorf("%q to %v: slip at line %d, %d to %d, want %q", c.line, c.to, s.Line, s.Col, s.End, c.slip)
		}
		if !strings.Contains(s.Message, c.says) {
			t.Errorf("%q to %v: says %q, want it to say %q", c.line, c.to, s.Message, c.says)
		}
	}
}

// A shell's own way is no slip, nor is what single quotes hold, nor a
// line the shell takes as a comment.
func TestAShellsOwnWayIsNoSlip(t *testing.T) {
	for _, c := range []struct {
		to   shellsetup.Route
		line string
	}{
		{ps, "$env:PATH += ';C:\\bin'"},
		{ps, "Get-ChildItem -Recurse | Where-Object { $_.Length -gt 1kb }"},
		{ps, "if ($a -eq 1) { Write-Host 'export A=1' }"},
		{ps, "# export A=1"},
		{ps, "do {"},
		{ps, "1..3 | % { $_ * 2 }"},
		{ps, "docker run --rm `"},
		{ps, "cd C:\\"},
		{ps, "git commit -m 'fix && run'"},
		{cmd, "set JAVA_HOME=C:\\jdk"},
		{cmd, "echo %PATH%"},
		{cmd, "dir /s /b"},
		{cmd, "REM export A=1"},
		{cmd, ":: $env:X"},
		{cmd, "copy a.txt b.txt ^"},
		{bash, "export PATH=$PATH:/opt/bin"},
		{bash, "date +%Y%m%d-%H%M"},
		{bash, "echo $HOME && ls -la"},
		{bash, "if [ -f x ]; then echo yes; fi"},
		{bash, "x=1"},
		{bash, "make \\"},
		{bash, "echo 'Get-ChildItem $env:X'"},
		{bash, "# REM is the Command Prompt's"},
		{bash, "cat <<EOF"},
	} {
		if got := Find(c.line, c.to); len(got) != 0 {
			t.Errorf("%q to %v: found %v, want none", c.line, c.to, got)
		}
	}
}

// One slip a line at most, the first on it; and nothing for a shell
// not known.
func TestOneSlipALineAndNoneForAnUnknownShell(t *testing.T) {
	got := Find("export A=1 && echo %B%\nfine", ps)
	if len(got) != 1 || got[0].Col != 1 {
		t.Fatalf("found %v, want the export alone", got)
	}
	if got := Find("export A=1", shellsetup.NoRoute); got != nil {
		t.Fatalf("for no known shell, found %v", got)
	}
}
