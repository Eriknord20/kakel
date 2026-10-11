package view

import (
	"strings"
	"testing"
	"time"

	gi "github.com/marrasen/gunim/input"
	"github.com/marrasen/gunim/widget"
	"github.com/marrasen/kakel/app"
	"github.com/marrasen/kakel/internal/sessiontest"
	"github.com/marrasen/kakel/screen"
	"github.com/marrasen/kakel/shellsetup"
	"github.com/marrasen/kakel/vt"
)

func TestPasteLinesLeavesOutTheLastBreak(t *testing.T) {
	for s, want := range map[string]int{
		"ls":            1,
		"ls\n":          1,
		"ls\r\n":        1,
		"a\nb":          2,
		"a\r\nb\r\n":    2,
		"a\rb":          2,
		"a\n\nb":        3,
		"\n\n":          2,
		"one\ntwo\nthr": 3,
	} {
		if got := pasteLines(s); got != want {
			t.Errorf("pasteLines(%q) = %d, want %d", s, got, want)
		}
	}
	if needsCheck("ls -la\n") {
		t.Error("a line copied whole opens in the editor")
	}
	if !needsCheck(strings.Repeat("x", pasteBytes+1)) {
		t.Error("a paste over 5 KB goes straight in")
	}
}

// pasteStage is a window with one pane, p1, whose session records
// what it is sent, with the paste check on or off.
func pasteStage(t *testing.T, check bool) (*Window, *sessiontest.Typed) {
	t.Helper()
	win, sh, publish := windowStage(t)
	typed := sessiontest.New()
	quiet := screen.Hooks{Output: func() {}, Title: func(string) {}, Exit: func() {}, Clipboard: func(string) {}}
	sh.Set("p1", screen.Open(typed, vt.DefaultPalette(), quiet))
	t.Cleanup(func() { _ = sh.Get("p1").T.Close() })
	publish(app.State{Panes: []app.Pane{{ID: "p1", Title: "Terminal 1"}}, Stage: &app.Box{Pane: "p1"}, Focus: "p1", PasteCheck: check})
	return win, typed
}

// sentSoon waits a moment for the session to be sent want.
func sentSoon(typed *sessiontest.Typed, want string) string {
	got := typed.Sent()
	for end := time.Now().Add(time.Second); !strings.Contains(got, want) && time.Now().Before(end); got = typed.Sent() {
		time.Sleep(5 * time.Millisecond)
	}
	return got
}

func pressKey(k gi.Key, mods gi.Mods) {
	lastWindow.Input(gi.KeyPress{Key: k, Mods: mods, Time: time.Now()})
	frames(5)
}

// pasteArea is the editor in the paste dialog open.
func pasteArea(t *testing.T, win *Window) *widget.CodeEditor {
	t.Helper()
	if win.dialog == nil {
		t.Fatal("no dialog opened")
	}
	for _, f := range formOf(win.dialog.Body).Children() {
		if a, ok := f.(*widget.CodeEditor); ok {
			return a
		}
	}
	t.Fatal("the paste dialog has no editor")
	return nil
}

func TestALinePastedGoesStraightIn(t *testing.T) {
	win, typed := pasteStage(t, true)
	lastUI.SetClipboard("ls -la\n")
	win.terms["p1"].pasteClipboard(lastUI)
	frames(5)
	if win.dialog != nil {
		t.Fatal("one line opened a dialog")
	}
	if got := sentSoon(typed, "ls -la"); got != "ls -la\r" {
		t.Fatalf("the pane was sent %q", got)
	}
}

func TestLinesPastedOpenInAnEditorFirst(t *testing.T) {
	win, typed := pasteStage(t, true)
	lastUI.SetClipboard("echo one\r\necho two\r\n")
	win.terms["p1"].pasteClipboard(lastUI)
	frames(5)
	area := pasteArea(t, win)
	if win.dialog.Title != "Paste 2 Lines" {
		t.Fatalf("the dialog is titled %q", win.dialog.Title)
	}
	if area.Text() != "echo one\necho two\n" {
		t.Fatalf("the editor holds %q", area.Text())
	}
	if got := typed.Sent(); got != "" {
		t.Fatalf("before Paste, the pane was sent %q", got)
	}
	if lastUI.Focused() != area {
		t.Fatalf("the keyboard is in %T, not the editor", lastUI.Focused())
	}
	// Edited, and Shift+Enter starting a line, then Enter pastes.
	area.SetText("echo three", lastUI)
	area.GoTo(1, 11, lastUI)
	pressKey(gi.KeyEnter, gi.ModShift)
	lastWindow.Input(gi.TextInput{Text: "echo four"})
	frames(5)
	if win.dialog.Title != "Paste 2 Lines" {
		t.Fatalf("edited to two lines, the dialog is titled %q", win.dialog.Title)
	}
	pressKey(gi.KeyEnter, 0)
	if got := sentSoon(typed, "four"); got != "echo three\recho four" {
		t.Fatalf("the pane was sent %q", got)
	}
}

func TestAPasteCancelledSendsNothing(t *testing.T) {
	win, typed := pasteStage(t, true)
	lastUI.SetClipboard("rm -rf build\nmake\n")
	win.terms["p1"].pasteClipboard(lastUI)
	frames(5)
	pasteArea(t, win)
	pressKey(gi.KeyEscape, 0)
	time.Sleep(50 * time.Millisecond)
	if got := typed.Sent(); got != "" {
		t.Fatalf("cancelled, the pane was sent %q", got)
	}
}

