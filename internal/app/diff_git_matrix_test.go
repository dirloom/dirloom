package app

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/dirloom/dirloom/internal/gittest"
)

// emptyTree is the id Git gives the empty tree in SHA-1 repositories.
const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// requireSymmetricAndDeterministic compares A→B with B→A: every ADDED is a
// REMOVED, every CHANGED swaps before and after, every MOVED swaps from and
// path. Running the same request twice gives the same model.
func requireSymmetricAndDeterministic(t *testing.T, a, b DiffSourceSpec) comparison.StructuralDiff {
	t.Helper()
	forward := mustDiff(t, DiffRequest{A: a, B: b})
	again := mustDiff(t, DiffRequest{A: a, B: b})
	if !reflect.DeepEqual(forward, again) {
		t.Fatal("diff is not deterministic")
	}
	backward := mustDiff(t, DiffRequest{A: b, B: a})
	mirror := func(diff comparison.StructuralDiff, swap bool) []string {
		var lines []string
		for _, change := range diff.Changes {
			op, path, from := change.Op, change.Path, change.From
			before, after := change.Before, change.After
			if swap {
				before, after = after, before
				switch op {
				case comparison.OpAdded:
					op = comparison.OpRemoved
				case comparison.OpRemoved:
					op = comparison.OpAdded
				case comparison.OpMoved:
					path, from = from, path
				}
			}
			line := string(op) + " " + string(from) + " " + string(path)
			if before != nil {
				line += " " + string(before.Kind) + targetText(before.Target)
			}
			if after != nil {
				line += " " + string(after.Kind) + targetText(after.Target)
			}
			lines = append(lines, line)
		}
		slices.Sort(lines)
		return lines
	}
	if got, want := mirror(backward, true), mirror(forward, false); !slices.Equal(got, want) {
		t.Fatalf("not symmetric:\n%q\n%q", got, want)
	}
	if forward.SourceA != backward.SourceB || forward.SourceB != backward.SourceA {
		t.Fatalf("sources are not swapped: %+v %+v", forward, backward)
	}
	return forward
}

func targetText(target *string) string {
	if target == nil {
		return ""
	}
	return "->" + *target
}

func TestDiffGitBranchesTagsAndCommits(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("src/a.go", "package src")
	repo.Write("src/b.go", "package src")
	repo.Write("node", "a file")
	repo.Git("add", "--all")
	repo.Index("120000", repo.Blob([]byte("a")), "link")
	first := repo.Commit("first")
	repo.Git("tag", "--annotate", "--message", "v1", "v1")
	repo.Git("switch", "--quiet", "--create", "feature")
	repo.Git("rm", "--quiet", "src/b.go", "node")
	repo.Write("src/c.go", "package src")
	repo.Write("node/x.go", "package node")
	repo.Git("add", "src/c.go", "node/x.go")
	repo.Index("120000", repo.Blob([]byte("b")), "link")
	second := repo.Commit("second")
	t.Chdir(repo.Dir)

	want := []string{"CHANGED link", "CHANGED node", "ADDED node/x.go", "REMOVED src/b.go", "ADDED src/c.go"}
	diff := requireSymmetricAndDeterministic(t, gitSpec("main"), gitSpec("feature"))
	requireChanges(t, diff, want...)
	link, node := diff.Changes[0], diff.Changes[1]
	if *link.Before.Target != "a" || *link.After.Target != "b" || node.Before.Kind != artifact.KindFile || node.After.Kind != artifact.KindDirectory {
		t.Fatalf("changed states %+v %+v / %+v %+v", link.Before, link.After, node.Before, node.After)
	}
	for _, pair := range [][2]string{{"v1", "feature"}, {first, second}, {first[:10], "feature~0"}} {
		other := mustDiff(t, DiffRequest{A: gitSpec(pair[0]), B: gitSpec(pair[1])})
		if !reflect.DeepEqual(other, diff) {
			t.Fatalf("%s..%s differs from main..feature", pair[0], pair[1])
		}
	}

	// A detached HEAD resolves like the commit it points to.
	repo.Git("checkout", "--quiet", "--detach", first)
	detached := mustDiff(t, DiffRequest{A: gitSpec("HEAD"), B: gitSpec("feature")})
	if !reflect.DeepEqual(detached, diff) {
		t.Fatal("detached HEAD differs from main")
	}
}

