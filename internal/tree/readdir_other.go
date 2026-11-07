//go:build !windows

package tree

import (
	"io/fs"
	"os"
)

// readDir lists a directory. Unix lets a directory be renamed or removed
// while it is open: os.ReadDir needs no change (anomaly A10 is Windows only).
func readDir(location string) ([]fs.DirEntry, error) { return os.ReadDir(location) }
