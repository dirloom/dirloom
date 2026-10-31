package history

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/dirloom/dirloom/internal/gittest"
	"github.com/dirloom/dirloom/internal/identity"
	"github.com/dirloom/dirloom/internal/source"
	"github.com/dirloom/dirloom/internal/tree"
)

func TestMain(m *testing.M) {
	code := gittest.Main(m)
	removeBenchmarkRepositories()
	os.Exit(code)
}

func mustWalk(t *testing.T, request Request) Result {
	t.Helper()
	if request.Limit == 0 {
		request.Limit = DefaultLimit
	}
	result, err := Walk(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func walkError(t *testing.T, request Request, code string) {
	t.Helper()
	if request.Limit == 0 {
		request.Limit = DefaultLimit
	}
	result, err := Walk(context.Background(), request)
	var typed *artifact.Error
	if !errors.As(err, &typed) || typed.Code != code || typed.Internal {
		t.Fatalf("error %v, want %s", err, code)
	}
	if result.To != "" || result.Entries != nil {
		t.Fatalf("a failed walk returned a result: %+v", result)
	}
}

func commits(result Result) []string {
	out := make([]string, len(result.Entries))
	for i, entry := range result.Entries {
		out[i] = entry.Commit
	}
	return out
}

func requireCommits(t *testing.T, result Result, end End, want ...string) {
	t.Helper()
	if got := strings.Join(commits(result), ","); got != strings.Join(want, ",") || result.End != end {
		t.Fatalf("commits %s end %s, want %s end %s", got, result.End, strings.Join(want, ","), end)
	}
}

// crossCheck compares every entry with what the diff path observes for the
// same pair: the summary of comparing git:<parent> with git:<commit>, and
// the fingerprint of git:<commit>.
func crossCheck(t *testing.T, dir string, result Result) {
	t.Helper()
	ctx := context.Background()
	gitSource := func(revision string) *source.Git {
		return &source.Git{Revision: revision, Dir: dir, Scan: tree.ScanOptions{RootName: "git"}}
	}
	for _, entry := range result.Entries {
		art, err := gitSource(entry.Commit).Observe(ctx)
		if err != nil {
			t.Fatal(err)
		}
		fingerprint, err := identity.Compute(*art)
		if err != nil || fingerprint != entry.Fingerprint || art.CountNodes() != entry.NodeCount {
			t.Fatalf("%s: fingerprint or node count differs from git:%s (%v)", entry.Commit, entry.Commit, err)
		}
		if !entry.HasBase {
			continue
		}
		base := "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
		if len(entry.Parents) > 0 {
			base = entry.Parents[0]
		}
		diff, err := comparison.Compare(ctx, gitSource(base), gitSource(entry.Commit), comparison.Options{MoveDetection: result.MoveDetection})
		if err != nil {
			t.Fatal(err)
		}
		if diff.Summary != entry.Summary || entry.Structural != (diff.Summary.Total > 0) {
			t.Fatalf("%s: summary %+v, diff %+v", entry.Commit, entry.Summary, diff.Summary)
		}
	}
}

// linearRepository builds A ← B ← C ← D ← E: B removes and adds, C changes
// contents only, D relocates a directory, E turns a file into a directory.
func linearRepository(t *testing.T) (*gittest.Repo, []string) {
	t.Helper()
	repo := gittest.New(t)
	repo.Write("src/a.go", "package src")
	repo.Write("src/b.go", "package src")
	repo.Write("old/payment/api/handler.go", "package api")
	repo.Write("old/payment/service.go", "package payment")
	repo.Write("node", "a file")
	a := repo.CommitAll("A")
	repo.Git("rm", "--quiet", "src/b.go")
	repo.Write("src/c.go", "package src")
	b := repo.CommitAll("B")
	repo.Write("src/a.go", "package src // rewritten")
	c := repo.CommitAll("C")
	repo.Git("mv", "old/payment", "payment")
	d := repo.CommitAll("D")
	repo.Git("rm", "--quiet", "node")
	repo.Write("node/x.go", "package node")
	e := repo.CommitAll("E")
	return repo, []string{a, b, c, d, e}
}

func TestWalkLinearHistory(t *testing.T) {
	repo, ids := linearRepository(t)
	a, b, c, d, e := ids[0], ids[1], ids[2], ids[3], ids[4]

	result := mustWalk(t, Request{Dir: repo.Dir})
	requireCommits(t, result, EndRoot, e, d, c, b, a)
	if result.To != e || result.From != "" || result.Limit != DefaultLimit || result.MoveDetection != comparison.MoveDetectionStructuralV1 {
		t.Fatalf("range %+v", result)
	}
	byCommit := map[string]Entry{}
	for _, entry := range result.Entries {
		byCommit[entry.Commit] = entry
	}
	if !byCommit[e].Structural || byCommit[e].Summary.Changed != 1 || byCommit[e].Summary.Added != 1 {
		t.Fatalf("kind change %+v", byCommit[e].Summary)
	}
	if byCommit[d].Summary.Moved != 1 || byCommit[d].Summary.Total != 2 {
		t.Fatalf("relocation %+v", byCommit[d].Summary)
	}
	if byCommit[c].Structural || byCommit[c].Summary.Total != 0 || byCommit[c].Fingerprint != byCommit[b].Fingerprint {
		t.Fatalf("contents-only commit %+v", byCommit[c])
	}
	if byCommit[a].Summary.Added != byCommit[a].NodeCount-1 || len(byCommit[a].Parents) != 0 {
		t.Fatalf("root commit %+v", byCommit[a])
	}
	if byCommit[e].CommittedAt.Format(time.RFC3339) != "2026-01-01T00:00:00Z" {
		t.Fatalf("committed at %v", byCommit[e].CommittedAt)
	}
	crossCheck(t, repo.Dir, result)

	plain := mustWalk(t, Request{Dir: repo.Dir, MoveDetection: comparison.MoveDetectionNone})
	moved := plain.Entries[1]
	if moved.Commit != d || moved.Summary.Moved != 0 || moved.Summary.Added != 4 || moved.Summary.Removed != 5 {
		t.Fatalf("relocation without moves %+v", moved.Summary)
	}
	crossCheck(t, repo.Dir, plain)

	structural := result.StructuralEntries()
	if len(structural) != 4 || structural[2].Commit != b {
		t.Fatalf("structural entries %v", structural)
	}
	if again := mustWalk(t, Request{Dir: repo.Dir}); fmt.Sprint(again) != fmt.Sprint(result) {
		t.Fatal("history is not deterministic")
	}
}

func TestWalkLimitAndFrom(t *testing.T) {
	repo, ids := linearRepository(t)
	_, b, c, d, e := ids[0], ids[1], ids[2], ids[3], ids[4]

	requireCommits(t, mustWalk(t, Request{Dir: repo.Dir, Limit: 1}), EndLimit, e)
	requireCommits(t, mustWalk(t, Request{Dir: repo.Dir, Limit: 99}), EndRoot, ids[4], ids[3], ids[2], ids[1], ids[0])

	// With E as D in the contract's example: --from C lists E, D.
	requireCommits(t, mustWalk(t, Request{Dir: repo.Dir, From: c}), EndFrom, e, d)
	requireCommits(t, mustWalk(t, Request{Dir: repo.Dir, From: c, Limit: 2}), EndFrom, e, d)
	requireCommits(t, mustWalk(t, Request{Dir: repo.Dir, From: c, Limit: 1}), EndLimit, e)
	empty := mustWalk(t, Request{Dir: repo.Dir, From: "HEAD"})
	requireCommits(t, empty, EndFrom)
	if empty.To != e || empty.From != e || empty.Entries == nil {
		t.Fatalf("empty history %+v", empty)
	}
	fromTag := mustWalk(t, Request{Dir: repo.Dir, Revision: d, From: b})
	requireCommits(t, fromTag, EndFrom, d, c)
	if fromTag.From != b {
		t.Fatalf("from %s", fromTag.From)
	}

	repo.Git("switch", "--quiet", "--create", "side", ids[1])
	repo.Write("side.txt", "side")
	side := repo.CommitAll("side")
	repo.Git("switch", "--quiet", "main")
	walkError(t, Request{Dir: repo.Dir, From: side}, artifact.CodeHistoryFromNotReached)
}

func TestWalkMergesFollowTheFirstParent(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("base.txt", "x")
	base := repo.CommitAll("base")
	for _, branch := range []string{"left", "right"} {
		repo.Git("switch", "--quiet", "--create", branch, base)
		repo.Write(branch+"/file", branch)
		repo.CommitAll(branch)
	}
	repo.Git("switch", "--quiet", "main")
	repo.Write("main.txt", "main")
	mainline := repo.CommitAll("main")
	repo.Git("merge", "--quiet", "--no-ff", "left", "-m", "merge left")
	merge := repo.Git("rev-parse", "HEAD")
	repo.Git("merge", "--quiet", "--no-ff", "right", "-m", "merge right")
	second := repo.Git("rev-parse", "HEAD")

	result := mustWalk(t, Request{Dir: repo.Dir})
	requireCommits(t, result, EndRoot, second, merge, mainline, base)
	if len(result.Entries[1].Parents) != 2 || result.Entries[1].Parents[1] != repo.Git("rev-parse", "left") {
		t.Fatalf("merge parents %v", result.Entries[1].Parents)
	}
	if result.Entries[1].Summary.Added != 2 {
		t.Fatalf("merge summary against its first parent %+v", result.Entries[1].Summary)
	}
	crossCheck(t, repo.Dir, result)

	octopus := gittest.New(t)
	octopus.Write("a", "a")
	root := octopus.CommitAll("root")
	for _, branch := range []string{"b1", "b2"} {
		octopus.Git("switch", "--quiet", "--create", branch, root)
		octopus.Write(branch, branch)
		octopus.CommitAll(branch)
	}
	octopus.Git("switch", "--quiet", "main")
	octopus.Write("m", "m")
	octopus.CommitAll("main")
	octopus.Git("merge", "--quiet", "--no-ff", "b1", "b2", "-m", "octopus")
	result = mustWalk(t, Request{Dir: octopus.Dir})
	if len(result.Entries) != 3 || len(result.Entries[0].Parents) != 3 || result.Entries[0].Summary.Added != 2 {
		t.Fatalf("octopus %+v", result.Entries)
	}
	crossCheck(t, octopus.Dir, result)
}

func TestWalkEmptyCommitsAndEmptyRoot(t *testing.T) {
	repo := gittest.New(t)
	empty := repo.CommitTree(repo.RawTree())
	repo.Git("update-ref", "refs/heads/main", empty)
	repo.Write("a.txt", "a")
	filled := repo.CommitAll("filled")
	same := repo.Commit("no change")

	result := mustWalk(t, Request{Dir: repo.Dir})
	requireCommits(t, result, EndRoot, same, filled, empty)
	if result.Entries[0].Structural || result.Entries[0].Tree != result.Entries[1].Tree {
		t.Fatalf("empty commit %+v", result.Entries[0])
	}
	if !result.Entries[1].Structural || result.Entries[1].Summary.Added != 1 {
		t.Fatalf("filled %+v", result.Entries[1])
	}
	if root := result.Entries[2]; root.Structural || !root.HasBase || root.NodeCount != 1 {
		t.Fatalf("empty root %+v", root)
	}
}

func TestWalkShallowBoundaries(t *testing.T) {
	origin, ids := linearRepository(t)
	clone := gittest.Open(t, filepath.Join(t.TempDir(), "shallow"))
	origin.Git("clone", "--quiet", "--depth", "2", fileURL(origin.Dir), clone.Dir)

	result := mustWalk(t, Request{Dir: clone.Dir})
	requireCommits(t, result, EndShallow, ids[4], ids[3])
	tail := result.Entries[1]
	if tail.HasBase || tail.Structural || tail.Summary.Total != 0 || len(tail.Parents) != 1 {
		t.Fatalf("shallow tail %+v", tail)
	}
	if structural := result.StructuralEntries(); len(structural) != 1 || structural[0].Commit != ids[4] {
		t.Fatalf("structural entries %v", structural)
	}
	walkError(t, Request{Dir: clone.Dir, From: ids[4] + "~5"}, artifact.CodeGitRevisionNotFound)

	// A missing parent that is not a recorded boundary stays an error, even
	// in a shallow repository.
	orphan := commitWithAbsentParent(clone)
	walkError(t, Request{Dir: clone.Dir, Revision: orphan}, artifact.CodeGitObjectUnavailable)

	full := gittest.New(t)
	full.Write("a", "a")
	full.CommitAll("a")
	walkError(t, Request{Dir: full.Dir, Revision: commitWithAbsentParent(full)}, artifact.CodeGitObjectUnavailable)
}

// commitWithAbsentParent writes a commit whose recorded parent object does
// not exist and is not a shallow boundary; git commit-tree refuses that.
func commitWithAbsentParent(repo *gittest.Repo) string {
	data := "tree " + repo.Git("rev-parse", "HEAD^{tree}") + "\nparent " + strings.Repeat("e", 40) +
		"\nauthor A <a@x> 1767225600 +0000\ncommitter C <c@x> 1767225600 +0000\n\norphan\n"
	return repo.GitInput([]byte(data), "hash-object", "-t", "commit", "-w", "--literally", "--stdin")
}

func TestWalkIgnoresReplaceRefsAndGrafts(t *testing.T) {
	repo, ids := linearRepository(t)
	recordedTree := repo.Git("rev-parse", ids[3]+"^{tree}")
	repo.Git("replace", ids[3], ids[0])
	if err := os.WriteFile(filepath.Join(repo.Dir, ".git", "info", "grafts"), []byte(ids[4]+" "+ids[1]+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := mustWalk(t, Request{Dir: repo.Dir})
	requireCommits(t, result, EndRoot, ids[4], ids[3], ids[2], ids[1], ids[0])
	if result.Entries[1].Tree != recordedTree {
		t.Fatalf("replaced commit observed as %s, recorded %s", result.Entries[1].Tree, recordedTree)
	}
}

func TestWalkRejectsInvalidCommitsAndRevisions(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("src/a.go", "package src")
	repo.CommitAll("first")
	tree := repo.Git("rev-parse", "HEAD^{tree}")
	for _, header := range []string{"committer C <c@x> 1 +1500", "committer C <c@x> 01 +0000", "author A <a@x> 1 +0000"} {
		data := "tree " + tree + "\n" + header + "\n\nbad\n"
		bad := repo.GitInput([]byte(data), "hash-object", "-t", "commit", "-w", "--literally", "--stdin")
		walkError(t, Request{Dir: repo.Dir, Revision: bad}, artifact.CodeGitCommitInvalid)
	}
	for _, revision := range []string{"HEAD:src", "HEAD^{tree}", "HEAD:src/a.go", "nope"} {
		walkError(t, Request{Dir: repo.Dir, Revision: revision}, artifact.CodeGitRevisionNotFound)
	}
	unborn := gittest.New(t)
	walkError(t, Request{Dir: unborn.Dir}, artifact.CodeGitRevisionNotFound)
	walkError(t, Request{Dir: t.TempDir()}, artifact.CodeGitRepositoryNotFound)
	if _, err := Walk(context.Background(), Request{Dir: repo.Dir, Limit: 0}); !artifact.IsInternal(err) {
		t.Fatalf("limit 0: %v", err)
	}
	if _, err := Walk(context.Background(), Request{Dir: repo.Dir, Limit: 1, Revision: "-x"}); !artifact.IsInternal(err) {
		t.Fatalf("unchecked revision: %v", err)
	}
}

func TestWalkFailsWithoutPartialResult(t *testing.T) {
	repo, ids := linearRepository(t)
	// Corrupt the tree object of the third commit (C).
	tree := repo.Git("rev-parse", ids[2]+"^{tree}")
	path := filepath.Join(repo.Dir, ".git", "objects", tree[:2], tree[2:])
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not a zlib stream"), 0o600); err != nil {
		t.Fatal(err)
	}
	walkError(t, Request{Dir: repo.Dir}, artifact.CodeGitObjectUnavailable)
	// Stopping before the corrupt tree is a normal history.
	requireCommits(t, mustWalk(t, Request{Dir: repo.Dir, Limit: 1}), EndLimit, ids[4])
}

func TestWalkSHA256AndLayouts(t *testing.T) {
	repo := gittest.New(t, "--object-format=sha256")
	repo.Write("a.txt", "a")
	first := repo.CommitAll("first")
	repo.Write("dir/b.txt", "b")
	second := repo.CommitAll("second")
	result := mustWalk(t, Request{Dir: repo.Dir})
	requireCommits(t, result, EndRoot, second, first)
	if len(result.Entries[0].Tree) != 64 || result.Entries[0].Summary.Added != 2 {
		t.Fatalf("sha256 %+v", result.Entries[0])
	}
	crossCheckSHA256Root := result.Entries[1]
	if crossCheckSHA256Root.Summary.Added != 1 {
		t.Fatalf("sha256 root %+v", crossCheckSHA256Root)
	}

	bare := filepath.Join(t.TempDir(), "bare.git")
	repo.Git("clone", "--quiet", "--bare", repo.Dir, bare)
	worktree := filepath.Join(t.TempDir(), "linked")
	repo.Git("worktree", "add", "--quiet", "--detach", worktree, "HEAD")
	for _, dir := range []string{bare, worktree, filepath.Join(repo.Dir, "dir")} {
		if other := mustWalk(t, Request{Dir: dir}); fmt.Sprint(other) != fmt.Sprint(result) {
			t.Fatalf("%s: history differs", dir)
		}
	}
}

// countdownContext is cancelled by its n-th Err call.
type countdownContext struct {
	context.Context
	mu        sync.Mutex
	remaining int
	done      chan struct{}
}

func (c *countdownContext) Done() <-chan struct{} { return c.done }

func (c *countdownContext) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.remaining > 0 {
		c.remaining--
		if c.remaining > 0 {
			return nil
		}
		close(c.done)
	}
	return context.Canceled
}

func TestWalkCancellation(t *testing.T) {
	repo, _ := linearRepository(t)
	cancelled := 0
	for _, n := range []int{1, 5, 20, 60, 200, 100000} {
		ctx := &countdownContext{Context: context.Background(), remaining: n, done: make(chan struct{})}
		start := time.Now()
		result, err := Walk(ctx, Request{Dir: repo.Dir, Limit: DefaultLimit})
		if elapsed := time.Since(start); elapsed > 20*time.Second {
			t.Fatalf("n=%d: walk took %v", n, elapsed)
		}
		if err != nil {
			if !strings.Contains(err.Error(), context.Canceled.Error()) {
				t.Fatalf("n=%d: %v", n, err)
			}
			if result.Entries != nil {
				t.Fatalf("n=%d: a cancelled walk returned entries", n)
			}
			cancelled++
		}
	}
	if cancelled < 3 {
		t.Fatalf("only %d walks were cancelled", cancelled)
	}
}

func fileURL(dir string) string {
	slashed := filepath.ToSlash(dir)
	if !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed
	}
	return "file://" + slashed
}
