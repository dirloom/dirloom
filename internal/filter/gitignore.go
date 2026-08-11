package filter

import gitignore "github.com/git-pkgs/gitignore"

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
func (g *GitIgnore) AddPatterns(data []byte, relativeDirectory, _ string) error {
	if g == nil {
		return nil
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
