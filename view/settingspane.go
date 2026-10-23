package view

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/marrasen/gunim"
	"github.com/marrasen/gunim/geom"
	"github.com/marrasen/gunim/icon"
	gi "github.com/marrasen/gunim/input"
	"github.com/marrasen/gunim/paint"
	"github.com/marrasen/gunim/theme"
	"github.com/marrasen/gunim/widget"

	"github.com/marrasen/kakel/app"
	"github.com/marrasen/kakel/conf"
	"github.com/marrasen/kakel/keys"
	"github.com/marrasen/kakel/look"
	"github.com/marrasen/kakel/relay"
	"github.com/marrasen/kakel/remote"
	"github.com/marrasen/kakel/serve"
	"github.com/marrasen/kakel/settings"
	"github.com/marrasen/kakel/themes"
	"github.com/marrasen/kakel/words"
)

// The Settings pane: everything kakel keeps about how it behaves, in
// tabs, each a column of sections. A switch or a choice takes effect
// as it is made; a line typed is kept with the Save beside it, which
// lights once it differs from what is kept, and says what is wrong with
// it there. The pane shows what the program publishes, and leaves alone
// a line being typed.

// updateChoices are the update setting's choices, as Settings lists
// them and writes them.
var updateChoices = []struct{ title, value string }{
	{"Tell me", settings.UpdatesNotify}, {"Install them", settings.UpdatesInstall}, {"Off", settings.UpdatesOff},
}

// launcherFilesChoices are where files from the launcher or the tray
// open, as Settings lists them and writes them.
var launcherFilesChoices = []struct{ title, value string }{
	{"New tab", settings.LauncherFilesTab}, {"Beside", settings.LauncherFilesBeside}, {"New window", settings.LauncherFilesWindow},
}

// settingsTabs are the pane's tabs, in order, and their icons.
var settingsTabs = []struct {
	title string
	icon  *icon.Icon
}{
	{"General", icon.Settings},
	{"Appearance", icon.Palette},
	{"Terminal", icon.SquareTerminal},
	{"Notifications", icon.Bell},
	{"Sharing", icon.Share2},
	{"Files", icon.Folder},
}

// settingsWidth is the widest the pane's sections grow, so a wide window
// keeps its lines short enough to read.
const settingsWidth = 720

// The room in the pane: around a tab's sections, between sections,
// between a section's rows, under a heading, between a setting's title
// and what it does, between a setting's words and its control, and
// between buttons.
var (
	settingsPad      = theme.Insets("kakel.settings.pad", geom.Insets{Left: 28, Right: 28, Top: 20, Bottom: 28})
	settingsSections = theme.Length("kakel.settings.sections", 22)
	settingsRows     = theme.Length("kakel.settings.rows", 16)
	settingsHeading  = theme.Length("kakel.settings.heading", 8)
	settingsDetail   = theme.Length("kakel.settings.detail", 3)
	settingsControl  = theme.Length("kakel.settings.control", 24)
	settingsButtons  = theme.Length("kakel.settings.buttons", 8)
)

// settingsPane is the Settings pane.
type settingsPane struct {
	w    *Window
	tabs *widget.Tabs

	// General.
	autostart, tray, folders, titleBar, beta *widget.Switch
	foldersRow, installRow                   *maybeShown
	launcher                                 *savedLine
	launcherFiles                            *widget.Segmented
	updates                                  *widget.Dropdown

	// Appearance.
	theme, font *widget.Dropdown
	themeNames  []string
	fontNames   []string
	size        *widget.NumberField

	// Terminal.
	startIn, termProgram *savedLine
	// proxy is the HTTP proxy relays are reached through.
	proxy      *savedLine
	shell      *widget.Dropdown
	shellIDs   []string
	known      *widget.Dropdown
	pasteCheck *widget.Switch
	// shellTitles are shell's items beside shellIDs: a renamed profile
	// changes them while its ID stays.
	shellTitles []string
	// profiles lists the profiles, which editP and removeP need one of.
	profiles       *widget.Label
	editP, removeP *widget.Button

	// Notifications.
	iface         *widget.Checkbox
	sounds, rings []*widget.Checkbox

	// Sharing.
	atStart *widget.Segmented
	// tunnels lets plain ssh's tunnels connect machines here.
	tunnels *widget.Switch
	port    *widget.NumberField
	reach   *widget.Dropdown
	keys    *widget.Label
	serving *widget.Button
	removeK *widget.Button

	// st is the state last shown, which the controls send from.
	st app.State
}

