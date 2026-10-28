// Package shellsetup holds the lines that teach a shell to say what it
// is doing.
//
// A shell is a separate program, and kakel only sees the bytes it
// prints. So it cannot know which directory the shell is in, or where
// one command's output ends and the next begins, unless the shell says
// so. The shell says so by printing escape sequences nobody sees: OSC 7
// for the directory, OSC 133 around each command.
//
// The lines here are typed into the shell as soon as it starts, the way
// a user would type them. That is the one way that works everywhere: on
// a server the login shell is started by sshd and kakel never sees a
// command line to add to.
package shellsetup

import (
	"strings"

	"github.com/marrasen/kakel/shells"
)

// Route is the kind of shell the lines are written for.
type Route int

const (
	// NoRoute is a shell nothing is typed into.
	NoRoute Route = iota

	// Posix is bash, zsh and anything close enough to them, which is
	// what a server's login shell is taken to be.
	Posix

	// PowerShell is Windows PowerShell and PowerShell.
	PowerShell

	// Cmd is the Command Prompt.
	Cmd
)

// RouteFor is the kind of shell an argv runs.
//
// An empty argv means a login shell on a machine at the far end, which
// is taken to be POSIX: sshd starts whatever the account is set to, and
// nothing in the protocol says what that is.
func RouteFor(argv []string) Route {
	if len(argv) == 0 {
		return Posix
	}
	switch strings.ToLower(shells.CommandBase(argv[0])) {
	case "cmd", "cmd.exe":
		return Cmd
	case "powershell", "powershell.exe", "pwsh", "pwsh.exe":
		return PowerShell
	}
	return Posix
}

// Knows reports whether argv runs a shell the lines here are written
// for. A program it does not know, such as nushell, python or ssh,
// would take them as its own input, so a profile running one is left
// alone.
//
// A shell handed a command or a script, as by bash -c, cmd /c or
// pwsh -File, runs that instead of reading its terminal, and the lines
// would go to the command. So it is not known either.
//
// wsl is a shell only when it starts the distribution's login shell. A
// command it is given, as in wsl -e nu or wsl htop, is the program the
// lines would reach, so that is the one asked about.
func Knows(argv []string) bool {
	if len(argv) == 0 {
		return false
	}
	args := argv[1:]
	switch strings.TrimSuffix(strings.ToLower(shells.CommandBase(argv[0])), ".exe") {
	case "bash", "zsh", "sh", "ksh", "dash":
		return !posixScripted(args)
	case "cmd":
		return !cmdScripted(args)
	case "powershell", "pwsh":
		return !powerShellScripted(args)
	case "wsl":
		if run := wslRuns(args); len(run) > 0 {
			return Knows(run)
		}
		return true
	}
	return false
}

// posixScripted reports whether a POSIX shell's arguments give it
// something to run: -c, alone or among other letters as in -lc, or a
// script, the first word that is not an option.
func posixScripted(args []string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			return i+1 < len(args)
		case arg == "-o" || arg == "+o" || arg == "-O" || arg == "+O" || arg == "--rcfile" || arg == "--init-file":
			// Each takes the word after it as its value.
			i++
		case strings.HasPrefix(arg, "--") || strings.HasPrefix(arg, "+"):
		case strings.HasPrefix(arg, "-"):
			if strings.Contains(arg, "c") {
				return true
			}
		default:
			return true
		}
	}
	return false
}

// cmdScripted reports whether cmd's arguments give it a command to run
// and end with, /c. A command given with /k leaves the prompt open, and
// the words after /k are that command, not cmd's own.
func cmdScripted(args []string) bool {
	for _, arg := range args {
		if len(arg) < 2 || arg[0] != '/' {
			continue
		}
		switch arg[1] | 0x20 {
		case 'c':
			return true
		case 'k':
			return false
		}
	}
	return false
}

// powerShellScripted reports whether PowerShell's arguments give it a
// command, a script or an encoded command to run, unless -NoExit before
// it keeps the prompt open. PowerShell takes a parameter cut short, as
// -c or -com for -Command, so any such start counts.
func powerShellScripted(args []string) bool {
	stays := false
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			continue
		}
		name := strings.ToLower(strings.TrimLeft(arg, "-"))
		switch {
		case len(name) >= 3 && strings.HasPrefix("noexit", name):
			stays = true
		case name == "ec" || name != "" && (strings.HasPrefix("command", name) || strings.HasPrefix("file", name) || strings.HasPrefix("encodedcommand", name)):
			return !stays
		}
	}
	return false
}

