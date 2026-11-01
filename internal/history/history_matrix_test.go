package history

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/gittest"
)

func TestWalkModeOnlyCommitIsNotStructural(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("tool.sh", "#!/bin/sh")
	repo.CommitAll("first")
	repo.Git("update-index", "--chmod=+x", "tool.sh")
	modeOnly := repo.Commit("mode only")
	result := mustWalk(t, Request{Dir: repo.Dir})
	entry := result.Entries[0]
	if entry.Commit != modeOnly || entry.Structural || !entry.HasBase || entry.Fingerprint != result.Entries[1].Fingerprint || entry.Tree == result.Entries[1].Tree {
		t.Fatalf("mode-only commit %+v", entry)
	}
}

func TestWalkRevisionFormsGiveTheSameHistory(t *testing.T) {
	repo, ids := linearRepository(t)
	head := ids[4]
	repo.Git("tag", "--annotate", "--message", "release", "release", head)
	repo.Git("tag", "lightweight", head)
	want := fmt.Sprint(mustWalk(t, Request{Dir: repo.Dir, Revision: head}))
	for _, revision := range []string{"HEAD", "main", "release", "lightweight", head[:9], ":/E", head + "^{commit}"} {
		if got := fmt.Sprint(mustWalk(t, Request{Dir: repo.Dir, Revision: revision})); got != want {
			t.Fatalf("%s: history differs", revision)
		}
	}
	repo.Git("checkout", "--quiet", "--detach", head)
	if got := fmt.Sprint(mustWalk(t, Request{Dir: repo.Dir})); got != want {
		t.Fatal("detached HEAD: history differs")
	}
}

func TestWalkPartialCloneIsNeverFetched(t *testing.T) {
	origin := gittest.New(t)
	origin.Write("src/main.go", "package main")
	origin.Git("add", "--all")
	target := origin.Blob([]byte("src/main.go"))
	origin.Index("120000", target, "link")
	origin.Commit("first")
	origin.Write("docs/guide.md", "guide")
	origin.CommitAll("second")
	origin.Git("config", "uploadpack.allowfilter", "true")
	dir := filepath.Join(t.TempDir(), "clone")
	origin.Git("clone", "--quiet", "--no-checkout", "--filter=blob:none", fileURL(origin.Dir), dir)
	clone := gittest.Open(t, dir)
	// Even with the file protocol allowed, no lazy fetch may happen.
	clone.Git("config", "protocol.file.allow", "always")

	walkError(t, Request{Dir: dir}, artifact.CodeGitObjectUnavailable)
	t.Setenv("GIT_NO_LAZY_FETCH", "1")
	if _, err := clone.Run(nil, "cat-file", "-e", target); err == nil {
		t.Fatal("the link blob was fetched")
	}
}

// Git accepts commit headers that run to the end of the object, with no empty
// line and no message, and refuses a header line without its line feed.
// History accepts and refuses exactly the same objects.
func TestWalkCommitHeaderTermination(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("src/a.go", "package src")
	first := repo.CommitAll("first")
	tree := repo.Git("rev-parse", "HEAD^{tree}")
	headers := "tree " + tree + "\nparent " + first + "\nauthor A <a@x> 1767225600 +0000\ncommitter C <c@x> 1767225600 +0000"
	// Without --literally, git hash-object runs its fsck checks.
	accepted := repo.GitInput([]byte(headers+"\n"), "hash-object", "-t", "commit", "-w", "--stdin")
	result := mustWalk(t, Request{Dir: repo.Dir, Revision: accepted})
	if len(result.Entries) != 2 || result.Entries[0].Commit != accepted || result.Entries[0].Structural || result.End != EndRoot {
		t.Fatalf("headers to the end of the object: %+v", result)
	}
	if _, err := repo.Run([]byte(headers), "hash-object", "-t", "commit", "-w", "--stdin"); err == nil {
		t.Fatal("git accepted an unterminated header")
	}
	unterminated := repo.GitInput([]byte(headers), "hash-object", "-t", "commit", "-w", "--literally", "--stdin")
	walkError(t, Request{Dir: repo.Dir, Revision: unterminated}, artifact.CodeGitCommitInvalid)
}