func TestDiffGitSymlinks(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("file", "x")
	repo.Write("links/keep", "k")
	repo.Write("elsewhere/keep", "k")
	repo.Git("add", "--all")
	base := repo.Commit("base")
	repo.Index("120000", repo.Blob([]byte("../file")), "links/new")
	repo.Index("120000", repo.Blob([]byte("/etc/hosts")), "abs-posix")
	repo.Index("120000", repo.Blob([]byte(`C:\Windows\win.ini`)), "abs-windows")
	added := repo.Commit("added")
	repo.Index("120000", repo.Blob([]byte("../other")), "links/new")
	retargeted := repo.Commit("retargeted")
	repo.Git("update-index", "--force-remove", "links/new")
	repo.Index("120000", repo.Blob([]byte("../other")), "elsewhere/new")
	relocated := repo.Commit("relocated")
	t.Chdir(repo.Dir)

	diff := requireSymmetricAndDeterministic(t, gitSpec(base), gitSpec(added))
	requireChanges(t, diff, "ADDED abs-posix", "ADDED abs-windows", "ADDED links/new")
	for i, want := range []string{"/etc/hosts", `C:\Windows\win.ini`} {
		if got := *diff.Changes[i].After.Target; got != want {
			t.Fatalf("absolute target %q, want %q as recorded", got, want)
		}
	}
	diff = requireSymmetricAndDeterministic(t, gitSpec(added), gitSpec(retargeted))
	requireChanges(t, diff, "CHANGED links/new")
	diff = requireSymmetricAndDeterministic(t, gitSpec(retargeted), gitSpec(relocated))
	requireChanges(t, diff, "MOVED links/new -> elsewhere/new (1)")
	if diff.Changes[0].Evidence != comparison.EvidenceName || *diff.Changes[0].After.Target != "../other" {
		t.Fatalf("relocated link %+v", diff.Changes[0])
	}
}

func TestDiffGitSubmoduleDeletionAndEmptyTree(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("src/a/x.go", "package a")
	repo.Write("src/b/y.go", "package b")
	repo.Write("src/b/deep/z.go", "package deep")
	first := repo.CommitAll("first")
	// The submodule commit does not exist here: it is never read.
	repo.Index("160000", strings.Repeat("5", 40), "vendor/lib")
	withSubmodule := repo.Commit("submodule")
	repo.Git("rm", "--quiet", "-r", "src/b")
	deleted := repo.Commit("deleted")
	t.Chdir(repo.Dir)

	diff := requireSymmetricAndDeterministic(t, gitSpec(first), gitSpec(withSubmodule))
	requireChanges(t, diff, "ADDED vendor", "ADDED vendor/lib")
	if diff.Changes[1].After.Kind != artifact.KindDirectory {
		t.Fatalf("submodule kind %s", diff.Changes[1].After.Kind)
	}
	diff = requireSymmetricAndDeterministic(t, gitSpec(withSubmodule), gitSpec(deleted))
	requireChanges(t, diff, "REMOVED src/b", "REMOVED src/b/deep", "REMOVED src/b/deep/z.go", "REMOVED src/b/y.go")

	diff = requireSymmetricAndDeterministic(t, gitSpec(emptyTree), gitSpec(first))
	if diff.Summary.Added != diff.SourceB.NodeCount-1 || diff.Summary.Total != diff.Summary.Added || diff.SourceA.NodeCount != 1 {
		t.Fatalf("empty tree: %+v %+v", diff.Summary, diff.SourceB)
	}
	if diff.SourceA.Tree != emptyTree || diff.SourceA.Commit != "" {
		t.Fatalf("empty tree source %+v", diff.SourceA)
	}
}

