package view

import (
	"fmt"
	"image/color"
	"slices"
	"strings"
	"time"

	"github.com/marrasen/kakel/app"

	"github.com/marrasen/kakel/look"

	"github.com/marrasen/gunim"
	"github.com/marrasen/gunim/access"
	"github.com/marrasen/gunim/anim"
	"github.com/marrasen/gunim/geom"
	"github.com/marrasen/gunim/icon"
	gi "github.com/marrasen/gunim/input"
	"github.com/marrasen/gunim/paint"
	"github.com/marrasen/gunim/theme"
	"github.com/marrasen/gunim/widget"

	"github.com/marrasen/kakel/settings"
	"github.com/marrasen/kakel/words"
)

// jobsPane shows the file jobs as cards, newest at the bottom: each
// with its progress, what it is on, and a button to cancel it. The
// finished ones stay, saying how they ended, until they are cleared.
type jobsPane struct {
	w     *Window
	head  *widget.Label
	clear *widget.Button
	empty *widget.Label
	list  *widget.List
	body  *widget.Scroll
	// none says there are no jobs, and the empty line shows.
	none bool
}

// Handle implements [gunim.Handler]: the keyboard coming into the pane
// makes it the one in front.
func (p *jobsPane) Handle(e gi.Event, u *gunim.UI) bool {
	if _, ok := e.(gi.FocusEntered); ok && p.w != nil {
		p.w.entered(p.w.paneOfKind(app.KindJobs), u)
	}
	return false
}

func newJobsPane() *jobsPane {
	p := &jobsPane{
		head:  widget.NewLabel("Jobs"),
		clear: iconButton(icon.ListX, "Clear Finished"),
		empty: widget.NewLabel("Copies, moves and deletes show here as they run."),
		list:  widget.NewList(),
		none:  true,
	}
	p.head.Size = widget.DialogTitleSize
	p.empty.Color = look.Faint
	p.clear.OnClick = widget.Sends(app.ClearJobs{})
	p.body = widget.NewScroll(widget.NewPad(p.list))
	return p
}

// show brings the cards up to date with jobs.
func (p *jobsPane) show(jobs []app.Job, u *gunim.UI) {
	widget.Sync(p.list, u, jobs,
		func(j app.Job) widget.Key { return widget.Key(j.ID) },
		func(j app.Job) *jobCard { return newJobCard(j) },
		func(c *jobCard, j app.Job, u *gunim.UI) { c.show(j, u) })
	p.none = len(jobs) == 0
}

// Children implements [gunim.Composite].
func (p *jobsPane) Children() []gunim.Node {
	return []gunim.Node{p.head, p.clear, p.empty, p.body}
}

// Layout implements [gunim.Node]: the heading and Clear Finished along
// the top, the cards under them.
func (p *jobsPane) Layout(c gunim.Constraints, _ gunim.Frame, kids gunim.Children) geom.Size {
	const pad, top = 20, 56
	head, clear, empty, body := kids.At(0), kids.At(1), kids.At(2), kids.At(3)
	cs := clear.Layout(gunim.Constraints{Max: geom.Sz(c.Max.W, top)})
	clear.Place(geom.Pt(c.Max.W-pad-cs.W, (top-cs.H)/2))
	hs := head.Layout(gunim.Constraints{Max: geom.Sz(max(0, c.Max.W-3*pad-cs.W), top)})
	head.Place(geom.Pt(pad, (top-hs.H)/2))
	empty.Layout(gunim.Constraints{Max: geom.Sz(max(0, c.Max.W-2*pad), top)})
	empty.Place(geom.Pt(pad, top+8))
	body.Layout(gunim.Tight(geom.Sz(c.Max.W, max(0, c.Max.H-top))))
	body.Place(geom.Pt(0, top))
	return c.Max
}

// Paint implements [gunim.Node].
func (p *jobsPane) Paint(pt *paint.Painter, f gunim.Frame, box geom.Size, kids gunim.Children) {
	pt.RRect(geom.Rect{Max: box.Point()}, 0, paint.Solid(widget.Background.Get(f.Theme)))
	kids.At(0).Paint(pt)
	kids.At(1).Paint(pt)
	if p.none {
		kids.At(2).Paint(pt)
	}
	kids.At(3).Paint(pt)
}

