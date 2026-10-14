// Package relay joins a served kakel window and a window connecting to
// it when neither can reach the other: both reach a relay instead, over
// WebSockets on HTTPS, which gets through the HTTP proxies a plain port
// does not.
//
// The relay only carries bytes. What goes over the stream it joins is
// the serve protocol, SSH from end to end, so the relay cannot read it
// and cannot sign in as anyone: the served window still checks the key
// of whoever connects, and the connecting window still checks the host
// key of the window it reaches.
//
// A relay's URL carries a secret, /k/<secret>, which the relay makes on
// its first run. Without it the relay answers nothing, so a relay found
// by a stranger is not a free tunnel for their traffic.
//
// A served window registers with a code. A window connecting gives the
// relay the same code, and the relay asks the served window for a
// stream of its own to join to that one.
package relay

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// secretPath is the part of a relay's URL before its secret.
const secretPath = "/k/"

// URL is a relay's address with its secret, made sound: https when
// nothing was said, no query, no fragment and no slash at the end.
type URL struct{ u *url.URL }

// ParseURL reads a relay's URL as the user typed or pasted it: with or
// without https://, and with a code after a # or not, which is dropped
// here (see ParseLink for that).
func ParseURL(s string) (URL, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return URL{}, errors.New("type the relay's URL")
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return URL{}, fmt.Errorf("%q is not a URL", s)
	}
	switch strings.ToLower(u.Scheme) {
	case "https", "wss":
		u.Scheme = "https"
	case "http", "ws":
		u.Scheme = "http"
	default:
		return URL{}, fmt.Errorf("a relay is reached over https, not %s", u.Scheme)
	}
	if u.Host == "" || u.Hostname() == "" {
		return URL{}, fmt.Errorf("%q names no machine", s)
	}
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/")
	at := strings.LastIndex(u.Path, secretPath)
	if at < 0 || at+len(secretPath) == len(u.Path) || strings.Contains(u.Path[at+len(secretPath):], "/") {
		return URL{}, errors.New("the relay's URL ends in /k/ and its secret, as the relay printed it when it started")
	}
	u.RawPath, u.RawQuery, u.Fragment, u.RawFragment, u.User = "", "", "", "", nil
	return URL{u: u}, nil
}

// String is the URL in full, secret and all.
func (r URL) String() string {
	if r.u == nil {
		return ""
	}
	return r.u.String()
}

// Host is the relay's machine, and its port when one was given: what
// to call it without giving its secret away.
func (r URL) Host() string {
	if r.u == nil {
		return ""
	}
	return r.u.Host
}

// IsZero reports a URL never parsed.
func (r URL) IsZero() bool { return r.u == nil }

// to is the URL of one of the relay's doors.
func (r URL) to(door string) string {
	u := *r.u
	u.Path += "/" + door
	return u.String()
}

// codeChars are the characters of a code: Crockford's base 32, which
// leaves out I, L, O and U, so a code read aloud or copied by hand
// comes out the same.
const codeChars = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// codeLen is how many characters a code has: 40 bits, more than anyone
// guesses by asking a relay that answers a few at a time.
const codeLen = 8

// NewCode makes a code for a served window, as XXXX-XXXX.
func NewCode() string {
	var b [codeLen]byte
	_, _ = rand.Read(b[:])
	for i := range b {
		b[i] = codeChars[int(b[i])%len(codeChars)]
	}
	return string(b[:4]) + "-" + string(b[4:])
}

// ParseCode reads a code as typed: any case, with or without the dash,
// with spaces, and with the letters taken for digits mended.
func ParseCode(s string) (string, error) {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		switch r {
		case ' ', '-', '\t':
			continue
		case 'O':
			r = '0'
		case 'I', 'L':
			r = '1'
		}
		if !strings.ContainsRune(codeChars, r) {
			return "", fmt.Errorf("%q is not a code: a code is eight letters and digits, as XXXX-XXXX", strings.TrimSpace(s))
		}
		b.WriteRune(r)
	}
	c := b.String()
	if len(c) != codeLen {
		return "", fmt.Errorf("%q is not a code: a code is eight letters and digits, as XXXX-XXXX", strings.TrimSpace(s))
	}
	return c[:4] + "-" + c[4:], nil
}

// Link is a window served through a relay: the relay and the code.
type Link struct {
	Relay URL
	Code  string
}

// ParseLink reads a link written by Link.String: the relay's URL with
// the code after a #.
func ParseLink(s string) (Link, error) {
	s = strings.TrimSpace(s)
	hash := strings.LastIndex(s, "#")
	if hash < 0 {
		return Link{}, errors.New("no code in " + s)
	}
	r, err := ParseURL(s[:hash])
	if err != nil {
		return Link{}, err
	}
	code, err := ParseCode(s[hash+1:])
	if err != nil {
		return Link{}, err
	}
	return Link{Relay: r, Code: code}, nil
}

// IsLink reports whether s is written as a link to a window through a
// relay rather than as a machine and a port.
func IsLink(s string) bool {
	s = strings.TrimSpace(s)
	return strings.Contains(s, "://") && strings.Contains(s, "#")
}

// String is the relay's URL with the code after a #: all it takes to
// reach the window, kept as a window's address.
func (l Link) String() string { return l.Relay.String() + "#" + l.Code }

// Shown is what to call the window to the user: the code and the
// relay's machine, and not its secret.
func (l Link) Shown() string { return l.Code + " on " + l.Relay.Host() }

// KnownAs is the name the window's host key is kept under: the code at
// the relay, as a machine name of its own. A window served under
// another code, or through another relay, is another window to check.
func (l Link) KnownAs() string {
	return net.JoinHostPort(strings.ToLower(l.Code)+"."+l.Relay.u.Hostname(), "443")
}

// Same reports whether two links reach the same window.
func (l Link) Same(o Link) bool {
	return l.Code == o.Code && l.Relay.String() == o.Relay.String()
}

// Shown is what to call the window at addr: the relay and code for a
// link, and addr itself for anything else.
func Shown(addr string) string {
	if l, err := ParseLink(addr); err == nil {
		return l.Shown()
	}
	return addr
}
