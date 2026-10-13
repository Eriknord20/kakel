package view

import (
	"slices"
	"strconv"
	"strings"

	"github.com/marrasen/kakel/app"
	"github.com/marrasen/kakel/machines"

	"github.com/marrasen/gunim"
	"github.com/marrasen/gunim/geom"
	"github.com/marrasen/gunim/icon"
	"github.com/marrasen/gunim/widget"

	"github.com/marrasen/kakel/relay"
	"github.com/marrasen/kakel/remote"
	"github.com/marrasen/kakel/words"
)

// servingDialog serves the window, or, while it is served, says where
// and to whom.
func (w *Window) servingDialog(s app.Serving, u *gunim.UI) {
	if s.On {
		w.servedDialog(s, u)
		return
	}
	form := widget.NewForm().Add("", widget.NewLabel("A connected window can open shells here, use the ones running, and read and write files as you."))
	addAllowed(form, s)
	onPort := widget.NewCheckbox("On a port of this machine")
	onPort.SetChecked(!s.NoPort, nil)
	port := widget.NewTextField()
	port.SetText(strconv.Itoa(s.Port), nil)
	port.Placeholder = "0 picks a free port"
	where := widget.NewDropdown(widget.Labels("This machine only", "All networks"))
	where.Label = "Listen on"
	if s.Anywhere {
		where.SetSelected(1, nil)
	}
	onRelay := widget.NewCheckbox("Through a relay")
	onRelay.SetChecked(s.RelayOn, nil)
	relayURL := widget.NewTextField()
	relayURL.SetText(s.RelayURL, nil)
	relayURL.Placeholder = "https://relay.example.com/k/…"
	relayVia, viaIDs := w.throughPicker(s.RelayVia)
	enable := func() {
		port.Disabled, where.Disabled = !onPort.Checked(), !onPort.Checked()
		relayURL.Disabled = !onRelay.Checked()
		relayVia.Disabled = !onRelay.Checked() || len(viaIDs) <= 1
	}
	enable()
	onPort.OnChange = func(bool, *gunim.UI) gunim.Intent { enable(); return nil }
	onRelay.OnChange = func(bool, *gunim.UI) gunim.Intent { enable(); return nil }
	// Said beside the field, as the field opens with a port in it and
	// a placeholder would never show.
	form.Add("", onPort).Add("Port", port).Add("", widget.NewLabel("0 picks a free port.")).Add("Listen on", where)
	form.Add("", onRelay).Add("Relay", relayURL).Add("Through", relayVia).
		Add("", widget.NewLabel(relayCodeSays(s)))
	d := widget.NewDialog("Serve This Window")
	d.Body = form
	d.SetButtons("Serve", "Cancel")
	w.keyActions(d, s)
	d.Check = func() string {
		n, err := strconv.Atoi(strings.TrimSpace(port.Text()))
		switch {
		case !onPort.Checked() && !onRelay.Checked():
			return "Serve on a port, through a relay, or both."
		case onPort.Checked() && (err != nil || n < 0 || n > 65535):
			return "The port is a number from 0 to 65535."
		case onRelay.Checked() && strings.TrimSpace(relayURL.Text()) == "":
			return "Paste the relay's URL, as kakel -relay printed it."
		case len(s.Allowed) == 0:
			return "Add a key that may connect first."
		}
		if onRelay.Checked() {
			if _, err := relay.ParseURL(relayURL.Text()); err != nil {
				return words.UpperFirst(err.Error()) + "."
			}
		}
		return ""
	}
	d.OnAccept = func(u *gunim.UI) gunim.Intent {
		// Shown once it is served: the host key, to check from the
		// other end. Nothing, if serving did not start.
		w.servingAsked, w.servingTries = true, s.Tries
		return app.StartServing{Port: port.Text(), Anywhere: where.Selected() == 1,
			NoPort: !onPort.Checked(), Relay: onRelay.Checked(), RelayURL: relayURL.Text(),
			RelayVia: viaIDs[max(0, min(relayVia.Selected(), len(viaIDs)-1))]}
	}
	d.OnDismiss = widget.Sends(app.DialogClosed{})
	w.openDialog(d, u)
}