func newSettingsPane(w *Window) *settingsPane {
	p := &settingsPane{w: w}
	pages := []gunim.Node{p.general(), p.appearance(), p.terminal(), p.notifications(), p.sharing(), p.files()}
	titles := make([]string, len(settingsTabs))
	icons := make([]*icon.Icon, len(settingsTabs))
	for i, t := range settingsTabs {
		titles[i], icons[i] = t.title, t.icon
	}
	p.tabs = widget.NewTabs(titles, pages...)
	p.tabs.Icons = icons
	return p
}

// settingsPage is one tab's sections, scrolling, no wider than
// settingsWidth.
func settingsPage(sections ...gunim.Node) gunim.Node {
	col := widget.Column(sections...)
	col.Cross, col.Gap = widget.CrossStretch, settingsSections
	pad := widget.NewPad(widget.NewSized(col, settingsWidth, 0))
	pad.Padding = settingsPad
	row := widget.Row(pad)
	row.Justify = widget.JustifyCenter
	return widget.NewScroll(row)
}

// settingsSection is a heading and its rows, on a card.
func settingsSection(title string, rows ...gunim.Node) gunim.Node {
	h := widget.NewLabel(title)
	h.Face = widget.BoldFont
	col := widget.Column(rows...)
	col.Cross, col.Gap = widget.CrossStretch, settingsRows
	all := widget.Column(h, widget.NewCard(col))
	all.Cross, all.Gap = widget.CrossStretch, settingsHeading
	return all
}

// settingRow is a setting's line: its title, with what it does under it
// in the faint ink, and its control at the right.
func settingRow(title, detail string, control gunim.Node) gunim.Node {
	t := widget.NewLabel(title)
	left := []gunim.Node{t}
	if detail != "" {
		left = append(left, faintLabel(detail))
	}
	text := widget.Column(left...)
	text.Cross, text.Gap = widget.CrossStretch, settingsDetail
	if control == nil {
		return text
	}
	r := widget.Row(text, control).Grow(text, 1)
	r.Cross, r.Gap = widget.CrossCenter, settingsControl
	return r
}

// faintLabel is a line in the faint ink, as what a setting does is said.
func faintLabel(s string) *widget.Label {
	l := widget.NewLabel(s)
	l.Color = look.Faint
	return l
}

// buttons is a row of buttons, from the left.
func buttons(bs ...gunim.Node) gunim.Node {
	r := widget.Row(bs...)
	r.Gap = settingsButtons
	return r
}

// settingButton is a button that runs the window's command id.
func (p *settingsPane) settingButton(label, id string) *widget.Button {
	b := widget.NewButton(label)
	b.OnClick = func(u *gunim.UI) gunim.Intent { p.w.run(id, u); return nil }
	return b
}

// send sends in from the pane.
func (p *settingsPane) send(in gunim.Intent, u *gunim.UI) { u.Send(p.tabs, in) }

// savedLine is a line typed and kept with the Save beside it: the field,
// the button, a line that says what is wrong with it, and what is kept.
type savedLine struct {
	field *widget.TextField
	save  *widget.Button
	note  *widget.Label
	kept  string
	// check says what is wrong with a line, or "".
	check func(s string) string
	node  gunim.Node
}

// newSavedLine makes a line kept by keep, the intent its text makes.
func (p *settingsPane) newSavedLine(placeholder string, width float32, check func(string) string, keep func(string) gunim.Intent) *savedLine {
	l := &savedLine{field: widget.NewTextField(), save: widget.NewButton("Save"), note: widget.NewLabel(""), check: check}
	l.field.Placeholder = placeholder
	l.note.Color = widget.ToastErrorInk
	l.save.Disabled = true
	commit := func(u *gunim.UI) {
		s := strings.TrimSpace(l.field.Text())
		if l.check != nil {
			if why := l.check(s); why != "" {
				l.note.Text = why
				u.Invalidate()
				return
			}
		}
		l.note.Text = ""
		l.kept = s
		l.save.Disabled = true
		p.send(keep(s), u)
		u.Invalidate()
	}
	l.save.OnClick = func(u *gunim.UI) gunim.Intent { commit(u); return nil }
	l.field.OnChange = func(s string, u *gunim.UI) gunim.Intent {
		l.save.Disabled = strings.TrimSpace(s) == l.kept
		l.note.Text = ""
		u.Invalidate()
		return nil
	}
	l.field.Keys = func(k gi.KeyPress, u *gunim.UI) bool {
		if k.Key != gi.KeyEnter || k.Mods != 0 {
			return false
		}
		commit(u)
		return true
	}
	row := widget.Row(widget.NewSized(l.field, width, 0), l.save)
	row.Cross, row.Gap = widget.CrossCenter, settingsButtons
	col := widget.Column(row, l.note)
	col.Cross, col.Gap = widget.CrossEnd, settingsDetail
	l.node = col
	return l
}

