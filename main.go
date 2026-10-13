// Command kakel is a terminal emulator, drawn with gunim.
//
// It runs shells here and on other machines through its own sessions,
// VT parser and key encoder, and draws their screens with gunim's
// CellGrid, beside the file manager, tunnels and the rest of the window.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime/pprof"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/marrasen/kakel/view"

	"github.com/marrasen/kakel/app"
	"github.com/marrasen/kakel/conf"
	"github.com/marrasen/kakel/internal/sound"
	"github.com/marrasen/kakel/relay"

	"github.com/marrasen/kakel/look"

	"github.com/marrasen/kakel/screen"

	"github.com/marrasen/gunim"
	"github.com/marrasen/gunim/driver"
	"github.com/marrasen/gunim/filemanager"
	"github.com/marrasen/gunim/geom"
	"github.com/marrasen/gunim/install"
	"github.com/marrasen/gunim/theme"
	"github.com/marrasen/gunim/widget"

	"github.com/marrasen/gunim/single"
	"github.com/marrasen/kakel/appicon"
	"github.com/marrasen/kakel/settings"
)

func main() {
	// A relay has no window and is never installed: it runs on a server
	// with no screen, from wherever it was put.
	if relay.Asked(os.Args[1:]) {
		if err := runRelay(); err != nil {
			log.Fatal(err)
		}
		return
	}
	// Nor has a pipe to a window through a relay, which ssh starts as
	// its ProxyCommand.
	if relay.PipeAsked(os.Args[1:]) {
		if err := relay.Pipe(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
			log.Fatal(err)
		}
		return
	}
	// A kakel started from anywhere but its own folder opens gunim's
	// installer, and `kakel -install` and `kakel -uninstall` are done
	// there; the installed one goes on.
	install.Run(app.Installer())
	err := run()
	// A restart into a new copy, as after an update: started once this
	// one has stopped listening, so it runs as the one.
	if exe := app.RestartInto(); exe != "" {
		if serr := exec.Command(exe).Start(); serr != nil {
			log.Printf("couldn't start %s again: %v", exe, serr)
		}
	}
	if err != nil {
		log.Fatal(err)
	}
}

// runRelay runs kakel as a relay until it is interrupted or stopped.
func runRelay() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	dir, err := conf.Private()
	if err != nil {
		return err
	}
	return relay.Main(ctx, os.Args[1:], dir, os.Stderr)
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	opts, err := app.ParseOptions(os.Args[1:])
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	if alone, err := app.RunAlone(ctx, opts); alone {
		return err
	}
	if path := os.Getenv("KAKEL_PROFILE"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		if err := pprof.StartCPUProfile(f); err != nil {
			return err
		}
		defer pprof.StopCPUProfile()
	}

	// From here on what kakel logs is kept for the Window Log too, so
	// the lines of starting are there as well.
	app.CaptureLog()

	// One kakel for the user: one already running is handed the
	// command line, and opens a window for it.
	var handovers <-chan single.Handover
	if opts.OneOfMany() {
		if dir, err := settings.Dir(); err == nil {
			cwd, _ := os.Getwd()
			if taken, err := single.Hand(dir, single.Handover{Args: os.Args[1:], Dir: cwd}); taken {
				return nil
			} else if err != nil {
				log.Print(err)
			}
			var stop func()
			if handovers, stop, err = single.Listen(ctx, dir); err != nil {
				log.Printf("kakel runs alone: %v", err)
			} else {
				// Gone before the process is, so the next kakel does not
				// find it.
				defer stop()
			}
		}
	}
	if opts.Quits() {
		// No kakel was running to end.
		return nil
	}

	err = gunim.Main(ctx, func(a *gunim.App) error {
		sh := screen.NewShells()
		all, trouble := look.LoadSaying()
		ws := &ownWindows{app: a, sh: sh, all: all, place: opts.WindowPlace, systemFrame: opts.SystemTitleBar}
		// Sounds play through the app, as its settings say; the speakers
		// open only once one is wanted.
		sounds := sound.New()
		a.SetCues(sounds)
		// Where it was as it last closed, or else sized for the font.
		w, c, err := ws.open(gunim.WindowOptions{Size: opts.WindowSize(), Place: opts.WindowPlace(), Hidden: opts.StartsHidden()})
		if err != nil {
			return err
		}
		if opts.ShowStats() {
			go ws.logStats(ctx)
		}
		go stressWindows(ctx, a)
		return app.Start(ctx, app.Config{
			Client: c, Window: w, Shells: sh, OpenWindow: ws.openFrom, WindowClosed: ws.closed, Options: opts,
			Themes: all, ThemeTrouble: trouble, RegisterThemes: ws.registerThemes,
			Tray: app.Tray{Set: a.SetTray, StayOpen: a.StayOpen, Notify: a.TrayNotify}, Handovers: handovers,
			OpenLauncher: ws.openLauncher, HotKeys: a.RegisterHotKey, OpenPrompt: ws.openPrompt, Sound: sounds.Set,
			OpenOverview: ws.openOverview, Monitors: a.Monitors,
			// The file manager's windows, gunim's own, outside kakel's
			// tabs; they end with the program.
			Files: filemanager.NewHub(ctx, a),
			// gunim's update windows, in the installer's look.
			Updates: updateWindows{ctx: ctx, app: a},
		})
	})
	if errors.Is(err, driver.ErrNoDriver) {
		log.Print("gunim has no driver for this operating system")
		return nil
	}
	return err
}

