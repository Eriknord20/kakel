// Package wrongshell finds the lines of a paste written for another
// shell than the one it goes to: bash pasted into PowerShell, PowerShell
// into the Command Prompt, and the Command Prompt into bash, each way
// round. It looks for what is plainly one shell's and wrong in another,
// such as export in PowerShell or $env:PATH in the Command Prompt, and
// says how the shell the paste goes to writes it. Text in single quotes
// is passed over, and so is a line the shell takes as a comment.
package wrongshell

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/marrasen/kakel/shellsetup"
)

// A Slip is a place in a paste written for another shell. Line and Col
// count from 1; End is the column just after the slip.
type Slip struct {
	Line, Col, End int
	Message        string
}

// rule is one shell's way of writing something that is wrong in
// another: where it matches a line, the part in its first group, or the
// whole match without one, is the slip.
type rule struct {
	re  *regexp.Regexp
	msg string
}

func r(expr, msg string) rule { return rule{re: regexp.MustCompile(expr), msg: msg} }

// The ways each shell writes things, as rules to check a paste for when
// it goes to another shell.
var (
	cmdlet = `\b((?:Get|Set|New|Remove|Invoke|Start|Stop|Test|Write|Read|Select|Where|ForEach|Sort|Group|Measure|Out|` +
		`Copy|Move|Rename|Add|Clear|Import|Export|ConvertTo|ConvertFrom|Enable|Disable|Install|Uninstall|Update|` +
		`Resolve|Split|Join|Wait|Restart|Format)-[A-Z][A-Za-z]+)\b`

	// psEnv is PowerShell's environment variable, and psEnvSet one set
	// at the start of a line.
	psEnv    = `(?i)(\$env:[a-z_][a-z0-9_]*)`
	psEnvSet = `(?i)^\s*(\$env:[a-z_][a-z0-9_]*)\s*=`
	// psAssign is a PowerShell variable set at the start of a line.
	psAssign = `^\s*(\$[A-Za-z_]\w*)\s*=`

	bashExport   = `(?:^|;)\s*(export)\s+[A-Za-z_]\w*=`
	bashAssign   = `^\s*([A-Za-z_]\w*)=\S`
	bashContinue = `\s(\\)\s*$`
	devNull      = `(/dev/null)\b`
	bashBlock    = `(?:^|;)\s*(then|fi|done|esac|elif)(?:\s|;|$)`
	bashTest     = `^\s*(?:if|while)\s+(\[\[?)\s`
	bashFor      = `^\s*(for)\s+[A-Za-z_]\w*\s+in\s`
	hereDoc      = `(<<-?\s*['"]?[A-Za-z_]\w*['"]?)\s*$`

	cmdVar      = `(%[A-Za-z_]\w+%)`
	cmdSet      = `(?i)^\s*@?(set)\s+(?:/[ap]\s+)?[a-z_]\w*=`
	cmdRem      = `(?i)^\s*@?(rem)(?:\s|$)`
	cmdColons   = `^\s*(::)`
	cmdEchoOff  = `(?i)^\s*(@?echo\s+off)\s*$`
	cmdContinue = `\s(\^)\s*$`
)

