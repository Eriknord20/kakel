package winattrs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"

	"github.com/marrasen/gunim/filemanager"
)

func init() { Lookup, Space, Keep = lookup, space, keep }

// keep has the cloud provider keep the item at the SFTP path p on this
// device, or free its space, as Explorer does.
func keep(ctx context.Context, home, p string, on bool) error {
	full := filepath.FromSlash(Resolve(filepath.ToSlash(home), p))
	k, ok := filemanager.LocalFS().(filemanager.CloudKeeper)
	if !ok {
		return errors.ErrUnsupported
	}
	return k.KeepOnDevice(ctx, full, on)
}

// lookup reads the attributes of the item at the SFTP path p, as os
// reads them, long paths included, and without reading the item, which
// for a file kept online would download it.
func lookup(home, p string, follow bool) (uint32, bool) {
	full := filepath.FromSlash(Resolve(filepath.ToSlash(home), p))
	stat := os.Lstat
	if follow {
		stat = os.Stat
	}
	info, err := stat(full)
	if err != nil {
		return 0, false
	}
	d, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return 0, false
	}
	attrs := d.FileAttributes &^ InCloud
	if filemanager.InCloudFolder(filepath.Dir(full)) {
		attrs |= InCloud
	}
	return attrs, true
}

// space reads how much room the volume holding the SFTP path p has, as
// Windows says: free to the user, and in all. The top of the drives, /,
// is on no volume.
func space(home, p string) (free, total uint64, ok bool) {
	full := filepath.FromSlash(Resolve(filepath.ToSlash(home), p))
	if len(full) < 2 || full[1] != ':' {
		return 0, 0, false
	}
	name, err := windows.UTF16PtrFromString(full)
	if err != nil {
		return 0, 0, false
	}
	var avail, all, allFree uint64
	if err := windows.GetDiskFreeSpaceEx(name, &avail, &all, &allFree); err != nil {
		return 0, 0, false
	}
	return avail, all, true
}