// updateWindows opens gunim's windows of an update on app, in kakel's
// installer's look.
type updateWindows struct {
	ctx context.Context
	app *gunim.App
}

func (u updateWindows) ShowUpdate(up install.Update) error {
	return install.ShowUpdate(u.ctx, u.app, app.Installer(), up)
}

func (u updateWindows) ShowWhatsNew(from string) error {
	return install.ShowWhatsNew(u.ctx, u.app, app.Installer(), from)
}

func (u updateWindows) ShowAbout(o install.About) error {
	return install.ShowAbout(u.ctx, u.app, app.Installer(), o)
}

// ownWindows opens kakel's windows, each with the window's view
// mounted, and names the themes to every one of them.
type ownWindows struct {
	// systemFrame says whether a window opens with the system's title
	// bar and frame.
	systemFrame func() bool
	app         *gunim.App
	sh          *screen.Shells
	mu          sync.Mutex
	all         []look.Themed
	win         []*gunim.Window
	// place is where the last window was as it closed, for one opened
	// with none open.
	place func() *driver.Placement
}

// open opens a window with o's size and place.
func (ws *ownWindows) open(o gunim.WindowOptions) (*gunim.Window, gunim.Client, error) {
	o.Title = app.ProgramName
	// The system's title bar and frame, where the settings ask for
	// them, as they say as the window opens.
	if ws.systemFrame != nil {
		o.SystemFrame = ws.systemFrame()
	}
	o.Icons = appicon.Images()
	// The close button asks first, as Exit does for the last window.
	o.AskToClose = app.CloseWindow{}
	// Files drag from a file manager pane while another window is in
	// front, as from Explorer's.
	o.DragFromBehind = true
	w, err := ws.app.NewWindow(o)
	if err != nil {
		return nil, gunim.Client{}, fmt.Errorf("kakel: %w", err)
	}
	c := w.Client()
	ws.mu.Lock()
	all := ws.all
	ws.win = append(ws.win, w)
	ws.mu.Unlock()
	look.Register(w, all)
	// The file manager panes' views, which kakel's themes colour.
	filemanager.RegisterViews(w)
	// Each window its own keys, which the shortcuts file changes there.
	keys := view.Shortcuts()
	gunim.RegisterView(w, "window", func(app.State) *view.Window { return view.NewWindow(ws.sh, keys, all) },
		func(win *view.Window, st app.State, u *gunim.UI) { win.Update(st, u) })
	gunim.RegisterPatch(w, "window", func(win *view.Window, _ app.OutputArrived, u *gunim.UI) { win.OutputArrived(u) })
	if err := c.Mount(gunim.Root, "window", "window", app.State{}, app.WindowTopic); err != nil {
		return nil, gunim.Client{}, err
	}
	return w, c, nil
}

