//go:build windows

package filter

import (
	"io/fs"
	"path/filepath"
	"strings"
	"syscall"
)

func isHidden(_ string, name string, info fs.FileInfo) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return ok && data.FileAttributes&syscall.FILE_ATTRIBUTE_HIDDEN != 0
}

func samePath(left, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}