// relayCodeSays is what the Serve dialog says of the code the window
// is served under on a relay.
func relayCodeSays(s app.Serving) string {
	if s.Code == "" {
		return "Kakel makes a code for the other end to type, and keeps it."
	}
	return "The other end types the code " + s.Code + ". It stays the same each time."
}

// relaySays is how serving through the relay is going.
func relaySays(s app.Serving) string {
	switch {
	case s.Reached:
		return "Reached. Windows connect with the code."
	case s.RelayTrouble != "":
		return "Not reached: " + s.RelayTrouble + ". Trying again."
	}
	return "Reaching the relay…"
}

// servedDialog says where the window is served and who is connected.
func (w *Window) servedDialog(s app.Serving, u *gunim.UI) {
	form := widget.NewForm()
	if s.Addr != "" {
		form.Add("Address", widget.NewLabel(s.Addr))
	}
	var code, state *widget.Label
	if s.Relayed {
		code, state = widget.NewLabel(s.Code), widget.NewLabel(relaySays(s))
		form.Add("Relay", widget.NewLabel(s.RelayHost)).Add("Code", code).Add("", state)
	}
	form.Add("Host key", widget.NewLabel(s.Fingerprint))
	who := widget.NewLabel(connectedSays(s))
	form.Add("Connected", who)
	addAllowed(form, s)
	d := widget.NewDialog("Serving This Window")
	d.Body = form
	d.SetButtons("Done", "")
	w.keyActions(d, s)
	if s.Relayed {
		d.AddAction("Copy Link", func(u *gunim.UI) gunim.Intent { return app.CopyRelayLink{} })
		d.AddAction("New Code", func(u *gunim.UI) gunim.Intent { return app.NewRelayCode{} })
	}
	d.AddButton("Disconnect All", func(u *gunim.UI) gunim.Intent { return app.DisconnectClients{} })
	d.AddButton("Stop Serving", func(u *gunim.UI) gunim.Intent { return app.StopServing{} })
	d.OnAccept, d.OnDismiss = widget.Sends(app.DialogClosed{}), widget.Sends(app.DialogClosed{})
	w.served = &servedShown{d: d, who: who, code: code, state: state}
	w.openDialog(d, u)
}

// addAllowed adds the keys that may connect to form, or why none can.
func addAllowed(form *widget.Form, s app.Serving) {
	switch {
	case s.Problem != "":
		form.Add("", widget.NewLabel(words.UpperFirst(s.Problem)+"."))
	case len(s.Keys) == 0:
		form.Add("Allowed", widget.NewLabel("No key may connect yet. Add the public key of the machine you will connect from."))
	default:
		lines := make([]string, len(s.Keys))
		for i, k := range s.Keys {
			lines[i] = k.Name
			if k.Name != k.Fingerprint {
				lines[i] += "  ·  " + shortPrint(k.Fingerprint)
			}
		}
		form.Add("Allowed", widget.NewLabel(strings.Join(lines, "\n")))
	}
}

// shortPrint is a key's fingerprint cut to what tells keys apart at a
// glance.
func shortPrint(fp string) string {
	if r := []rune(fp); len(r) > 19 {
		return string(r[:19]) + "…"
	}
	return fp
}

// keyActions adds Add Key… and Remove Key… to d, a dialog of serving,
// which close it and open their own; the serving dialog opens again
// once the change has been tried.
func (w *Window) keyActions(d *widget.Dialog, s app.Serving) {
	if s.Problem != "" {
		return
	}
	d.AddAction("Add Key…", func(u *gunim.UI) gunim.Intent {
		d.Close(u)
		w.addKeyDialogFrom(s, true, u)
		return nil
	})
	if len(s.Keys) > 0 {
		d.AddAction("Remove Key…", func(u *gunim.UI) gunim.Intent {
			d.Close(u)
			w.removeKeyPickerFrom(s, true, u)
			return nil
		})
	}
}

