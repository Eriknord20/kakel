//go:build !windows

package app

// removeConPTY has nothing to take away: kakel carries an OpenConsole on
// Windows alone.
func removeConPTY() {}

// recentFolder is "": Windows alone keeps Recent items as shortcuts.
func recentFolder() string { return "" }
