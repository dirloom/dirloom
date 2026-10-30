package gitsource

import (
	"fmt"
	"io/fs"
	"time"
	"unicode/utf8"
)

// Locations of the Git backend are object ids: a tree id for a directory and
// a blob id for a link. A submodule is an empty directory that is never read.
const (
	submoduleLocation = ""
	// invalidLocation is never a valid id, so reading it fails as an
	// internal invariant.
	invalidLocation = "\x00"
)

// Backend serves one committed tree to tree.Scanner through one Batch. It
// reads a tree object only when the scanner enters that directory.
type Backend struct {
	batch *Batch

	// The scanner reads a directory's .gitignore, then lists it: the last
	// parsed tree serves both.
	cached         bool
	cachedLocation string
	cachedEntries  []TreeEntry
}

// NewBackend returns a backend reading through batch.
func NewBackend(batch *Batch) *Backend {
	return &Backend{batch: batch}
}

func (b *Backend) entries(location string) ([]TreeEntry, error) {
	if location == submoduleLocation {
		return nil, nil
	}
	if b.cached && b.cachedLocation == location {
		return b.cachedEntries, nil
	}
	data, err := b.batch.read(location, treeObject)
	if err != nil {
		return nil, err
	}
	entries, err := ParseTree(data, b.batch.format)
	if err != nil {
		return nil, err
	}
	b.cached, b.cachedLocation, b.cachedEntries = true, location, entries
	return entries, nil
}

// ReadDir lists the tree at location.
func (b *Backend) ReadDir(location string) ([]fs.DirEntry, error) {
	entries, err := b.entries(location)
	if err != nil {
		return nil, err
	}
	out := make([]fs.DirEntry, len(entries))
	for i, entry := range entries {
		out[i] = dirEntry{entry: entry}
	}
	return out, nil
}

// Child returns the object id of entry, or the submodule location.
func (b *Backend) Child(_ string, entry fs.DirEntry) string {
	child, ok := entry.(dirEntry)
	if !ok {
		return invalidLocation
	}
	if child.entry.Mode == ModeGitlink {
		return submoduleLocation
	}
	return child.entry.ID
}

// ReadGitIgnore returns the .gitignore blob of the tree at location. A
// .gitignore that is a link, a submodule or a directory is not a control
// file, as for a working tree.
func (b *Backend) ReadGitIgnore(location, relativeDirectory string) ([]byte, bool, error) {
	entries, err := b.entries(location)
	if err != nil {
		return nil, false, fmt.Errorf("read directory %q: %w", displayDirectory(relativeDirectory), err)
	}
	for _, entry := range entries {
		if entry.Name != ".gitignore" {
			continue
		}
		if entry.Mode != ModeFile {
			return nil, false, nil
		}
		data, err := b.batch.read(entry.ID, gitIgnoreObject)
		if err != nil {
			return nil, false, fmt.Errorf("read gitignore %q: %w", displayGitIgnore(relativeDirectory), err)
		}
		return data, true, nil
	}
	return nil, false, nil
}

// ReadLink returns the link target recorded in the blob at location: valid
// UTF-8, never resolved and never rewritten.
func (b *Backend) ReadLink(location, relativePath string) (string, error) {
	data, err := b.batch.read(location, linkObject)
	if err != nil {
		return "", fmt.Errorf("read link %q: %w", relativePath, err)
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("read link %q: %w", relativePath, treeInvalid("link target is not valid UTF-8"))
	}
	return string(data), nil
}

func displayDirectory(relativeDirectory string) string {
	if relativeDirectory == "" {
		return "."
	}
	return relativeDirectory
}

func displayGitIgnore(relativeDirectory string) string {
	if relativeDirectory == "" {
		return ".gitignore"
	}
	return relativeDirectory + "/.gitignore"
}

// dirEntry exposes a tree entry to the scanner. Its file information has no
// operating-system value: Sys is nil.
type dirEntry struct {
	entry TreeEntry
}

func (e dirEntry) Name() string               { return e.entry.Name }
func (e dirEntry) IsDir() bool                { return e.entry.fileMode().IsDir() }
func (e dirEntry) Type() fs.FileMode          { return e.entry.fileMode().Type() }
func (e dirEntry) Info() (fs.FileInfo, error) { return fileInfo(e), nil }

type fileInfo struct {
	entry TreeEntry
}

func (i fileInfo) Name() string       { return i.entry.Name }
func (i fileInfo) Size() int64        { return 0 }
func (i fileInfo) Mode() fs.FileMode  { return i.entry.fileMode() }
func (i fileInfo) ModTime() time.Time { return time.Time{} }
func (i fileInfo) IsDir() bool        { return i.entry.fileMode().IsDir() }
func (i fileInfo) Sys() any           { return nil }

func (e TreeEntry) fileMode() fs.FileMode {
	switch e.Mode {
	case ModeTree, ModeGitlink:
		return fs.ModeDir | 0o755
	case ModeLink:
		return fs.ModeSymlink | 0o777
	default:
		return 0o644
	}
}