// show shows s as what is kept. A line the user has changed and not yet
// saved keeps what they typed.
func (l *savedLine) show(s string, _ *gunim.UI) {
	was, text := l.kept, strings.TrimSpace(l.field.Text())
	l.kept = s
	if text == was || text == s {
		if l.field.Text() != s {
			l.field.SetText(s, nil)
		}
		l.save.Disabled = true
		return
	}
	l.save.Disabled = false
}

func (p *settingsPane) general() gunim.Node {
	p.autostart = widget.NewSwitch("")
	p.autostart.OnChange = func(_ bool, u *gunim.UI) gunim.Intent { p.send(app.ToggleAutostart{}, u); return nil }
	p.tray = widget.NewSwitch("")
	p.tray.OnChange = func(_ bool, u *gunim.UI) gunim.Intent { p.send(app.ToggleTray{}, u); return nil }
	p.folders = widget.NewSwitch("")
	p.folders.OnChange = func(_ bool, u *gunim.UI) gunim.Intent { p.send(app.ToggleFolders{}, u); return nil }
	p.titleBar = widget.NewSwitch("")
	p.titleBar.OnChange = func(on bool, u *gunim.UI) gunim.Intent {
		in := app.SaveLook{Sounds: p.st.Sounds, Rings: p.st.Rings, SystemTitleBar: on}
		p.send(in, u)
		return nil
	}
	p.launcher = p.newSavedLine(app.DefaultLauncherKey, 180, func(s string) string {
		if s == "" || strings.EqualFold(s, "none") {
			return ""
		}
		if _, err := app.LauncherHotKey(s); err != nil {
			return words.UpperFirst(err.Error()) + "."
		}
		return ""
	}, func(s string) gunim.Intent { return app.SetLauncherKey{Key: s} })
	where := make([]string, len(launcherFilesChoices))
	for i, c := range launcherFilesChoices {
		where[i] = c.title
	}
	p.launcherFiles = widget.NewSegmented(where...)
	p.launcherFiles.OnChange = func(i int, u *gunim.UI) gunim.Intent {
		return app.SetLauncherFiles{Where: launcherFilesChoices[max(0, min(i, len(launcherFilesChoices)-1))].value}
	}
	titles := make([]string, len(updateChoices))
	for i, c := range updateChoices {
		titles[i] = c.title
	}
	p.updates = widget.NewDropdown(widget.Labels(titles...))
	p.updates.Label = "New releases"
	p.updates.OnChange = func(i int, u *gunim.UI) gunim.Intent {
		p.send(app.SetUpdates{What: updateChoices[max(0, min(i, len(updateChoices)-1))].value}, u)
		return nil
	}
	p.beta = widget.NewSwitch("")
	p.beta.OnChange = func(on bool, u *gunim.UI) gunim.Intent { p.send(app.SetBeta{On: on}, u); return nil }
	about := widget.NewButton("About kakel…")
	about.OnClick = func(u *gunim.UI) gunim.Intent { p.send(app.ShowAbout{}, u); return nil }
	install := p.settingButton("Install kakel…", "app.install")
	p.installRow = &maybeShown{child: settingRow("Install kakel", "For you alone, with a Start menu entry. Starting with the computer, updates and opening folders are for the installed kakel.", install)}
	p.foldersRow = &maybeShown{child: settingRow("Default file manager", "Folders and drives opened anywhere, and Win+E, open in kakel's file manager in place of File Explorer.", p.folders)}
	rows := []gunim.Node{
		p.installRow,
		settingRow("Start with the computer", "Into the tray, with no window.", p.autostart),
		settingRow("Show kakel in the tray", "Closing the last window leaves kakel there, with its launcher key.", p.tray),
	}
	rows = append(rows, p.foldersRow)
	return settingsPage(
		settingsSection("Startup", rows...),
		settingsSection("Launcher",
			settingRow("Launcher key", "Opens the launcher from any program. Super is the Windows key; none takes no key.", p.launcher.node),
			settingRow("Files open in", "Files from the launcher or the tray. Beside splits the last pane you used, if it is a file manager.", p.launcherFiles)),
		settingsSection("Windows",
			settingRow("Use the system's title bar", "The title bar your window manager draws, with its own buttons, for the windows opened from now on.", p.titleBar)),
		settingsSection("Updates",
			settingRow("New releases", "Installed, an update starts the next time kakel does.", p.updates),
			settingRow("Beta releases", "Take betas too: what comes next, sooner, and less tried.", p.beta),
			settingRow("About kakel", "The version, what each release changed, and a check for updates.", about)),
	)
}

