package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/marrasen/gunim/filemanager"
	"github.com/marrasen/kakel/jobs"
	"github.com/marrasen/kakel/vfs"
)

// runOp runs op to its end, and returns why it stopped.
func runOp(t *testing.T, op jobs.Op) error {
	t.Helper()
	q := jobs.New(1)
	j := q.Start(t.Context(), op, jobs.Options{})
	<-j.Done()
	return j.Progress().Err
}

// A copy job reads a transfer's items through the file manager's files,
// as it does those of an archive, folders and all.
func TestACopyReadsThroughTheFileManager(t *testing.T) {
	from, into := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(from, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(from, "sub", "a.txt"), []byte("inside"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := &fmSource{f: filemanager.LocalFS()}
	err := runOp(t, jobs.Op{Kind: jobs.Copy, From: src, At: from, Names: []string{"sub"}, To: vfs.NewLocal(), Into: into})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(into, "sub", "a.txt")); err != nil || string(got) != "inside" {
		t.Fatalf("the copy holds %q, %v", got, err)
	}
}

// Nothing is taken from where the items come from: a move fails, and
// leaves them there.
func TestNothingIsTakenFromTheFileManagersFiles(t *testing.T) {
	from, into := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(from, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := &fmSource{f: filemanager.LocalFS()}
	err := runOp(t, jobs.Op{Kind: jobs.Move, From: src, At: from, Names: []string{"a.txt"}, To: vfs.NewLocal(), Into: into})
	if !errors.Is(err, errFromArchive) {
		t.Fatalf("the move ended with %v", err)
	}
	if _, err := os.Stat(filepath.Join(from, "a.txt")); err != nil {
		t.Fatalf("the item left: %v", err)
	}
}
