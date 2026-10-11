package app

import (
	"os"

	"golang.org/x/sys/windows"

	"github.com/marrasen/kakel/internal/conpty"
)

// removeConPTY takes away the OpenConsole kakel put in the user's cache
// to run its panes through, as kakel is uninstalled. The kakel running
// has ended by then, and the OpenConsoles it started with it. Another
// kakel still running, as a portable copy, keeps them in use, and they
// stay for it.
func removeConPTY() {
	if dir, err := conpty.Dir(); err == nil {
		_ = os.RemoveAll(dir)
	}
}

// recentFolder is the folder of the user's Recent items, or "".
func recentFolder() string {
	dir, err := windows.KnownFolderPath(windows.FOLDERID_Recent, 0)
	if err != nil {
		return ""
	}
	return dir
}