func (p *settingsPane) appearance() gunim.Node {
	p.theme = widget.NewDropdown(widget.Labels(""))
	p.theme.Label = "Theme"
	p.theme.OnChange = func(i int, u *gunim.UI) gunim.Intent {
		if i < len(p.themeNames) {
			p.send(app.PickTheme{Name: p.themeNames[i]}, u)
		}
		return nil
	}
	p.font = widget.NewDropdown(widget.Labels(""))
	p.font.Label = "Font"
	p.font.OnChange = func(i int, u *gunim.UI) gunim.Intent {
		if i < len(p.fontNames) {
			p.send(app.PickFont{Name: p.fontNames[i]}, u)
		}
		return nil
	}
	p.size = widget.NewNumberField(float64(app.MinFontSize), float64(app.MaxFontSize))
	p.size.Suffix = " px"
	p.size.OnChange = func(v float64, u *gunim.UI) gunim.Intent { return app.SetFontSize{Size: float32(v)} }
	reset := widget.NewButton("Reset")
	reset.OnClick = func(u *gunim.UI) gunim.Intent { p.send(app.FontSize{}, u); return nil }
	size := widget.Row(widget.NewSized(p.size, 110, 0), reset)
	size.Cross, size.Gap = widget.CrossCenter, settingsButtons
	return settingsPage(
		settingsSection("Theme",
			settingRow("Theme", "The colours of the windows and of the terminals in them.", p.theme),
			settingRow("Edit theme", "How this theme moves and looks, the cursor too, tried in a preview before you save.", p.settingButton("Edit Theme…", "view.editTheme"))),
		settingsSection("Terminal text",
			settingRow("Font", "The monospaced families installed, and those kakel carries.", p.font),
			settingRow("Size", "Ctrl and the wheel, or Ctrl+= and Ctrl+-, change it too.", size)),
	)
}