// wslRuns is the command wsl's arguments run in the distribution, nil
// for its login shell: what follows -e, --exec or --, or else the first
// word that is not an option, which wsl runs through that shell.
func wslRuns(args []string) []string {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-e", "--exec", "--":
			return args[i+1:]
		case "-d", "--distribution", "--distribution-id", "-u", "--user", "--cd", "--shell-type":
			// Each takes the word after it as its value.
			i++
		case "~":
			// Starts the shell in the home folder, as Windows Terminal's
			// wsl.exe ~ -d Ubuntu does.
		default:
			if !strings.HasPrefix(args[i], "-") {
				return args[i:]
			}
		}
	}
	return nil
}

// Lines are what to type into a shell of that kind, one line each,
// ending with the one that clears the pane.
//
// The pane is cleared because the shell echoes what is typed into it,
// and a pane that opens on a screenful of setup is worse than one that
// opens empty. A shell's own banner goes with it.
func Lines(r Route) []string {
	switch r {
	case Posix:
		// The leading space keeps it out of the history of a shell set
		// to ignore lines that start with one.
		return []string{" " + posixLine, "clear"}
	case PowerShell:
		return []string{powerShellLine}
	case Cmd:
		// prompt takes the rest of its line literally, so the clear
		// cannot be joined onto it.
		return []string{cmdPrompt, "cls"}
	}
	return nil
}

// posixLine tells bash and zsh to mark the directory and each command.
//
// __gt_d runs before each prompt: it reports how the last command went,
// says where the shell is, and marks the prompt. __gt_c runs before a
// command and marks where its output starts. bash has no hook for that,
// so the debug trap stands in, skipping the calls this makes itself and
// firing once per command line.
const posixLine = `__gt_d(){ local c=$?; __gt_r=; printf '\033]133;D;%s\a\033]7;file://%s%s\a\033]133;A\a' "$c" "${HOSTNAME:-${HOST:-localhost}}" "$PWD"; }; ` +
	`__gt_c(){ case "$BASH_COMMAND" in __gt_*) return;; esac; [ -n "$__gt_r" ] || { __gt_r=1; printf '\033]133;C\a'; }; }; ` +
	`if [ -n "$ZSH_VERSION" ]; then autoload -Uz add-zsh-hook; add-zsh-hook precmd __gt_d; add-zsh-hook preexec __gt_c; ` +
	`else PROMPT_COMMAND="__gt_d${PROMPT_COMMAND:+;$PROMPT_COMMAND}"; trap __gt_c DEBUG; fi`

// powerShellLine wraps the prompt that is already there rather than
// replacing it, so a profile that sets one keeps it.
//
// The command mark goes on the function PSReadLine calls to read a
// line. Without PSReadLine the host reads the line itself and never
// calls it, so there is no mark and the rest still works.
const powerShellLine = `$global:GtPrompt=$function:prompt; ` +
	`function global:prompt { $ok=$?; $e=[char]27; $b=[char]7; $c=0; ` +
	`if(-not $ok){ if($global:LASTEXITCODE){$c=$global:LASTEXITCODE}else{$c=1} }; ` +
	`$h=(Get-Location).Path; ` +
	`Write-Host -NoNewline "$e]133;D;$c$b$e]7;file://localhost/$([uri]::EscapeUriString(($h -replace '\\','/')))$b$e]133;A$b"; ` +
	`$t=if($global:GtPrompt){& $global:GtPrompt}else{"PS $h> "}; "$t$e]133;B$b" }; ` +
	`if($function:PSConsoleHostReadLine){ $global:GtRead=$function:PSConsoleHostReadLine; ` +
	`function global:PSConsoleHostReadLine { $l=& $global:GtRead; Write-Host -NoNewline "$([char]27)]133;C$([char]7)"; $l } }; ` +
	`Clear-Host`

// cmdPrompt is the Command Prompt saying where it is.
//
// OSC 9;9 rather than OSC 7, because the prompt is built from what
// cmd.exe substitutes and it has no way to make a URL out of a path.
// $e is an escape, $p the directory and $g a greater-than sign.
const cmdPrompt = `prompt $e]9;9;$p$e\$p$g`

// Typed is the bytes to write to a shell's input, with a return after
// each line.
//
// A return rather than a newline: this goes in where the keyboard goes
// in, and the key sends a return.
func Typed(r Route) []byte {
	lines := Lines(r)
	if len(lines) == 0 {
		return nil
	}
	return []byte(strings.Join(lines, "\r") + "\r")
}
