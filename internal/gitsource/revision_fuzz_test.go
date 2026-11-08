package gitsource

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/gittest"
)

// FuzzResolveRevision is the Git ref fuzzer of D9 (v0.4.0-rc1 plan): any
// revision the CLI lets through (ValidateRevision) goes to the real Git on
// a small repository holding a branch, an annotated tag, a lightweight tag,
// a tag of a tree, a directory and a file. Whatever the text, resolution
// never panics nor hangs, and it either names full object ids or fails with
// a typed Git observation code, never an internal error.
func FuzzResolveRevision(f *testing.F) {
	for _, seed := range []string{
		"HEAD", "main", "HEAD~1", "HEAD^", "HEAD^{tree}", "HEAD^{commit}", "HEAD^{blob}", "HEAD:dir", "HEAD:file.txt",
		"HEAD:missing", "v1", "v1^{}", "light", "treetag", ":/second", ":/nothing matches", "HEAD@{0}", "@", "@{-1}",
		"main..HEAD", "main...HEAD", "HEAD~100", "0000", "deadbeef", "refs/heads/main", "refs/tags/v1", "HEAD:",
		"HEAD^{/second}", "v1^{tag}", "HEAD:dir/", "HEAD:./file.txt", "HEAD^-", "HEAD^!", "--all", "-x", ":0:file.txt",
		"ma\x00in", "\xff\xfe", "HEAD\ttab", " HEAD", "HEAD ", "~", "^", "@{u}", "HEAD@{1.year.ago}", "a..b..c",
	} {
		f.Add(seed)
	}
	gittest.Require(f)
	repo := gittest.New(f)
	repo.Write("file.txt", "first\n")
	repo.Write("dir/inner.txt", "inner\n")
	repo.CommitAll("first")
	repo.Write("file.txt", "second\n")
	repo.CommitAll("second")
	repo.Git("tag", "-a", "v1", "-m", "release")
	repo.Git("tag", "light")
	repo.Git("tag", "treetag", repo.Git("rev-parse", "HEAD^{tree}"))
	session, err := OpenSession(context.Background(), repo.Dir)
	if err != nil {
		f.Fatal(err)
	}
	f.Cleanup(func() { _ = session.Close() })

	f.Fuzz(func(t *testing.T, revision string) {
		if ValidateRevision(revision) != nil {
			return // the CLI refuses it, exit 2, before Git runs
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		resolved, err := session.Resolve(ctx, revision)
		if ctx.Err() != nil {
			t.Fatalf("resolving %q did not end within 30 s", revision)
		}
		if err == nil {
			if !resolved.Format.isObjectID(resolved.Tree) || (resolved.Commit != "" && !resolved.Format.isObjectID(resolved.Commit)) {
				t.Fatalf("resolving %q gave tree %q, commit %q", revision, resolved.Tree, resolved.Commit)
			}
			return
		}
		var typed *artifact.Error
		if !errors.As(err, &typed) || typed.Internal ||
			(typed.Code != artifact.CodeGitRevisionNotFound && typed.Code != artifact.CodeGitObjectUnavailable) {
			t.Fatalf("resolving %q failed with %#v, want git_revision_not_found or git_object_unavailable", revision, err)
		}
	})
}
