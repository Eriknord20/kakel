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

// The palette words what it opens as the launcher does, and finds it by
// the same words: "files on skylake" finds the files of a server saved
// and not connected to, and each shell here goes by its name, the one a
// new terminal starts said to be the default.
func TestThePaletteWordsWhatItOpensAsTheLauncherDoes(t *testing.T) {
	win, _, publish := windowStage(t)
	publish(app.State{Saved: []remote.Host{{ID: "sky", Name: "skylake", Address: "10.0.0.5"}},
		Machines:     []machines.Info{{ID: "sky", Name: "skylake"}},
		Shells:       []app.ShellChoice{{ID: "cmd", Title: "Command Prompt"}, {ID: "pwsh", Title: "PowerShell 7"}},
		DefaultShell: "pwsh"})
	items := make([]match.Item, len(win.palette.Items))
	for i, it := range win.palette.Items {
		items[i] = match.Item{Title: it.Title, Also: it.Also}
	}
	best := func(q string) widget.PaletteItem {
		t.Helper()
		found := match.Rank(items, q)
		if len(found) == 0 {
			t.Fatalf("%q finds nothing", q)
		}
		return win.palette.Items[found[0].Index]
	}
	if it := best("files on skylake"); it.Title != "Files on skylake" {
		t.Fatalf("files on skylake finds %q first", it.Title)
	}
	if it := best("terminal on skylake"); it.Title != "Terminal on skylake" {
		t.Fatalf("terminal on skylake finds %q first", it.Title)
	}
	if it := best("new file manager"); it.Title != "New File Manager" && !strings.HasPrefix(it.Title, "Files on") {
		t.Fatalf("new file manager finds %q first", it.Title)
	}
	for _, it := range win.palette.Items {
		if strings.HasPrefix(it.Title, "Terminal on This computer") || strings.HasPrefix(it.Title, "New Terminal on") {
			t.Fatalf("the palette still offers %q", it.Title)
		}
	}
	i := slices.IndexFunc(win.palette.Items, func(it widget.PaletteItem) bool { return it.Title == "PowerShell 7" })
	if i < 0 || win.palette.Items[i].Detail != "This computer · default" {
		t.Fatalf("the default shell is offered as %+v", win.palette.Items[max(i, 0)])
	}
	if j := slices.IndexFunc(win.palette.Items, func(it widget.PaletteItem) bool { return it.Title == "Command Prompt" }); j < 0 ||
		win.palette.Items[j].Detail != "This computer" {
		t.Fatal("another shell is said to be the default, or is missing")
	}
}
