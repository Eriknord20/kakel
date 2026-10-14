package relay

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// NoProxy is the proxy set that goes straight to the relay, whatever the
// system says.
const NoProxy = "none"

// ParseProxy reads a proxy as typed: a URL, http:// when none is said,
// with a user name and password for one that asks (Basic); or NoProxy.
func ParseProxy(s string) (*url.URL, error) {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, NoProxy) {
		return nil, nil
	}
	if !strings.Contains(s, "://") {
		s = "http://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Hostname() == "" {
		return nil, fmt.Errorf("%q is not a proxy: write it as http://proxy.example.com:8080", s)
	}
	switch u.Scheme {
	case "http", "https", "socks5":
	default:
		return nil, fmt.Errorf("a proxy is http://, https:// or socks5://, not %s://", u.Scheme)
	}
	return u, nil
}

// Proxy picks the proxy for each request to a relay: the one set, when
// there is one; else HTTPS_PROXY and the like, with NO_PROXY; else, on
// Windows, the proxy set for the system. A proxy found only in a PAC
// script is not found.
func Proxy(set string) (func(*http.Request) (*url.URL, error), error) {
	if strings.EqualFold(strings.TrimSpace(set), NoProxy) {
		return nil, nil
	}
	u, err := ParseProxy(set)
	if err != nil {
		return nil, err
	}
	if u != nil {
		return http.ProxyURL(u), nil
	}
	if fromEnvironment() {
		return http.ProxyFromEnvironment, nil
	}
	if sp, ok := systemProxy(); ok {
		return sp.pick, nil
	}
	return nil, nil
}

// fromEnvironment reports whether the environment names a proxy.
func fromEnvironment() bool {
	for _, k := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		if os.Getenv(k) != "" {
			return true
		}
	}
	return false
}

// sysProxy is a proxy the system names, and the machines it is not
// for.
type sysProxy struct {
	https, http *url.URL
	// bypass are the patterns of machines reached directly; local says
	// a name with no dot in it is one of them.
	bypass []string
	local  bool
}

// pick is the proxy for r, nil to go straight there.
func (p sysProxy) pick(r *http.Request) (*url.URL, error) {
	host := r.URL.Hostname()
	if p.bypassed(host) {
		return nil, nil
	}
	if r.URL.Scheme == "https" && p.https != nil {
		return p.https, nil
	}
	return p.http, nil
}

// bypassed reports whether host is reached without the proxy.
func (p sysProxy) bypassed(host string) bool {
	host = strings.ToLower(host)
	if p.local && !strings.Contains(host, ".") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}
	for _, b := range p.bypass {
		if matches(strings.ToLower(b), host) {
			return true
		}
	}
	return false
}

// matches reports whether host fits pattern, where * stands for any run
// of characters, as Windows writes the machines to bypass.
func matches(pattern, host string) bool {
	if !strings.Contains(pattern, "*") {
		return pattern == host
	}
	parts := strings.Split(pattern, "*")
	if !strings.HasPrefix(host, parts[0]) {
		return false
	}
	host = host[len(parts[0]):]
	for i, part := range parts[1:] {
		last := i == len(parts)-2
		if last {
			return strings.HasSuffix(host, part)
		}
		at := strings.Index(host, part)
		if at < 0 {
			return false
		}
		host = host[at+len(part):]
	}
	return true
}

// parseSystem reads a proxy as Windows writes it: one for everything,
// host:port, or one a scheme, "http=h:p;https=h:p", with the bypass
// list "a;*.b;<local>".
func parseSystem(server, override string) (sysProxy, error) {
	var p sysProxy
	asURL := func(s string) (*url.URL, error) {
		s = strings.TrimSpace(s)
		if s == "" {
			return nil, nil
		}
		return ParseProxy(s)
	}
	if !strings.Contains(server, "=") {
		u, err := asURL(server)
		if err != nil {
			return p, err
		}
		p.http, p.https = u, u
	} else {
		for part := range strings.SplitSeq(server, ";") {
			scheme, at, ok := strings.Cut(part, "=")
			if !ok {
				continue
			}
			u, err := asURL(at)
			if err != nil {
				return p, err
			}
			switch strings.ToLower(strings.TrimSpace(scheme)) {
			case "https":
				p.https = u
			case "http":
				p.http = u
			}
		}
	}
	if p.http == nil && p.https == nil {
		return p, errors.New("the system names no proxy for the web")
	}
	for b := range strings.SplitSeq(override, ";") {
		b = strings.TrimSpace(b)
		switch {
		case b == "":
		case strings.EqualFold(b, "<local>"):
			p.local = true
		default:
			p.bypass = append(p.bypass, b)
		}
	}
	return p, nil
}
