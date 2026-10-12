package view

import (
	"unicode/utf8"

	"github.com/marrasen/kakel/app"
	"github.com/marrasen/kakel/look"
	"github.com/marrasen/kakel/machines"

	"github.com/marrasen/gunim"
	"github.com/marrasen/gunim/anim"
	"github.com/marrasen/gunim/geom"
	"github.com/marrasen/gunim/icon"
	"github.com/marrasen/gunim/input"
	"github.com/marrasen/gunim/match"
	"github.com/marrasen/gunim/paint"
	"github.com/marrasen/gunim/text"
	"github.com/marrasen/gunim/theme"
	"github.com/marrasen/gunim/widget"
)

// The launcher's window: a field that finds a machine by its name as it
// is typed, over the machines found, and once something is typed, the
// shells here, files on each machine, saved commands and kakel's own
// windows too. The machine lit shows what can be opened on it as chips
// at the end of its line, the one Enter opens lit: what was opened there
// last, at first its first, a terminal or the shell a new terminal
// starts. Tab and Shift+Tab move the light along them, and so do Right
// and Left once the field's caret has gone as far as it goes. Escape,
// or a click on another program, closes it.

// launcherRow is the height of a line of the launcher.
const launcherRow = 34

// launcherRows is how many lines the launcher shows.
const launcherRows = 8

// LauncherSize is the launcher window's size.
var LauncherSize = geom.Sz(560, 12*2+36+8+launcherRows*launcherRow)

// Chips: the room inside each, between them, and before the first.
const (
	launchChipPad  = 8
	launchChipGap  = 4
	launchChipLead = 16
)

// Launcher is the launcher's view.
type Launcher struct {
	anim.Group
	field *widget.TextField
	st    app.LaunchState
	// opened is the opening shown, and found what the field finds, by
	// index into the machines and then the things. hot is the line lit,
	// and first the first line shown.
	opened     uint64
	found      []int
	hot, first int
	// chip is the chip lit on the machine lit, chipFor that machine's ID
	// and chipSet whether one was lit there; chipAt and chipW carry the
	// light, and chipOff how far the chips are scrolled to show it.
	// chipJump has them go there at once, for a line lit anew.
	chip     int
	chipFor  machines.ID
	chipSet  bool
	chipAt   *anim.Float
	chipW    *anim.Float
	chipOff  *anim.Float
	chipJump bool
	// chipHits are where the chips of the line lit were drawn, for a
	// click.
	chipHits    []geom.Rect
	runs        []text.Run
	notes       []text.Run
	chipRuns    []text.Run
	hint        text.Run
	listTop     float32
	pinned      bool
	shapedWidth float32
}

// NewLauncher returns the launcher's view.
func NewLauncher() *Launcher {
	l := &Launcher{field: widget.NewTextField(), chipAt: anim.NewFloat(0), chipW: anim.NewFloat(0), chipOff: anim.NewFloat(0)}
	l.Add(l.chipAt, l.chipW, l.chipOff)
	l.field.Icon = icon.Search
	// What is typed lights the best of what it finds.
	l.field.OnChange = func(string, *gunim.UI) gunim.Intent { l.hot, l.first = 0, 0; l.find(); return nil }
	// Right with the caret at the end, and Left with it at the start,
	// go along the chips.
	l.field.Keys = func(e input.KeyPress, u *gunim.UI) bool {
		if e.Mods != 0 || e.Key != input.KeyRight && e.Key != input.KeyLeft {
			return false
		}
		start, end := l.field.Selection()
		at := 0
		if e.Key == input.KeyRight {
			at = utf8.RuneCountInString(l.field.Text())
		}
		return start == end && end == at && l.key(e, u)
	}
	return l
}

// Update shows st, afresh for each opening.
func (l *Launcher) Update(st app.LaunchState, u *gunim.UI) {
	l.st = st
	if !l.pinned {
		l.pinned = true
		_ = u.SetPinned(true)
	}
	if st.Opened != l.opened {
		l.opened = st.Opened
		l.hot, l.first = 0, 0
		l.chipFor, l.chipSet = "", false
		l.field.SetText("", nil)
		u.Focus(l.field)
	}
	l.find()
	// The machines may come in another order: the light stays on the
	// machine lit, and its chip with it.
	if l.chipSet {
		for k, i := range l.found {
			if i < len(l.st.Machines) && l.st.Machines[i].ID == l.chipFor {
				l.hot = k
				break
			}
		}
	}
	u.Invalidate()
}

