package view

import (
	"slices"

	"github.com/marrasen/gunim"
	"github.com/marrasen/gunim/filemanager"
	"github.com/marrasen/gunim/widget"

	"github.com/marrasen/kakel/app"
)

// One menu bar for the window, which takes the lines of the pane in
// front: a file manager's File, Edit and View lines join kakel's menus
// of those names, and its Go menu comes after View, while it is in
// front. Its own menus hide, as kakel shows them.

// barMenu is a menu of the bar: its title and its lines.
type barMenu struct {
	title string
	items []menuItem
}

// baseLayout is kakel's own menus, before any pane's lines join them.
func baseLayout() []barMenu {
	out := make([]barMenu, len(menus))
	for i, m := range menus {
		out[i] = barMenu{title: m.title, items: slices.Clone(m.items)}
	}
	return out
}

// buildBar makes the bar's menus from the layout, and with refill fills
// those made from the window's state again: the servers. A window being
// made fills them as its state comes.
func (w *Window) buildBar(refill bool) {
	w.bar.Menus = w.bar.Menus[:0]
	for _, m := range w.layout {
		bm := widget.BarMenu{Title: m.title}
		var first []int
		for i, it := range m.items {
			if it.keyFirst {
				first = append(first, i)
			}
			hint := it.hint
			if chord, ok := w.keys.ChordFor(it.id); ok && !it.caption && it.pane == "" {
				hint = chordLabel(chord)
			}
			bm.Items = append(bm.Items, widget.MenuItem{Label: w.commandTitle(it.id, it.title), Hint: hint, Icon: commandIcons[it.id],
				Checked: it.on, Caption: it.caption, Break: it.group || (it.caption && i > 0)})
		}
		w.bar.Menus = append(w.bar.Menus, withAccessKeys(bm, first...))
	}
	if refill {
		w.servers(w.saved)
	}
}

// terminalOnly are kakel's lines for a terminal alone, which a file
// manager in front leaves out of the menus, with the Scrollback caption.
var terminalOnly = map[string]bool{
	"edit.pasteImage": true, "pane.scrollback": true, "view.scrollUp": true, "view.scrollDown": true,
}

// paneLayout is kakel's menus with the lines of the file manager in
// front, fm's menus, joined.
func paneLayout(fm []filemanager.Menu) []barMenu {
	out := baseLayout()
	for i := range out {
		out[i].items = slices.DeleteFunc(out[i].items, func(it menuItem) bool {
			return terminalOnly[it.id] || it.caption && it.title == "Scrollback"
		})
	}
	for _, m := range fm {
		var lines []menuItem
		for i, it := range m.Items {
			lines = append(lines, menuItem{id: "fm." + it.Cmd, title: it.Label, pane: it.Cmd, hint: it.Hint, on: it.Checked,
				group: it.Line || i == 0})
		}
		if len(lines) == 0 {
			continue
		}
		at := slices.IndexFunc(out, func(b barMenu) bool { return b.title == m.Title })
		if at < 0 {
			// A menu of its own, as Go, after View.
			after := slices.IndexFunc(out, func(b barMenu) bool { return b.title == "View" }) + 1
			lines[0].group = false
			out = slices.Insert(out, after, barMenu{title: m.Title, items: lines})
			continue
		}
		items := out[at].items
		where := len(items)
		if m.Title == "File" {
			// Before Settings, Close and Exit, which end a File menu.
			if i := slices.IndexFunc(items, func(it menuItem) bool { return it.id == "app.settings" }); i >= 0 {
				where = i
			}
		}
		out[at].items = slices.Insert(items, where, lines...)
	}
	return out
}

// layoutMenus takes the lines of the pane in front into the menus,
// building the bar again where their lines changed, not their ticks.
func (w *Window) layoutMenus(u *gunim.UI) {
	next := baseLayout()
	if _, ok := w.fmHosts[w.focused]; ok {
		if fm := filemanager.Menus(u, app.FilePaneViews(w.focused)); fm != nil {
			next = paneLayout(fm)
		}
	}
	if !sameLines(next, w.layout) {
		if w.bar.IsOpen() {
			// Not under the user's pointer: once the menu closes.
			return
		}
		w.layout = next
		w.buildBar(true)
		return
	}
	w.layout = next
}

// sameLines reports whether a and b have the same menus and lines, ticked
// alike or not.
func sameLines(a, b []barMenu) bool {
	return slices.EqualFunc(a, b, func(x, y barMenu) bool {
		return x.title == y.title && slices.EqualFunc(x.items, y.items, func(p, q menuItem) bool {
			p.on, q.on = false, false
			return p == q
		})
	})
}

// tickPaneMenus ticks the pane's lines as the pane is now, as a menu
// opens: what it shows changes with no word to the window.
func (w *Window) tickPaneMenus(u *gunim.UI) {
	if _, ok := w.fmHosts[w.focused]; !ok {
		return
	}
	next := paneLayout(filemanager.Menus(u, app.FilePaneViews(w.focused)))
	if !sameLines(next, w.layout) {
		return
	}
	for m := range next {
		for i, it := range next[m].items {
			if it.pane != "" && m < len(w.bar.Menus) && i < len(w.bar.Menus[m].Items) {
				w.bar.Menus[m].Items[i].Checked = it.on
			}
		}
	}
	w.layout = next
	u.Invalidate()
}

// runPane runs cmd, a line of the menus of the file manager in front.
func (w *Window) runPane(cmd string, u *gunim.UI) {
	filemanager.Run(u, app.FilePaneViews(w.focused), cmd)
}