func (p *settingsPane) terminal() gunim.Node {
	p.startIn = p.newSavedLine("your home folder", 280, func(s string) string {
		if s == "" {
			return ""
		}
		if _, err := app.FolderHere(s); err != nil {
			return words.UpperFirst(err.Error()) + "."
		}
		return ""
	}, func(s string) gunim.Intent {
		return app.SaveThisComputer{StartFolder: s, Shell: p.st.ChosenShell}
	})
	p.shell = widget.NewDropdown(widget.Labels("Your default shell"))
	p.shell.Label = "Shell"
	p.shell.OnChange = func(i int, u *gunim.UI) gunim.Intent {
		if i < len(p.shellIDs) {
			p.send(app.SaveThisComputer{StartFolder: p.st.ThisComputer.StartFolder, Shell: p.shellIDs[i]}, u)
		}
		return nil
	}
	p.termProgram = p.newSavedLine("kakel", 180, nil, func(s string) gunim.Intent { return app.SetTermProgram{Called: s} })
	p.known = widget.NewDropdown(widget.Labels(append([]string{"kakel"}, app.KnownTerminals...)...))
	p.known.Label = "Known terminals"
	p.known.OnChange = func(i int, u *gunim.UI) gunim.Intent {
		called := ""
		if i > 0 && i-1 < len(app.KnownTerminals) {
			called = app.KnownTerminals[i-1]
		}
		p.termProgram.field.SetText(called, nil)
		p.termProgram.kept = called
		p.termProgram.save.Disabled = true
		p.send(app.SetTermProgram{Called: called}, u)
		return nil
	}
	p.pasteCheck = widget.NewSwitch("")
	p.pasteCheck.OnChange = func(on bool, u *gunim.UI) gunim.Intent { p.send(app.SetPasteCheck{On: on}, u); return nil }
	p.profiles = widget.NewLabel("")
	addP := widget.NewButton("Add Profile…")
	addP.OnClick = func(u *gunim.UI) gunim.Intent { p.w.profileDialog(p.st.Profiles, nil, u); return nil }
	p.editP = widget.NewButton("Edit Profile…")
	p.editP.OnClick = func(u *gunim.UI) gunim.Intent { p.w.editProfilePickerFrom(p.st.Profiles, u); return nil }
	p.removeP = widget.NewButton("Remove Profile…")
	p.removeP.OnClick = func(u *gunim.UI) gunim.Intent { p.w.removeProfilePickerFrom(p.st.Profiles, u); return nil }
	return settingsPage(
		settingsSection("This computer",
			settingRow("Start in", "New terminals start here, unless kakel was started in a folder of its own, or they open from a terminal in another.", p.startIn.node),
			settingRow("Shell", "What a new terminal here runs.", p.shell)),
		settingsSection("Profiles",
			faintLabel("Shells you add, here or on a saved server, such as MSYS2, nushell or tmux, offered with the ones kakel finds. A profile has its own folder and variables."),
			p.profiles,
			buttons(addP, p.editP, p.removeP)),
		settingsSection("Terminal identity",
			settingRow("TERM_PROGRAM", "What programs are told the terminal is called. Blank says kakel. Another name can turn on features such as images, and can bring sequences that show as text. For new panes.", p.termProgram.node),
			settingRow("Known terminals", "", p.known)),
		settingsSection("Pasting",
			settingRow("Show before pasting", "Text of more than one line, or over 5 KB, opens in an editor first, to change before it goes in.", p.pasteCheck)),
	)
}

func (p *settingsPane) notifications() gunim.Node {
	cell := func(n gunim.Node, width float32) gunim.Node { return widget.NewSized(n, width, 0) }
	row := func(name gunim.Node, sound, ring gunim.Node) gunim.Node {
		r := widget.Row(name, cell(sound, settingsCheck), cell(ring, settingsCheck)).Grow(name, 1)
		r.Cross = widget.CrossCenter
		return r
	}
	save := func(u *gunim.UI) {
		in := app.SaveLook{SystemTitleBar: p.st.SystemTitleBar}
		in.Sounds.Interface = p.iface.Checked()
		for i, r := range alertRows {
			*r.field(&in.Sounds) = p.sounds[i].Checked()
			*r.field(&in.Rings) = p.rings[i].Checked()
		}
		p.send(in, u)
	}
	check := func(tip string) *widget.Checkbox {
		c := widget.NewCheckbox("")
		c.Tooltip = tip
		c.OnChange = func(_ bool, u *gunim.UI) gunim.Intent { save(u); return nil }
		return c
	}
	rows := []gunim.Node{row(widget.NewLabel(""), faintLabel("Sound"), faintLabel("Rings"))}
	p.iface = check("Sound: buttons, menus and switches")
	rows = append(rows, row(widget.NewLabel("Buttons, menus and switches"), p.iface, widget.NewLabel("")))
	for _, r := range alertRows {
		snd, rng := check("Sound: "+r.title), check("Rings: "+r.title)
		p.sounds, p.rings = append(p.sounds, snd), append(p.rings, rng)
		rows = append(rows, row(widget.NewLabel(r.title), snd, rng))
	}
	return settingsPage(
		settingsSection("What kakel tells you of",
			append([]gunim.Node{faintLabel("A sound, and rings that spread out from the window, for each kind of event.")}, rows...)...),
	)
}

// serveAtStart are the choices of what kakel does as it starts when the
// window was served as it last closed.
var serveAtStart = []struct{ title, value string }{
	{"Ask", settings.ServeAsk}, {"Serve again", settings.ServeAlways}, {"Don't serve", settings.ServeNever},
}