// jobCard is one job. Along its top, a badge with the kind of job in
// the colour of how it goes, what it does to what, a line on how far
// it has got or how it ended, and what can be done with it. Under them,
// while it runs, its progress and a graph of its speed; the folder it
// works from and the one it works to, each on its machine; and the
// items it works on, each waiting, going or done.
type jobCard struct {
	badge   *jobBadge
	title   *widget.Label
	summary *widget.Label
	bar     *widget.ProgressBar
	graph   *widget.LiveGraph
	// sampled is how many of the job's speed samples the graph has.
	sampled int
	route   *jobRoute
	items   *widget.Wrap
	chips   []*itemChip
	// running says the job runs, so the bar and the graph show.
	running bool
	// saved says the copy is on the saved list.
	saved  bool
	cancel *widget.Button
	save   *widget.Button
	repeat *widget.Button
	drop   *widget.IconButton
	// acts are the controls showing now, at the end of the top row.
	acts []gunim.Node
}

// jobCardKids is how many children a card has before its controls.
const jobCardKids = 7

// mostItems is how many of a job's items its card names.
const mostItems = 12

func newJobCard(j app.Job) *jobCard {
	c := &jobCard{
		badge: newJobBadge(), title: widget.NewLabel(""), summary: widget.NewLabel(""),
		bar: widget.NewProgressBar(), graph: widget.NewLiveGraph(app.SampleEvery, app.MostSpeeds),
		route:  newJobRoute(),
		cancel: iconButton(icon.CircleStop, "Cancel"), save: iconButton(icon.Bookmark, "Save"),
		repeat: iconButton(icon.Repeat, "Repeat"), drop: widget.NewIconButton(icon.X, "Take off the list"),
	}
	c.title.Face, c.title.MaxLines = widget.BoldFont, 1
	c.summary.Size, c.summary.MaxLines = smallText, 2
	c.graph.Label = func(v float64) string { return words.Size(int64(v)) + "/s" }
	c.cancel.Ghost, c.save.Ghost = true, true
	c.save.Tooltip = "Keep this copy on the Saved Copies list, to do again"
	id := j.ID
	c.cancel.OnClick, c.repeat.OnClick = widget.Sends(app.CancelJob{ID: id}), widget.Sends(app.RepeatJob{ID: id})
	c.drop.OnClick = widget.Sends(app.DropJob{ID: id})
	c.save.OnClick = func(*gunim.UI) gunim.Intent { return app.SaveCopy{ID: id, On: !c.saved} }
	var chips []gunim.Node
	for i, name := range j.Names {
		if i == mostItems-1 && len(j.Names) > mostItems {
			more := newItemChip(fmt.Sprintf("and %d more", len(j.Names)-i))
			more.plain = true
			chips = append(chips, more)
			break
		}
		chip := newItemChip(name)
		c.chips = append(c.chips, chip)
		chips = append(chips, chip)
	}
	c.items = widget.NewWrap(chips...)
	c.items.Gap = itemGap
	c.acts = c.want(j)
	c.fill(j, nil)
	return c
}

// itemGap is the space between the items on a job's card.
var itemGap = theme.Length("kakel.jobs.chip.gap", 6)

// want are the controls a job's card offers.
func (c *jobCard) want(j app.Job) []gunim.Node {
	switch {
	case !j.Done:
		return []gunim.Node{c.cancel}
	case j.Repeatable:
		return []gunim.Node{c.save, c.repeat, c.drop}
	}
	return []gunim.Node{c.drop}
}