// openFrom opens a window size large, its top left corner at at in
// from's space, or at place on the screen.
func (ws *ownWindows) openFrom(from *gunim.Window, at geom.Point, size geom.Size, place *driver.Placement) (gunim.Client, *gunim.Window, error) {
	o := gunim.WindowOptions{Size: size, Parent: from, Anchor: at}
	switch {
	case place != nil:
		// gunim places only a window with no parent.
		o.Place, o.Parent = place, nil
	case from == nil && ws.place != nil:
		// With none open, as from the tray: where the last one was.
		o.Place = ws.place()
	}
	w, c, err := ws.open(o)
	return c, w, err
}

// openLauncher opens the launcher's window, over the others in the
// middle of the main display, with its view mounted.
func (ws *ownWindows) openLauncher() (gunim.Client, error) {
	o := gunim.WindowOptions{Title: app.ProgramName, Size: view.LauncherSize, Icons: appicon.Images(), Pinned: true, TitleBar: view.NoTitleBar()}
	if mons := ws.app.Monitors(); len(mons) > 0 {
		m := mons[0]
		for _, o := range mons {
			if o.Primary {
				m = o
			}
		}
		area := m.WorkArea
		if area.Empty() {
			area = m.Bounds
		}
		scale := m.CoordsPerLogical
		if scale <= 0 {
			scale = 1
		}
		w, h := view.LauncherSize.W*scale, view.LauncherSize.H*scale
		c := area.Center()
		// A third of the way down, as a search box sits.
		o.Place = &driver.Placement{Bounds: geom.Rc(c.X-w/2, area.Min.Y+(area.Size().H-h)/3, w, h)}
	}
	w, err := ws.app.NewWindow(o)
	if err != nil {
		return gunim.Client{}, fmt.Errorf("kakel: %w", err)
	}
	ws.mu.Lock()
	all := ws.all
	ws.mu.Unlock()
	look.Register(w, all)
	gunim.RegisterView(w, "launcher", func(app.LaunchState) *view.Launcher { return view.NewLauncher() },
		func(l *view.Launcher, st app.LaunchState, u *gunim.UI) { l.Update(st, u) })
	c := w.Client()
	if err := c.Mount(gunim.Root, "launcher", "launcher", app.LaunchState{}, app.LauncherTopic); err != nil {
		c.Close()
		return gunim.Client{}, err
	}
	return c, nil
}

// openOverview opens All Panes' own window over the whole of monitor m,
// above the other windows, with its view mounted. It draws no frame,
// and comes and goes at once: All Panes moves the windows' cards in and
// out itself, and darkens the screen behind them. It opens in theme,
// see-through: in the default theme its first frame was opaque, and the
// screen went dark at once before the shade could fade in.
func (ws *ownWindows) openOverview(m driver.Monitor, theme string) (gunim.Client, *gunim.Window, error) {
	ws.mu.Lock()
	all := ws.all
	ws.mu.Unlock()
	o := gunim.WindowOptions{
		Title: "All Panes", Size: m.Bounds.Size(), Icons: appicon.Images(), Pinned: true, Fixed: true, Instant: true,
		TitleBar: view.NoTitleBar(), Place: &driver.Placement{Bounds: m.Bounds},
		// No line round it: it is the screen, not a window on it.
		Border: driver.Border{None: true},
	}
	for _, t := range all {
		if t.Name == theme {
			o.Theme = view.SeeThrough(t.Theme)
		}
	}
	w, err := ws.app.NewWindow(o)
	if err != nil {
		return gunim.Client{}, nil, fmt.Errorf("kakel: %w", err)
	}
	// The themes, each with the background see-through, so the screen
	// shows behind the cards.
	for _, t := range all {
		w.RegisterTheme(view.SeeThrough(t.Theme))
	}
	gunim.RegisterView(w, "overview", func(app.OverState) *view.Overview { return view.NewOverview(ws.sh, view.Shortcuts()) },
		func(o *view.Overview, st app.OverState, u *gunim.UI) { o.Update(st, u) })
	gunim.RegisterPatch(w, "overview", func(o *view.Overview, _ app.OutputArrived, u *gunim.UI) { o.OutputArrived(u) })
	c := w.Client()
	if err := c.Mount(gunim.Root, "overview", "overview", app.OverState{}, app.OverviewTopic); err != nil {
		c.Close()
		return gunim.Client{}, nil, err
	}
	return c, w, nil
}

