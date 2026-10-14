//go:build !windows

package relay

// systemProxy is the proxy set for the system: elsewhere than Windows,
// the environment is where it is set, and that is read already.
func systemProxy() (sysProxy, bool) { return sysProxy{}, false }