// addKeyDialogFrom asks for a key that may connect: pasted, or one of
// this machine's. With again, the serving dialog opens again once the
// key has been tried, as for one asked from there.
func (w *Window) addKeyDialogFrom(s app.Serving, again bool, u *gunim.UI) {
	text := widget.NewTextArea()
	text.Placeholder = "ssh-ed25519 AAAA… name@machine"
	choices := []string{"Paste one below"}
	for _, k := range s.Here {
		choices = append(choices, k.Name)
	}
	pick := widget.NewDropdown(widget.Labels(choices...))
	pick.Label = "Key"
	pick.Disabled = len(s.Here) == 0
	here := slices.Clone(s.Here)
	form := widget.NewForm().
		Add("", widget.NewLabel("The public key of the machine you will connect from: the line in its ~/.ssh/id_ed25519.pub, or the like.")).
		Add("Key", pick).
		Add("", text)
	d := widget.NewDialog("Add a Key That May Connect")
	d.Body = form
	d.SetButtons("Add", "Cancel")
	d.Check = func() string {
		if pick.Selected() == 0 && strings.TrimSpace(text.Text()) == "" {
			return "Paste a public key, or pick one on this machine."
		}
		return ""
	}
	d.OnAccept = func(u *gunim.UI) gunim.Intent {
		w.keysAsked, w.keysEdits = again, w.serving.Edits
		if i := pick.Selected() - 1; i >= 0 && i < len(here) {
			return app.AllowKey{Path: here[i].Path}
		}
		return app.AllowKey{Text: text.Text()}
	}
	d.OnDismiss = widget.Sends(app.DialogClosed{})
	w.openDialog(d, u)
}

// removeKeyPickerFrom offers the keys that may connect, to take one
// off; with again, as addKeyDialogFrom.
func (w *Window) removeKeyPickerFrom(s app.Serving, again bool, u *gunim.UI) {
	p := &widget.Palette{Placeholder: "The key that may no longer connect"}
	keys := slices.Clone(s.Keys)
	for _, k := range keys {
		p.Items = append(p.Items, widget.PaletteItem{Title: k.Name, Hint: k.Type + "  " + k.Fingerprint, Icon: icon.KeyRound})
	}
	p.OnPick = func(i int, u *gunim.UI) gunim.Intent {
		w.keysAsked, w.keysEdits = again, w.serving.Edits
		u.Send(w, app.DisallowKey{Fingerprint: keys[i].Fingerprint})
		return nil
	}
	w.keyPicker = p
	p.Open(w, geom.Rc(0, 48, w.size.W, 0), u)
}

// connectedSays is who is connected to the window served, one a line.
func connectedSays(s app.Serving) string {
	if len(s.Clients) == 0 {
		return "Nobody yet."
	}
	var lines []string
	for _, c := range s.Clients {
		lines = append(lines, c.Name+", from "+c.From)
		for _, t := range s.Tunnels {
			if t.Client == c.Name && t.From == c.From {
				lines = append(lines, "    a tunnel, "+t.Label+", on "+t.On)
			}
		}
	}
	return strings.Join(lines, "\n")
}

// servedShown is the dialog saying the window is served, while it is
// open, and its line of who is connected.
type servedShown struct {
	d   *widget.Dialog
	who *widget.Label
	// code and state are the code on the relay and how reaching it is
	// going, nil when the window is not served through one.
	code, state *widget.Label
}

// showServed keeps the dialog saying the window is served up to date
// as windows connect and go, closes it once serving stops, and opens
// it once serving asked for has started.
func (w *Window) showServed(s app.Serving, u *gunim.UI) {
	if sh := w.served; sh != nil {
		switch {
		case u.Presence(sh.d) == gunim.Exiting || w.dialog != sh.d:
			w.served = nil
		case !s.On:
			w.served = nil
			sh.d.Close(u)
		default:
			sh.who.Text = connectedSays(s)
			if sh.code != nil {
				sh.code.Text, sh.state.Text = s.Code, relaySays(s)
			}
			u.Invalidate()
		}
	}
	// A key added or taken off, once it has been tried: the serving
	// dialog again, with the keys as they are now.
	if w.keysAsked && s.Edits > w.keysEdits && (w.dialog == nil || u.Presence(w.dialog) == gunim.Exiting) {
		w.keysAsked = false
		w.servingDialog(s, u)
	}
	// The one asked for, once it has been tried: shown if it started,
	// once the dialog that asked has gone.
	if w.servingAsked && s.Tries > w.servingTries {
		if !s.On {
			w.servingAsked = false
		} else if w.dialog == nil || u.Presence(w.dialog) == gunim.Exiting {
			w.servingAsked = false
			w.servedDialog(s, u)
		}
	}
}

