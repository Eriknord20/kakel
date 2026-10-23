package shells

import (
	"errors"
	"strings"
)

// CommandBase is the program name in the first word of a command line.
//
// filepath.Base is not enough here. It splits on the separator of the
// machine this is running on, so a Linux build reads
// `C:\Windows\System32\cmd.exe` as one long name and recognises no
// shell in it. A pane runs a Windows shell whatever kakel itself is
// running on -- over SSH to a Windows server, or through WSL -- so a
// command line is taken apart with both separators everywhere, and the
// answer is the same on every machine.
//
// The cost is a program whose own name holds a backslash, which Linux
// allows and nothing anyone runs as a shell does.
func CommandBase(argv0 string) string {
	if i := strings.LastIndexAny(argv0, `/\`); i >= 0 {
		return argv0[i+1:]
	}
	return argv0
}

// SplitCommandLine is a command line as its program and arguments.
// Spaces and tabs part the words, and double quotes hold a word with
// spaces together, as Windows Terminal writes a profile's command:
// `"C:\Program Files\Git\bin\bash.exe" -i -l`.
//
// A backslash is a letter like any other, because a Windows path is
// full of them. So a word cannot hold a double quote.
func SplitCommandLine(line string) ([]string, error) {
	var (
		argv   []string
		word   strings.Builder
		inWord bool
		quoted bool
	)
	for _, r := range line {
		switch {
		case r == '"':
			quoted = !quoted
			inWord = true
		case !quoted && (r == ' ' || r == '\t'):
			if inWord {
				argv = append(argv, word.String())
				word.Reset()
				inWord = false
			}
		default:
			word.WriteRune(r)
			inWord = true
		}
	}
	if quoted {
		return nil, errors.New("unclosed quote")
	}
	if inWord {
		argv = append(argv, word.String())
	}
	return argv, nil
}

// JoinCommandLine is argv as SplitCommandLine reads it back: a word
// with a space or a tab in it, or none at all, is put in quotes.
func JoinCommandLine(argv []string) string {
	words := make([]string, len(argv))
	for i, w := range argv {
		if w == "" || strings.ContainsAny(w, " \t") {
			w = `"` + w + `"`
		}
		words[i] = w
	}
	return strings.Join(words, " ")
}
