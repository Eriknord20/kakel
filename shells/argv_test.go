package shells

import (
	"slices"
	"testing"
)

// A command line splits into its program and arguments the way Windows
// Terminal writes a profile's command, and comes back as it was.
func TestACommandLineSplitsAndJoinsBack(t *testing.T) {
	for _, c := range []struct {
		line string
		argv []string
	}{
		{`"C:\Program Files\Git\bin\bash.exe" -i -l`, []string{`C:\Program Files\Git\bin\bash.exe`, "-i", "-l"}},
		{`C:\msys64\usr\bin\bash.exe --login`, []string{`C:\msys64\usr\bin\bash.exe`, "--login"}},
		{`ssh -t box "tmux attach"`, []string{"ssh", "-t", "box", "tmux attach"}},
		{`nu ""`, []string{"nu", ""}},
		{`nu`, []string{"nu"}},
	} {
		got, err := SplitCommandLine(c.line)
		if err != nil {
			t.Errorf("%s: %v", c.line, err)
			continue
		}
		if !slices.Equal(got, c.argv) {
			t.Errorf("%s split into %q, want %q", c.line, got, c.argv)
		}
		if back := JoinCommandLine(got); back != c.line {
			t.Errorf("%q joined back as %s, want %s", got, back, c.line)
		}
	}
}

// Spaces around and between the words count for nothing, and quotes in
// the middle of a word only hold it together.
func TestACommandLineSplitsWhateverItsSpacing(t *testing.T) {
	for _, c := range []struct {
		line string
		argv []string
	}{
		{"  bash\t-i   -l  ", []string{"bash", "-i", "-l"}},
		{`--cd="C:\My Work"`, []string{`--cd=C:\My Work`}},
		{"", nil},
		{"   ", nil},
	} {
		got, err := SplitCommandLine(c.line)
		if err != nil {
			t.Errorf("%q: %v", c.line, err)
			continue
		}
		if !slices.Equal(got, c.argv) {
			t.Errorf("%q split into %q, want %q", c.line, got, c.argv)
		}
	}
}

// A quote left open is an error rather than a guess at where it ends.
func TestAnUnclosedQuoteIsAnError(t *testing.T) {
	for _, line := range []string{`"C:\Program Files\Git\bin\bash.exe -i`, `bash "`} {
		if got, err := SplitCommandLine(line); err == nil {
			t.Errorf("%s split into %q", line, got)
		}
	}
}