func TestDiffGitReplacedCommitUsesRecordedTree(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("a.txt", "a")
	first := repo.CommitAll("first")
	repo.Write("b.txt", "b")
	second := repo.CommitAll("second")
	t.Chdir(repo.Dir)
	before := mustDiff(t, DiffRequest{A: gitSpec(first), B: gitSpec(second)})
	repo.Git("replace", first, second)
	after := mustDiff(t, DiffRequest{A: gitSpec(first), B: gitSpec(second)})
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("replace refs changed the observation: %q vs %q", changeLines(before), changeLines(after))
	}
	requireChanges(t, after, "ADDED b.txt")
}

func TestDiffGitExcludedMissingSubtreeIsNeverRead(t *testing.T) {
	repo := gittest.New(t)
	blob := repo.Blob([]byte("x"))
	root := repo.RawTree(
		gittest.Entry{Mode: "100644", Name: "f", ID: blob},
		gittest.Entry{Mode: "40000", Name: "gone", ID: strings.Repeat("d", 40)},
	)
	live := writeTree(t, t.TempDir(), [][2]string{{"f", "x"}, {"gone/x", "x"}})
	store := t.TempDir()
	depth := 1
	snapshots := map[string]InspectRequest{
		"ignored.dlm.json": {Root: live, IgnorePatterns: []string{"gone/"}},
		"shallow.dlm.json": {Root: live, MaxDepth: &depth},
		"full.dlm.json":    {Root: live},
	}
	specs := make(map[string]DiffSourceSpec)
	for name, request := range snapshots {
		specs[name] = DiffSourceSpec{Kind: DiffSourceSnapshot, Path: mustDiffSnapshotFile(t, store, name, mustFilesystemSnapshotWith(t, live, request))}
	}
	t.Chdir(repo.Dir)

	for _, name := range []string{"ignored.dlm.json", "shallow.dlm.json"} {
		diff := mustDiff(t, DiffRequest{A: specs[name], B: gitSpec(root)})
		if DiffStatusOf(diff) != DiffNoDifferences {
			t.Fatalf("%s: %q", name, changeLines(diff))
		}
	}
	_, err := Diff(context.Background(), DiffRequest{A: specs["full.dlm.json"], B: gitSpec(root)})
	if code, _ := DiffDiagnostic(err); code != artifact.CodeGitObjectUnavailable || mustDiffSide(t, err) != comparison.SideB {
		t.Fatalf("subtree not excluded: %s %v", code, err)
	}
}

// equivalenceRepository commits a tree that a checkout materializes without
// loss on this platform: links only where the checkout creates real links.
func equivalenceRepository(t *testing.T) *gittest.Repo {
	t.Helper()
	repo := gittest.New(t)
	for path, content := range map[string]string{
		".gitignore":                "*.log\nbuild/\n",
		"README.md":                 "readme",
		"app.log":                   "ignored but committed",
		"build/out.bin":             "ignored directory",
		"src/.gitignore":            "generated/\n!keep.log\n",
		"src/main.go":               "package main",
		"src/keep.log":              "re-included",
		"src/generated/x.go":        "package generated",
		"src/deep/a/b/c/d.go":       "package c",
		".config/settings.json":     "{}",
		"node_modules/dep/index.js": "default-ignored",
		"docs/guide.md":             "guide",
		"docs/-dash.md":             "sorts before .",
		"docs/e\u0301.md":           "NFD name",
	} {
		repo.Write(path, content)
	}
	if runtime.GOOS != "windows" {
		for link, target := range map[string]string{"docs/readme-link": "../README.md", "src/dangling": "missing/target"} {
			if err := os.Symlink(target, filepath.Join(repo.Dir, filepath.FromSlash(link))); err != nil {
				t.Fatal(err)
			}
		}
	}
	repo.Git("add", "--all", "--force")
	repo.Commit("equivalence")
	if status := repo.Git("status", "--porcelain", "--ignored"); status != "" {
		t.Fatalf("checkout is not clean:\n%s", status)
	}
	return repo
}