// connectWindowDialog asks for a served window to connect to: at an
// address, or through a relay by its code.
func (w *Window) connectWindowDialog(u *gunim.UI) {
	how := widget.NewDropdown(widget.Labels("Directly", "Through a relay"))
	how.Label = "Connect"
	addr, key := widget.NewTextField(), widget.NewTextField()
	addr.Placeholder = "host, or host:" + strconv.Itoa(remote.ServePort)
	key.Placeholder = "the usual keys in ~/.ssh"
	relayURL, code := widget.NewTextField(), widget.NewTextField()
	relayURL.Placeholder = "https://relay.example.com/k/…"
	code.Placeholder = "XXXX-XXXX"
	if last := w.serving.RelayLast; last != "" {
		relayURL.SetText(last, nil)
		how.SetSelected(1, nil)
	}
	// A link pasted whole, the relay and the code, is split into the two.
	relayURL.OnChange = func(text string, u *gunim.UI) gunim.Intent {
		if l, err := relay.ParseLink(text); err == nil {
			relayURL.SetText(l.Relay.String(), u)
			code.SetText(l.Code, u)
		}
		return nil
	}
	via, viaIDs := w.throughPicker("")
	via.Disabled = len(viaIDs) <= 1
	applies := func() {
		viaRelay := how.Selected() == 1
		addr.Disabled = viaRelay
		relayURL.Disabled, code.Disabled = !viaRelay, !viaRelay
	}
	applies()
	addr.OnChange = func(text string, u *gunim.UI) gunim.Intent {
		if l, err := relay.ParseLink(text); err == nil {
			addr.SetText("", u)
			relayURL.SetText(l.Relay.String(), u)
			code.SetText(l.Code, u)
			how.SetSelected(1, u)
			applies()
			u.Invalidate()
		}
		return nil
	}
	how.OnChange = func(int, *gunim.UI) gunim.Intent { applies(); return nil }
	d := widget.NewDialog("Connect to Window")
	d.Body = widget.NewForm().
		Add("", widget.NewLabel("The other window must be served, with this machine's public key in its authorized keys.")).
		Add("Connect", how).Add("Address", addr).Add("Relay", relayURL).Add("Code", code).Add("Through", via).Add("Key file", key)
	d.SetButtons("Connect", "Cancel")
	link := func() (string, string) {
		u, err := relay.ParseURL(relayURL.Text())
		if err != nil {
			return "", words.UpperFirst(err.Error()) + "."
		}
		c, err := relay.ParseCode(code.Text())
		if err != nil {
			return "", words.UpperFirst(err.Error()) + "."
		}
		return relay.Link{Relay: u, Code: c}.String(), ""
	}
	d.Check = func() string {
		if how.Selected() == 1 {
			_, why := link()
			return why
		}
		if strings.TrimSpace(addr.Text()) == "" {
			return "Type the other window's address."
		}
		return ""
	}
	d.OnAccept = func(u *gunim.UI) gunim.Intent {
		to := addr.Text()
		if how.Selected() == 1 {
			to, _ = link()
		}
		return app.ConnectWindow{Addr: to, KeyFile: key.Text(), Via: viaIDs[max(0, min(via.Selected(), len(viaIDs)-1))]}
	}
	d.OnDismiss = widget.Sends(app.DialogClosed{})
	w.openDialog(d, u)
}

// throughPicker offers the saved servers to reach something through,
// after "This machine", with picked chosen, and their IDs in the same
// order, "" for this machine. A kakel window is not offered.
func (w *Window) throughPicker(picked machines.ID) (*widget.Dropdown, []machines.ID) {
	labels := []string{"This machine"}
	ids := []machines.ID{""}
	for _, h := range w.saved {
		if !h.Window {
			labels = append(labels, h.Name)
			ids = append(ids, machines.ID(h.ID))
		}
	}
	d := widget.NewDropdown(widget.Labels(labels...))
	d.Label = "Through"
	for i, id := range ids {
		if id != "" && id == picked {
			d.SetSelected(i, nil)
		}
	}
	return d, ids
}
