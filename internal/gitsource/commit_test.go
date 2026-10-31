package gitsource

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/gittest"
)

var (
	treeID   = strings.Repeat("a", 40)
	parentA  = strings.Repeat("b", 40)
	parentB  = strings.Repeat("c", 40)
	commitID = strings.Repeat("d", 40)
)

func commitObjectText(headers ...string) []byte {
	return []byte(strings.Join(headers, "\n") + "\n\nmessage\nwith lines\n")
}

func TestParseCommitAccepts(t *testing.T) {
	cases := map[string]struct {
		data    []byte
		parents []string
		at      string
	}{
		"root": {commitObjectText("tree "+treeID, "author A <a@x> 1 +0000", "committer C <c@x> 1767225600 +0000"), nil, "2026-01-01T00:00:00Z"},
		"merge": {commitObjectText("tree "+treeID, "parent "+parentA, "parent "+parentB, "author A <a@x> 1 +0000", "committer C <c@x> 1767225600 -0530"),
			[]string{parentA, parentB}, "2026-01-01T00:00:00Z"},
		"zone is dropped":   {commitObjectText("tree "+treeID, "committer C <c@x> 1767225600 +1400"), nil, "2026-01-01T00:00:00Z"},
		"epoch and maximum": {commitObjectText("tree "+treeID, "committer C D <c@x> 0 +0000"), nil, "1970-01-01T00:00:00Z"},
		"multi-line headers": {commitObjectText("tree "+treeID, "parent "+parentA, "author A <a@x> 1 +0000", "committer C <c@x> 1767225600 +0000",
			"encoding ISO-8859-1", "gpgsig -----BEGIN PGP SIGNATURE-----", " line one", " ", " -----END PGP SIGNATURE-----", "mergetag object "+parentB, " type commit"),
			[]string{parentA}, "2026-01-01T00:00:00Z"},
		"no message":                        {[]byte("tree " + treeID + "\ncommitter C <c@x> 1767225600 +0000\n"), nil, "2026-01-01T00:00:00Z"},
		"identity with spaces and no email": {commitObjectText("tree "+treeID, "committer Some Body Else 1767225600 +0100"), nil, "2026-01-01T00:00:00Z"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			commit, err := ParseCommit(commitID, tc.data, SHA1)
			if err != nil {
				t.Fatal(err)
			}
			if commit.ID != commitID || commit.Tree != treeID || strings.Join(commit.Parents, ",") != strings.Join(tc.parents, ",") {
				t.Fatalf("%+v", commit)
			}
			if got := commit.CommittedAt.Format(time.RFC3339); got != tc.at || commit.CommittedAt.Location() != time.UTC {
				t.Fatalf("committed at %s (%v)", got, commit.CommittedAt.Location())
			}
		})
	}
	maximum, err := ParseCommit(commitID, commitObjectText("tree "+treeID, "committer C <c@x> 253402300799 +0000"), SHA1)
	if err != nil || maximum.CommittedAt.Format(time.RFC3339) != "9999-12-31T23:59:59Z" {
		t.Fatalf("maximum timestamp: %v %v", maximum.CommittedAt, err)
	}
	sha256Tree := strings.Repeat("e", 64)
	if commit, err := ParseCommit(strings.Repeat("f", 64), commitObjectText("tree "+sha256Tree, "parent "+strings.Repeat("1", 64), "committer C <c@x> 1 +0000"), SHA256); err != nil || commit.Tree != sha256Tree {
		t.Fatalf("sha256: %+v %v", commit, err)
	}
}

func TestParseCommitRejects(t *testing.T) {
	committer := "committer C <c@x> 1767225600 +0000"
	cases := map[string][]byte{
		"empty":                      nil,
		"no tree":                    commitObjectText("parent "+parentA, committer),
		"author first":               commitObjectText("author A <a@x> 1 +0000", "tree "+treeID, committer),
		"two trees":                  commitObjectText("tree "+treeID, "tree "+treeID, committer),
		"tree after parent":          commitObjectText("tree "+treeID, "parent "+parentA, "tree "+treeID, committer),
		"parent after author":        commitObjectText("tree "+treeID, "author A <a@x> 1 +0000", "parent "+parentA, committer),
		"no committer":               commitObjectText("tree "+treeID, "author A <a@x> 1 +0000"),
		"two committers":             commitObjectText("tree "+treeID, committer, committer),
		"uppercase tree id":          commitObjectText("tree "+strings.ToUpper(treeID), committer),
		"short parent id":            commitObjectText("tree "+treeID, "parent "+parentA[:39], committer),
		"sha256 id in sha1":          commitObjectText("tree "+strings.Repeat("a", 64), committer),
		"continuation after tree":    commitObjectText("tree "+treeID, " folded", committer),
		"continuation first":         []byte(" folded\ntree " + treeID + "\n" + committer + "\n"),
		"header without value":       commitObjectText("tree "+treeID, "lonely", committer),
		"zone +1500":                 commitObjectText("tree "+treeID, "committer C <c@x> 1767225600 +1500"),
		"zone +0060":                 commitObjectText("tree "+treeID, "committer C <c@x> 1767225600 +0060"),
		"zone without sign":          commitObjectText("tree "+treeID, "committer C <c@x> 1767225600 0000"),
		"zone too short":             commitObjectText("tree "+treeID, "committer C <c@x> 1767225600 +000"),
		"signed timestamp":           commitObjectText("tree "+treeID, "committer C <c@x> -1 +0000"),
		"plus timestamp":             commitObjectText("tree "+treeID, "committer C <c@x> +1767225600 +0000"),
		"leading zero timestamp":     commitObjectText("tree "+treeID, "committer C <c@x> 01767225600 +0000"),
		"timestamp above maximum":    commitObjectText("tree "+treeID, "committer C <c@x> 253402300800 +0000"),
		"timestamp far too long":     commitObjectText("tree "+treeID, "committer C <c@x> 99999999999999999999 +0000"),
		"hex timestamp":              commitObjectText("tree "+treeID, "committer C <c@x> 1a +0000"),
		"missing timestamp":          commitObjectText("tree "+treeID, "committer C <c@x> +0000"),
		"committer without anything": commitObjectText("tree "+treeID, "committer 1767225600 +0000"),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ParseCommit(commitID, data, SHA1)
			requireCode(t, err, artifact.CodeGitCommitInvalid)
			requireMessage(t, err, commitID)
		})
	}
}

