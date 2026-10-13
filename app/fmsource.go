package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/marrasen/gunim/filemanager"
	"github.com/marrasen/kakel/vfs"
)

// errFromArchive is why a copy job changes nothing where its items come
// from: they are in an archive, which the file manager only reads.
var errFromArchive = errors.New("what is in an archive can only be read here")

// fmSource is a file manager's file system as the files a copy job reads
// from: the archive a transfer's items are in, which only the file
// manager opens. It reads, and changes nothing.
type fmSource struct {
	f filemanager.FS
}

// Name implements [vfs.FS].
func (s *fmSource) Name() string { return "Archive" }

// Place implements [vfs.FS]: a place of its own, so a job copies out of
// it and never renames.
func (s *fmSource) Place() any { return s }

// Roots implements [vfs.FS].
func (s *fmSource) Roots() []string { return nil }

// Sep implements [vfs.FS].
func (s *fmSource) Sep() byte {
	if s.f.Paths() == filemanager.SystemPaths {
		return os.PathSeparator
	}
	return '/'
}

// Home implements [vfs.FS].
func (s *fmSource) Home() (string, error) { return s.f.Home() }

// ReadDir implements [vfs.FS].
func (s *fmSource) ReadDir(at string) ([]vfs.Entry, error) {
	ds, err := s.f.ReadDir(context.Background(), at)
	if err != nil {
		return nil, err
	}
	out := make([]vfs.Entry, 0, len(ds))
	for _, d := range ds {
		info, err := d.Info()
		if err != nil {
			return nil, err
		}
		out = append(out, sourceEntry(info))
	}
	return out, nil
}

// Stat implements [vfs.FS].
func (s *fmSource) Stat(at string) (vfs.Entry, error) {
	info, err := s.f.Lstat(at)
	if err != nil {
		return vfs.Entry{}, err
	}
	return sourceEntry(info), nil
}

// sourceEntry is info as a copy job reads it. An archive holds no link
// the file manager follows, so none is one.
func sourceEntry(info fs.FileInfo) vfs.Entry {
	return vfs.Entry{Name: info.Name(), Size: info.Size(), Mode: info.Mode(), Mod: info.ModTime()}
}

// Open implements [vfs.FS].
func (s *fmSource) Open(at string) (io.ReadCloser, error) { return s.f.Open(at) }

// Create implements [vfs.FS]: nothing is made.
func (s *fmSource) Create(at string, _ fs.FileMode) (io.WriteCloser, error) {
	return nil, unchanged("make", at)
}

// Mkdir implements [vfs.FS]: nothing is made.
func (s *fmSource) Mkdir(at string, _ fs.FileMode) error { return unchanged("make", at) }

// Symlink implements [vfs.FS]: nothing is made.
func (s *fmSource) Symlink(_, at string) error { return unchanged("make", at) }

// Remove implements [vfs.FS]: nothing goes.
func (s *fmSource) Remove(at string) error { return unchanged("remove", at) }

// Rename implements [vfs.FS]: nothing moves.
func (s *fmSource) Rename(from, _ string) error { return unchanged("move", from) }

// Chmod implements [vfs.FS]: nothing changes.
func (s *fmSource) Chmod(at string, _ fs.FileMode) error { return unchanged("change", at) }

// Close implements [vfs.FS]: the file manager holds the archive, and
// lets it go.
func (s *fmSource) Close() error { return nil }

// unchanged is why what would do what to at did not.
func unchanged(what, at string) error {
	return fmt.Errorf("could not %s %s: %w", what, at, errFromArchive)
}
