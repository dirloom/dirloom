package tree

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Backend is the read-only storage a Scanner traverses. A location is an
// opaque string that only the backend interprets: an absolute path for the
// filesystem, an object id for a Git tree. The scanner owns traversal order,
// Capture Semantics and error wording; a backend only reads.
type Backend interface {
	// ReadDir lists the directory at location. Entries need not be sorted.
	ReadDir(location string) ([]fs.DirEntry, error)
	// Child returns the location of entry inside the directory at location.
	// It is also the full path given to filter.Policy.
	Child(location string, entry fs.DirEntry) string
	// ReadGitIgnore returns the .gitignore control file of the directory at
	// location, or ok=false when it is absent or not a regular file.
	// relativeDirectory names the directory in diagnostics.
	ReadGitIgnore(location, relativeDirectory string) (data []byte, ok bool, err error)
	// ReadLink returns the target of the link at location. relativePath names
	// the link in diagnostics. A backend that cannot read a target either
	// returns "" with no error (the filesystem keeps the link without a target)
	// or fails the scan.
	ReadLink(location, relativePath string) (string, error)
}

// filesystemBackend reads the operating-system filesystem. Locations are
// absolute paths.
type filesystemBackend struct{}

func (filesystemBackend) ReadDir(location string) ([]fs.DirEntry, error) {
	return os.ReadDir(location)
}

func (filesystemBackend) Child(location string, entry fs.DirEntry) string {
	return filepath.Join(location, entry.Name())
}

func (filesystemBackend) ReadGitIgnore(location, relativeDirectory string) ([]byte, bool, error) {
	path := filepath.Join(location, ".gitignore")
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("inspect gitignore %q: %w", displayGitIgnorePath(relativeDirectory), classifyFilesystemError(err))
	}
	// Git does not follow a symlink used as a working-tree .gitignore. Other
	// non-regular entries are displayable filesystem nodes, not control files.
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, false, nil
	}

	// #nosec G304 -- path is the fixed .gitignore control file beneath the user-selected directory.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("read gitignore %q: %w", displayGitIgnorePath(relativeDirectory), classifyFilesystemError(err))
	}
	return data, true, nil
}

// ReadLink keeps an unreadable link with an empty target, as scanning always
// has.
func (filesystemBackend) ReadLink(location, _ string) (string, error) {
	target, err := os.Readlink(location)
	if err != nil {
		return "", nil
	}
	return filepath.ToSlash(target), nil
}
