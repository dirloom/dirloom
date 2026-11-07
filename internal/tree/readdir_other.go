//go:build !windows

package tree

import (
	"context"
	"io/fs"
	"os"
)

// Unix lets a directory or a file be renamed or removed while it is open:
// os.ReadDir and os.ReadFile need no change, and nothing is retried
// (anomaly A10 is Windows only).

func readDir(location string) ([]fs.DirEntry, error) { return os.ReadDir(location) }

func readDirContext(_ context.Context, location string) ([]fs.DirEntry, error) {
	return readDir(location)
}

func readFile(path string) ([]byte, error) { return os.ReadFile(path) } // #nosec G304 -- a .gitignore beneath the user-selected root.