// fill says j on the card. The bar moves to its share, or jumps there
// with u nil, for a card new to the pane.
func (c *jobCard) fill(j app.Job, u *gunim.UI) {
	c.running = !j.Done
	c.bar.Indeterminate = j.Share < 0 && !j.Done
	if j.Share >= 0 {
		c.bar.SetValue(j.Share, u)
	}
	c.badge.show(j)
	c.title.Text = jobHeading(j)
	c.summary.Text = j.Detail
	if when := jobWhen(j, time.Now()); when != "" {
		c.summary.Text = strings.TrimPrefix(c.summary.Text+" · "+when, " · ")
	}
	c.summary.Color = look.Faint
	if j.Failed {
		c.summary.Color = widget.DialogProblem
	}
	// The samples taken since the card last looked, into the graph,
	// which slides on every frame while the job runs.
	fresh := min(j.Sampled-c.sampled, len(j.Speeds))
	for _, s := range j.Speeds[len(j.Speeds)-max(fresh, 0):] {
		c.graph.Add(float64(s))
	}
	c.sampled = j.Sampled
	c.graph.SetRunning(!j.Done)
	c.saved = j.Saved
	c.save.Active = j.Saved
	c.save.Label, c.save.Icon = "Save", icon.Bookmark
	if j.Saved {
		c.save.Label, c.save.Icon = "Saved", icon.BookmarkCheck
	}
	c.route.show(j.From, j.To)
	for i, chip := range c.chips {
		switch {
		case j.Current != "" && chip.name.Text == j.Current:
			chip.show(itemGoing)
		case j.Done && !j.Failed && !j.Stopped, i < j.Ticked:
			chip.show(itemDone)
		default:
			chip.show(itemWaiting)
		}
	}
}

// jobHeading says what a job does, or did, to what: "Copying 6 items",
// "Moved report.pdf", "Stopped deleting 2 items".
func jobHeading(j app.Job) string {
	what := words.Count(len(j.Names), "item")
	if len(j.Names) == 1 {
		what = j.Names[0]
	}
	verb := map[string][3]string{
		"copy":   {"copy", "copying", "copied"},
		"move":   {"move", "moving", "moved"},
		"delete": {"delete", "deleting", "deleted"},
	}[j.Kind]
	if verb[0] == "" {
		verb = [3]string{"copy", "copying", "copied"}
	}
	switch {
	case !j.Done:
		return upFirst(verb[1]) + " " + what
	case j.Failed:
		return "Couldn’t " + verb[0] + " " + what
	case j.Stopped:
		return "Stopped " + verb[1] + " " + what
	}
	return upFirst(verb[2]) + " " + what
}

// upFirst is s with its first letter a capital.
func upFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// jobWhen says when a job ran, as of now: when it started while it
// runs, and once it has stopped, when it started and how long it took.
func jobWhen(j app.Job, now time.Time) string {
	if j.Started.IsZero() {
		return ""
	}
	at := clockAt(j.Started, now)
	if j.Finished.IsZero() {
		return "started " + at
	}
	return at + ", took " + took(j.Finished.Sub(j.Started))
}

// clockAt is t as a clock says it, with the day where that is not today.
func clockAt(t, now time.Time) string {
	day := func(t time.Time) time.Time { y, m, d := t.Date(); return time.Date(y, m, d, 0, 0, 0, 0, t.Location()) }
	switch day(now).Sub(day(t)) {
	case 0:
		return t.Format("15:04")
	case 24 * time.Hour:
		return "yesterday " + t.Format("15:04")
	}
	return t.Format("2 Jan 15:04")
}

