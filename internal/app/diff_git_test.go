package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/dirloom/dirloom/internal/gittest"
	"github.com/dirloom/dirloom/internal/source"
)

func TestMain(m *testing.M) {
	os.Exit(gittest.Main(m))
}

func gitSpec(revision string) DiffSourceSpec {
	return DiffSourceSpec{Kind: DiffSourceGit, Path: revision}
}

// changeLines lists changes as "OP path" or "MOVED from -> path (nodes)".
func changeLines(diff comparison.StructuralDiff) []string {
	var lines []string
	for _, change := range diff.Changes {
		if change.Op == comparison.OpMoved {
			lines = append(lines, fmt.Sprintf("MOVED %s -> %s (%d)", change.From, change.Path, change.Nodes))
			continue
		}
		lines = append(lines, string(change.Op)+" "+string(change.Path))
	}
	return lines
}

func requireChanges(t *testing.T, diff comparison.StructuralDiff, want ...string) {
	t.Helper()
	if got := changeLines(diff); !slices.Equal(got, want) {
		t.Fatalf("changes %q, want %q", got, want)
	}
}

func requireGitRef(t *testing.T, ref comparison.SourceRef, repo *gittest.Repo, revision string, withCommit bool) {
	t.Helper()
	tree := repo.Git("rev-parse", revision+"^{tree}")
	commit := ""
	if withCommit {
		commit = repo.Git("rev-parse", revision+"^{commit}")
	}
	if ref.Kind != source.KindGit || ref.Tree != tree || ref.Commit != commit {
		t.Fatalf("source %+v, want tree %s commit %q", ref, tree, commit)
	}
}

func TestDiffGitAgainstGit(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("README.md", "readme")
	repo.Write("src/old/payment/api/handler.go", "package api")
	repo.Write("src/old/payment/service.go", "package payment")
	first := repo.CommitAll("first")
	repo.Git("mv", "src/old/payment", "src/payment")
	repo.Write("docs/guide.md", "guide")
	second := repo.CommitAll("second")
	t.Chdir(repo.Dir)

	diff := mustDiff(t, DiffRequest{A: gitSpec(first), B: gitSpec(second)})
	requireChanges(t, diff,
		"ADDED docs",
		"ADDED docs/guide.md",
		"REMOVED src/old",
		"MOVED src/old/payment -> src/payment (4)",
	)
	requireGitRef(t, diff.SourceA, repo, first, true)
	requireGitRef(t, diff.SourceB, repo, second, true)

	// Ancestry expressions resolve to the same commits.
	relative := mustDiff(t, DiffRequest{A: gitSpec("HEAD~1"), B: gitSpec("HEAD")})
	if !slices.Equal(changeLines(relative), changeLines(diff)) || relative.SourceA != diff.SourceA || relative.SourceB != diff.SourceB {
		t.Fatalf("HEAD~1..HEAD differs from explicit ids: %+v", relative)
	}

	// A subtree revision reports its tree and no commit: src/old before the
	// move and src after it hold the same payment subtree.
	subtree := mustDiff(t, DiffRequest{A: gitSpec(first + ":src/old"), B: gitSpec("HEAD:src")})
	if DiffStatusOf(subtree) != DiffNoDifferences {
		t.Fatalf("subtrees: %q", changeLines(subtree))
	}
	if subtree.SourceA.Commit != "" || subtree.SourceA.Tree != repo.Git("rev-parse", first+":src/old") || subtree.SourceB.Commit != "" {
		t.Fatalf("subtree sources %+v %+v", subtree.SourceA, subtree.SourceB)
	}
}

func TestDiffGitStructuralNature(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("src/app.go", "contents A")
	repo.Write("tool.sh", "#!/bin/sh")
	first := repo.CommitAll("first")
	repo.Write("src/app.go", "contents B, entirely rewritten")
	repo.Git("add", "--all")
	repo.Git("update-index", "--chmod=+x", "tool.sh")
	second := repo.Commit("contents and mode only")
	t.Chdir(repo.Dir)

	diff := mustDiff(t, DiffRequest{A: gitSpec(first), B: gitSpec(second)})
	if DiffStatusOf(diff) != DiffNoDifferences {
		t.Fatalf("contents-only change = %+v", diff.Changes)
	}
	if diff.SourceA.Tree == diff.SourceB.Tree {
		t.Fatal("fixture: the two trees are identical")
	}
}

func TestDiffGitAgainstGitAppliesNoCaptureSemantics(t *testing.T) {
	repo := gittest.New(t)
	repo.Write(".gitignore", "*.log\n")
	repo.Write("a.txt", "a")
	first := repo.CommitAll("first")
	repo.Write(".config/settings", "x")
	repo.Write("debug.log", "x")
	repo.Write("node_modules/dep/index.js", "x")
	repo.Git("add", "--all", "--force")
	second := repo.Commit("hidden, ignored and default-ignored entries")
	t.Chdir(repo.Dir)

	diff := mustDiff(t, DiffRequest{A: gitSpec(first), B: gitSpec(second)})
	requireChanges(t, diff,
		"ADDED .config",
		"ADDED .config/settings",
		"ADDED debug.log",
		"ADDED node_modules",
		"ADDED node_modules/dep",
		"ADDED node_modules/dep/index.js",
	)
}

