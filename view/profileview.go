package view

import (
	"slices"
	"strings"

	"github.com/marrasen/gunim"
	"github.com/marrasen/gunim/icon"
	"github.com/marrasen/gunim/widget"

	"github.com/marrasen/kakel/app"
	"github.com/marrasen/kakel/machines"
	"github.com/marrasen/kakel/remote"
	"github.com/marrasen/kakel/settings"
	"github.com/marrasen/kakel/words"
)

// profileDialog asks for a profile: a new one when old is nil, or old
// to edit. have are the profiles there are, whose names its name must
// not repeat on the same machine.
func (w *Window) profileDialog(have []settings.Profile, old *settings.Profile, u *gunim.UI) {
	name, line, dir := widget.NewTextField(), widget.NewTextField(), widget.NewTextField()
	env := widget.NewTextArea()
	env.Rows = 3
	env.Placeholder = "NAME=value, one a line"
	// The machine it runs on: this computer, or a saved server. A
	// profile stays where it was made, as its program and folder are
	// that machine's.
	hosts, labels := []string{""}, []string{"This computer"}
	for _, h := range w.saved {
		if !h.Window {
			hosts, labels = append(hosts, h.ID), append(labels, h.Name)
		}
	}
	if old != nil && !slices.Contains(hosts, old.HostID) {
		hosts, labels = append(hosts, old.HostID), append(labels, w.nameOf(machines.ID(old.HostID)))
	}
	machine := widget.NewDropdown(widget.Labels(labels...))
	machine.Label = "Machine"
	hostID := func() string { return hosts[max(0, min(machine.Selected(), len(hosts)-1))] }
	placeholders := func() {
		line.Placeholder, dir.Placeholder = `such as C:\msys64\usr\bin\bash.exe --login -i`, "Optional"
		if hostID() != "" {
			line.Placeholder = "such as tmux new -A -s main"
		}
	}
	machine.OnChange = func(int, *gunim.UI) gunim.Intent { placeholders(); return nil }
	id, title := "", "Add Profile"
	if old != nil {
		id, title = old.ID, "Edit Profile"
		machine.SetSelected(slices.Index(hosts, old.HostID), nil)
		machine.Disabled = true
		name.SetText(old.Name, nil)
		line.SetText(app.ProfileLine(*old), nil)
		dir.SetText(old.Folder, nil)
		env.SetText(strings.Join(old.Env, "\n"), nil)
	}
	placeholders()
	have = slices.Clone(have)
	parse := func() (settings.Profile, error) {
		return app.ParseProfile(have, id, hostID(), name.Text(), line.Text(), dir.Text(), env.Text())
	}
	form := widget.NewForm()
	if len(hosts) > 1 {
		form.Add("Machine", machine)
	}
	d := widget.NewDialog(title)
	d.Body = form.
		Add("Name", name).
		Add("Command line", line).
		Add("Folder", dir).
		Add("Environment", env)
	d.SetButtons("Save", "Cancel")
	d.Check = func() string {
		if _, err := parse(); err != nil {
			return words.UpperFirst(err.Error())
		}
		return ""
	}
	d.OnAccept = func(u *gunim.UI) gunim.Intent {
		p, err := parse()
		if err != nil {
			return nil
		}
		return app.SaveProfile{Profile: p}
	}
	d.OnDismiss = widget.Sends(app.DialogClosed{})
	w.openDialog(d, u)
}

// profilesOn are the profiles on the saved server m, the one its new
// terminal runs first; none on this computer or a kakel window.
func (w *Window) profilesOn(m machines.ID) []settings.Profile {
	if m == "" || !slices.ContainsFunc(w.saved, func(h remote.Host) bool { return machines.ID(h.ID) == m && !h.Window }) {
		return nil
	}
	var out []settings.Profile
	for _, p := range w.profiles {
		if machines.ID(p.HostID) == m {
			out = append(out, p)
		}
	}
	if i := slices.IndexFunc(out, func(p settings.Profile) bool { return p.ID == w.serverShells[string(m)] }); i > 0 {
		def := out[i]
		out = slices.Insert(slices.Delete(out, i, i+1), 0, def)
	}
	return out
}

// profileItems are the palette's lines for the profiles, a server's
// named with it.
func (w *Window) profileItems(have []settings.Profile) []widget.PaletteItem {
	items := make([]widget.PaletteItem, len(have))
	for i, p := range have {
		items[i] = widget.PaletteItem{Title: p.Name, Detail: w.profileOn(p), Hint: app.ProfileLine(p), Icon: icon.SquareTerminal}
	}
	return items
}

// profileOn says which server p runs on, "" for this computer.
func (w *Window) profileOn(p settings.Profile) string {
	if p.HostID == "" {
		return ""
	}
	return "on " + w.nameOf(machines.ID(p.HostID))
}

// editProfilePickerFrom offers the profiles, to edit one.
func (w *Window) editProfilePickerFrom(have []settings.Profile, u *gunim.UI) {
	have = slices.Clone(have)
	w.chooseFrom("The profile to edit", w.profileItems(have), func(i int, u *gunim.UI) { w.profileDialog(have, &have[i], u) }, u)
}

// removeProfilePickerFrom offers the profiles, to remove one.
func (w *Window) removeProfilePickerFrom(have []settings.Profile, u *gunim.UI) {
	have = slices.Clone(have)
	w.chooseFrom("The profile to remove", w.profileItems(have), func(i int, u *gunim.UI) { u.Send(w, app.RemoveProfile{ID: have[i].ID}) }, u)
}

// profilesSays is the profiles, one a line with the server it runs on
// and its command line.
func (w *Window) profilesSays(have []settings.Profile) string {
	if len(have) == 0 {
		return "None yet."
	}
	lines := make([]string, len(have))
	for i, p := range have {
		lines[i] = p.Name + "  ·  "
		if on := w.profileOn(p); on != "" {
			lines[i] += on + "  ·  "
		}
		lines[i] += app.ProfileLine(p)
	}
	return strings.Join(lines, "\n")
}
