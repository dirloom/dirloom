//go:build !windows

package filter

import (
	"io/fs"
	"path/filepath"
	"strings"
)

func isHidden(_ string, name string, _ fs.FileInfo) bool {
	return strings.HasPrefix(name, ".")
}

func samePath(left, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}