func FuzzParseCommit(f *testing.F) {
	f.Add(commitObjectText("tree "+treeID, "parent "+parentA, "committer C <c@x> 1767225600 +0000"))
	f.Add(commitObjectText("tree "+treeID, "committer C <c@x> 253402300799 -1200", "gpgsig x", " y"))
	f.Add([]byte("tree " + treeID + "\ncommitter C <c@x> 0 +0000"))
	f.Fuzz(func(t *testing.T, data []byte) {
		commit, err := ParseCommit(commitID, data, SHA1)
		if err != nil {
			var typed *artifact.Error
			if !asArtifactError(err, &typed) || typed.Code != artifact.CodeGitCommitInvalid {
				t.Fatalf("unexpected error %v", err)
			}
			return
		}
		if !SHA1.isObjectID(commit.Tree) || !strings.HasPrefix(string(data), "tree "+commit.Tree) {
			t.Fatalf("accepted tree %q", commit.Tree)
		}
		for _, parent := range commit.Parents {
			if !SHA1.isObjectID(parent) || !strings.Contains(string(data), "\nparent "+parent) {
				t.Fatalf("accepted parent %q", parent)
			}
		}
		if seconds := commit.CommittedAt.Unix(); seconds < 0 || seconds > maxCommitTimestamp || commit.CommittedAt.Location() != time.UTC {
			t.Fatalf("committed at %v", commit.CommittedAt)
		}
	})
}

func TestParseShallow(t *testing.T) {
	for name, tc := range map[string]struct {
		data string
		want int
	}{
		"empty":               {"", 0},
		"one":                 {parentA + "\n", 1},
		"two without newline": {parentA + "\n" + parentB, 2},
		"duplicate":           {parentA + "\n" + parentA + "\n", 1},
	} {
		set, err := parseShallow([]byte(tc.data), SHA1)
		if err != nil || len(set) != tc.want {
			t.Fatalf("%s: %v %v", name, set, err)
		}
	}
	for name, data := range map[string]string{
		"blank line":     parentA + "\n\n",
		"leading blank":  "\n" + parentA,
		"short id":       parentA[:39] + "\n",
		"uppercase":      strings.ToUpper(parentA) + "\n",
		"trailing space": parentA + " \n",
		"crlf":           parentA + "\r\n",
		"sha256 in sha1": strings.Repeat("a", 64) + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseShallow([]byte(data), SHA1)
			requireCode(t, err, artifact.CodeGitShallowInvalid)
		})
	}
}

func FuzzParseShallow(f *testing.F) {
	f.Add([]byte(parentA + "\n" + parentB + "\n"))
	f.Add([]byte(parentA))
	f.Fuzz(func(t *testing.T, data []byte) {
		set, err := parseShallow(data, SHA1)
		if err != nil {
			requireCode(t, err, artifact.CodeGitShallowInvalid)
			return
		}
		for id := range set {
			if !SHA1.isObjectID(id) {
				t.Fatalf("accepted %q", id)
			}
		}
	})
}

