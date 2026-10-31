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
