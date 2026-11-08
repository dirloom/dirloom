//go:build windows

package tree

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// readDir lists a directory as os.ReadDir does, through a handle that also
// shares delete access (anomaly A10 of the v0.4.0-rc1 plan). os.ReadDir
// opens the directory without FILE_SHARE_DELETE: while a scan lists it, the
// user cannot rename or remove it, and while the user renames it, the scan
// cannot open it. Only the open differs from os.ReadDir: the entries, their
// metadata and the errors of the listing are Go's own, read from the handle
// by (*os.File).ReadDir, and sorted by name as os.ReadDir sorts them.
//
// One error differs on purpose: a directory being deleted is reported as no
// longer existing (errDeletePending), where os.ReadDir reports a denied
// access.
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

// retryDelays are the waits between the attempts to open a directory that
// another process holds for an instant: five attempts within 50 ms. Tests
// change them.
var retryDelays = []time.Duration{5 * time.Millisecond, 10 * time.Millisecond, 15 * time.Millisecond, 20 * time.Millisecond}

// readDirContext is readDir with a bounded retry of the two transient
// failures to open a directory (Q5 of the v0.4.0-rc1 plan):
//   - ERROR_SHARING_VIOLATION: another process holds the directory without
//     sharing read access;
//   - a directory being deleted (errDeletePending), which disappears once
//     its last handle closes.
//
// Any other error, a denied access included, is returned at once. After the
// last attempt the error is returned as it is: a lasting sharing violation
// stays one, and a directory still being deleted is reported as no longer
// existing. Cancellation ends the wait with ctx.Err().
func readDirContext(ctx context.Context, location string) ([]fs.DirEntry, error) {
	for attempt := 0; ; attempt++ {
		entries, err := readDir(location)
		if err == nil || !transientOpen(err) || attempt == len(retryDelays) {
			return entries, err
		}
		timer := time.NewTimer(retryDelays[attempt])
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

// transientOpen reports whether err is a failure to open a directory that
// may succeed an instant later.
func transientOpen(err error) bool {
	var path *os.PathError
	return errors.As(err, &path) && path.Op == "open" &&
		(errors.Is(path.Err, windows.ERROR_SHARING_VIOLATION) || errors.Is(path.Err, errDeletePending))
}

// errDeletePending is a directory or file being deleted. Windows refuses to
// open it with ERROR_ACCESS_DENIED until its last handle closes, then it no
// longer exists: it is reported as fs.ErrNotExist, never as a denied
// permission. Only the NT status tells it from an ACL denial.
var errDeletePending error = deletePending{}

type deletePending struct{}

func (deletePending) Error() string        { return "the entry is being deleted" }
func (deletePending) Is(target error) bool { return target == fs.ErrNotExist }

// openDirectory opens location as os.ReadDir does through syscall.Open
// (Go 1.26): generic read access, backup semantics, no inheritable handle,
// the path made long-path safe as os.fixLongPath makes it, and ENOTDIR when
// the handle is not a directory's. The differences are FILE_SHARE_DELETE in
// the share mode and errDeletePending.
func openDirectory(location string) (*os.File, error) {
	h, err := openShared(location, windows.FILE_ATTRIBUTE_READONLY|windows.FILE_FLAG_BACKUP_SEMANTICS)
	if err != nil {
		return nil, err
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

// readFile reads a file as os.ReadFile does, through a handle that also
// shares delete access: reading a .gitignore never blocks its removal or
// its renaming (A10). The open is os.Open's (syscall.Open with O_RDONLY);
// the reading and its errors are Go's, through (*os.File).Read.
func readFile(path string, limit int64) ([]byte, error) {
	h, err := openShared(path, windows.FILE_ATTRIBUTE_READONLY|windows.FILE_FLAG_BACKUP_SEMANTICS)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(h), path)
	defer func() { _ = f.Close() }()
	return io.ReadAll(io.LimitReader(f, limit+1))
}

// openShared calls CreateFile as syscall.Open does for O_RDONLY, with
// FILE_SHARE_DELETE added, and reports a deletion in progress as
// errDeletePending. The NT status is read on the thread that made the call,
// before any other system call can change it.
func openShared(location string, attrs uint32) (windows.Handle, error) {
	if location == "" {
		return windows.InvalidHandle, &os.PathError{Op: "open", Path: location, Err: syscall.ENOENT}
	}
	name, err := windows.UTF16PtrFromString(longPathForOS(location))
	if err != nil {
		return windows.InvalidHandle, &os.PathError{Op: "open", Path: location, Err: err}
	}
	deletePendingKnown := procRtlGetLastNtStatus.Find() == nil
	runtime.LockOSThread()
	h, err := windows.CreateFile(name, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, attrs, 0)
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) && deletePendingKnown && lastNTStatus() == windows.STATUS_DELETE_PENDING {
		err = errDeletePending
	}
	runtime.UnlockOSThread()
	if err != nil {
		return windows.InvalidHandle, &os.PathError{Op: "open", Path: location, Err: err}
	}
	return h, nil
}

var procRtlGetLastNtStatus = windows.NewLazySystemDLL("ntdll.dll").NewProc("RtlGetLastNtStatus")

// lastNTStatus returns the NT status of the last failed system call of this
// thread; Win32 maps STATUS_DELETE_PENDING and STATUS_ACCESS_DENIED to the
// same ERROR_ACCESS_DENIED.
func lastNTStatus() windows.NTStatus {
	r, _, _ := procRtlGetLastNtStatus.Call()
	return windows.NTStatus(uint32(r))
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
