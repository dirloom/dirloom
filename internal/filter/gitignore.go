package filter

import (
	"bufio"
	"bytes"

	gitignore "github.com/git-pkgs/gitignore"

	"github.com/dirloom/dirloom/internal/artifact"
)

// GitIgnore encapsulates the third-party wildmatch implementation so the
// scanner and tree model do not depend on it directly.
type GitIgnore struct {
	matcher *gitignore.Matcher
}

// NewGitIgnore deliberately avoids global and .git/info excludes. Dirloom's
// v0.1 contract starts at the explicitly inspected root.
func NewGitIgnore() *GitIgnore {
	return &GitIgnore{matcher: gitignore.New("")}
}

// AddPatterns adds one .gitignore file scoped to its containing directory.
// The backends refuse a line above MaxGitIgnoreLine before the data gets
// here; one that still arrives is an internal error, never a silent loss of
// rules.
func (g *GitIgnore) AddPatterns(data []byte, relativeDirectory, source string) error {
	if g == nil {
		return nil
	}
	if line := LongGitIgnoreLine(data); line != 0 {
		return artifact.InternalError("gitignore %q reached the matcher with line %d above the line limit", source, line)
	}
	g.matcher.AddPatterns(data, relativeDirectory)
	// Git treats malformed patterns as non-matches rather than making the
	// command fail. The dependency records them through Errors(), but Dirloom
	// deliberately preserves Git's silent behavior in v0.1.
	return nil
}

// Match applies last-match-wins Git semantics to a normalized relative path.
func (g *GitIgnore) Match(relativePath string, isDir bool) bool {
	return g != nil && g.matcher.MatchPath(relativePath, isDir)
}

// MaxGitIgnoreLine is the longest .gitignore line the matcher reads, in
// bytes before its newline: 65,535 bytes, one less than 64 KiB. The
// dependency reads lines with a bufio.Scanner that stops at a longer line
// without reporting it, which drops that line and every rule after it.
// Git has no such limit, so Dirloom refuses the file instead (D7 of the
// v0.4.0-rc1 plan).
const MaxGitIgnoreLine = bufio.MaxScanTokenSize - 1

// LongGitIgnoreLine returns the number, from 1, of the first line of data
// above MaxGitIgnoreLine, counted up to its newline or the end of data, or
// 0 when every line fits.
func LongGitIgnoreLine(data []byte) int {
	for number := 1; len(data) > 0; number++ {
		end := bytes.IndexByte(data, '\n')
		if end < 0 {
			end = len(data)
		}
		if end > MaxGitIgnoreLine {
			return number
		}
		data = data[min(end+1, len(data)):]
	}
	return 0
}
