package notify

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var driveTypes = map[uint32]string{
	windows.DRIVE_UNKNOWN:     "unknown",
	windows.DRIVE_NO_ROOT_DIR: "no-root-dir",
	windows.DRIVE_REMOVABLE:   "removable",
	windows.DRIVE_FIXED:       "fixed",
	windows.DRIVE_REMOTE:      "remote",
	windows.DRIVE_CDROM:       "cdrom",
	windows.DRIVE_RAMDISK:     "ramdisk",
}

// FSType describes the volume holding path: drive type and file system name.
func FSType(path string) (string, error) {
	vol, err := volumeRoot(path)
	if err != nil {
		return "", err
	}
	name, _, err := Classify(path)
	if err != nil {
		return "", err
	}
	fsName := make([]uint16, windows.MAX_PATH+1)
	vp, _ := windows.UTF16PtrFromString(vol)
	if err := windows.GetVolumeInformation(vp, nil, 0, nil, nil, nil, &fsName[0], uint32(len(fsName))); err != nil {
		return fmt.Sprintf("%s, volume %s, file system unknown (%v)", name, vol, err), nil
	}
	return fmt.Sprintf("%s, volume %s, %s", name, vol, windows.UTF16ToString(fsName)), nil
}

// Classify returns the drive type and whether it is DRIVE_REMOTE (F13).
func Classify(path string) (string, bool, error) {
	vol, err := volumeRoot(path)
	if err != nil {
		return "", false, err
	}
	vp, _ := windows.UTF16PtrFromString(vol)
	t := windows.GetDriveType(vp)
	return driveTypes[t], t == windows.DRIVE_REMOTE, nil
}

func volumeRoot(path string) (string, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	buf := make([]uint16, windows.MAX_PATH+1)
	if err := windows.GetVolumePathName(p, &buf[0], uint32(len(buf))); err != nil {
		return "", fmt.Errorf("GetVolumePathName %s: %w", path, err)
	}
	return windows.UTF16ToString(buf), nil
}

var procGetProcessHandleCount = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetProcessHandleCount")

// OpenHandles counts the process's open handles.
func OpenHandles() (int, error) {
	var n uint32
	r, _, err := procGetProcessHandleCount.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&n)))
	if r == 0 {
		return 0, err
	}
	return int(n), nil
}
