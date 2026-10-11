package app

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf16"
)

// A program that asks Windows to show a file in its folder, as Firefox
// does for a download, has it open the folder with kakel when kakel
// opens folders: kakel's command gets the folder alone. The file is
// most likely the newest of the Recent items in that folder, as Firefox
// adds each download there, and that is the file selected.

// recentScan is how many of the newest Recent items are looked through.
const recentScan = 300

// shellPick is what to open for dir, a folder Windows opened with kakel:
// the newest of the files in it that the shortcuts in the folder recent
// point at, where there is one, or else dir.
func shellPick(recent, dir string) string {
	if recent == "" || dir == "" {
		return dir
	}
	entries, err := os.ReadDir(recent)
	if err != nil {
		return dir
	}
	type link struct {
		name string
		at   time.Time
	}
	var links []link
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".lnk") {
			continue
		}
		if info, err := e.Info(); err == nil {
			links = append(links, link{e.Name(), info.ModTime()})
		}
	}
	slices.SortFunc(links, func(a, b link) int { return b.at.Compare(a.at) })
	for i, l := range links {
		if i == recentScan {
			break
		}
		// Windows names a shortcut for the file it points at, and a
		// target that says another folder is a namesake's.
		if data, err := os.ReadFile(filepath.Join(recent, l.name)); err == nil {
			if target := linkTarget(data); target != "" && !samePath(filepath.Dir(target), dir) {
				continue
			}
		}
		file := filepath.Join(dir, l.name[:len(l.name)-len(".lnk")])
		if info, err := os.Stat(file); err == nil && info.Mode().IsRegular() {
			return file
		}
	}
	return dir
}

// linkTarget is the path on a local drive the Windows shortcut data
// points at, or "" where it says none or none that can be read: a
// shortcut to a share, or one with its path in the system's code page
// beyond ASCII alone.
func linkTarget(data []byte) string {
	const (
		headerSize   = 0x4c
		hasIDList    = 1
		hasLinkInfo  = 2
		localAndBase = 1
	)
	if len(data) < headerSize || binary.LittleEndian.Uint32(data) != headerSize {
		return ""
	}
	flags := binary.LittleEndian.Uint32(data[0x14:])
	at := headerSize
	if flags&hasIDList != 0 {
		if len(data) < at+2 {
			return ""
		}
		at += 2 + int(binary.LittleEndian.Uint16(data[at:]))
	}
	if flags&hasLinkInfo == 0 || len(data) < at+0x1c {
		return ""
	}
	info := data[at:]
	size := int(binary.LittleEndian.Uint32(info))
	if size < 0x1c || size > len(info) {
		return ""
	}
	info = info[:size]
	if binary.LittleEndian.Uint32(info[8:])&localAndBase == 0 {
		return ""
	}
	u32 := func(off int) int { return int(binary.LittleEndian.Uint32(info[off:])) }
	base, suffix := ansiz(info, u32(0x10)), ansiz(info, u32(0x18))
	if u32(4) >= 0x24 && len(info) >= 0x24 {
		base, suffix = utf16z(info, u32(0x1c)), utf16z(info, u32(0x20))
	}
	if base == "" {
		return ""
	}
	return base + suffix
}

// ansiz is the string ending in a zero byte at off in b, or "" where
// there is none or it goes past ASCII, which the code page decides.
func ansiz(b []byte, off int) string {
	if off <= 0 || off >= len(b) {
		return ""
	}
	end := slices.Index(b[off:], 0)
	if end < 0 {
		return ""
	}
	for _, c := range b[off : off+end] {
		if c >= 0x80 {
			return ""
		}
	}
	return string(b[off : off+end])
}

// utf16z is the UTF-16 string ending in a zero at off in b, or "".
func utf16z(b []byte, off int) string {
	if off <= 0 || off >= len(b) {
		return ""
	}
	var units []uint16
	for i := off; i+1 < len(b); i += 2 {
		u := binary.LittleEndian.Uint16(b[i:])
		if u == 0 {
			return string(utf16.Decode(units))
		}
		units = append(units, u)
	}
	return ""
}
