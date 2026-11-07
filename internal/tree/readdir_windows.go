//go:build windows

package tree

import (
	"io/fs"
	"os"
	"slices"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// readDir lists a directory as os.ReadDir does, through a handle that also
// shares delete access (anomaly A10 of the v0.4.0-rc1 plan). os.ReadDir
// opens the directory without FILE_SHARE_DELETE: while a scan lists it, the
// user cannot rename or remove it, and while the user renames it, the scan
// cannot open it. Only the open differs from os.ReadDir: the entries, their
// metadata and the errors of the listing are Go's own, read from the handle
// by (*os.File).ReadDir, and sorted by name as os.ReadDir sorts them.
func readDir(location string) ([]fs.DirEntry, error) {
	f, err := openDirectory(location)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	entries, err := f.ReadDir(-1)
	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	return entries, err
}

// openDirectory opens location as os.ReadDir does through syscall.Open
// (Go 1.26): generic read access, backup semantics, no inheritable handle,
// the path made long-path safe as os.fixLongPath makes it, and ENOTDIR when
// the handle is not a directory's. The one difference is FILE_SHARE_DELETE
// in the share mode.
func openDirectory(location string) (*os.File, error) {
	if location == "" {
		return nil, &os.PathError{Op: "open", Path: location, Err: syscall.ENOENT}
	}
	name, err := windows.UTF16PtrFromString(longPathForOS(location))
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: location, Err: err}
	}
	h, err := windows.CreateFile(name, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_READONLY|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: location, Err: err}
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		_ = windows.CloseHandle(h)
		return nil, &os.PathError{Op: "open", Path: location, Err: err}
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		_ = windows.CloseHandle(h)
		return nil, &os.PathError{Op: "open", Path: location, Err: syscall.ENOTDIR}
	}
	return os.NewFile(uintptr(h), location), nil
}

// nativeLongPaths reports what the Go runtime decides at startup: from
// Windows 10 build 15063 on, it marks the process long-path aware and the os
// package passes paths unchanged; before, os adds the \\?\ prefix to a long
// path.
var nativeLongPaths = func() bool {
	v := windows.RtlGetVersion()
	return v.MajorVersion > 10 || (v.MajorVersion == 10 && (v.MinorVersion > 0 || v.BuildNumber >= 15063))
}()

// longPathForOS returns location as the os package would pass it to
// CreateFile. Locations are absolute and clean.
func longPathForOS(location string) string {
	if nativeLongPaths {
		return location
	}
	return extendedPath(location)
}
