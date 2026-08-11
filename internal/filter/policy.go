package filter

import (
	"io/fs"
)

// Policy evaluates Dirloom's filtering layers in their specified order.
type Policy struct {
	OutputPath           string
	UseDefaultIgnores    bool
	Explicit             *IgnoreMatcher
	Git                  *GitIgnore
	IncludeHidden        bool
	pathsReferToSameFile func(string, string) bool
}

// NewPolicy creates a filter policy. OutputPath must already be absolute when
// present, as must the full path passed to Excludes.
func NewPolicy(outputPath string, useDefaults bool, explicit *IgnoreMatcher, git *GitIgnore, includeHidden bool) *Policy {
	return &Policy{
		OutputPath:           outputPath,
		UseDefaultIgnores:    useDefaults,
		Explicit:             explicit,
		Git:                  git,
		IncludeHidden:        includeHidden,
		pathsReferToSameFile: samePath,
	}
}

// Excludes reports whether a descendant should be omitted and, for a
// directory, pruned. The root is never passed to this method.
func (p *Policy) Excludes(fullPath, relativePath string, entry fs.DirEntry, info fs.FileInfo) bool {
	isDir := info.IsDir()
	name := entry.Name()

	if p.OutputPath != "" && p.pathsReferToSameFile(fullPath, p.OutputPath) {
		return true
	}
	if p.UseDefaultIgnores && isDir {
		if _, excluded := DefaultDirectories[name]; excluded {
			return true
		}
	}
	if p.Explicit != nil && p.Explicit.Match(relativePath, isDir) {
		return true
	}
	if p.Git != nil && p.Git.Match(relativePath, isDir) {
		return true
	}
	return !p.IncludeHidden && isHidden(fullPath, name, info)
}
