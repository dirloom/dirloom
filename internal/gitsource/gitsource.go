// Package gitsource reads committed Git trees for Git Tree Source v1. It runs
// the git executable with a fixed, hardened set of plumbing commands and
// treats everything Git returns as untrusted input.
package gitsource

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dirloom/dirloom/internal/artifact"
)

// The minimum Git version: GIT_NO_LAZY_FETCH appears in 2.45.
const (
	minimumMajor   = 2
	minimumMinor   = 45
	minimumVersion = "2.45"
)

// ObjectFormat is a repository's object id format. EmptyTree is the id of
// the empty tree, which Git serves in every repository of that format.
type ObjectFormat struct {
	Name      string
	RawSize   int
	EmptyTree string
}

// The object formats Git Tree Source v1 reads.
var (
	SHA1   = ObjectFormat{Name: "sha1", RawSize: 20, EmptyTree: "4b825dc642cb6eb9a060e54bf8d69288fbee4904"}
	SHA256 = ObjectFormat{Name: "sha256", RawSize: 32, EmptyTree: "6ef19b41225c5369f1c104d45d8d85efa9b057b53b14b4b9b939dd74decc5321"}
)

func objectFormatNamed(name string) (ObjectFormat, bool) {
	switch name {
	case SHA1.Name:
		return SHA1, true
	case SHA256.Name:
		return SHA256, true
	default:
		return ObjectFormat{}, false
	}
}

// isObjectID reports whether id is a full lowercase hex id of this format.
func (f ObjectFormat) isObjectID(id string) bool {
	if f.RawSize == 0 || len(id) != 2*f.RawSize {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// objectKind is what a batch read expects: a Git type and a size limit.
type objectKind struct {
	typ   string
	limit int64
	label string
}

// Per-object limits of Git Tree Source v1.
var (
	treeObject      = objectKind{typ: "tree", limit: 32 << 20, label: "tree object"}
	gitIgnoreObject = objectKind{typ: "blob", limit: 1 << 20, label: ".gitignore blob"}
	linkObject      = objectKind{typ: "blob", limit: 4096, label: "link target blob"}
)

func formatLimit(limit int64) string {
	if limit >= 1<<20 && limit%(1<<20) == 0 {
		return fmt.Sprintf("%d MiB", limit>>20)
	}
	return fmt.Sprintf("%d bytes", limit)
}

// ValidateRevision rejects a revision that must never reach Git. Such a
// revision is a usage error, reported before any Git command runs. An index
// entry (":<path>", ":<stage>:<path>") is refused because resolving it reads
// the index; ":/<text>" searches commit messages and stays allowed.
func ValidateRevision(revision string) error {
	switch {
	case revision == "":
		return errors.New("git revision must not be empty")
	case strings.HasPrefix(revision, "-"):
		return errors.New("git revision must not start with '-'")
	case strings.ContainsAny(revision, "\x00\n"):
		return errors.New("git revision must not contain NUL or a newline")
	case strings.HasPrefix(revision, ":") && !strings.HasPrefix(revision, ":/"):
		return errors.New("git revision must not name an index entry (:<path>); name a commit, a tag, or a tree such as HEAD:<path>")
	default:
		return nil
	}
}

func unavailable(format string, args ...any) error {
	return artifact.New(artifact.CodeGitUnavailable, fmt.Sprintf(format, args...), false)
}

func repositoryNotFound(format string, args ...any) error {
	return artifact.New(artifact.CodeGitRepositoryNotFound, fmt.Sprintf(format, args...), false)
}

func revisionNotFound(format string, args ...any) error {
	return artifact.New(artifact.CodeGitRevisionNotFound, fmt.Sprintf(format, args...), false)
}

func objectUnavailable(format string, args ...any) error {
	return artifact.New(artifact.CodeGitObjectUnavailable, fmt.Sprintf(format, args...), false)
}

func treeInvalid(format string, args ...any) error {
	return artifact.New(artifact.CodeGitTreeInvalid, fmt.Sprintf(format, args...), false)
}

func commitInvalid(format string, args ...any) error {
	return artifact.New(artifact.CodeGitCommitInvalid, fmt.Sprintf(format, args...), false)
}

func shallowInvalid(format string, args ...any) error {
	return artifact.New(artifact.CodeGitShallowInvalid, fmt.Sprintf(format, args...), false)
}

// quoteToken quotes a short, untrusted token for a diagnostic.
func quoteToken(token string) string {
	return quoteBounded(token, 32)
}

// quoteName quotes an untrusted entry name for a diagnostic.
func quoteName(name string) string {
	return quoteBounded(name, 255)
}

func quoteBounded(text string, limit int) string {
	if len(text) > limit {
		return fmt.Sprintf("%q...", text[:limit])
	}
	return fmt.Sprintf("%q", text)
}