func (p *settingsPane) sharing() gunim.Node {
	titles := make([]string, len(serveAtStart))
	for i, c := range serveAtStart {
		titles[i] = c.title
	}
	p.tunnels = widget.NewSwitch("")
	p.tunnels.OnChange = func(on bool, u *gunim.UI) gunim.Intent { p.send(app.SetTunnelsConnect{On: on}, u); return nil }
	p.atStart = widget.NewSegmented(titles...)
	p.atStart.OnChange = func(i int, u *gunim.UI) gunim.Intent {
		return app.SetServeAtStart{When: serveAtStart[max(0, min(i, len(serveAtStart)-1))].value}
	}
	p.port = widget.NewNumberField(0, 65535)
	// A port is a name more than an amount: typed, not dragged to.
	p.port.NoDrag = true
	p.port.OnChange = func(v float64, u *gunim.UI) gunim.Intent {
		return app.SetServeDefaults{Port: int(v), Anywhere: p.reach.Selected() == 1}
	}
	p.reach = widget.NewDropdown(widget.Labels("This machine only", "All networks"))
	p.reach.Label = "Listen on"
	p.reach.OnChange = func(i int, u *gunim.UI) gunim.Intent {
		p.send(app.SetServeDefaults{Port: int(p.port.Value()), Anywhere: i == 1}, u)
		return nil
	}
	p.proxy = p.newSavedLine("the system's", 280, func(s string) string {
		if _, err := relay.ParseProxy(s); err != nil {
			return words.UpperFirst(err.Error()) + "."
		}
		return ""
	}, func(s string) gunim.Intent { return app.SetRelayProxy{Proxy: s} })
	p.keys = widget.NewLabel("")
	add := widget.NewButton("Add Key…")
	add.OnClick = func(u *gunim.UI) gunim.Intent { p.w.addKeyDialogFrom(p.st.Serving, false, u); return nil }
	p.removeK = widget.NewButton("Remove Key…")
	p.removeK.OnClick = func(u *gunim.UI) gunim.Intent { p.w.removeKeyPickerFrom(p.st.Serving, false, u); return nil }
	p.serving = widget.NewButton("Serve This Window…")
	p.serving.OnClick = func(u *gunim.UI) gunim.Intent {
		if p.st.Serving.On {
			p.send(app.StopServing{}, u)
			return nil
		}
		p.w.run("serve.window", u)
		return nil
	}
	return settingsPage(
		settingsSection("Serving",
			faintLabel("Another kakel window, on a machine whose key is allowed, can take a served window over: open shells here, work in the panes, and read and write files as you."),
			settingRow("Now", "", p.serving),
			settingRow("When kakel starts", "If the window was served as kakel last closed.", p.atStart),
			settingRow("Port", "0 picks a free port.", widget.NewSized(p.port, 110, 0)),
			settingRow("Listen on", "", p.reach),
			settingRow("Tunnels with ssh", "Plain ssh on another machine can make tunnels through a served window.", p.settingButton("How…", "serve.ssh")),
			settingRow("Tunnels may connect", "A tunnel through a saved server not connected here connects it, when that needs nothing asked, and one may go through a kakel window connected here. Off, tunnels reach only the servers connected here, as a kakel window connecting does.", p.tunnels)),
		settingsSection("Keys that may connect",
			p.keys,
			buttons(add, p.removeK)),
		settingsSection("Relays",
			faintLabel("A relay joins two windows that cannot reach each other: run kakel -relay on a server, then serve or connect through it."),
			settingRow("Proxy", "Empty uses HTTPS_PROXY or, on Windows, the system's proxy; none goes straight to the relay. A PAC script or a Windows (NTLM) proxy login is not supported.",
				p.proxy.node)),
	)
}