// rules are what to look for in a paste going to each kind of shell.
var rules = map[shellsetup.Route][]rule{
	shellsetup.PowerShell: {
		r(bashExport, "bash: write $env:NAME = 'value'"),
		r(bashAssign, "bash: write $NAME = 'value'"),
		r(bashContinue, "bash: end the line with ` instead"),
		r(devNull, "bash: write $null"),
		r(bashBlock, "bash: write if (…) { … }, foreach (…) { … }"),
		r(bashTest, "bash: write if (…) { … }"),
		r(bashFor, "bash: write foreach ($x in …) { … }"),
		r(hereDoc, "bash: write @' … '@"),
		r(`^\s*(rm\s+-[a-zA-Z]*[rf][a-zA-Z]*)\b`, "bash: write Remove-Item -Recurse -Force"),
		r(`^\s*(ls\s+-[a-zA-Z]+)\b`, "bash: write ls -Force"),
		r(`^\s*(source)\s`, "bash: write . .\\script.ps1"),
		r(cmdVar, "Command Prompt: write $env:NAME"),
		r(cmdSet, "Command Prompt: write $env:NAME = 'value'"),
		r(cmdRem, "Command Prompt: write # for a comment"),
		r(cmdColons, "Command Prompt: write # for a comment"),
		r(cmdEchoOff, "Command Prompt: leave it out"),
		r(cmdContinue, "Command Prompt: end the line with ` instead"),
	},
	shellsetup.Cmd: {
		r(psEnvSet, "PowerShell: write set NAME=value"),
		r(psEnv, "PowerShell: write %NAME%"),
		r(psAssign, "PowerShell: write set name=value"),
		r(cmdlet, "PowerShell: run it in PowerShell"),
		r("(`)\\s*$", "PowerShell: end the line with ^ instead"),
		r(bashContinue, "bash: end the line with ^ instead"),
		r(bashExport, "bash: write set NAME=value"),
		r(bashAssign, "bash: write set NAME=value"),
		r(devNull, "bash: write nul"),
		r(bashBlock, "bash: write if … ( … )"),
		r(bashFor, "bash: write for %x in (…) do …"),
		r(hereDoc, "bash: the Command Prompt has none"),
		r(`^\s*(#)`, "bash: write REM for a comment"),
		r(`^\s*(ls|cat|rm|cp|mv|grep|clear|pwd|touch)(?:\s|$)`, "bash: write dir, type, del, copy, move, findstr, cls or cd"),
		r(`(\$\{?[A-Za-z_]\w*\}?)`, "bash: write %NAME%"),
	},
	shellsetup.Posix: {
		r(psEnvSet, "PowerShell: write export NAME=value"),
		r(psEnv, "PowerShell: write $NAME"),
		r(cmdlet, "PowerShell: run it in PowerShell"),
		r(`^\s*(\$[A-Za-z_]\w*\s+=)\s`, "PowerShell: write name=value, with no spaces"),
		r("(`)\\s*$", "PowerShell: end the line with \\ instead"),
		r(cmdVar, "Command Prompt: write $NAME"),
		r(cmdSet, "Command Prompt: write NAME=value"),
		r(cmdRem, "Command Prompt: write # for a comment"),
		r(cmdColons, "Command Prompt: write # for a comment"),
		r(cmdEchoOff, "Command Prompt: leave it out"),
		r(cmdContinue, "Command Prompt: end the line with \\ instead"),
		r(`(?i)^\s*(cls|del|findstr|cd\s+/d)(?:\s|$)`, "Command Prompt: write clear, rm, grep or cd"),
		r(`(?i)^\s*(dir\s+/[a-z])`, "Command Prompt: write ls"),
	},
}

// Find returns the slips in text, a paste going to a shell of kind to:
// at most one on a line, the first.
func Find(text string, to shellsetup.Route) []Slip {
	rs := rules[to]
	if len(rs) == 0 {
		return nil
	}
	var out []Slip
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if comment(line, to) {
			continue
		}
		masked := unquoted(line, to)
		best, bestAt := -1, len(masked)+1
		var bestEnd int
		for k, ru := range rs {
			m := ru.re.FindStringSubmatchIndex(masked)
			if m == nil {
				continue
			}
			at, end := m[0], m[1]
			if len(m) >= 4 && m[2] >= 0 {
				at, end = m[2], m[3]
			}
			if at < bestAt {
				best, bestAt, bestEnd = k, at, end
			}
		}
		if best < 0 {
			continue
		}
		col := utf8.RuneCountInString(line[:bestAt]) + 1
		out = append(out, Slip{Line: i + 1, Col: col, End: col + utf8.RuneCountInString(line[bestAt:bestEnd]),
			Message: rs[best].msg})
	}
	return out
}

// comment reports whether to takes line as a comment: one starting with
// # in bash and PowerShell, REM or :: in the Command Prompt.
func comment(line string, to shellsetup.Route) bool {
	t := strings.TrimSpace(line)
	switch to {
	case shellsetup.Posix, shellsetup.PowerShell:
		return strings.HasPrefix(t, "#")
	case shellsetup.Cmd:
		l := strings.ToLower(t)
		return strings.HasPrefix(l, "::") || l == "rem" || strings.HasPrefix(l, "rem ") || strings.HasPrefix(l, "@rem ")
	}
	return false
}

// unquoted is line with what single quotes hold blanked out, rune for
// rune, where to takes it literally: bash and PowerShell do, the
// Command Prompt does not. The quotes stay.
func unquoted(line string, to shellsetup.Route) string {
	if to == shellsetup.Cmd {
		return line
	}
	var b strings.Builder
	in := false
	for _, c := range line {
		switch {
		case c == '\'':
			in = !in
			b.WriteRune(c)
		case in:
			// The same number of bytes, so the columns stay.
			b.WriteString(strings.Repeat(" ", utf8.RuneLen(c)))
		default:
			b.WriteRune(c)
		}
	}
	return b.String()
}
