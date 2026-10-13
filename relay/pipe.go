package relay

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"sync"
)

// PipeAsked reports whether the command line asks for a pipe to a
// window through a relay: -relay-pipe first.
func PipeAsked(args []string) bool {
	return len(args) > 0 && (args[0] == "-relay-pipe" || args[0] == "--relay-pipe")
}

// Pipe joins the window served through a relay that the command line's
// link names, and carries what comes in on in to it and what it says to
// out, until either end is done: a ProxyCommand for plain ssh, as in
//
//	ssh -o ProxyCommand="kakel -relay-pipe https://relay.example.com/k/…#XXXX-XXXX" -N -L 8080:intranet:80 kakel@desk
//
// The relay is reached through the proxy -proxy names, or else the one
// the system does.
func Pipe(ctx context.Context, args []string, in io.Reader, out io.Writer, says io.Writer) error {
	fs := flag.NewFlagSet("kakel -relay-pipe", flag.ContinueOnError)
	fs.SetOutput(says)
	proxy := fs.String("proxy", "", "the HTTP proxy to reach the relay through: a URL, or none; empty for the system's")
	fs.Usage = func() {
		_, _ = fmt.Fprintln(says, "kakel -relay-pipe [-proxy URL] <relay URL>#<code> carries standard input and output to a window served through a relay, as an ssh ProxyCommand.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("kakel -relay-pipe: give the relay's URL and the code, as https://relay.example.com/k/<secret>#XXXX-XXXX")
	}
	link, err := ParseLink(fs.Arg(0))
	if err != nil {
		return fmt.Errorf("kakel -relay-pipe: %w", err)
	}
	pick, err := Proxy(*proxy)
	if err != nil {
		return fmt.Errorf("kakel -relay-pipe: %w", err)
	}
	c, err := Join(ctx, link, Dialer{Proxy: pick})
	if err != nil {
		return fmt.Errorf("kakel -relay-pipe: %w", err)
	}
	var once sync.Once
	done := make(chan struct{})
	end := func() { once.Do(func() { _ = c.Close(); close(done) }) }
	go func() { _, _ = io.Copy(c, in); end() }()
	go func() { _, _ = io.Copy(out, c); end() }()
	select {
	case <-done:
	case <-ctx.Done():
		end()
	}
	return nil
}
