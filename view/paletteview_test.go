package view

import (
	"slices"
	"strings"
	"testing"

	"github.com/marrasen/kakel/app"
	"github.com/marrasen/kakel/machines"
	"github.com/marrasen/kakel/remote"

	"github.com/marrasen/gunim/match"
	"github.com/marrasen/gunim/widget"
)

// The palette finds commands by the words people use for them, and has
// the ones the menus alone had.
func TestThePaletteFindsCommandsByOtherWords(t *testing.T) {
	win, _, _ := windowStage(t)
	find := func(title string) widget.PaletteItem {
		t.Helper()
		i := slices.IndexFunc(win.palette.Items, func(it widget.PaletteItem) bool { return it.Title == title })
		if i < 0 {
			t.Fatalf("the palette has no %q", title)
		}
		return win.palette.Items[i]
	}
	if it := find("Exit"); !slices.Contains(it.Also, "quit") {
		t.Fatalf("Exit is found by %v", it.Also)
	}
	if it := find("Larger Font"); !slices.Contains(it.Also, "zoom in") {
		t.Fatalf("Larger Font is found by %v", it.Also)
	}
	find("Scroll Page Up")
	find("Open the Menus")
}

// The palette starts with what opens in this window, called for what it
// opens from the pane in front: the shell a new terminal runs, or the
// server it goes to, and where Files opens. A file manager in a window
// of its own comes later, and the menus call them the same.
func TestThePaletteStartsWithANewTerminalAndFiles(t *testing.T) {
	win, _, publish := windowStage(t)
	first := func() []string {
		var titles []string
		for _, it := range win.palette.Items[:2] {
			titles = append(titles, it.Title)
		}
		return titles
	}
	if want := []string{"New Terminal", "Files on This computer"}; !slices.Equal(first(), want) {
		t.Fatalf("with no shell known, the palette starts with %q, want %q", first(), want)
	}
	shells := []app.ShellChoice{{ID: "cmd", Title: "Command Prompt"}, {ID: "pwsh", Title: "PowerShell 7"}}
	publish(app.State{Shells: shells, DefaultShell: "pwsh", HereShell: "Command Prompt"})
	if want := []string{"New Command Prompt", "Files on This computer"}; !slices.Equal(first(), want) {
		t.Fatalf("in front of a Command Prompt, the palette starts with %q, want %q", first(), want)
	}
	plain := func(s string) string { return strings.ReplaceAll(s, "&", "") }
	if file := win.bar.Menus[0].Items; plain(file[0].Label) != "New Command Prompt" || plain(file[1].Label) != "Files on This computer" {
		t.Fatalf("the File menu starts with %q and %q", file[0].Label, file[1].Label)
	}
	publish(app.State{Shells: shells, DefaultShell: "pwsh", HereMachine: "sky",
		Machines: []machines.Info{{ID: "sky", Name: "skylake"}}, Saved: []remote.Host{{ID: "sky", Name: "skylake"}}})
	if want := []string{"New Terminal on skylake", "Files on skylake"}; !slices.Equal(first(), want) {
		t.Fatalf("in front of a pane on skylake, the palette starts with %q, want %q", first(), want)
	}
	n := 0
	for _, it := range win.palette.Items {
		switch it.Title {
		case "Files on skylake", "Terminal on skylake":
			n++
		case "New Terminal, Default Shell":
			t.Fatal("the palette still has New Terminal, Default Shell")
		}
	}
	if n != 1 {
		t.Fatalf("the palette has skylake's files and terminal %d times besides the new ones, want the files once", n)
	}
	if !slices.ContainsFunc(win.palette.Items, func(it widget.PaletteItem) bool { return it.Title == "Files in a New Window" }) {
		t.Fatal("the palette has no Files in a New Window")
	}
}

// Next Pane goes in the sidebar's order, the machine's panes together,
// not in the order the panes were opened.
func TestNextPaneGoesInTheSidebarsOrder(t *testing.T) {
	win, _, publish := windowStage(t)
	panes := []app.Pane{{ID: "p1", Title: "one", Kind: app.KindFileManager}, {ID: "p2", Title: "two", Kind: app.KindFileManager, Machine: "srv"}, {ID: "p3", Title: "three", Kind: app.KindFileManager}}
	publish(app.State{Panes: panes, Stage: &app.Box{Pane: "p1"}, Focus: "p1"})
	for len(lastWindow.Client().Intents()) > 0 {
		<-lastWindow.Client().Intents()
	}
	win.run("pane.nextInSidebar", lastUI)
	var got app.FocusPane
	for got.Pane == "" {
		if in, ok := nextIntent(t).(app.FocusPane); ok {
			got = in
		}
	}
	if got.Pane != "p3" {
		t.Fatalf("Next Pane from one went to %s, want three, the next on this computer", got.Pane)
	}
}

