// Package gittest builds throwaway Git repositories for tests with the git
// CLI, pinned identities and dates, and no user or system configuration.
// Only tests import it.
package gittest

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Main isolates every git process of a test binary from the user's and the
// system's configuration and from any enclosing repository, then runs the
// tests. Call it from TestMain.
func Main(m *testing.M) int {
	dir, err := os.MkdirTemp("", "dirloom-gittest-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() { _ = os.RemoveAll(dir) }()
	global := filepath.Join(dir, "gitconfig")
	if err := os.WriteFile(global, nil, 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, key := range []string{
		"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY",
		"GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_COMMON_DIR", "GIT_NAMESPACE",
		"GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT",
	} {
		_ = os.Unsetenv(key)
	}
	settings := map[string]string{
		"GIT_CONFIG_NOSYSTEM":     "1",
		"GIT_CONFIG_GLOBAL":       global,
		"GIT_CEILING_DIRECTORIES": os.TempDir(),
		"GIT_AUTHOR_NAME":         "Dirloom Test",
		"GIT_AUTHOR_EMAIL":        "test@dirloom.invalid",
		"GIT_AUTHOR_DATE":         "2026-01-01T00:00:00Z",
		"GIT_COMMITTER_NAME":      "Dirloom Test",
		"GIT_COMMITTER_EMAIL":     "test@dirloom.invalid",
		"GIT_COMMITTER_DATE":      "2026-01-01T00:00:00Z",
	}
	for key, value := range settings {
		if err := os.Setenv(key, value); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	return m.Run()
}

// Require skips the test when git 2.45 or newer is unavailable, and fails it
// instead on CI, where the Git matrix must run.
func Require(t testing.TB) {
	t.Helper()
	out, err := exec.Command("git", "version").Output()
	var major, minor int
	if err == nil {
		_, err = fmt.Sscanf(string(out), "git version %d.%d", &major, &minor)
	}
	if err == nil && (major > 2 || (major == 2 && minor >= 45)) {
		return
	}
	reason := fmt.Sprintf("git 2.45 or newer is required (version output %q, error %v)", strings.TrimSpace(string(out)), err)
	if os.Getenv("CI") != "" {
		t.Fatal(reason)
	}
	t.Skip(reason)
}

// Repo is a repository in a test-owned temporary directory.
type Repo struct {
	t   testing.TB
	Dir string
}

// New creates an empty repository on branch main. initArgs are extra
// `git init` arguments, such as --bare or --object-format=sha256.
func New(t testing.TB, initArgs ...string) *Repo {
	t.Helper()
	Require(t)
	repo := &Repo{t: t, Dir: t.TempDir()}
	repo.Git(append([]string{"init", "--quiet", "--initial-branch=main"}, initArgs...)...)
	return repo
}

// Open wraps an existing directory, such as a clone or a linked worktree.
func Open(t testing.TB, dir string) *Repo {
	return &Repo{t: t, Dir: dir}
}

// Git runs git in the repository and returns its standard output without
// the final line terminator. A failure fails the test.
func (r *Repo) Git(args ...string) string {
	r.t.Helper()
	return r.GitInput(nil, args...)
}

// GitInput is Git with input on standard input.
func (r *Repo) GitInput(input []byte, args ...string) string {
	r.t.Helper()
	out, err := r.Run(input, args...)
	if err != nil {
		r.t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimRight(out, "\r\n")
}

// Run runs git and returns standard output, or an error that includes
// standard error.
func (r *Repo) Run(input []byte, args ...string) (string, error) {
	// #nosec G204 -- test helper running git with test-chosen arguments.
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// Write creates or replaces a file of the work tree, with its parents.
func (r *Repo) Write(path, content string) {
	r.t.Helper()
	full := filepath.Join(r.Dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		r.t.Fatal(err)
	}
}

// CommitAll stages the whole work tree, commits, and returns the commit id.
func (r *Repo) CommitAll(message string) string {
	r.t.Helper()
	r.Git("add", "--all")
	r.Git("commit", "--quiet", "--allow-empty", "--message", message)
	return r.Git("rev-parse", "HEAD")
}

// Commit commits the index as it is and returns the commit id.
func (r *Repo) Commit(message string) string {
	r.t.Helper()
	r.Git("commit", "--quiet", "--allow-empty", "--message", message)
	return r.Git("rev-parse", "HEAD")
}

// Blob writes content as a blob and returns its id.
func (r *Repo) Blob(content []byte) string {
	r.t.Helper()
	return r.GitInput(content, "hash-object", "-w", "--stdin")
}

// Index adds an entry with an explicit mode to the index, without a work
// tree file. It creates links and submodules on every platform.
func (r *Repo) Index(mode, id, path string) {
	r.t.Helper()
	r.Git("update-index", "--add", "--cacheinfo", mode+","+id+","+path)
}

// Entry is one raw tree entry; ID is hex.
type Entry struct {
	Mode string
	Name string
	ID   string
}

// RawTree writes a tree object from entries exactly as given, without Git's
// validation or ordering, and returns its id.
func (r *Repo) RawTree(entries ...Entry) string {
	r.t.Helper()
	var data []byte
	for _, entry := range entries {
		id, err := hex.DecodeString(entry.ID)
		if err != nil {
			r.t.Fatalf("entry %q: %v", entry.Name, err)
		}
		data = append(data, entry.Mode...)
		data = append(data, ' ')
		data = append(data, entry.Name...)
		data = append(data, 0)
		data = append(data, id...)
	}
	return r.GitInput(data, "hash-object", "-t", "tree", "-w", "--literally", "--stdin")
}

// CommitTree writes a commit of tree with the given parents and returns its
// id.
func (r *Repo) CommitTree(tree string, parents ...string) string {
	r.t.Helper()
	args := []string{"commit-tree", tree, "-m", "tree " + tree}
	for _, parent := range parents {
		args = append(args, "-p", parent)
	}
	return r.Git(args...)
}