func openSession(t *testing.T, dir string) *Session {
	t.Helper()
	session, err := OpenSession(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestSessionReadsCommits(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("a.txt", "a")
	root := repo.CommitAll("root")
	repo.Git("switch", "--quiet", "--create", "side")
	repo.Write("b.txt", "b")
	side := repo.CommitAll("side")
	repo.Git("switch", "--quiet", "main")
	repo.Write("c.txt", "c")
	mainline := repo.CommitAll("main")
	repo.Git("merge", "--quiet", "--no-ff", "side", "-m", "merge")
	merge := repo.Git("rev-parse", "HEAD")
	session := openSession(t, repo.Dir)

	commit, err := session.ReadCommit(merge)
	if err != nil {
		t.Fatal(err)
	}
	if commit.Tree != repo.Git("rev-parse", "HEAD^{tree}") || len(commit.Parents) != 2 || commit.Parents[0] != mainline || commit.Parents[1] != side {
		t.Fatalf("merge %+v", commit)
	}
	if commit.CommittedAt.Format(time.RFC3339) != "2026-01-01T00:00:00Z" {
		t.Fatalf("committed at %v", commit.CommittedAt)
	}
	first, err := session.ReadCommit(root)
	if err != nil || len(first.Parents) != 0 {
		t.Fatalf("root %+v %v", first, err)
	}

	// A missing object is reported as missing, and the session stays usable.
	_, err = session.ReadCommit(strings.Repeat("0", 40))
	if !IsMissing(err) {
		t.Fatalf("missing commit: %v", err)
	}
	requireCode(t, err, artifact.CodeGitObjectUnavailable)
	if again, err := session.ReadCommit(root); err != nil || again.Tree != first.Tree {
		t.Fatalf("read after a missing object: %+v %v", again, err)
	}
	// A tree is not a commit: the protocol position is lost from there.
	_, err = session.ReadCommit(commit.Tree)
	requireCode(t, err, artifact.CodeGitTreeInvalid)
	if IsMissing(err) {
		t.Fatal("a wrong type is not missing")
	}
}

func TestSessionResolvesCommits(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("src/a.go", "package src")
	first := repo.CommitAll("first commit")
	repo.Write("b.txt", "b")
	second := repo.CommitAll("second commit")
	repo.Git("tag", "--annotate", "--message", "v1", "v1", first)
	repo.Git("tag", "--annotate", "--message", "nested", "nested", "v1")
	repo.Git("tag", "--annotate", "--message", "tree", "treetag", repo.Git("rev-parse", "HEAD^{tree}"))
	session := openSession(t, repo.Dir)
	ctx := context.Background()

	for revision, want := range map[string]string{
		"HEAD": second, "main": second, "HEAD~1": first, second[:10]: second, "v1": first, "nested": first,
		":/first commit": first, "HEAD^{/first}": first,
	} {
		got, err := session.ResolveCommit(ctx, revision)
		if err != nil || got != want {
			t.Fatalf("%s: %s %v, want %s", revision, got, err, want)
		}
	}
	for _, revision := range []string{"HEAD:src", "HEAD^{tree}", "treetag", "HEAD:b.txt", "nope", "HEAD~5", strings.Repeat("f", 40)} {
		_, err := session.ResolveCommit(ctx, revision)
		requireCode(t, err, artifact.CodeGitRevisionNotFound)
	}
	if _, err := session.ResolveCommit(ctx, ":src/a.go"); !artifact.IsInternal(err) {
		t.Fatalf("unchecked index revision: %v", err)
	}
}

func TestSessionShallowBoundaries(t *testing.T) {
	origin := gittest.New(t)
	for i := 1; i <= 4; i++ {
		origin.Write(strings.Repeat("f", i), "x")
		origin.CommitAll("commit")
	}
	url := fileURL(origin.Dir)
	base := t.TempDir()
	shallowDir := filepath.Join(base, "shallow")
	origin.Git("clone", "--quiet", "--depth", "2", url, shallowDir)
	bareDir := filepath.Join(base, "bare.git")
	origin.Git("clone", "--quiet", "--bare", "--depth", "2", url, bareDir)
	shallow := gittest.Open(t, shallowDir)
	worktree := filepath.Join(base, "linked")
	shallow.Git("worktree", "add", "--quiet", "--detach", worktree, "HEAD")
	boundary := shallow.Git("rev-parse", "HEAD~1")
	if err := os.MkdirAll(filepath.Join(shallowDir, "sub", "dir"), 0o750); err != nil {
		t.Fatal(err)
	}

	for name, dir := range map[string]string{
		"checkout":     shallowDir,
		"subdirectory": filepath.Join(shallowDir, "sub", "dir"),
		"worktree":     worktree,
		"bare":         bareDir,
	} {
		set, err := openSession(t, dir).ShallowBoundaries(context.Background())
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, ok := set[boundary]; !ok || len(set) != 1 {
			t.Fatalf("%s: boundaries %v, want %s", name, set, boundary)
		}
	}

	full, err := openSession(t, origin.Dir).ShallowBoundaries(context.Background())
	if err != nil || len(full) != 0 {
		t.Fatalf("full repository: %v %v", full, err)
	}

	list := filepath.Join(shallowDir, ".git", "shallow")
	if err := os.WriteFile(list, []byte("not an id\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = openSession(t, shallowDir).ShallowBoundaries(context.Background())
	requireCode(t, err, artifact.CodeGitShallowInvalid)
	if err := os.WriteFile(list, []byte(strings.Repeat(boundary+"\n", maxShallowList/41+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = openSession(t, shallowDir).ShallowBoundaries(context.Background())
	requireCode(t, err, artifact.CodeGitShallowInvalid)
	requireMessage(t, err, "16 MiB limit")
}

func fileURL(dir string) string {
	slashed := filepath.ToSlash(dir)
	if !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed
	}
	return "file://" + slashed
}