// titles are the titles of what the launcher lists now: the machines,
// and once something is typed, the rest too, after them.
func (l *Launcher) titles() []match.Item {
	var out []match.Item
	for _, m := range l.st.Machines {
		out = append(out, match.Item{Title: m.Name, Also: []string{m.Note}})
	}
	if l.field.Text() != "" {
		for _, t := range l.st.Things {
			out = append(out, match.Item{Title: t.Title, Also: t.Also})
		}
	}
	return out
}

// find lists what the field finds.
func (l *Launcher) find() {
	l.found = l.found[:0]
	for _, f := range match.Rank(l.titles(), l.field.Text()) {
		l.found = append(l.found, f.Index)
	}
	l.hot = min(max(l.hot, 0), max(len(l.found)-1, 0))
	l.runs = nil
}

// Children implements [gunim.Composite].
func (l *Launcher) Children() []gunim.Node { return []gunim.Node{l.field} }

// Layout implements [gunim.Node].
func (l *Launcher) Layout(c gunim.Constraints, f gunim.Frame, kids gunim.Children) geom.Size {
	const pad = 12
	fs := kids.At(0).Layout(gunim.Tight(geom.Sz(c.Max.W-2*pad, widget.FieldHeight.Get(f.Theme))))
	kids.At(0).Place(geom.Pt(pad, pad))
	l.listTop = pad + fs.H + 8
	return c.Max
}

// Paint implements [gunim.Node].
func (l *Launcher) Paint(p *paint.Painter, f gunim.Frame, box geom.Size, kids gunim.Children) {
	th := f.Theme
	p.RRect(geom.Rect{Max: box.Point()}, 0, paint.Solid(look.SidebarFill.Get(th)))
	kids.At(0).Paint(p)
	size := widget.TextSize.Get(th)
	lit := l.litMachine()
	if l.runs == nil || l.shapedWidth != box.W {
		l.shapedWidth = box.W
		l.runs, l.notes = l.runs[:0], l.notes[:0]
		for _, i := range l.found {
			title, note := l.line(i)
			l.runs = append(l.runs, text.Default().Shape(title, size))
			l.notes = append(l.notes, text.Default().Shape(note, smallText.Get(th)))
		}
		l.chipRuns = l.chipRuns[:0]
		if lit != nil {
			for _, a := range lit.Actions {
				l.chipRuns = append(l.chipRuns, text.Default().Shape(a.Title, smallText.Get(th)))
			}
		}
		l.hint = text.Default().Shape(l.hintText(), smallText.Get(th))
	}
	ink, faint := widget.Ink.Get(th), look.Faint.Get(th)
	l.chipHits = l.chipHits[:0]
	if len(l.found) == 0 {
		t := text.Default().Shape("Nothing is called that", size)
		t.Paint(p, geom.Pt(20, l.listTop+(launcherRow-t.Height())/2), faint)
		return
	}
	l.first = min(max(l.first, l.hot-launcherRows+1), l.hot)
	for k := l.first; k < len(l.found) && k < l.first+launcherRows; k++ {
		r := geom.Rc(8, l.listTop+float32(k-l.first)*launcherRow, box.W-16, launcherRow-2)
		if k == l.hot {
			p.RRect(r, look.RowRadius.Get(th), paint.Solid(look.RowActive.Get(th)))
		}
		ic := icon.Server
		switch {
		case l.thing(l.found[k]) != nil:
			ic = thingIcons[l.thing(l.found[k]).Kind]
		case l.st.Machines[l.found[k]].ID == "":
			ic = icon.Laptop
		case l.st.Machines[l.found[k]].Window:
			ic = icon.AppWindow
		}
		if ic == nil {
			ic = icon.SquareTerminal
		}
		mid := r.Min.Y + r.Size().H/2
		drawIcon(p, ic, geom.Rc(r.Min.X+10, mid-8, 16, 16), faint, 1.3)
		run := l.runs[k]
		run.Paint(p, geom.Pt(r.Min.X+36, mid-run.Height()/2), ink)
		end := r.Min.X + 36 + run.Advance
		if n := l.notes[k]; n.Advance > 0 {
			n.Paint(p, geom.Pt(end+10, mid-n.Height()/2), faint)
			end += 10 + n.Advance
		}
		if k != l.hot {
			continue
		}
		if lit != nil && len(l.chipRuns) == len(lit.Actions) {
			l.paintChips(p, th, geom.Rc(end+launchChipLead, r.Min.Y, r.Max.X-6-(end+launchChipLead), r.Size().H))
			continue
		}
		h := l.hint
		h.Paint(p, geom.Pt(r.Max.X-12-h.Advance, mid-h.Height()/2), faint)
	}
}