func TestDiffGitAgainstSnapshot(t *testing.T) {
	repo := gittest.New(t)
	repo.Write(".gitignore", "*.log\n")
	repo.Write("README.md", "readme")
	repo.Write("src/main.go", "package main")
	repo.Write("src/deep/inner/x.go", "package inner")
	repo.Write("debug.log", "committed but ignored")
	repo.Write(".config/settings", "hidden")
	repo.Write("node_modules/dep/index.js", "default-ignored")
	repo.Git("add", "--all", "--force")
	first := repo.Commit("first")
	if status := repo.Git("status", "--porcelain", "--ignored"); status != "" {
		t.Fatalf("checkout is not clean:\n%s", status)
	}
	store := t.TempDir()
	capture := InspectRequest{Root: repo.Dir, UseDefaultIgnores: true, UseGitIgnore: true}
	snap := DiffSourceSpec{Kind: DiffSourceSnapshot, Path: mustDiffSnapshotFile(t, store, "checkout.dlm.json", mustFilesystemSnapshotWith(t, repo.Dir, capture))}
	depth := 2
	capture.MaxDepth = &depth
	shallow := DiffSourceSpec{Kind: DiffSourceSnapshot, Path: mustDiffSnapshotFile(t, store, "shallow.dlm.json", mustFilesystemSnapshotWith(t, repo.Dir, capture))}
	t.Chdir(repo.Dir)

	// A snapshot of a clean, lossless checkout equals the commit under the
	// snapshot's Capture Semantics, in both orders.
	for _, request := range []DiffRequest{
		{A: snap, B: gitSpec("HEAD")},
		{A: gitSpec("HEAD"), B: snap},
		{A: shallow, B: gitSpec("HEAD")},
	} {
		diff := mustDiff(t, request)
		if DiffStatusOf(diff) != DiffNoDifferences {
			t.Fatalf("%+v: %q", request, changeLines(diff))
		}
	}
	diff := mustDiff(t, DiffRequest{A: gitSpec("HEAD"), B: snap})
	requireGitRef(t, diff.SourceA, repo, first, true)
	if diff.SourceB.Kind != source.KindSnapshot || diff.SourceB.Tree != "" || diff.SourceB.Commit != "" || diff.SourceA.NodeCount != diff.SourceB.NodeCount {
		t.Fatalf("sources %+v %+v", diff.SourceA, diff.SourceB)
	}

	repo.Write("src/new.go", "package main")
	repo.Write("trace.log", "ignored by the committed .gitignore")
	repo.Write("src/deep/inner/more/y.go", "beyond the shallow depth")
	repo.Git("add", "--all", "--force")
	repo.Commit("second")
	requireChanges(t, mustDiff(t, DiffRequest{A: snap, B: gitSpec("HEAD")}),
		"ADDED src/deep/inner/more",
		"ADDED src/deep/inner/more/y.go",
		"ADDED src/new.go",
	)
	requireChanges(t, mustDiff(t, DiffRequest{A: shallow, B: gitSpec("HEAD")}), "ADDED src/new.go")
	requireChanges(t, mustDiff(t, DiffRequest{A: gitSpec("HEAD"), B: shallow}), "REMOVED src/new.go")
}

func TestDiffGitAgainstLiveIsUsage(t *testing.T) {
	live := DiffSourceSpec{Kind: DiffSourceLive, Path: t.TempDir()}
	for _, request := range []DiffRequest{
		{A: gitSpec("HEAD"), B: live},
		{A: live, B: gitSpec("HEAD")},
		{A: gitSpec(":src/a.go"), B: gitSpec("HEAD")},
		{A: gitSpec("HEAD"), B: gitSpec("-x")},
	} {
		if _, err := Diff(context.Background(), request); !DiffUsage(err) {
			t.Fatalf("%+v: %v", request, err)
		}
	}
}

func TestDiffGitFailuresCarryTheirSide(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("a.txt", "a")
	repo.CommitAll("first")
	store := t.TempDir()
	good := DiffSourceSpec{Kind: DiffSourceSnapshot, Path: mustDiffSnapshotFile(t, store, "good.dlm.json", mustFilesystemSnapshot(t, repo.Dir))}
	missing := DiffSourceSpec{Kind: DiffSourceSnapshot, Path: filepath.Join(store, "missing.dlm.json")}
	t.Chdir(repo.Dir)

	cases := []struct {
		name    string
		request DiffRequest
		side    comparison.Side
		code    string
	}{
		{"unknown revision on a", DiffRequest{A: gitSpec("nope"), B: gitSpec("HEAD")}, comparison.SideA, artifact.CodeGitRevisionNotFound},
		{"unknown revision on b", DiffRequest{A: gitSpec("HEAD"), B: gitSpec("nope")}, comparison.SideB, artifact.CodeGitRevisionNotFound},
		{"blob revision", DiffRequest{A: good, B: gitSpec("HEAD:a.txt")}, comparison.SideB, artifact.CodeGitRevisionNotFound},
		{"snapshot first", DiffRequest{A: gitSpec("nope"), B: missing}, comparison.SideB, "snapshot_read_failure"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Diff(context.Background(), tc.request)
			if side := mustDiffSide(t, err); side != tc.side {
				t.Fatalf("side %s, want %s (%v)", side, tc.side, err)
			}
			if code, _ := DiffDiagnostic(err); code != tc.code {
				t.Fatalf("code %s, want %s (%v)", code, tc.code, err)
			}
		})
	}
	_, err := Diff(context.Background(), DiffRequest{A: gitSpec("nope"), B: gitSpec("HEAD")})
	if ClassifyDiffFailure(err) != FailureObservation {
		t.Fatalf("classification %s", ClassifyDiffFailure(err))
	}

	t.Chdir(t.TempDir())
	_, err = Diff(context.Background(), DiffRequest{A: gitSpec("HEAD"), B: good})
	if side := mustDiffSide(t, err); side != comparison.SideA {
		t.Fatalf("side %s", side)
	}
	if code, _ := DiffDiagnostic(err); code != artifact.CodeGitRepositoryNotFound || ClassifyDiffFailure(err) != FailureObservation {
		t.Fatalf("outside a repository: %s %v", code, err)
	}
}
