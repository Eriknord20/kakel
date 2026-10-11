package view

import (
	"fmt"
	"strings"

	"github.com/marrasen/gunim"
	"github.com/marrasen/gunim/icon"
	"github.com/marrasen/gunim/syntax"
	"github.com/marrasen/gunim/widget"
	"github.com/marrasen/kakel/app"
	"github.com/marrasen/kakel/shellsetup"
	"github.com/marrasen/kakel/words"
	"github.com/marrasen/kakel/wrongshell"
)

// pasteBytes is how large a paste on one line may be before it opens in
// the editor too: a page of text, as Windows Terminal counts it.
const pasteBytes = 5 << 10

// pasteRows is how many lines the editor shows before it scrolls, and
// pasteWidth how wide the dialog is.
const (
	pasteRows  = 16
	pasteWidth = 720
)

// pasteLines is how many lines s holds. A line break at the end starts
// no line of its own: a line copied whole carries one.
func pasteLines(s string) int {
	s = strings.TrimSuffix(s, "\n")
	s = strings.TrimSuffix(s, "\r")
	return strings.Count(s, "\n") + strings.Count(s, "\r") - strings.Count(s, "\r\n") + 1
}

// needsCheck reports whether a paste opens in the editor before it
// reaches the pane: more than one line, or more than pasteBytes.
func needsCheck(s string) bool {
	return len(s) > pasteBytes || pasteLines(s) > 1
}

// checkPaste opens text pasted into t in an editor, to change or cancel
// before it reaches the pane, when it is more than one line or large.
// It reports whether it did; otherwise the paste goes straight in. The
// editor colours the text as the pane's shell reads it.
func (w *Window) checkPaste(t *term, s string, u *gunim.UI) bool {
	if !w.pasteCheck || !needsCheck(s) {
		return false
	}
	// One kind of line break, as the editor shows them: the paste turns
	// each into Enter on its way all the same.
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	kind, running := w.shellKind(t.id), t.sh.T.RunningAProgram()
	text := widget.NewCodeEditor()
	text.Label = "Text to paste"
	text.Highlight = pasteColours(kind, running)
	text.MaxRows = pasteRows
	text.SetText(s, nil)
	d := widget.NewDialog(pasteTitle(s))
	d.Icon = icon.ClipboardPaste
	d.Width = pasteWidth
	along, bracketed := len(t.alongOthers()), t.sh.T.Bracketed()
	// Lines written for another shell are marked, as an editor marks
	// mistakes, and counted above the text.
	marks := pasteSlips(s, kind, running)
	text.SetMarks(marks, u)
	says := widget.NewLabel(pasteSays(len(marks), along, bracketed))
	d.Body = widget.NewForm().Add("", says).Add("", text)
	d.SetButtons("Paste", "Cancel")
	d.OnAccept = func(*gunim.UI) gunim.Intent {
		t.paste(text.Text())
		return app.DialogClosed{}
	}
	// Enter pastes from the editor too, and Shift+Enter starts a line.
	text.OnCommit = func(s string, u *gunim.UI) gunim.Intent {
		d.Close(u)
		t.paste(s)
		return app.DialogClosed{}
	}
	// The title counts the lines as they are edited, and the marks
	// follow the edits.
	text.OnChange = func(s string, u *gunim.UI) gunim.Intent {
		d.Title = pasteTitle(s)
		marks := pasteSlips(s, kind, running)
		text.SetMarks(marks, u)
		says.Text = pasteSays(len(marks), along, bracketed)
		u.Invalidate()
		return nil
	}
	d.OnDismiss = widget.Sends(app.DialogClosed{})
	w.openDialog(d, u)
	return true
}

// shellKind is the kind of shell pane id runs.
func (w *Window) shellKind(id string) shellsetup.Route {
	for _, p := range w.panes {
		if p.ID == id {
			return p.ShellKind
		}
	}
	return shellsetup.NoRoute
}

// pasteColours colours a paste as the shell it goes to reads it: bash's
// way, PowerShell's or the Command Prompt's. A paste goes in plain when
// a program the shell started is reading instead, or the shell is not
// known.
func pasteColours(kind shellsetup.Route, programRunning bool) syntax.Highlighter {
	if programRunning {
		return nil
	}
	switch kind {
	case shellsetup.Posix:
		return syntax.Bash
	case shellsetup.PowerShell:
		return syntax.PowerShell
	case shellsetup.Cmd:
		return syntax.Batch
	}
	return nil
}

// pasteSlips marks the lines of a paste written for another shell than
// kind, the shell it goes to, with how that shell writes them. A paste
// to a program the shell started, or to a shell not known, is not
// looked at.
func pasteSlips(s string, kind shellsetup.Route, programRunning bool) []widget.CodeMark {
	if programRunning {
		return nil
	}
	var marks []widget.CodeMark
	for _, slip := range wrongshell.Find(s, kind) {
		marks = append(marks, widget.CodeMark{Line: slip.Line, Col: slip.Col, End: slip.End, Message: slip.Message})
	}
	return marks
}

// pasteTitle is the paste dialog's title: how many lines, or for one
// line, how large.
func pasteTitle(s string) string {
	if n := pasteLines(s); n > 1 {
		return fmt.Sprintf("Paste %d Lines", n)
	}
	return "Paste " + words.Size(int64(len(s)))
}

// pasteSays is what the paste dialog says above the text: how many
// lines look written for another shell, the other panes it reaches,
// that each line break is Enter where the program takes the paste as
// typed, and how to start a line.
func pasteSays(slips, along int, bracketed bool) string {
	var says []string
	switch {
	case slips == 1:
		says = append(says, "1 line looks written for another shell.")
	case slips > 1:
		says = append(says, fmt.Sprintf("%d lines look written for another shell.", slips))
	}
	if along > 0 {
		says = append(says, "It also goes to "+words.Count(along, "other pane")+" typing along.")
	}
	if !bracketed {
		says = append(says, "Each line break acts as Enter in this pane.")
	}
	says = append(says, "Shift+Enter starts a new line.")
	return strings.Join(says, " ")
}