// TestDiffGitEquivalenceWithCleanCheckout is the equivalence property: under
// the same Capture Semantics, a snapshot of a clean, lossless checkout of a
// revision and the revision itself are NO_DIFFERENCES, in both orders.
func TestDiffGitEquivalenceWithCleanCheckout(t *testing.T) {
	repo := equivalenceRepository(t)
	store := t.TempDir()
	zero, one, three := 0, 1, 3
	captures := map[string]InspectRequest{
		"defaults": {UseDefaultIgnores: true, UseGitIgnore: true},
		// Without default ignores, the checkout's own .git directory is a
		// file present in the checkout but not committed: exclude it.
		"nothing filtered": {IncludeHidden: true, IgnorePatterns: []string{".git/"}},
		"hidden":           {UseDefaultIgnores: true, UseGitIgnore: true, IncludeHidden: true},
		"no gitignore":     {UseDefaultIgnores: true},
		"no defaults":      {UseGitIgnore: true},
		"explicit":         {UseDefaultIgnores: true, UseGitIgnore: true, IgnorePatterns: []string{"docs/", "*.go"}},
		"dirs only":        {UseDefaultIgnores: true, UseGitIgnore: true, DirectoriesOnly: true},
		"depth 0":          {UseGitIgnore: true, MaxDepth: &zero},
		"depth 1":          {UseGitIgnore: true, MaxDepth: &one},
		"depth 3 hidden":   {UseGitIgnore: true, IncludeHidden: true, MaxDepth: &three, IgnorePatterns: []string{".git/"}},
	}
	snapshots := make(map[string]DiffSourceSpec)
	for name, request := range captures {
		request.Root = repo.Dir
		file := strings.ReplaceAll(name, " ", "-") + ".dlm.json"
		snapshots[name] = DiffSourceSpec{Kind: DiffSourceSnapshot, Path: mustDiffSnapshotFile(t, store, file, mustFilesystemSnapshotWith(t, repo.Dir, request))}
	}
	t.Chdir(repo.Dir)
	for name, snap := range snapshots {
		t.Run(name, func(t *testing.T) {
			for _, request := range []DiffRequest{{A: snap, B: gitSpec("HEAD")}, {A: gitSpec("HEAD"), B: snap}} {
				diff := mustDiff(t, request)
				if DiffStatusOf(diff) != DiffNoDifferences {
					t.Fatalf("%q", changeLines(diff))
				}
			}
		})
	}
}

// checkoutSnapshot snapshots the work tree of repo with default capture, to a
// file outside the repository.
func checkoutSnapshot(t *testing.T, repo *gittest.Repo, request InspectRequest) DiffSourceSpec {
	t.Helper()
	request.Root = repo.Dir
	return DiffSourceSpec{Kind: DiffSourceSnapshot, Path: mustDiffSnapshotFile(t, t.TempDir(), "checkout.dlm.json", mustFilesystemSnapshotWith(t, repo.Dir, request))}
}

var defaultCapture = InspectRequest{UseDefaultIgnores: true, UseGitIgnore: true}

