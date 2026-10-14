package view

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/marrasen/gunim/theme"
	"github.com/marrasen/gunim/themeedit"
	"github.com/marrasen/gunim/widget"

	"github.com/marrasen/kakel/app"
	"github.com/marrasen/kakel/look"
	"github.com/marrasen/kakel/themes"
)

// The theme editor opens in a tab, puts the cursor first, and keeps a
// cursor made to jump in the theme the window is drawn in, once Save is
// pressed: an edit alone keeps and changes nothing, and closing the
// pane drops it.
func TestTheThemeEditorKeepsACursorThatJumps(t *testing.T) {
	win, _, publish := windowStage(t)
	var looks []look.Themed
	for _, th := range themes.Built() {
		l, err := look.Of(th)
		if err != nil {
			t.Fatal(err)
		}
		looks = append(looks, l)
	}
	open := app.State{Panes: []app.Pane{{ID: "p1", Title: "Theme", Kind: app.KindThemeEditor}}, Stage: &app.Box{Pane: "p1"}, Focus: "p1",
		Theme: "Dark", Looks: looks}
	publish(open)
	for range 3 {
		lastWindow.Frame(time.Second / 60)
	}
	if win.themeEditor == nil {
		t.Fatal("the theme editor's pane was not made")
	}
	ed := win.themeEditor.ed
	if chosen := ed.Chosen(); len(chosen) == 0 || chosen[0] != widget.Caret.Key() {
		t.Fatalf("the editor puts %v first, want the cursor", chosen)
	}
	if chosen := ed.Chosen(); !slices.Contains(chosen, theme.Scale.Key()) {
		t.Fatalf("the editor has no Animations: %v", chosen)
	}
	drain := func() (saved []app.SaveThemeEdits) {
		for len(lastWindow.Client().Intents()) > 0 {
			if in, ok := (<-lastWindow.Client().Intents()).Intent.(app.SaveThemeEdits); ok {
				saved = append(saved, in)
			}
		}
		return saved
	}
	drain()
	stage := win.onStage.ThemeScope().Active()
	if err := ed.Set(widget.Caret.Key(), themeedit.Instant, true, lastUI); err != nil {
		t.Fatal(err)
	}
	lastWindow.Frame(time.Second / 60)
	if saved := drain(); len(saved) != 0 {
		t.Fatalf("an edit was saved before Save: %v", saved)
	}
	if got := win.onStage.ThemeScope().Active(); got.Has(widget.Caret.Key()) && !stage.Has(widget.Caret.Key()) {
		t.Fatal("an edit not saved reached the panes")
	}
	if lastUI.Theme().Active().Has(widget.Caret.Key()) {
		t.Fatal("an edit not saved reached the window")
	}
	// Closed unsaved, the edit is gone.
	publish(app.State{Panes: []app.Pane{{ID: "p2", Title: "Terminal"}}, Stage: &app.Box{Pane: "p2"}, Focus: "p2", Theme: "Dark", Looks: looks})
	if ed.Unsaved() || ed.Overrides().Has(widget.Caret.Key()) {
		t.Fatal("closed without saving, the editor kept its edit")
	}
	publish(open)
	if err := ed.Set(widget.Caret.Key(), themeedit.Instant, true, lastUI); err != nil {
		t.Fatal(err)
	}
	ed.Save(lastUI)
	for {
		if in, ok := nextIntent(t).(app.SaveThemeEdits); ok {
			if in.Theme != "Dark" || !strings.Contains(string(in.Edits), `"motion.caret"`) {
				t.Fatalf("the editor saved %s for %q", in.Edits, in.Theme)
			}
			return
		}
	}
}

// The cursor is a slider for how long it glides, from a jump up to a
// whole second.
func TestTheThemeEditorsCursorIsASlider(t *testing.T) {
	f := themeSections()[0].Fields[0]
	if f.Key != widget.Caret.Key() || f.Min != 0 || f.Max != 1 {
		t.Fatalf("the cursor is %+v", f)
	}
	if def := widget.Caret.Default().Response; def <= f.Min || def >= f.Max {
		t.Fatalf("the cursor's own glide, %v, is off the slider", def)
	}
}