// openPrompt opens a window of its own for the question q, sized to it
// in the theme name, over the other windows, centred over the window the
// user last worked in, or near, or else on the main display, with the question's view mounted. Its title is
// the question's, for the taskbar and Alt+Tab to name it.
func (ws *ownWindows) openPrompt(q app.Ask, name string, near *gunim.Window) (gunim.Client, error) {
	ws.mu.Lock()
	all := ws.all
	ws.mu.Unlock()
	var th look.Themed
	for _, t := range all {
		if t.Name == name {
			th = t
		}
	}
	size := view.PromptSize(q, th.Theme)
	// A compact title bar names the question, closes it, and moves the
	// window out of the way of what the user needs to read.
	bar := widget.NewTitleBar(q.Title)
	// A title to move it by, and nothing to minimize, maximize or close
	// it by: Cancel and Escape do.
	bar.Compact, bar.NoMinimize, bar.NoMaximize, bar.NoClose = true, true, true, true
	size.H += widget.TitleBarCompactHeight.Get(theme.NewLive(th.Theme))
	// Over the window the user last worked in, kakel's or a file
	// manager's, or else kakel's window in front.
	var at *driver.Placement
	if r, ok := ws.app.FocusedBounds(); ok && onAMonitor(ws.app.Monitors(), r) {
		at = &driver.Placement{Bounds: r}
	} else if near != nil {
		if p, ok := near.Placement(); ok {
			at = &p
		}
	}
	w, err := ws.app.NewWindow(gunim.WindowOptions{
		Title: q.Title, Size: size, Icons: appicon.Images(), Pinned: true, Fixed: true, TitleBar: bar,
		Place: view.PromptPlace(ws.app.Monitors(), at, size),
	})
	if err != nil {
		return gunim.Client{}, fmt.Errorf("kakel: %w", err)
	}
	look.Register(w, all)
	gunim.RegisterView(w, "prompt", func(app.Ask) *view.Prompt { return view.NewPrompt() },
		func(p *view.Prompt, q app.Ask, u *gunim.UI) { p.Update(q, u) })
	c := w.Client()
	_ = c.SetTheme(name)
	if err := c.Mount(gunim.Root, "prompt", "prompt", q); err != nil {
		c.Close()
		return gunim.Client{}, err
	}
	return c, nil
}

// closed lets w go, closed: held here, it would keep all it showed, the
// terminals and their history too.
func (ws *ownWindows) closed(w *gunim.Window) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.win = slices.DeleteFunc(ws.win, func(o *gunim.Window) bool { return o == w })
}

// registerThemes names all to every window, and to those opened later.
func (ws *ownWindows) registerThemes(all []look.Themed) {
	ws.mu.Lock()
	ws.all = all
	list := slices.Clone(ws.win)
	ws.mu.Unlock()
	for _, w := range list {
		look.Register(w, all)
	}
}

// logStats prints, each second, how many frames each window drew and
// how many screen updates arrived and were merged into them: every
// window kakel opened, numbered in the order it opened, that drew
// anything in that second.
func (ws *ownWindows) logStats(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	last := map[*gunim.Window]gunim.Stats{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		ws.mu.Lock()
		list := slices.Clone(ws.win)
		ws.mu.Unlock()
		for i, w := range list {
			s, was := w.Stats(), last[w]
			last[w] = s
			if s == was {
				continue
			}
			log.Printf("window %d: frames %d, updates %d, merged %d", i+1, s.Frames-was.Frames, s.Commands-was.Commands, s.Coalesced-was.Coalesced)
		}
	}
}

// onAMonitor reports whether the middle of r is on one of monitors: a
// minimized window is far off them all, as Windows puts it.
func onAMonitor(monitors []driver.Monitor, r geom.Rect) bool {
	mid := r.Center()
	for _, m := range monitors {
		if m.Bounds.Contains(mid) {
			return true
		}
	}
	return false
}
