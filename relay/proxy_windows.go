package relay

import (
	"golang.org/x/sys/windows/registry"
)

// systemProxy is the proxy set in Windows' Internet Options, for the
// user, when one is turned on.
func systemProxy() (sysProxy, bool) {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return sysProxy{}, false
	}
	defer func() { _ = k.Close() }()
	on, _, err := k.GetIntegerValue("ProxyEnable")
	if err != nil || on == 0 {
		return sysProxy{}, false
	}
	server, _, err := k.GetStringValue("ProxyServer")
	if err != nil {
		return sysProxy{}, false
	}
	override, _, _ := k.GetStringValue("ProxyOverride")
	p, err := parseSystem(server, override)
	if err != nil {
		return sysProxy{}, false
	}
	return p, true
}