func TestAPasteIsColouredAsItsShellReadsIt(t *testing.T) {
	for _, c := range []struct {
		kind    shellsetup.Route
		running bool
		want    string
	}{
		{shellsetup.Posix, false, "variable"},
		{shellsetup.PowerShell, false, "variable"},
		{shellsetup.Cmd, false, "plain"},
		{shellsetup.Posix, true, ""},
		{shellsetup.NoRoute, false, ""},
	} {
		h := pasteColours(c.kind, c.running)
		if h == nil {
			if c.want != "" {
				t.Errorf("%v: no colours", c.kind)
			}
			continue
		}
		if c.want == "" {
			t.Errorf("%v, a program running %v: coloured, want plain", c.kind, c.running)
			continue
		}
		got := "plain"
		for _, tk := range h("echo $HOME") {
			if tk.Start == 5 {
				got = tk.Kind.String()
			}
		}
		if got != c.want {
			t.Errorf("%v: $HOME is %s, want %s", c.kind, got, c.want)
		}
	}
}

func TestAPastesEditorTakesItsPanesShell(t *testing.T) {
	win, sh, publish := windowStage(t)
	typed := sessiontest.New()
	quiet := screen.Hooks{Output: func() {}, Title: func(string) {}, Exit: func() {}, Clipboard: func(string) {}}
	sh.Set("p1", screen.Open(typed, vt.DefaultPalette(), quiet))
	t.Cleanup(func() { _ = sh.Get("p1").T.Close() })
	publish(app.State{Panes: []app.Pane{{ID: "p1", Title: "Terminal 1", ShellKind: shellsetup.PowerShell}},
		Stage: &app.Box{Pane: "p1"}, Focus: "p1", PasteCheck: true})
	lastUI.SetClipboard("$x = 1\nWrite-Host $x")
	win.terms["p1"].pasteClipboard(lastUI)
	frames(5)
	if got := kindsOf(pasteArea(t, win)); got != "variable operator number function variable" {
		t.Fatalf("the paste is coloured %q, want PowerShell's colours", got)
	}
}

// kindsOf lists the kinds of the tokens an editor colours its code in.
func kindsOf(c *widget.CodeEditor) string {
	var out []string
	for _, tk := range c.Highlight(c.Text()) {
		out = append(out, tk.Kind.String())
	}
	return strings.Join(out, " ")
}

func TestPasteCheckOffPastesLinesStraightIn(t *testing.T) {
	win, typed := pasteStage(t, false)
	lastUI.SetClipboard("echo one\necho two")
	win.terms["p1"].pasteClipboard(lastUI)
	frames(5)
	if win.dialog != nil {
		t.Fatal("with the check off, a dialog opened")
	}
	if got := sentSoon(typed, "two"); got != "echo one\recho two" {
		t.Fatalf("the pane was sent %q", got)
	}
}

// Lines pasted into a shell they were not written for are marked, with
// what is wrong underlined and how the shell writes it, and counted
// above the text. An edit that puts a line right takes its mark away.
func TestAPasteForAnotherShellIsMarked(t *testing.T) {
	win, sh, publish := windowStage(t)
	typed := sessiontest.New()
	quiet := screen.Hooks{Output: func() {}, Title: func(string) {}, Exit: func() {}, Clipboard: func(string) {}}
	sh.Set("p1", screen.Open(typed, vt.DefaultPalette(), quiet))
	t.Cleanup(func() { _ = sh.Get("p1").T.Close() })
	publish(app.State{Panes: []app.Pane{{ID: "p1", Title: "Terminal 1", ShellKind: shellsetup.PowerShell}},
		Stage: &app.Box{Pane: "p1"}, Focus: "p1", PasteCheck: true})
	lastUI.SetClipboard("cd project\nexport GOOS=windows\ngo build ./... 2>/dev/null")
	win.terms["p1"].pasteClipboard(lastUI)
	frames(5)
	area := pasteArea(t, win)
	marks := area.Marks()
	if len(marks) != 2 || marks[0].Line != 2 || marks[0].Col != 1 || marks[0].End != 7 || marks[1].Line != 3 {
		t.Fatalf("the marks are %+v, want export on line 2 and /dev/null on line 3", marks)
	}
	if !strings.Contains(marks[0].Message, "$env:") {
		t.Fatalf("the mark says %q, not how PowerShell writes it", marks[0].Message)
	}
	var says *widget.Label
	for _, c := range formOf(win.dialog.Body).Children() {
		if l, ok := c.(*widget.Label); ok && strings.Contains(l.Text, "Shift+Enter") {
			says = l
		}
	}
	if says == nil {
		t.Fatal("the dialog says nothing above the text")
	}
	if !strings.HasPrefix(says.Text, "2 lines look written for another shell.") {
		t.Fatalf("the dialog says %q", says.Text)
	}
	area.SetText("cd project\n$env:GOOS = 'windows'\ngo build ./... 2>/dev/null", lastUI)
	lastWindow.Input(gi.TextInput{Text: " "})
	frames(5)
	if marks := area.Marks(); len(marks) != 1 || marks[0].Line != 3 {
		t.Fatalf("put right, the marks are %+v, want /dev/null's alone", marks)
	}
	if !strings.HasPrefix(says.Text, "1 line looks") {
		t.Fatalf("put right, the dialog says %q", says.Text)
	}
}
