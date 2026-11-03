package watch

import (
	"os"

	"golang.org/x/sys/windows"
)

func identityOf(path string) (Identity, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return Identity{}, &os.PathError{Op: "open", Path: path, Err: err}
	}
	// Every share mode, and the handle is closed at once: reading the
	// identity never locks the directory or its ancestors.
	h, err := windows.CreateFile(p, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return Identity{}, &os.PathError{Op: "open", Path: path, Err: err}
	}
	defer func() { _ = windows.CloseHandle(h) }()
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		return Identity{}, &os.PathError{Op: "GetFileInformationByHandle", Path: path, Err: err}
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return Identity{}, ErrNotDirectory
	}
	return Identity{
		Volume: uint64(info.VolumeSerialNumber),
		File:   uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow),
	}, nil
}