// paintChips draws what can be opened on the machine lit, in area at the
// end of its line: against its right, or where they are too many to
// fit, from its left and scrolled to show the one lit. The light slides
// from chip to chip.
func (l *Launcher) paintChips(p *paint.Painter, th *theme.Live, area geom.Rect) {
	if area.Size().W <= 0 {
		return
	}
	xs := make([]float32, len(l.chipRuns))
	ws := make([]float32, len(l.chipRuns))
	var all float32
	for i, run := range l.chipRuns {
		if i > 0 {
			all += launchChipGap
		}
		xs[i], ws[i] = all, run.Advance+2*launchChipPad
		all += ws[i]
	}
	// Scrolled as little as shows the chip lit whole.
	off := l.chipOff.Target()
	if l.chip < len(xs) {
		off = min(off, xs[l.chip])
		off = max(off, xs[l.chip]+ws[l.chip]-area.Size().W)
	}
	off = min(max(off, 0), max(all-area.Size().W, 0))
	start := area.Min.X
	if all < area.Size().W {
		start = area.Max.X - all
	}
	spring := widget.Quick.Get(th)
	switch {
	case l.chip >= len(xs):
	case l.chipJump:
		l.chipJump = false
		l.chipOff.Jump(off)
		l.chipAt.Jump(xs[l.chip])
		l.chipW.Jump(ws[l.chip])
	default:
		l.chipOff.Animate(off, spring)
		l.chipAt.Animate(xs[l.chip], spring)
		l.chipW.Animate(ws[l.chip], spring)
	}
	h := min(22, area.Size().H-6)
	top := area.Min.Y + (area.Size().H-h)/2
	o := start - l.chipOff.Value()
	defer p.Layer(paint.LayerOpts{Bounds: area, Opacity: 1, Clip: true})()
	p.RRect(geom.Rc(o+l.chipAt.Value(), top, l.chipW.Value(), h), h/2, paint.Solid(widget.ButtonPrimaryFill.Get(th)))
	faint, onLit := look.Faint.Get(th), widget.ButtonPrimaryInk.Get(th)
	edge := faint
	edge.A = uint8(float32(edge.A) * 0.35)
	from, to := l.chipAt.Value(), l.chipAt.Value()+l.chipW.Value()
	for i, run := range l.chipRuns {
		r := geom.Rc(o+xs[i], top, ws[i], h)
		// Clicked only where it shows.
		l.chipHits = append(l.chipHits, geom.Rect{Min: geom.Pt(max(r.Min.X, area.Min.X), r.Min.Y), Max: geom.Pt(min(r.Max.X, area.Max.X), r.Max.Y)})
		// As lit as the light covers it, so its text brightens and its
		// edge fades as the light slides onto it, and back as it leaves.
		lit := min(max((min(to, xs[i]+ws[i])-max(from, xs[i]))/ws[i], 0), 1)
		if lit < 1 {
			e := edge
			e.A = uint8(float32(e.A) * (1 - lit))
			p.RRectStroke(r.Inset(geom.Uniform(0.5)), h/2, paint.Fill{}, paint.Stroke{Width: 1, Color: e})
		}
		run.Paint(p, geom.Pt(r.Min.X+launchChipPad, top+(h-run.Height())/2), anim.Mix(anim.ColorCodec, faint, onLit, lit))
	}
}

// thingIcons are the icons of the kinds of things the launcher finds.
var thingIcons = map[string]*icon.Icon{
	"terminal": icon.SquareTerminal, "files": icon.Folder, "log": icon.ScrollText, "command": icon.SquareChevronRight,
	"servers": icon.Server, "secrets": icon.Lock, "window": icon.AppWindow, "panes": icon.LayoutGrid,
	"settings": icon.Settings, "about": icon.Info, "copy": icon.Copy, "tunnel": icon.Cable,
}

// thing is found item i when it is one of the things rather than a
// machine, and nil otherwise.
func (l *Launcher) thing(i int) *app.LaunchThing {
	n := len(l.st.Machines)
	if i < n || i-n >= len(l.st.Things) {
		return nil
	}
	return &l.st.Things[i-n]
}

// litMachine is the machine on the line lit, nil where that is a thing
// or there is none. Lit anew, its chip lit is what Enter opens there.
func (l *Launcher) litMachine() *app.LaunchMachine {
	if l.hot >= len(l.found) || l.thing(l.found[l.hot]) != nil {
		return nil
	}
	m := &l.st.Machines[l.found[l.hot]]
	if !l.chipSet || l.chipFor != m.ID {
		l.chipFor, l.chipSet, l.chipJump = m.ID, true, true
		l.chip = m.Default
		l.runs = nil
	}
	l.chip = min(max(l.chip, 0), max(len(m.Actions)-1, 0))
	return m
}

// line is the title and the note of found item i.
func (l *Launcher) line(i int) (string, string) {
	if t := l.thing(i); t != nil {
		return t.Title, t.Note
	}
	m := l.st.Machines[i]
	return m.Name, m.Note
}

