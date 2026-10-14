package view

import (
	"github.com/marrasen/gunim"
	"github.com/marrasen/gunim/geom"
	gi "github.com/marrasen/gunim/input"
	"github.com/marrasen/gunim/paint"
	"github.com/marrasen/gunim/theme"
	"github.com/marrasen/gunim/themeedit"
	"github.com/marrasen/gunim/widget"

	"github.com/marrasen/kakel/app"
	"github.com/marrasen/kakel/look"
)

// themePane is gunim's theme editor in a tab of its own, on the theme
// the window is drawn in. The changes are a draft, which the editor's
// preview alone wears: trying values out changes nothing else. Save
// keeps them, in the themes file under the theme's name, and every
// window takes them. Closing the pane without saving drops them.
type themePane struct {
	w  *Window
	ed *themeedit.Editor
	// name is the theme being edited.
	name string
}

// themeSections are the values the editor puts first, in kakel's words.
func themeSections() []themeedit.Section {
	return []themeedit.Section{
		{Title: "Terminal", Fields: []themeedit.Field{{
			Key: widget.Caret.Key(), Label: "Cursor", Detail: "How long a terminal's cursor takes to glide along a line.",
			Min: 0, Max: 1,
			Presets: []themeedit.Preset{{Label: "Jumps", Value: themeedit.Instant}, {Label: "Glides", Value: widget.Caret.Default()}},
		}}},
		{Title: "Motion", Fields: []themeedit.Field{
			{Key: widget.Quick.Key(), Label: "Quick moves", Detail: "Buttons, switches and highlights as they answer the pointer."},
			{Key: widget.Settle.Key(), Label: "Settling", Detail: "Panels, tabs and panes as they open, close and move."},
			{Key: widget.Bounce.Key(), Label: "Bounces", Detail: "What springs into place, such as a toast."},
			{Key: theme.Switch.Key(), Label: "Theme change", Detail: "Every colour as the window takes another theme."},
		}},
		{Title: "Colours", Fields: []themeedit.Field{
			{Key: widget.Accent.Key(), Label: "Accent", Detail: "Focus rings, links and what is chosen."},
			{Key: widget.Selection.Key(), Label: "Selection", Detail: "Behind text selected."},
			{Key: widget.Background.Key(), Label: "Ground", Detail: "Behind the window's own parts."},
			{Key: widget.Ink.Key(), Label: "Text", Detail: "The window's own words."},
		}},
		{Title: "Shape", Fields: []themeedit.Field{
			{Key: widget.ButtonRadius.Key(), Label: "Button corners", Detail: "How round a button is.", Min: 0, Max: 18},
			{Key: widget.DialogRadius.Key(), Label: "Dialog corners", Detail: "How round a dialog is.", Min: 0, Max: 24},
			{Key: widget.TextSize.Key(), Label: "Text size", Detail: "The window's own words; terminals keep their font size.", Min: 10, Max: 22},
		}},
	}
}

// newThemePane makes the editor on the theme the window is drawn in.
func newThemePane(w *Window) *themePane {
	p := &themePane{w: w}
	t := w.looks[w.themeNow]
	p.name = t.Name
	p.ed = themeedit.New(themeedit.Options{
		Base:      t.Plain,
		Overrides: t.Edits,
		Sections:  themeSections(),
		OnSave: func(over theme.Theme, u *gunim.UI) gunim.Intent {
			edits, err := theme.MarshalValues(over)
			if err != nil {
				w.failed("Couldn't save the changes to "+p.name, err.Error(), u)
				return nil
			}
			return app.SaveThemeEdits{Theme: p.name, Edits: edits}
		},
	})
	return p
}

// follow takes the theme the window is drawn in now into the editor,
// when the user has picked another, or when the themes were read again.
func (p *themePane) follow(t look.Themed, u *gunim.UI) {
	if t.Name == p.name {
		return
	}
	p.name = t.Name
	p.ed.SetBase(t.Plain, u)
	p.ed.SetOverrides(t.Edits, u)
}

// drop forgets changes not saved, as the pane closes: opened again, it
// starts from the theme as saved.
func (p *themePane) drop(u *gunim.UI) { p.ed.Discard(u) }

// Children implements [gunim.Composite].
func (p *themePane) Children() []gunim.Node { return []gunim.Node{p.ed} }

// Layout implements [gunim.Node].
func (p *themePane) Layout(c gunim.Constraints, _ gunim.Frame, kids gunim.Children) geom.Size {
	k := kids.At(0)
	k.Layout(gunim.Tight(c.Max))
	k.Place(geom.Point{})
	return c.Max
}

// Paint implements [gunim.Node].
func (p *themePane) Paint(pt *paint.Painter, f gunim.Frame, box geom.Size, kids gunim.Children) {
	pt.RRect(geom.Rect{Max: box.Point()}, 0, paint.Solid(widget.Background.Get(f.Theme)))
	kids.At(0).Paint(pt)
}

// Handle implements [gunim.Handler]: the pane says it has the keyboard,
// as the window's panes do.
func (p *themePane) Handle(e gi.Event, u *gunim.UI) bool {
	if _, ok := e.(gi.FocusEntered); ok {
		p.w.entered(p.w.paneOfKind(app.KindThemeEditor), u)
	}
	return false
}
