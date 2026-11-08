package tree

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/dirloom/dirloom/internal/filter"
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
	// Junction reports whether the entry at location, whose metadata is info,
	// is a Windows junction. Only the filesystem backend can find one.
	Junction(location string, info fs.FileInfo) (bool, error)
}

// filesystemBackend reads the operating-system filesystem. Locations are
// absolute paths.
type filesystemBackend struct{}

// ReadDir lists the directory as os.ReadDir does. On Windows it also shares
// delete access, so that a scan neither blocks nor is blocked by a rename or
// a removal of the directory, and it retries an open that another process
// blocks for an instant (anomaly A10, readdir_windows.go).
func (b filesystemBackend) ReadDir(location string) ([]fs.DirEntry, error) {
	return b.ReadDirContext(context.Background(), location)
}

// ReadDirContext is ReadDir whose retry ends on cancellation.
func (filesystemBackend) ReadDirContext(ctx context.Context, location string) ([]fs.DirEntry, error) {
	return readDirContext(ctx, location)
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

	tooLarge := fmt.Errorf("read gitignore %q: file exceeds the 1 MiB limit", displayGitIgnorePath(relativeDirectory))
	if info.Size() > MaxGitIgnoreBytes {
		return nil, false, tooLarge
	}
	// The fixed .gitignore control file beneath the user-selected directory,
	// read without blocking its removal on Windows (A10), and never beyond
	// its limit, whatever Lstat said.
	data, err := readFile(path, MaxGitIgnoreBytes)
	if err != nil {
		return nil, false, fmt.Errorf("read gitignore %q: %w", displayGitIgnorePath(relativeDirectory), classifyFilesystemError(err))
	}
	if len(data) > MaxGitIgnoreBytes {
		return nil, false, tooLarge
	}
	if line := filter.LongGitIgnoreLine(data); line != 0 {
		return nil, false, fmt.Errorf("read gitignore %q: line %d is 64 KiB or longer, above the line limit", displayGitIgnorePath(relativeDirectory), line)
	}
	return data, true, nil
}

// MaxGitIgnoreBytes bounds a .gitignore, live as in Git (D7 of the
// v0.4.0-rc1 plan): its patterns accumulate across directories, and every
// entry is tested against all of them. It equals the limit of a .gitignore
// blob in a Git source.
const MaxGitIgnoreBytes = 1 << 20

// ReadLink keeps an unreadable link with an empty target, as scanning always
// has.
func (filesystemBackend) ReadLink(location, _ string) (string, error) {
	target, err := os.Readlink(location)
	if err != nil {
		return "", nil
	}
	return filepath.ToSlash(target), nil
}

func (filesystemBackend) Junction(location string, info fs.FileInfo) (bool, error) {
	return IsJunction(location, info)
}