// TestDiffGitEquivalenceExceptions shows each documented exception of Git
// Tree Source v1: the checkout and the revision differ as the contract says.
func TestDiffGitEquivalenceExceptions(t *testing.T) {
	t.Run("links checked out as files", func(t *testing.T) {
		repo := gittest.New(t)
		repo.Write("README.md", "readme")
		repo.Git("add", "--all")
		repo.Index("120000", repo.Blob([]byte("README.md")), "link")
		repo.Commit("link")
		repo.Git("config", "core.symlinks", "false")
		repo.Git("checkout", "--quiet", "--", "link")
		snap := checkoutSnapshot(t, repo, defaultCapture)
		t.Chdir(repo.Dir)
		diff := mustDiff(t, DiffRequest{A: snap, B: gitSpec("HEAD")})
		requireChanges(t, diff, "CHANGED link")
		if diff.Changes[0].Before.Kind != artifact.KindFile || diff.Changes[0].After.Kind != artifact.KindSymlink {
			t.Fatalf("%+v %+v", diff.Changes[0].Before, diff.Changes[0].After)
		}
	})

	t.Run("absolute link target inside the checkout", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("the checkout needs real symbolic links")
		}
		repo := gittest.New(t)
		repo.Write("README.md", "readme")
		if err := os.Symlink(filepath.Join(repo.Dir, "README.md"), filepath.Join(repo.Dir, "link")); err != nil {
			t.Fatal(err)
		}
		repo.CommitAll("absolute link")
		snap := checkoutSnapshot(t, repo, defaultCapture)
		t.Chdir(repo.Dir)
		diff := mustDiff(t, DiffRequest{A: snap, B: gitSpec("HEAD")})
		requireChanges(t, diff, "CHANGED link")
		if *diff.Changes[0].Before.Target != "README.md" || *diff.Changes[0].After.Target != filepath.ToSlash(filepath.Join(repo.Dir, "README.md")) {
			t.Fatalf("targets %q -> %q", *diff.Changes[0].Before.Target, *diff.Changes[0].After.Target)
		}
	})

	t.Run("initialized submodule", func(t *testing.T) {
		sub := gittest.New(t)
		sub.Write("lib.go", "package lib")
		sub.CommitAll("lib")
		repo := gittest.New(t)
		repo.Write("main.go", "package main")
		repo.Git("add", "--all")
		repo.Git("-c", "protocol.file.allow=always", "submodule", "--quiet", "add", sub.Dir, "vendor/lib")
		repo.Commit("submodule")
		snap := checkoutSnapshot(t, repo, defaultCapture)
		t.Chdir(repo.Dir)
		requireChanges(t, mustDiff(t, DiffRequest{A: snap, B: gitSpec("HEAD")}), "REMOVED vendor/lib/lib.go")
	})

	t.Run("names the platform cannot materialize", func(t *testing.T) {
		repo := gittest.New(t)
		repo.Index("100644", repo.Blob([]byte("upper")), "README.md")
		repo.Index("100644", repo.Blob([]byte("lower")), "readme.md")
		repo.Commit("case collision")
		repo.Git("checkout", "--quiet", "--", ".")
		snap := checkoutSnapshot(t, repo, defaultCapture)
		t.Chdir(repo.Dir)
		diff := mustDiff(t, DiffRequest{A: snap, B: gitSpec("HEAD")})
		if caseInsensitive(t, repo.Dir) {
			if diff.Summary.Added != 1 || diff.Summary.Total != 1 {
				t.Fatalf("case-insensitive checkout: %q", changeLines(diff))
			}
		} else if DiffStatusOf(diff) != DiffNoDifferences {
			t.Fatalf("case-sensitive checkout: %q", changeLines(diff))
		}
	})

	t.Run(".gitignore above 1 MiB", func(t *testing.T) {
		repo := gittest.New(t)
		repo.Write(".gitignore", strings.Repeat("#\n", 1<<19)+"#")
		repo.Write("a.txt", "a")
		repo.CommitAll("large gitignore")
		snap := checkoutSnapshot(t, repo, defaultCapture)
		t.Chdir(repo.Dir)
		_, err := Diff(context.Background(), DiffRequest{A: snap, B: gitSpec("HEAD")})
		if code, message := DiffDiagnostic(err); code != artifact.CodeGitTreeInvalid || !strings.Contains(message, "1 MiB limit") {
			t.Fatalf("%s %s", code, message)
		}
	})

	t.Run("files present in the checkout but not committed", func(t *testing.T) {
		repo := gittest.New(t)
		repo.Write("a.txt", "a")
		repo.CommitAll("first")
		repo.Write("scratch.txt", "untracked")
		snap := checkoutSnapshot(t, repo, defaultCapture)
		t.Chdir(repo.Dir)
		requireChanges(t, mustDiff(t, DiffRequest{A: snap, B: gitSpec("HEAD")}), "REMOVED scratch.txt")
	})
}

func caseInsensitive(t *testing.T, dir string) bool {
	t.Helper()
	probe := filepath.Join(dir, "case-probe")
	if err := os.WriteFile(probe, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(probe) }()
	_, err := os.Stat(filepath.Join(dir, "CASE-PROBE"))
	return err == nil
}
