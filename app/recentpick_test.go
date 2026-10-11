package app

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
	"unicode/utf16"
)

// shortcut is a Windows shortcut to target, with an item list ahead of
// its link info as the shell writes one, the path in the code page or,
// with unicode, in UTF-16 too.
func shortcut(target string, unicode bool) []byte {
	le := binary.LittleEndian
	header := make([]byte, 0x4c)
	le.PutUint32(header, 0x4c)
	le.PutUint32(header[0x14:], 1|2)
	idList := []byte{2, 0, 0xaa, 0xbb}

	headerSize := 0x1c
	if unicode {
		headerSize = 0x24
	}
	volume := []byte{0x10, 0, 0, 0, 3, 0, 0, 0, 0, 0, 0, 0, 0x10, 0, 0, 0}
	ansi := append([]byte(target), 0)
	suffix := []byte{0}
	info := make([]byte, headerSize)
	info = append(info, volume...)
	baseAt := len(info)
	info = append(info, ansi...)
	suffixAt := len(info)
	info = append(info, suffix...)
	var wideAt, wideSuffixAt int
	if unicode {
		wideAt = len(info)
		for _, u := range utf16.Encode([]rune(target)) {
			info = le.AppendUint16(info, u)
		}
		info = le.AppendUint16(info, 0)
		wideSuffixAt = len(info)
		info = le.AppendUint16(info, 0)
	}
	le.PutUint32(info, uint32(len(info)))
	le.PutUint32(info[4:], uint32(headerSize))
	le.PutUint32(info[8:], 1)
	le.PutUint32(info[0xc:], uint32(headerSize))
	le.PutUint32(info[0x10:], uint32(baseAt))
	le.PutUint32(info[0x18:], uint32(suffixAt))
	if unicode {
		le.PutUint32(info[0x1c:], uint32(wideAt))
		le.PutUint32(info[0x20:], uint32(wideSuffixAt))
	}
	out := append(header, idList...)
	return append(out, info...)
}

func TestAShortcutSaysWhereItPoints(t *testing.T) {
	for _, c := range []struct {
		name string
		data []byte
		want string
	}{
		{"code page", shortcut(`C:\Users\m\Downloads\a.pdf`, false), `C:\Users\m\Downloads\a.pdf`},
		{"utf-16", shortcut(`C:\Users\Åsa\Hämtade filer\ö.pdf`, true), `C:\Users\Åsa\Hämtade filer\ö.pdf`},
		{"code page past ascii", shortcut("C:\\Users\\\xc5sa\\a.pdf", false), ""},
		{"cut short", shortcut(`C:\a.pdf`, false)[:0x60], ""},
		{"no shortcut", []byte("hello"), ""},
	} {
		if got := linkTarget(c.data); got != c.want {
			t.Errorf("%s: target %q, want %q", c.name, got, c.want)
		}
	}
}

func TestAFolderOpensOnItsNewestRecentFile(t *testing.T) {
	recent, dir, other := t.TempDir(), t.TempDir(), t.TempDir()
	write := func(path string) {
		t.Helper()
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	link := func(name, target string, age time.Duration) {
		t.Helper()
		p := filepath.Join(recent, name+".lnk")
		data := []byte("not one the shell wrote")
		if target != "" {
			data = shortcut(target, true)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		at := time.Now().Add(-age)
		if err := os.Chtimes(p, at, at); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(dir, "old.pdf"))
	write(filepath.Join(dir, "new.zip"))
	write(filepath.Join(dir, "same.txt"))
	write(filepath.Join(other, "same.txt"))
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	link("old.pdf", "", 3*time.Hour)
	link("new.zip", "", 2*time.Hour)
	if got, want := shellPick(recent, dir), filepath.Join(dir, "new.zip"); got != want {
		t.Fatalf("opens %q, want the newest download %q", got, want)
	}

	// A namesake elsewhere, a folder, and a file gone are passed over.
	link("same.txt", filepath.Join(other, "same.txt"), time.Hour)
	link("sub", filepath.Join(dir, "sub"), 30*time.Minute)
	link("gone.iso", filepath.Join(dir, "gone.iso"), time.Minute)
	if got, want := shellPick(recent, dir), filepath.Join(dir, "new.zip"); got != want {
		t.Fatalf("opens %q, want %q", got, want)
	}
	if got, want := shellPick(recent, other), filepath.Join(other, "same.txt"); got != want {
		t.Fatalf("opens %q, want %q", got, want)
	}

	empty := t.TempDir()
	if got := shellPick(recent, empty); got != empty {
		t.Fatalf("a folder with nothing recent opens %q, want the folder %q", got, empty)
	}
	if got := shellPick("", dir); got != dir {
		t.Fatalf("with no Recent items %q opens, want %q", got, dir)
	}
}