// The palette has each machine on a line of its own, found by its name,
// with what opens on it as chips, as the launcher has them: a terminal,
// its files, its log, and for a server saved, editing and removing it.
// This computer's terminals are its shells by name, the default first.
// No line of its own says "Terminal on" a machine, "Files on" one that
// is not in front, or connects to, edits or removes a server. Each shell
// here goes by its name too, the default said to be.
func TestThePaletteHasAMachineALineWithChips(t *testing.T) {
	win, _, publish := windowStage(t)
	publish(app.State{Saved: []remote.Host{{ID: "sky", Name: "skylake", Address: "10.0.0.5"}, {ID: "bk", Name: "backup"}},
		Machines: []machines.Info{{ID: "sky", Name: "skylake"}}, Connected: []machines.ID{"sky"}, Accounts: []machines.ID{"sky"},
		Shells:       []app.ShellChoice{{ID: "cmd", Title: "Command Prompt"}, {ID: "pwsh", Title: "PowerShell 7"}},
		DefaultShell: "pwsh"})
	items := make([]match.Item, len(win.palette.Items))
	for i, it := range win.palette.Items {
		items[i] = match.Item{Title: it.Title, Also: it.Also}
	}
	best := func(q string) int {
		t.Helper()
		found := match.Rank(items, q)
		if len(found) == 0 {
			t.Fatalf("%q finds nothing", q)
		}
		return found[0].Index
	}
	sky := best("skylake")
	if it := win.palette.Items[sky]; it.Title != "skylake" || !slices.Equal(it.Chips, []string{"Terminal", "Files", "Connection Log", "Edit", "Remove"}) || it.Detail != "connected" {
		t.Fatalf("skylake finds %q with chips %q", it.Title, it.Chips)
	}
	if it := win.palette.Items[best("10.0.0.5")]; it.Title != "skylake" {
		t.Fatalf("skylake's address finds %q", it.Title)
	}
	if it := win.palette.Items[best("backup")]; !slices.Equal(it.Chips, []string{"Terminal", "Files", "Connect", "Edit", "Remove"}) {
		t.Fatalf("backup, not connected to, has chips %q", it.Chips)
	}
	here := slices.IndexFunc(win.palette.Items, func(it widget.PaletteItem) bool { return it.Title == "This computer" })
	if it := win.palette.Items[max(here, 0)]; here < 0 || !slices.Equal(it.Chips, []string{"PowerShell 7", "Command Prompt", "Files", "Settings"}) {
		t.Fatalf("this computer has chips %q", it.Chips)
	}
	for _, it := range win.palette.Items {
		for _, gone := range []string{"Terminal on ", "Files on skylake", "Connect to skylake", "Edit Server ", "Remove Server ", "New Terminal on", "This Computer"} {
			if strings.HasPrefix(it.Title, gone) {
				t.Fatalf("the palette still offers %q", it.Title)
			}
		}
	}
	// A chip picked opens it, and is the one lit on the machine next.
	for len(lastWindow.Client().Intents()) > 0 {
		<-lastWindow.Client().Intents()
	}
	win.palette.OnChip(sky, 1, lastUI)
	var got app.OpenFilesOn
	for got.Machine == "" {
		if in, ok := nextIntent(t).(app.OpenFilesOn); ok {
			got = in
		}
	}
	if got.Machine != "sky" {
		t.Fatalf("skylake's Files opened files on %q", got.Machine)
	}
	publish(app.State{Saved: []remote.Host{{ID: "sky", Name: "skylake"}}, Machines: []machines.Info{{ID: "sky", Name: "skylake"}}})
	i := slices.IndexFunc(win.palette.Items, func(it widget.PaletteItem) bool { return it.Title == "skylake" })
	if i < 0 || win.palette.Items[i].Chip != 1 {
		t.Fatalf("skylake lights chip %d after Files was picked, want 1", win.palette.Items[max(i, 0)].Chip)
	}
}

// Each shell here goes by its name, the one a new terminal starts said
// to be the default.
func TestThePaletteNamesTheShellsHere(t *testing.T) {
	win, _, publish := windowStage(t)
	publish(app.State{Shells: []app.ShellChoice{{ID: "cmd", Title: "Command Prompt"}, {ID: "pwsh", Title: "PowerShell 7"}},
		DefaultShell: "pwsh"})
	i := slices.IndexFunc(win.palette.Items, func(it widget.PaletteItem) bool { return it.Title == "PowerShell 7" })
	if i < 0 || win.palette.Items[i].Detail != "This computer · default" {
		t.Fatalf("the default shell is offered as %+v", win.palette.Items[max(i, 0)])
	}
	if j := slices.IndexFunc(win.palette.Items, func(it widget.PaletteItem) bool { return it.Title == "Command Prompt" }); j < 0 ||
		win.palette.Items[j].Detail != "This computer" {
		t.Fatal("another shell is said to be the default, or is missing")
	}
}
