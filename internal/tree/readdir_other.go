//go:build !windows

package tree

import (
	"context"
	"io"
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

// readFile reads at most limit+1 bytes of path, so that a caller can refuse
// a file beyond its limit without reading it whole.
func readFile(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path) // #nosec G304 -- a .gitignore beneath the user-selected root.
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(io.LimitReader(f, limit+1))
}