func (p *settingsPane) files() gunim.Node {
	var rows []gunim.Node
	if dir, err := conf.Dir(); err == nil {
		for _, f := range []struct{ what, name string }{
			{"Settings", settings.File}, {"Saved servers", remote.BookFile}, {"Themes", themes.File},
			{"Shortcuts", keys.File}, {"Authorized keys", serve.AuthFile}, {"Known windows", app.KnownWindowsFile},
		} {
			rows = append(rows, settingRow(f.what, "", selectable(filepath.Join(dir, f.name))))
		}
	}
	if key, err := serve.HostKeyPath(); err == nil {
		rows = append(rows, settingRow("Serving key", "", selectable(key)))
	}
	rows = append(rows, faintLabel("SSH keys and known_hosts stay in ~/.ssh."))
	var place []gunim.Node
	if own, beside, err := conf.CarriesItsOwn(); err == nil {
		if own {
			place = append(place, faintLabel("Portable: the files are kept beside kakel, in "+beside+". The serving key is in there too, and is only as private as that folder."))
		} else if made, err := conf.IsDir(beside); err == nil && !made {
			portable := widget.NewButton("Make Portable")
			portable.OnClick = func(u *gunim.UI) gunim.Intent { p.send(app.MakePortable{}, u); return nil }
			place = append(place, settingRow("Make kakel portable", "Keep the files beside kakel, in "+beside+", so it carries them, as on a USB stick.", portable))
		}
	}
	sections := []gunim.Node{settingsSection("Where kakel keeps its files", rows...)}
	if len(place) > 0 {
		sections = append(sections, settingsSection("Portable", place...))
	}
	sections = append(sections,
		settingsSection("Themes and shortcuts",
			settingRow("Themes", "A file of themes to change, started from the one in use; read again after a change.",
				buttons(p.settingButton("Start the File", "view.themesStart"), p.settingButton("Read Again", "view.themesReload"))),
			settingRow("Shortcuts", "A file of the window's keys to change; read again after a change.",
				buttons(p.settingButton("Start the File", "shortcuts.write"), p.settingButton("Read Again", "shortcuts.reload"))),
			settingRow("Saved servers", "Read again after changing the file.", p.settingButton("Read Again", "server.reload"))),
	)
	return settingsPage(sections...)
}

// show brings the controls up to date with st, leaving a line being
// typed alone.
func (p *settingsPane) show(st app.State, u *gunim.UI) {
	p.st = st
	setOn := func(s *widget.Switch, on bool) {
		if s.Checked() != on {
			s.SetChecked(on, u)
		}
	}
	// General.
	setOn(p.autostart, st.Update.Autostart)
	p.autostart.Disabled = !st.Update.Installed
	setOn(p.tray, st.InTray)
	setOn(p.tunnels, st.Serving.TunnelsConnect)
	setOn(p.folders, st.Update.Folders)
	p.folders.Disabled = !st.Update.Installed
	p.installRow.set(st.Update.Installable && !st.Update.Installed, u)
	p.foldersRow.set(st.Update.FoldersHere, u)
	setOn(p.titleBar, st.SystemTitleBar)
	p.launcher.show(st.LauncherKey, u)
	for i, c := range launcherFilesChoices {
		if c.value == st.LauncherFiles && p.launcherFiles.Selected() != i {
			p.launcherFiles.SetSelected(i, u)
		}
	}
	for i, c := range updateChoices {
		if c.value == st.Update.Updates {
			p.updates.SetSelected(i, nil)
		}
	}
	p.updates.Disabled = !st.Update.Installed
	setOn(p.beta, st.Update.Beta)
	p.beta.Disabled = !st.Update.Installed
	// Appearance.
	if !slices.Equal(st.Themes, p.themeNames) {
		p.themeNames = slices.Clone(st.Themes)
		p.theme.SetItems(widget.Labels(p.themeNames...))
	}
	if i := slices.Index(p.themeNames, st.Theme); i >= 0 {
		p.theme.SetSelected(i, nil)
	}
	if !slices.Equal(st.Fonts, p.fontNames) {
		p.fontNames = slices.Clone(st.Fonts)
		p.font.SetItems(widget.Labels(p.fontNames...))
	}
	for i, name := range p.fontNames {
		if app.FontCommandID(name) == app.FontCommandID(st.Font.Name) {
			p.font.SetSelected(i, nil)
		}
	}
	if !u.HasFocus(p.size) && float32(p.size.Value()) != st.FontSize {
		p.size.SetValue(float64(st.FontSize), u)
	}
	// Terminal.
	p.startIn.show(st.ThisComputer.StartFolder, u)
	titles, ids := []string{"Your default shell"}, []string{""}
	for _, sh := range st.Shells {
		titles, ids = append(titles, sh.Title), append(ids, sh.ID)
	}
	if !slices.Equal(ids, p.shellIDs) || !slices.Equal(titles, p.shellTitles) {
		p.shellIDs, p.shellTitles = ids, titles
		p.shell.SetItems(widget.Labels(titles...))
	}
	p.shell.SetSelected(max(slices.Index(p.shellIDs, st.ChosenShell), 0), nil)
	p.termProgram.show(st.TermProgram, u)
	p.proxy.show(st.Serving.Proxy, u)
	setOn(p.pasteCheck, st.PasteCheck)
	p.profiles.Text = p.w.profilesSays(st.Profiles)
	p.editP.Disabled, p.removeP.Disabled = len(st.Profiles) == 0, len(st.Profiles) == 0
	p.known.SetSelected(0, nil)
	if i := slices.Index(app.KnownTerminals, st.TermProgram); i >= 0 {
		p.known.SetSelected(i+1, nil)
	}
	// Notifications.
	p.iface.SetChecked(st.Sounds.Interface, nil)
	for i, r := range alertRows {
		p.sounds[i].SetChecked(*r.field(&st.Sounds), nil)
		p.rings[i].SetChecked(*r.field(&st.Rings), nil)
	}
	// Sharing.
	s := st.Serving
	for i, c := range serveAtStart {
		if c.value == s.AtStart && p.atStart.Selected() != i {
			p.atStart.SetSelected(i, u)
		}
	}
	if !u.HasFocus(p.port) && int(p.port.Value()) != s.Port {
		p.port.SetValue(float64(s.Port), u)
	}
	p.reach.SetSelected(0, nil)
	if s.Anywhere {
		p.reach.SetSelected(1, nil)
	}
	switch {
	case s.Problem != "":
		p.keys.Text = words.UpperFirst(s.Problem) + "."
	case len(s.Keys) == 0:
		p.keys.Text = "None yet. Add the public key of the machine you will connect from."
	default:
		lines := make([]string, len(s.Keys))
		for i, k := range s.Keys {
			lines[i] = k.Name
			if k.Name != k.Fingerprint {
				lines[i] += "  ·  " + shortPrint(k.Fingerprint)
			}
		}
		p.keys.Text = strings.Join(lines, "\n")
	}
	p.removeK.Disabled = len(s.Keys) == 0
	if s.On {
		p.serving.Label = "Stop Serving"
	} else {
		p.serving.Label = "Serve This Window…"
	}
	u.Invalidate()
}

