package app

import (
	"context"
	"errors"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/marrasen/gunim/install"

	"github.com/marrasen/gunim/single"
	"github.com/marrasen/kakel/appicon"
	"github.com/marrasen/kakel/conf"
	"github.com/marrasen/kakel/settings"
)

// Installing kakel is gunim's install package: a kakel started from
// anywhere but its own folder opens the installer, which puts it where
// a user's programs go, for that user alone and with no administrator,
// with a Start menu entry or a desktop file, an entry the system lists
// to take it away again, and, when asked, a shortcut on the desktop and
// a start with the computer, into the tray. `kakel -install` and
// `kakel -uninstall` do the same from a shell. The installed kakel's
// updates are gunim's too: Settings › General › New releases sets whether a new release
// is put in place by itself, asked about first, or left; the questions
// and the restart are kakel's own, in updates.go.

// Installer is how kakel installs itself, for gunim's installer.
func Installer() install.App {
	return install.App{
		Name:        ProgramName,
		ID:          ProgramName,
		Version:     releaseVersion(),
		Publisher:   "Marcus Johansson",
		Description: "Terminals, files and tunnels on your machines",
		URL:         "https://github.com/marrasen/kakel",
		IconFunc:    func() image.Image { return appicon.Draw(256) },
		Categories:  "System;TerminalEmulator;",
		Autostart:   &install.Autostart{Args: []string{"-tray"}, Label: "Start kakel with the computer, in the tray"},
		// The folder follows -files, the last argument.
		Folders: &install.Folders{Args: []string{"-files"}},
		Updates: releases{},
		// The public key the releases are signed with: see RELEASING.md.
		UpdateKey: "j2nZwJ7kHSNj2JxWTwQs/GrWL1n+Zqg8F4tAybrtwIY=",
		// kakel kept its own Updates setting before gunim kept the mode:
		// an install taken on goes on as its user had it.
		UpdateMode:   install.UpdateMode(updatesSetting()),
		Available:    func(r install.Release) { toLive(func(a *app) { a.offerUpdate(r) }) },
		Updated:      func(r install.Release) { toLive(func(a *app) { a.updated(r) }) },
		Formerly:     formerly(),
		Data:         dataDirs(),
		Uninstalling: func(context.Context, install.Installation) error { removeConPTY(); return nil },
		Quit:         quitRunning,
	}
}

// releases are kakel's releases on GitHub, beta releases with them
// where updates take those: asked each time they are looked for, so
// turning beta releases on or off takes at the next look, with no
// restart.
type releases struct{}

// github is where the releases are, as the settings say now.
func (releases) github() install.GitHub {
	return install.GitHub{Repo: "marrasen/kakel", Asset: releaseAsset, Prerelease: wantsBeta()}
}

// Latest implements [install.Source].
func (r releases) Latest(ctx context.Context) (install.Release, error) {
	return r.github().Latest(ctx)
}

// ReleaseNotes implements [install.Changelog].
func (r releases) ReleaseNotes(ctx context.Context) ([]install.ReleaseNotes, error) {
	return r.github().ReleaseNotes(ctx)
}

// wantsBeta reports whether updates take beta releases too: as the
// settings say, and where they say nothing, when this build is a beta,
// so a beta goes on to the next.
func wantsBeta() bool {
	if path, err := settings.Path(); err == nil {
		if s, err := settings.Load(path); err == nil {
			if on, set := s.Beta(); set {
				return on
			}
		}
	}
	return isBeta(thisVersion())
}

// releaseVersion is this build's version when it is a release, and ""
// for a build from a working tree, which then runs as it is: it neither
// opens the installer nor looks for updates.
func releaseVersion() string {
	if v := thisVersion(); isRelease(v) {
		return v
	}
	return ""
}

// releaseAsset names a release's archive for a system, as `make
// release` names it and as kakel's releases have always been named, so
// an older kakel's own updates and the install scripts find it too.
func releaseAsset(version, goos, goarch string) string {
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return ProgramName + "_v" + strings.TrimPrefix(version, "v") + "_" + goos + "_" + goarch + ext
}

// formerly is where kakel installed itself before gunim's installer:
// ~/.local/bin/kakel on Linux. On Windows it went where it goes now.
func formerly() []string {
	if runtime.GOOS != "linux" {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{filepath.Join(home, ".local", "bin", ProgramName)}
}

// dataDirs are the folders kakel keeps the user's settings, saved
// servers and keys in, which an uninstall offers to take too.
func dataDirs() []string {
	var out []string
	if d, err := conf.Dir(); err == nil {
		out = append(out, d)
	}
	if d, err := conf.Private(); err == nil && !slices.Contains(out, d) {
		out = append(out, d)
	}
	return out
}

// updatesSetting is kakel's own Updates setting, as it stood before
// gunim kept the mode, and as Settings › General › New releases keeps it beside gunim's.
func updatesSetting() string {
	path, err := settings.Path()
	if err != nil {
		return settings.UpdatesNotify
	}
	s, err := settings.Load(path)
	if err != nil {
		return settings.UpdatesNotify
	}
	return s.Updates()
}

// quitRunning ends the kakel running, as before an uninstall, asking
// it as kakel -quit does, and waits for it to go.
func quitRunning(ctx context.Context) error {
	dir, err := settings.Dir()
	if err != nil {
		return err
	}
	taken, _ := single.Hand(dir, single.Handover{Args: []string{"-quit"}})
	if !taken {
		return nil
	}
	for end := time.Now().Add(60 * time.Second); time.Now().Before(end); {
		if !single.Running(dir) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return errors.New("the kakel running didn't end; close it and try again")
}
