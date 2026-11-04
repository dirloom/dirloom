package tree

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// fileAttributeTagInfo is FILE_ATTRIBUTE_TAG_INFO.
type fileAttributeTagInfo struct {
	FileAttributes uint32
	ReparseTag     uint32
}

// heldHandles counts the handles IsJunction holds: zero once it returns.
var heldHandles atomic.Int64

// IsJunction reports whether the filesystem entry at path, whose metadata is
// info, is a junction: a reparse point tagged IO_REPARSE_TAG_MOUNT_POINT,
// which also covers a mounted volume folder. The tag decides, not the mode Go
// derives from it: ModeIrregular by default since Go 1.23, ModeSymlink with
// GODEBUG=winsymlink=0. The tag is read through a handle opened on the entry
// itself, never on its target, and closed before returning.
func IsJunction(path string, info fs.FileInfo) (bool, error) {
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok || data.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT == 0 {
		return false, nil
	}
	name, err := windows.UTF16PtrFromString(extendedPath(path))
	if err != nil {
		return false, &os.PathError{Op: "open", Path: path, Err: err}
	}
	// Every share mode, so reading the tag never locks the entry.
	handle, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return false, &os.PathError{Op: "open", Path: path, Err: err}
	}
	heldHandles.Add(1)
	defer func() {
		if windows.CloseHandle(handle) == nil {
			heldHandles.Add(-1)
		}
	}()
	var tag fileAttributeTagInfo
	//nolint:gosec // G103: FILE_ATTRIBUTE_TAG_INFO is read into a fixed-size Go struct.
	if err := windows.GetFileInformationByHandleEx(handle, windows.FileAttributeTagInfo, (*byte)(unsafe.Pointer(&tag)), uint32(unsafe.Sizeof(tag))); err != nil {
		return false, &os.PathError{Op: "GetFileInformationByHandleEx", Path: path, Err: err}
	}
	return tag.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 && tag.ReparseTag == windows.IO_REPARSE_TAG_MOUNT_POINT, nil
}

// extendedPath adds the \\?\ prefix Win32 calls need for a path longer than
// MAX_PATH, as the os package does. Locations are absolute and clean.
func extendedPath(path string) string {
	if len(path) < 248 || !filepath.IsAbs(path) || strings.HasPrefix(path, `\\?\`) {
		return path
	}
	if strings.HasPrefix(path, `\\`) {
		return `\\?\UNC\` + path[2:]
	}
	return `\\?\` + path
}