// Children implements [gunim.Composite].
func (p *settingsPane) Children() []gunim.Node { return []gunim.Node{p.tabs} }

// Layout implements [gunim.Node].
func (p *settingsPane) Layout(c gunim.Constraints, _ gunim.Frame, kids gunim.Children) geom.Size {
	k := kids.At(0)
	k.Layout(gunim.Tight(c.Max))
	k.Place(geom.Point{})
	return c.Max
}

// Paint implements [gunim.Node].
func (p *settingsPane) Paint(pt *paint.Painter, f gunim.Frame, box geom.Size, kids gunim.Children) {
	pt.RRect(geom.Rect{Max: box.Point()}, 0, paint.Solid(widget.Background.Get(f.Theme)))
	kids.At(0).Paint(pt)
}

// Handle implements [gunim.Handler]: the pane says it has the keyboard,
// as the window's panes do.
func (p *settingsPane) Handle(e gi.Event, u *gunim.UI) bool {
	if _, ok := e.(gi.FocusEntered); ok {
		p.w.entered(p.w.paneOfKind(app.KindSettings), u)
	}
	return false
}

// maybeShown is a row that shows only where it applies: Install kakel
// for a copy that is not installed, and the default file manager where
// folders can open in kakel. Hidden, it takes no room.
type maybeShown struct {
	child gunim.Node
	shown bool
}

// set shows the row or hides it.
func (m *maybeShown) set(on bool, u *gunim.UI) {
	if m.shown == on {
		return
	}
	m.shown = on
	if on {
		u.Insert(m, m.child)
	} else {
		u.Remove(m.child)
	}
}

// Children implements [gunim.Composite].
func (m *maybeShown) Children() []gunim.Node {
	if !m.shown {
		return nil
	}
	return []gunim.Node{m.child}
}

// Layout implements [gunim.Node].
func (m *maybeShown) Layout(c gunim.Constraints, _ gunim.Frame, kids gunim.Children) geom.Size {
	if kids.Len() == 0 {
		return geom.Size{W: c.Min.W}
	}
	k := kids.At(0)
	size := k.Layout(c)
	k.Place(geom.Point{})
	return size
}

// Paint implements [gunim.Node].
func (m *maybeShown) Paint(p *paint.Painter, _ gunim.Frame, _ geom.Size, kids gunim.Children) {
	for k := range kids.All {
		k.Paint(p)
	}
}