// hintText says what Enter does on a thing lit; a machine's chips say
// it for themselves.
func (l *Launcher) hintText() string {
	if l.hot < len(l.found) && l.thing(l.found[l.hot]) != nil {
		return "Enter opens it"
	}
	return ""
}

// CaptionRects implements [gunim.Caption]: the launcher is a window
// with no title bar, moved by its edge round the field.
func (l *Launcher) CaptionRects(size geom.Size) []geom.Rect {
	const pad = 12
	return []geom.Rect{geom.Rc(0, 0, size.W, pad), geom.Rc(0, 0, pad, l.listTop), geom.Rc(size.W-pad, 0, pad, l.listTop)}
}

// Handle implements [gunim.Handler]: the keys the field passes on, a
// click on a line or a chip, and the window losing the keyboard.
func (l *Launcher) Handle(e input.Event, u *gunim.UI) bool {
	switch e := e.(type) {
	case input.WindowFocusLost:
		u.Send(l, app.CloseLauncher{})
		return false
	case input.PointerDown:
		k := int((e.Pos.Y - l.listTop) / launcherRow)
		if e.Button != input.ButtonPrimary || e.Pos.Y < l.listTop || k >= launcherRows || l.first+k >= len(l.found) {
			return false
		}
		if l.first+k == l.hot {
			for i, r := range l.chipHits {
				if r.Contains(e.Pos) {
					l.chip = i
				}
			}
		}
		l.hot = l.first + k
		l.runs = nil
		u.Invalidate()
		l.pick(u)
		return true
	case input.Scroll:
		// A line at a notch, the list staying within what it found.
		step := 1
		if e.Delta.Y > 0 {
			step = -1
		}
		l.first = min(max(l.first+step, 0), max(len(l.found)-launcherRows, 0))
		l.hot = min(max(l.hot, l.first), l.first+launcherRows-1)
		l.runs = nil
		u.Invalidate()
		return true
	case input.KeyPress:
		return l.key(e, u)
	}
	return false
}

// key moves the light along the lines and the chips, picks, and closes.
func (l *Launcher) key(e input.KeyPress, u *gunim.UI) bool {
	switch e.Key {
	case input.KeyUp:
		l.hot = max(l.hot-1, 0)
	case input.KeyDown:
		l.hot = min(l.hot+1, max(len(l.found)-1, 0))
	case input.KeyEnter, input.KeyKPEnter:
		l.pick(u)
	case input.KeyTab:
		// Round the chips, the other way with Shift.
		if m := l.litMachine(); m != nil && len(m.Actions) > 0 {
			step := 1
			if e.Mods.Has(input.ModShift) {
				step = len(m.Actions) - 1
			}
			l.chip = (l.chip + step) % len(m.Actions)
		}
	case input.KeyRight, input.KeyLeft:
		// The field's caret has gone as far as it goes, as its Keys
		// says: along the chips, as far as the last or the first.
		m := l.litMachine()
		if m == nil {
			return false
		}
		to := l.chip + 1
		if e.Key == input.KeyLeft {
			to = l.chip - 1
		}
		if to < 0 || to >= len(m.Actions) {
			return false
		}
		l.chip = to
	case input.KeyEscape:
		u.Send(l, app.CloseLauncher{})
		return true
	default:
		return false
	}
	l.runs = nil
	u.Invalidate()
	return true
}

// pick opens the line lit: the chip lit on a machine, or the thing lit.
func (l *Launcher) pick(u *gunim.UI) {
	if l.hot >= len(l.found) {
		return
	}
	if t := l.thing(l.found[l.hot]); t != nil {
		u.Send(l, app.Launch{Machine: t.Machine, Action: t.Action})
		return
	}
	m := l.litMachine()
	if m != nil && l.chip < len(m.Actions) {
		u.Send(l, app.Launch{Machine: m.ID, Action: m.Actions[l.chip].ID})
	}
}

// noTitleBar is a title bar of no height, for a window with none: the
// launcher, which Escape closes and its edge moves.
type noTitleBar struct{}

// NoTitleBar returns a title bar that takes no room and shows nothing.
func NoTitleBar() gunim.TitleBar { return noTitleBar{} }

// Layout implements [gunim.Node].
func (noTitleBar) Layout(c gunim.Constraints, _ gunim.Frame, _ gunim.Children) geom.Size {
	return c.Constrain(geom.Sz(c.Max.W, 0))
}

// Paint implements [gunim.Node].
func (noTitleBar) Paint(*paint.Painter, gunim.Frame, geom.Size, gunim.Children) {}

// SetTitle implements [gunim.TitleBar].
func (noTitleBar) SetTitle(string) {}
