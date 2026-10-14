package winattrs_test

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/marrasen/kakel/internal/winattrs"
	"github.com/marrasen/kakel/vfs"
	"github.com/pkg/sftp"
)

// A folder listed and an item stated through the proxy carry the cloud
// attributes of the items that have any, and nothing for the others.
func TestTheCloudAttributesComeWithTheReplies(t *testing.T) {
	dir := t.TempDir()
	// The same names in two folders, kept differently: the attributes
	// are looked up by the whole path.
	for _, sub := range []string{"", "other"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o700); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"here.txt", "online.txt", "pinned.txt", "local.txt"} {
			if err := os.WriteFile(filepath.Join(dir, sub, name), []byte(name), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	slashed := filepath.ToSlash(dir)
	var mu sync.Mutex
	followed := map[string]bool{}
	lookup := func(_, p string, follow bool) (uint32, bool) {
		// As the server names it, /C:/x on Windows, as the test does.
		p = winattrs.Resolve("", p)
		mu.Lock()
		followed[p] = follow
		mu.Unlock()
		switch p {
		case slashed + "/online.txt":
			return winattrs.RecallOnDataAccess | 0x20, true
		case slashed + "/pinned.txt", slashed + "/other/online.txt":
			return winattrs.Pinned, true
		case slashed + "/local.txt":
			return winattrs.InCloud | 0x20, true
		}
		return 0x20, true
	}
	here, there := net.Pipe()
	server, err := sftp.NewServer(winattrs.Proxy(there, dir, lookup, func(_, p string) (uint64, uint64, bool) {
		// A volume of a terabyte, 300 GB of it free.
		return 300 << 30, 1 << 40, true
	}, nil))
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.Serve() }()
	t.Cleanup(func() { _ = server.Close() })
	client, err := sftp.NewClientPipe(here, here)
	if err != nil {
		t.Fatal(err)
	}
	f := vfs.NewSFTP("win", nil, client, client.Close)
	t.Cleanup(func() { _ = f.Close() })

	entries, err := f.ReadDir(slashed)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]uint32{}
	for _, e := range entries {
		got[e.Name] = e.Attrs
	}
	if got["here.txt"] != 0 || got["online.txt"] != winattrs.RecallOnDataAccess || got["pinned.txt"] != winattrs.Pinned ||
		got["local.txt"] != winattrs.InCloud {
		t.Fatalf("the listing carries %v", got)
	}
	if !winattrs.OnlineOnly(got["online.txt"]) || winattrs.OnlineOnly(got["pinned.txt"]) {
		t.Fatal("online only is not told from pinned")
	}
	others, err := f.ReadDir(slashed + "/other")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range others {
		if want := map[string]uint32{"online.txt": winattrs.Pinned}[e.Name]; e.Attrs != want {
			t.Fatalf("in the other folder, %s carries %#x", e.Name, e.Attrs)
		}
	}
	e, err := f.Stat(slashed + "/online.txt")
	if err != nil || e.Attrs != winattrs.RecallOnDataAccess || e.Size != int64(len("online.txt")) {
		t.Fatalf("the stat is %+v, %v", e, err)
	}
	info, err := client.Stat(slashed + "/online.txt")
	if err != nil || vfs.WinAttrs(info) != winattrs.RecallOnDataAccess {
		t.Fatalf("a stat that follows links carries %v, %v", info, err)
	}
	mu.Lock()
	follows := followed[winattrs.Resolve("", slashed+"/online.txt")]
	mu.Unlock()
	if !follows {
		t.Fatal("a stat that follows links looked up the link")
	}
	// The volume's room comes from the proxy, which answers for it.
	if free, total, err := f.Space(slashed); err != nil || free != 300<<30 || total != 1<<40 {
		t.Fatalf("the room is %d free of %d, %v", free, total, err)
	}
	// What is read is read as it is.
	r, err := f.Open(slashed + "/here.txt")
	if err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 20)
	n, _ := r.Read(b)
	_ = r.Close()
	if string(b[:n]) != "here.txt" {
		t.Fatalf("read %q", b[:n])
	}
}

