package relay

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

// SecretFile is the file a relay keeps its secret in, in kakel's
// private directory.
const SecretFile = "relay_secret"

// Asked reports whether the command line asks for a relay: -relay
// first, before anything else, so it is told apart before any window
// or installer is thought of.
func Asked(args []string) bool {
	return len(args) > 0 && (args[0] == "-relay" || args[0] == "--relay")
}

// Main runs a relay as the command line asks, with no window, until ctx
// ends. dir is where its secret and certificates are kept. What it does
// is said on out.
func Main(ctx context.Context, args []string, dir string, out io.Writer) error {
	fs := flag.NewFlagSet("kakel -relay", flag.ContinueOnError)
	fs.SetOutput(out)
	listen := fs.String("listen", "",
		"address to listen on: 127.0.0.1:8080 for plain HTTP behind a proxy that does HTTPS (the default),"+
			" :443 with -domain")
	domain := fs.String("domain", "",
		"the relay's domain name: HTTPS with a certificate from Let's Encrypt, which needs port 80 and this port reachable")
	httpAt := fs.String("http", ":80", "with -domain, where Let's Encrypt checks the domain is this machine's")
	fs.StringVar(&dir, "dir", dir, "where the relay keeps its secret, and its certificates with -domain")
	newSecret := fs.Bool("new-secret", false, "make a new secret: the URL printed before stops working")
	fs.Usage = func() {
		_, _ = fmt.Fprintln(out, "kakel -relay joins served kakel windows and the windows connecting to them, with no window of its own.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("kakel -relay: %q is not something it takes", fs.Arg(0))
	}
	if *listen == "" {
		*listen = "127.0.0.1:8080"
		if *domain != "" {
			*listen = ":443"
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	secret, err := keptSecret(filepath.Join(dir, SecretFile), *newSecret)
	if err != nil {
		return err
	}
	logger := log.New(out, "", log.LstdFlags)
	s := NewServer(secret)
	s.Log = logger.Printf

	// IdleTimeout too: a caller turned away could otherwise hold its
	// connection open, saying nothing, for as long as it liked.
	srv := &http.Server{Handler: s, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: time.Minute,
		ErrorLog: log.New(io.Discard, "", 0)}
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		return fmt.Errorf("kakel -relay: listen on %s: %w", *listen, err)
	}
	where := "https://" + *domain
	if *domain == "" {
		where = "https://<the name your proxy answers to>"
		_, _ = fmt.Fprintf(out, "kakel relay: plain HTTP on %s, for a proxy doing HTTPS in front of it\n", ln.Addr())
	} else {
		m := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(*domain),
			Cache:      autocert.DirCache(filepath.Join(dir, "relay-certs")),
		}
		srv.TLSConfig = &tls.Config{GetCertificate: m.GetCertificate, NextProtos: []string{"http/1.1", "acme-tls/1"}, MinVersion: tls.VersionTLS12}
		ln = tls.NewListener(ln, srv.TLSConfig)
		challenge := &http.Server{Addr: *httpAt, Handler: m.HTTPHandler(nil), ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout: time.Minute}
		go func() {
			if err := challenge.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Printf("kakel relay: Let's Encrypt cannot check the domain on %s: %v", *httpAt, err)
			}
		}()
		defer func() { _ = challenge.Close() }()
		_, _ = fmt.Fprintf(out, "kakel relay: HTTPS on %s for %s\n", ln.Addr(), *domain)
	}
	_, _ = fmt.Fprintf(out, "kakel relay: its URL is %s%s%s\n", where, secretPath, secret)

	stopped := make(chan error, 1)
	go func() { stopped <- srv.Serve(ln) }()
	select {
	case err := <-stopped:
		return err
	case <-ctx.Done():
	}
	sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Neither closes the WebSockets handed over; the process ending
	// does, which is what comes next.
	_ = srv.Shutdown(sctx)
	_ = srv.Close()
	return nil
}

// keptSecret is the secret kept at path, made and kept there the first
// time, or again when fresh.
func keptSecret(path string, fresh bool) (string, error) {
	if !fresh {
		b, err := os.ReadFile(path)
		if err == nil && len(strings.TrimSpace(string(b))) >= 16 {
			return strings.TrimSpace(string(b)), nil
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
	}
	secret := NewSecret()
	if err := os.WriteFile(path, []byte(secret+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("keep the relay's secret: %w", err)
	}
	return secret, nil
}
