package source

import (
	"strconv"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/gitsource"
	"github.com/dirloom/dirloom/internal/gittest"
	"github.com/dirloom/dirloom/internal/snapshot"
	"github.com/dirloom/dirloom/internal/tree"
)

// setGitLimits lowers the limits of a Git observation for one test.
func setGitLimits(t *testing.T, nodes, pathBytes int) {
	t.Helper()
	old := gitLimits
	gitLimits.nodes, gitLimits.pathBytes = nodes, pathBytes
	t.Cleanup(func() { gitLimits = old })
}

// mktree writes a tree of entries ("<mode> <type> <id>\t<name>") and
// returns its id.
func mktree(repo *gittest.Repo, entries []string) string {
	return repo.GitInput([]byte(strings.Join(entries, "\n")+"\n"), "mktree")
}

// gitBomb commits a tree of shared subtrees: a tree of files blobs, then
// levels trees of fan entries, each pointing at the tree below. A few
// hundred bytes of objects describe files × fan^levels files.
func gitBomb(t *testing.T, files, fan, levels int) (*gittest.Repo, string) {
	t.Helper()
	repo := gittest.New(t)
	blob := repo.GitInput([]byte("x\n"), "hash-object", "-w", "--stdin")
	entries := make([]string, files)
	for i := range entries {
		entries[i] = "100644 blob " + blob + "\tf" + strconv.Itoa(i)
	}
	id := mktree(repo, entries)
	for range levels {
		entries = make([]string, fan)
		for i := range entries {
			entries[i] = "040000 tree " + id + "\td" + strconv.Itoa(i)
		}
		id = mktree(repo, entries)
	}
	return repo, repo.CommitTree(id)
}

// TestGitObservationLimitsAreASnapshots: a Git observation holds no more
// than a snapshot may, and a live .gitignore no more than a Git one.
func TestGitObservationLimitsAreASnapshots(t *testing.T) {
	if GitMaxNodes != snapshot.MaxNodes || GitMaxPathBytes != snapshot.MaxBytes {
		t.Fatalf("Git limits (%d nodes, %d bytes) differ from the snapshot's (%d, %d)",
			GitMaxNodes, GitMaxPathBytes, snapshot.MaxNodes, snapshot.MaxBytes)
	}
	if tree.MaxGitIgnoreBytes != gitsource.MaxGitIgnoreBytes {
		t.Fatalf(".gitignore limits differ: live %d, Git %d", tree.MaxGitIgnoreBytes, gitsource.MaxGitIgnoreBytes)
	}
}

// TestGitBombStopsAtTheNodeLimit: 4 files under 10 × 10 shared trees are
// 400 files, 511 nodes with the root and the directories. One node over the
// limit fails with git_tree_too_large; exactly at the limit, the tree is
// observed.
func TestGitBombStopsAtTheNodeLimit(t *testing.T) {
	repo, commit := gitBomb(t, 4, 10, 2)
	setGitLimits(t, 510, 1<<30)
	_, _, err := observeGit(t, repo.Dir, commit, tree.ScanOptions{})
	requireGitCode(t, err, artifact.CodeGitTreeTooLarge, "more than 510 nodes", "the limit of a Git observation")

	setGitLimits(t, 511, 1<<30)
	art, _, err := observeGit(t, repo.Dir, commit, tree.ScanOptions{})
	if err != nil {
		t.Fatalf("observing exactly 511 nodes: %v", err)
	}
	if n := len(describe(art)); n != 511 {
		t.Fatalf("observed %d nodes, want 511", n)
	}
}

// TestDeepGitTreeStopsAtThePathLimit: the paths of a chain grow with the
// square of its depth, so a chain is bounded by its paths, not its nodes.
func TestDeepGitTreeStopsAtThePathLimit(t *testing.T) {
	repo := gittest.New(t)
	blob := repo.GitInput([]byte("x\n"), "hash-object", "-w", "--stdin")
	id := mktree(repo, []string{"100644 blob " + blob + "\tleaf"})
	const depth = 40
	for range depth {
		id = mktree(repo, []string{"040000 tree " + id + "\tdd"})
	}
	commit := repo.CommitTree(id)
	// Directory k (1-based) has a path of 3k-1 bytes; the leaf, 3·depth+4.
	total := 3*depth + 4
	for k := 1; k <= depth; k++ {
		total += 3*k - 1
	}

	setGitLimits(t, 1<<30, total-1)
	_, _, err := observeGit(t, repo.Dir, commit, tree.ScanOptions{})
	requireGitCode(t, err, artifact.CodeGitTreeTooLarge, "exceed "+strconv.Itoa(total-1)+" bytes")

	setGitLimits(t, 1<<30, total)
	if _, _, err := observeGit(t, repo.Dir, commit, tree.ScanOptions{}); err != nil {
		t.Fatalf("observing %d path bytes exactly: %v", total, err)
	}
}

// TestGitHistoryTreesHaveTheLimits: a history observes each tree through
// ObserveGitTree, under the same limits.
func TestGitHistoryTreesHaveTheLimits(t *testing.T) {
	repo, commit := gitBomb(t, 4, 10, 2)
	setGitLimits(t, 100, 1<<30)
	session, err := gitsource.OpenSession(t.Context(), repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session.Close() }()
	resolved, err := session.Resolve(t.Context(), commit)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ObserveGitTree(t.Context(), session, resolved.Tree, tree.ScanOptions{RootName: "repo"})
	requireGitCode(t, err, artifact.CodeGitTreeTooLarge, "more than 100 nodes")
}