// A path the server is asked about is its own: from home when relative,
// and a Windows drive without the slash before it.
func TestResolve(t *testing.T) {
	for _, c := range [][3]string{
		{"C:/Users/me", "/C:/Users/me/x", "C:/Users/me/x"},
		{"C:/Users/me", "Documents", "C:/Users/me/Documents"},
		{"/home/me", "/etc", "/etc"},
		{"/home/me", "x", "/home/me/x"},
		{"/", "/c:", "c:/"},
	} {
		if got := winattrs.Resolve(c[0], c[1]); got != c[2] {
			t.Errorf("Resolve(%q, %q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}

// serveWith serves dir through the proxy, with keep, and returns the
// connected kakel's file system.
func serveWith(t *testing.T, dir string, keep func(ctx context.Context, home, p string, keep bool) error) *vfs.SFTP {
	t.Helper()
	here, there := net.Pipe()
	server, err := sftp.NewServer(winattrs.Proxy(there, dir, func(string, string, bool) (uint32, bool) { return 0, false }, nil, keep))
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.Serve() }()
	t.Cleanup(func() { _ = server.Close() })
	client, err := sftp.NewClientPipe(here, here)
	if err != nil {
		t.Fatal(err)
	}
	f := vfs.NewSFTP("win", nil, client, client.Close)
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// The connected kakel asks the serving one to keep an item on its device,
// or free its space, and hears how it went; a kakel that cannot says so
// before it is asked.
func TestTheProxyKeepsItemsOnTheDeviceOrFreesThem(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	slashed := filepath.ToSlash(dir)
	type ask struct {
		path string
		keep bool
	}
	asked := make(chan ask, 4)
	f := serveWith(t, dir, func(_ context.Context, home, p string, keep bool) error {
		asked <- ask{winattrs.Resolve(home, p), keep}
		if strings.HasSuffix(p, "nope") {
			return errors.New("the provider said no")
		}
		return nil
	})
	if !f.CanKeepOnDevice() {
		t.Fatal("the connected kakel does not know the serving one can keep items")
	}
	if err := f.KeepOnDevice(slashed+"/a.txt", false); err != nil {
		t.Fatal(err)
	}
	if got := <-asked; got.path != winattrs.Resolve("", slashed+"/a.txt") || got.keep {
		t.Errorf("the serving kakel was asked %+v", got)
	}
	if err := f.KeepOnDevice(slashed+"/a.txt", true); err != nil || !(<-asked).keep {
		t.Errorf("kept, the answer is %v", err)
	}
	err := f.KeepOnDevice(slashed+"/nope", true)
	<-asked
	if err == nil || err.Error() != "win: the provider said no" {
		t.Errorf("the failure comes back as %v", err)
	}
	// Other changes to an item go to the server as before.
	when := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := f.Chtimes(slashed+"/a.txt", when); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(dir, "a.txt")); err != nil || !info.ModTime().Equal(when) {
		t.Errorf("the time set through the proxy is %v, %v", info.ModTime(), err)
	}
	// The listing goes on as before.
	if es, err := f.ReadDir(slashed); err != nil || len(es) != 1 {
		t.Errorf("after, the folder lists %v, %v", es, err)
	}
	plain := serveWith(t, dir, nil)
	if plain.CanKeepOnDevice() || !errors.Is(plain.KeepOnDevice(slashed+"/a.txt", false), errors.ErrUnsupported) {
		t.Error("a kakel with nothing to keep items with offers to")
	}
}

// Two items kept or freed at once are the most: a third is refused, not
// left waiting behind them.
func TestTheProxyKeepsTwoItemsAtOnceAtMost(t *testing.T) {
	dir := t.TempDir()
	let := make(chan struct{})
	started := make(chan struct{}, 4)
	f := serveWith(t, dir, func(context.Context, string, string, bool) error {
		started <- struct{}{}
		<-let
		return nil
	})
	errs := make(chan error, 2)
	for range 2 {
		go func() { errs <- f.KeepOnDevice(filepath.ToSlash(dir), true) }()
	}
	<-started
	<-started
	if err := f.KeepOnDevice(filepath.ToSlash(dir), false); err == nil || !strings.Contains(err.Error(), "busy") {
		t.Errorf("a third at once is answered %v", err)
	}
	close(let)
	for range 2 {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
}