// took says how long d is, to the second, or to the minute past an hour.
func took(d time.Duration) string {
	switch {
	case d < time.Second:
		return "under a second"
	case d < time.Minute:
		return fmt.Sprintf("%d s", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%d min %d s", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%d h %d min", int(d.Hours()), int(d.Minutes())%60)
}

// show brings the card up to date with j.
func (c *jobCard) show(j app.Job, u *gunim.UI) {
	c.fill(j, u)
	want := c.want(j)
	for _, n := range c.acts {
		if !slices.Contains(want, n) {
			u.Remove(n)
		}
	}
	for _, n := range want {
		if !slices.Contains(c.acts, n) {
			u.Insert(c, n)
		}
	}
	c.acts = want
}

// Children implements [gunim.Composite].
func (c *jobCard) Children() []gunim.Node {
	return append([]gunim.Node{c.badge, c.title, c.summary, c.bar, c.graph, c.route, c.items}, c.acts...)
}

// Layout implements [gunim.Node]: the badge, the words beside it and
// the controls at the end make the top row, and the rest lines up
// under the words.
func (c *jobCard) Layout(cs gunim.Constraints, _ gunim.Frame, kids gunim.Children) geom.Size {
	const pad, gap, badgeGap = 16, 12, 14
	w := cs.Max.W
	badge, title, summary, bar, graph, route, items := kids.At(0), kids.At(1), kids.At(2), kids.At(3), kids.At(4), kids.At(5), kids.At(6)
	bs := badge.Layout(gunim.Constraints{})
	left := float32(pad + bs.W + badgeGap)
	inner := max(0, w-left-pad)
	// The controls, from the end of the top row.
	byNode := map[gunim.Node]gunim.Child{}
	for i := jobCardKids; i < kids.Len(); i++ {
		byNode[kids.At(i).Node()] = kids.At(i)
	}
	right, row := w-pad, float32(0)
	type placed struct {
		k gunim.Child
		s geom.Size
	}
	var acts []placed
	for i := len(c.acts) - 1; i >= 0; i-- {
		if k, ok := byNode[c.acts[i]]; ok {
			s := k.Layout(gunim.Constraints{Max: geom.Sz(w, 40)})
			right -= s.W
			acts = append(acts, placed{k, s})
			k.Place(geom.Pt(right, 0))
			right -= 4
			row = max(row, s.H)
		}
	}
	words := max(0, right-left-gap)
	ts := title.Layout(gunim.Constraints{Max: geom.Sz(words, 40)})
	ss := summary.Layout(gunim.Constraints{Max: geom.Sz(words, 80)})
	text := ts.H + 3 + ss.H
	top := max(bs.H, text, row)
	badge.Place(geom.Pt(pad, pad+(top-bs.H)/2))
	title.Place(geom.Pt(left, pad+(top-text)/2))
	summary.Place(geom.Pt(left, pad+(top-text)/2+ts.H+3))
	x := right + 4
	for i := len(acts) - 1; i >= 0; i-- {
		a := acts[i]
		a.k.Place(geom.Pt(x, pad+(top-a.s.H)/2))
		x += a.s.W + 4
	}
	y := pad + top + gap
	if c.running {
		s := bar.Layout(gunim.Constraints{Max: geom.Sz(inner, 20)})
		bar.Place(geom.Pt(left, y))
		y += s.H + 8
		if gs := graph.Layout(gunim.Constraints{Max: geom.Sz(inner, 56)}); gs.H > 0 {
			graph.Place(geom.Pt(left, y))
			y += gs.H + gap
		}
	}
	if rs := route.Layout(gunim.Constraints{Max: geom.Sz(inner, 0)}); rs.H > 0 {
		route.Place(geom.Pt(left, y))
		y += rs.H + gap
	}
	if len(c.chips) > 0 {
		is := items.Layout(gunim.Constraints{Max: geom.Sz(inner, 0)})
		items.Place(geom.Pt(left, y))
		y += is.H + gap
	}
	return cs.Constrain(geom.Sz(w, y-gap+pad))
}

// Paint implements [gunim.Node].
func (c *jobCard) Paint(p *paint.Painter, f gunim.Frame, box geom.Size, kids gunim.Children) {
	p.RRect(geom.Rect{Max: box.Point()}, widget.CardRadius.Get(f.Theme), paint.Solid(widget.CardFill.Get(f.Theme)))
	for i := 0; i < kids.Len(); i++ {
		switch {
		case (i == 3 || i == 4) && !c.running, i == 6 && len(c.chips) == 0:
			continue
		}
		kids.At(i).Paint(p)
	}
}

// jobBadge is the kind of job, as an icon on a square of the colour of
// how it goes: the accent while it runs, then green, red when it
// failed, or faint when it was stopped.
type jobBadge struct {
	icon *widget.Icon
	tint theme.Token[color.NRGBA]
}

func newJobBadge() *jobBadge {
	return &jobBadge{icon: widget.NewIcon(icon.Copy, ""), tint: widget.Accent}
}

// show says j on the badge.
func (b *jobBadge) show(j app.Job) {
	switch j.Kind {
	case "move":
		b.icon.Icon = icon.FolderInput
	case "delete":
		b.icon.Icon = icon.Trash2
	default:
		b.icon.Icon = icon.Copy
	}
	switch {
	case !j.Done:
		b.tint = widget.Accent
	case j.Failed:
		b.tint = widget.ToastErrorInk
	case j.Stopped:
		b.tint = look.Faint
	default:
		b.tint = widget.ToastSuccessInk
	}
	b.icon.Color = b.tint
}

// Children implements [gunim.Composite].
func (b *jobBadge) Children() []gunim.Node { return []gunim.Node{b.icon} }

// Layout implements [gunim.Node].
func (b *jobBadge) Layout(_ gunim.Constraints, _ gunim.Frame, kids gunim.Children) geom.Size {
	const side = 38
	s := kids.At(0).Layout(gunim.Constraints{Max: geom.Sz(side, side)})
	kids.At(0).Place(geom.Pt((side-s.W)/2, (side-s.H)/2))
	return geom.Sz(side, side)
}

// Paint implements [gunim.Node].
func (b *jobBadge) Paint(p *paint.Painter, f gunim.Frame, box geom.Size, kids gunim.Children) {
	fill := b.tint.Get(f.Theme)
	fill.A = 0x2a
	p.RRect(geom.Rect{Max: box.Point()}, 10, paint.Solid(fill))
	kids.At(0).Paint(p)
}

// jobRoute is where a job works: the folder it works from, an arrow
// and the folder it works to, side by side where there is room and one
// over the other where there is not. A delete has the one folder.
type jobRoute struct {
	from, to *jobEndBox
	arrow    *widget.Icon
	// one says there is only the folder the job works in.
	one bool
}

func newJobRoute() *jobRoute {
	r := &jobRoute{from: newJobEndBox(), to: newJobEndBox(), arrow: widget.NewIcon(icon.ArrowRight, "to")}
	r.arrow.Color = look.Faint
	return r
}

// show says the folders from and to.
func (r *jobRoute) show(from, to app.JobEnd) {
	r.from.show(from)
	r.to.show(to)
	r.one = to.Path == ""
}

// Children implements [gunim.Composite].
func (r *jobRoute) Children() []gunim.Node { return []gunim.Node{r.from, r.arrow, r.to} }

// Layout implements [gunim.Node].
func (r *jobRoute) Layout(cs gunim.Constraints, _ gunim.Frame, kids gunim.Children) geom.Size {
	const gap = 10
	w := cs.Max.W
	from, arrow, to := kids.At(0), kids.At(1), kids.At(2)
	if r.from.path.Text == "" {
		return geom.Size{}
	}
	if r.one {
		s := from.Layout(gunim.Tight(geom.Sz(w, 0)))
		from.Place(geom.Point{})
		return geom.Sz(w, s.H)
	}
	as := arrow.Layout(gunim.Constraints{})
	if w >= 420 {
		// Side by side, as tall as each other.
		bw := (w - as.W - 2*gap) / 2
		h := max(from.Layout(gunim.Constraints{Min: geom.Sz(bw, 0), Max: geom.Sz(bw, 0)}).H,
			to.Layout(gunim.Constraints{Min: geom.Sz(bw, 0), Max: geom.Sz(bw, 0)}).H)
		from.Layout(gunim.Tight(geom.Sz(bw, h)))
		to.Layout(gunim.Tight(geom.Sz(bw, h)))
		r.arrow.Icon = icon.ArrowRight
		from.Place(geom.Point{})
		arrow.Place(geom.Pt(bw+gap, (h-as.H)/2))
		to.Place(geom.Pt(bw+as.W+2*gap, 0))
		return geom.Sz(w, h)
	}
	// One over the other, the arrow pointing down between them.
	r.arrow.Icon = icon.ArrowDown
	fs := from.Layout(gunim.Tight(geom.Sz(w, 0)))
	from.Place(geom.Point{})
	arrow.Place(geom.Pt(14, fs.H+4))
	y := fs.H + 4 + as.H + 4
	ts := to.Layout(gunim.Tight(geom.Sz(w, 0)))
	to.Place(geom.Pt(0, y))
	return geom.Sz(w, y+ts.H)
}

// Paint implements [gunim.Node].
func (r *jobRoute) Paint(p *paint.Painter, _ gunim.Frame, _ geom.Size, kids gunim.Children) {
	kids.At(0).Paint(p)
	if !r.one {
		kids.At(1).Paint(p)
		kids.At(2).Paint(p)
	}
}

// jobEndBox is a folder a job works in: the machine it is on, its name
// and its whole path. A click, or Enter, opens the folder in a file
// manager tab; under the pointer the box lights and shows an open
// folder at its corner.
type jobEndBox struct {
	anim.Group
	machine    *widget.Icon
	on         *widget.Label
	name, path *widget.Label
	opens      *widget.Icon
	// open is what a click sends, and nil for a box with no folder.
	open gunim.Intent
	// hover and ring run from 0 to 1 as the pointer comes over the box
	// and as it takes the keyboard.
	hover, ring *anim.Float
	click       widget.Clicker
	tip         widget.PartTip
	size        geom.Size
}

func newJobEndBox() *jobEndBox {
	b := &jobEndBox{machine: widget.NewIcon(icon.Monitor, ""), on: widget.NewLabel(""), name: widget.NewLabel(""), path: widget.NewLabel(""),
		opens: widget.NewIcon(icon.FolderOpen, ""), hover: anim.NewFloat(0), ring: anim.NewFloat(0)}
	b.Add(b.hover)
	b.Add(b.ring)
	b.machine.Color, b.machine.Size = look.Faint, endIconSize
	b.opens.Color, b.opens.Size = look.Faint, endIconSize
	b.on.Size, b.on.Color, b.on.MaxLines = smallText, look.Faint, 1
	b.name.MaxLines = 1
	b.path.Size, b.path.Color, b.path.MaxLines = smallText, look.Faint, 2
	return b
}

// endIconSize is the size of the machine's icon over a folder.
var endIconSize = theme.Length("kakel.jobs.machine.icon", 14)

// show says e in the box.
func (b *jobEndBox) show(e app.JobEnd) {
	b.machine.Icon = icon.Server
	if e.Local {
		b.machine.Icon = icon.Monitor
	}
	b.on.Text, b.name.Text, b.path.Text = e.Machine, e.Folder, e.Path
	b.open = nil
	if e.Dir != "" {
		b.open = app.OpenFilesOn{Machine: e.On, Path: e.Dir, NewTab: true}
	}
}

// Children implements [gunim.Composite].
func (b *jobEndBox) Children() []gunim.Node {
	return []gunim.Node{b.machine, b.on, b.name, b.path, b.opens}
}

// Handle implements [gunim.Handler].
func (b *jobEndBox) Handle(e gi.Event, u *gunim.UI) bool {
	if b.open == nil {
		return false
	}
	b.tip.Handle(e, u, b, "Open "+b.name.Text+" in Files")
	switch e := e.(type) {
	case gi.PointerEnter:
		b.hover.Animate(1, widget.Quick.Get(u.Theme()))
	case gi.PointerLeave:
		b.hover.Animate(0, widget.Settle.Get(u.Theme()))
	case gi.PointerDown:
		if e.Button != gi.ButtonPrimary {
			return false
		}
		b.click.Press(e, 0)
		return true
	case gi.PointerUp:
		inside := -1
		if (geom.Rect{Max: b.size.Point()}).Contains(e.Pos) {
			inside = 0
		}
		if b.click.Release(e, inside) {
			b.fire(u)
		}
		return true
	case gi.KeyPress:
		if e.Mods != 0 || e.Key != gi.KeyEnter && e.Key != gi.KeyKPEnter && e.Key != gi.KeySpace {
			return false
		}
		b.fire(u)
		return true
	case gi.FocusRing:
		to := float32(0)
		if e.On {
			to = 1
		}
		b.ring.Animate(to, widget.RingFade.Get(u.Theme()))
	case gi.FocusLost:
		b.ring.Animate(0, widget.RingFade.Get(u.Theme()))
	}
	return false
}

// fire opens the folder.
func (b *jobEndBox) fire(u *gunim.UI) {
	b.tip.Hide(u)
	u.Cue(gunim.CuePress, b)
	u.Send(b, b.open)
}

// Focusable implements [gunim.Focusable]: Tab comes to a box with a
// folder to open.
func (b *jobEndBox) Focusable() bool { return b.open != nil }

// FocusOnPress implements [gunim.PressFocuser]: a click opens the
// folder and leaves the keyboard where it was.
func (b *jobEndBox) FocusOnPress() bool { return false }

// Cursor implements [gunim.CursorShaper].
func (b *jobEndBox) Cursor(geom.Point) gi.Cursor {
	if b.open == nil {
		return gi.CursorArrow
	}
	return gi.CursorHand
}

// Access implements [gunim.Accessible].
func (b *jobEndBox) Access() access.Info {
	return access.Info{Role: access.RoleButton, Name: "Open " + b.name.Text + " on " + b.on.Text, Actions: []string{access.ActionPress}}
}

// AccessAct implements [gunim.AccessActor].
func (b *jobEndBox) AccessAct(r access.Request, u *gunim.UI) bool {
	if r.Action != access.ActionPress || b.open == nil {
		return false
	}
	b.fire(u)
	return true
}

// Layout implements [gunim.Node]: the machine along the top, the
// folder's name under it, and its path under that unless the name says
// it all. The box is as tall as it is asked to be, or as its lines.
func (b *jobEndBox) Layout(cs gunim.Constraints, _ gunim.Frame, kids gunim.Children) geom.Size {
	const padX, padY = 12, 10
	w := cs.Max.W
	inner := max(0, w-2*padX)
	machine, on, name, path, opens := kids.At(0), kids.At(1), kids.At(2), kids.At(3), kids.At(4)
	ms := machine.Layout(gunim.Constraints{})
	ops := opens.Layout(gunim.Constraints{})
	opens.Place(geom.Pt(w-padX-ops.W, padY))
	os := on.Layout(gunim.Constraints{Max: geom.Sz(max(0, inner-ms.W-6-ops.W-6), 0)})
	line := max(ms.H, os.H)
	machine.Place(geom.Pt(padX, padY+(line-ms.H)/2))
	on.Place(geom.Pt(padX+ms.W+6, padY+(line-os.H)/2))
	y := padY + line + 6
	ns := name.Layout(gunim.Constraints{Max: geom.Sz(inner, 0)})
	name.Place(geom.Pt(padX, y))
	y += ns.H
	if b.path.Text != b.name.Text {
		ps := path.Layout(gunim.Constraints{Max: geom.Sz(inner, 0)})
		path.Place(geom.Pt(padX, y+2))
		y += 2 + ps.H
	}
	b.size = geom.Sz(w, max(y+padY, cs.Min.H))
	return b.size
}

// Paint implements [gunim.Node].
func (b *jobEndBox) Paint(p *paint.Painter, f gunim.Frame, box geom.Size, kids gunim.Children) {
	b.size = box
	fill := faintFill(f)
	lit := min(max(b.hover.Value(), 0), 1)
	fill.A += uint8(float32(0x12) * lit)
	r := geom.Rect{Max: box.Point()}
	p.RRect(r, 8, paint.Solid(fill))
	widget.FocusRing(p, r, 8, b.ring.Value(), f.Theme)
	for i := 0; i < kids.Len(); i++ {
		switch {
		case i == 3 && b.path.Text == b.name.Text, i == 4 && lit < 0.5:
			continue
		}
		kids.At(i).Paint(p)
	}
}

// faintFill is the ink, barely there, to set something apart on a card
// in a dark theme or a light one.
func faintFill(f gunim.Frame) color.NRGBA {
	c := widget.Ink.Get(f.Theme)
	c.A = 0x0d
	return c
}

// itemState is how far a job has got with one of its items.
type itemState int

const (
	itemWaiting itemState = iota
	itemGoing
	itemDone
)

// itemChip is an item a job works on, marked by how far the job has
// got with it. A plain one is only words, such as "and 3 more".
type itemChip struct {
	mark  *widget.Icon
	name  *widget.Label
	plain bool
}

func newItemChip(name string) *itemChip {
	c := &itemChip{mark: widget.NewIcon(icon.CircleDashed, ""), name: widget.NewLabel(name)}
	c.mark.Size = endIconSize
	c.name.Size, c.name.MaxLines = smallText, 1
	c.show(itemWaiting)
	return c
}

// show marks the chip s.
func (c *itemChip) show(s itemState) {
	c.mark.Spin = false
	switch s {
	case itemGoing:
		c.mark.Icon, c.mark.Color, c.mark.Spin = icon.LoaderCircle, widget.Accent, true
	case itemDone:
		c.mark.Icon, c.mark.Color = icon.CircleCheck, widget.ToastSuccessInk
	default:
		c.mark.Icon, c.mark.Color = icon.CircleDashed, look.Faint
	}
	c.name.Color = widget.Ink
	if s == itemWaiting {
		c.name.Color = look.Faint
	}
}

// Children implements [gunim.Composite].
func (c *itemChip) Children() []gunim.Node { return []gunim.Node{c.mark, c.name} }

// Layout implements [gunim.Node].
func (c *itemChip) Layout(cs gunim.Constraints, _ gunim.Frame, kids gunim.Children) geom.Size {
	const padX, h = 8, 24
	x := float32(padX)
	if !c.plain {
		ms := kids.At(0).Layout(gunim.Constraints{})
		kids.At(0).Place(geom.Pt(x, (h-ms.H)/2))
		x += ms.W + 6
	}
	ns := kids.At(1).Layout(gunim.Constraints{Max: geom.Sz(max(0, cs.Max.W-x-padX), 0)})
	kids.At(1).Place(geom.Pt(x, (h-ns.H)/2))
	return geom.Sz(x+ns.W+padX, h)
}

// Paint implements [gunim.Node].
func (c *itemChip) Paint(p *paint.Painter, f gunim.Frame, box geom.Size, kids gunim.Children) {
	if c.plain {
		kids.At(1).Paint(p)
		return
	}
	p.RRect(geom.Rect{Max: box.Point()}, box.H/2, paint.Solid(faintFill(f)))
	kids.At(0).Paint(p)
	kids.At(1).Paint(p)
}

// copiesPane lists the copies kept, to do again: Enter does the one
// the cursor is on, and Delete forgets it.
type copiesPane struct {
	w     *Window
	bar   *buttonBar
	table *widget.Table
	col   *widget.Flex
	kept  map[widget.Key]settings.SavedCopy
}

func newCopiesPane(w *Window) *copiesPane {
	p := &copiesPane{w: w, bar: newButtonBar(), kept: map[widget.Key]settings.SavedCopy{}}
	p.table = widget.NewTable(widget.TableColumn{Title: "What"}, widget.TableColumn{Title: "From → To", Width: 380})
	p.table.Row = func(k widget.Key) widget.TableRow {
		c := p.kept[k]
		return widget.TableRow{Cells: []string{app.CopiedWhat(c), app.CopiedWhere(c, p.w.nameOf)}}
	}
	p.table.OnActivate = func(k widget.Key, u *gunim.UI) gunim.Intent { return app.RunSavedCopy{Saved: p.kept[k]} }
	p.col = widget.Column(p.table, p.bar).Grow(p.table, 1)
	p.col.Cross, p.col.Gap = widget.CrossStretch, noGap
	return p
}

// show brings the list up to date.
func (p *copiesPane) show(saved []settings.SavedCopy, u *gunim.UI) {
	clear(p.kept)
	keys := make([]widget.Key, 0, len(saved))
	for i, c := range saved {
		k := widget.Key(fmt.Sprintf("%03d", i))
		p.kept[k] = c
		keys = append(keys, k)
	}
	p.table.SetKeys(keys, u)
	if len(saved) == 0 {
		p.bar.set("None saved. A finished copy has a Save this copy box.", u)
		return
	}
	p.bar.set(words.Count(len(saved), "copy")+" saved · Enter does the one selected again, Delete removes it", u)
}

// Children implements [gunim.Composite].
func (p *copiesPane) Children() []gunim.Node { return []gunim.Node{p.col} }

// Layout implements [gunim.Node].
func (p *copiesPane) Layout(c gunim.Constraints, _ gunim.Frame, kids gunim.Children) geom.Size {
	k := kids.At(0)
	k.Layout(gunim.Tight(c.Max))
	k.Place(geom.Point{})
	return c.Max
}

// Paint implements [gunim.Node].
func (p *copiesPane) Paint(pt *paint.Painter, f gunim.Frame, box geom.Size, kids gunim.Children) {
	pt.RRect(geom.Rect{Max: box.Point()}, 0, paint.Solid(widget.Background.Get(f.Theme)))
	kids.At(0).Paint(pt)
}

// Handle implements [gunim.Handler]: Delete forgets the copy the
// cursor is on.
func (p *copiesPane) Handle(e gi.Event, u *gunim.UI) bool {
	if _, ok := e.(gi.FocusEntered); ok {
		p.w.entered(p.w.paneOfKind(app.KindCopies), u)
		return false
	}
	if k, ok := e.(gi.KeyPress); ok && k.Key == gi.KeyDelete && k.Mods == 0 {
		if at, ok := p.table.Cursor(); ok {
			u.Send(p.table, app.ForgetCopy{Saved: p.kept[at]})
			return true
		}
	}
	return false
}

// iconButton returns a button with ic before its label.
func iconButton(ic *icon.Icon, label string) *widget.Button {
	b := widget.NewButton(label)
	b.Icon = ic
	return b
}
