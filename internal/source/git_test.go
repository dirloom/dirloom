package source

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/filter"
	"github.com/dirloom/dirloom/internal/gittest"
	"github.com/dirloom/dirloom/internal/tree"
)

func TestMain(m *testing.M) {
	code := gittest.Main(m)
	removeBenchmarkRepositories()
	os.Exit(code)
}

// describe lists every node as "path kind" or "path kind -> target".
func describe(art *artifact.Artifact) []string {
	var out []string
	var walk func(node artifact.Node)
	walk = func(node artifact.Node) {
		line := string(node.Path) + " " + string(node.Kind)
		if node.Kind.HasTarget() {
			line += " -> " + node.Target
		}
		out = append(out, line)
		for _, child := range node.Children {
			walk(child)
		}
	}
	walk(art.Root)
	slices.Sort(out)
	return out
}

func observeGit(t *testing.T, dir, revision string, scan tree.ScanOptions) (*artifact.Artifact, *Git, error) {
	t.Helper()
	if scan.RootName == "" {
		scan.RootName = "repo"
	}
	source := &Git{Revision: revision, Dir: dir, Scan: scan}
	art, err := source.Observe(context.Background())
	return art, source, err
}

func requireGitCode(t *testing.T, err error, code string, parts ...string) {
	t.Helper()
	var typed *artifact.Error
	if !errors.As(err, &typed) || typed.Code != code || typed.Internal {
		t.Fatalf("error %v, want code %s", err, code)
	}
	for _, part := range parts {
		if !strings.Contains(typed.Message, part) {
			t.Fatalf("message %q lacks %q", typed.Message, part)
		}
	}
}

func requireNodes(t *testing.T, art *artifact.Artifact, want ...string) {
	t.Helper()
	slices.Sort(want)
	if got := describe(art); !slices.Equal(got, want) {
		t.Fatalf("nodes:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestGitSourceMapsCommittedTree(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("README.md", "readme")
	repo.Write("src/main.go", "package main")
	repo.Write("src/tool.sh", "#!/bin/sh")
	repo.Write(".hidden/x", "x")
	repo.Git("add", "--all")
	repo.Git("update-index", "--chmod=+x", "src/tool.sh")
	repo.Index("120000", repo.Blob([]byte("../README.md")), "docs/readme-link")
	repo.Index("120000", repo.Blob([]byte("/etc/hosts")), "abs-posix")
	repo.Index("120000", repo.Blob([]byte(`C:\Windows\win.ini`)), "abs-windows")
	repo.Index("120000", repo.Blob([]byte(`a\b`)), "backslash")
	repo.Index("120000", repo.Blob([]byte("e\u0301")), "nfd-target")
	repo.Index("160000", strings.Repeat("1", 40), "vendor/lib")
	commit := repo.Commit("first")

	art, source, err := observeGit(t, repo.Dir, "HEAD", tree.ScanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	requireNodes(t, art,
		". directory",
		".hidden directory",
		".hidden/x file",
		"README.md file",
		"abs-posix symlink -> /etc/hosts",
		`abs-windows symlink -> C:\Windows\win.ini`,
		`backslash symlink -> a\b`,
		"docs directory",
		"docs/readme-link symlink -> ../README.md",
		"nfd-target symlink -> \u00e9",
		"src directory",
		"src/main.go file",
		"src/tool.sh file",
		"vendor directory",
		"vendor/lib directory",
	)
	if source.Kind() != KindGit || source.Commit() != commit || source.Tree() != repo.Git("rev-parse", "HEAD^{tree}") {
		t.Fatalf("kind %s commit %s tree %s", source.Kind(), source.Commit(), source.Tree())
	}
	if art.DisplayRootName != "repo" {
		t.Fatalf("root label %q", art.DisplayRootName)
	}
}

func TestGitSourceSubtreeAndSHA256(t *testing.T) {
	repo := gittest.New(t, "--object-format=sha256")
	repo.Write("src/main.go", "package main")
	repo.Write("src/lib/util.go", "package lib")
	repo.Write("other.txt", "x")
	repo.CommitAll("first")

	art, source, err := observeGit(t, repo.Dir, "HEAD:src", tree.ScanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	requireNodes(t, art, ". directory", "lib directory", "lib/util.go file", "main.go file")
	if source.Commit() != "" || len(source.Tree()) != 64 || source.Tree() != repo.Git("rev-parse", "HEAD:src") {
		t.Fatalf("commit %q tree %q", source.Commit(), source.Tree())
	}
}

// captureScan builds the scan options of a snapshot's Capture Semantics, as
// inspect does for a live directory, without an output path.
func captureScan(t *testing.T, rootName string, useGitIgnore, useDefaults, includeHidden bool, patterns ...string) tree.ScanOptions {
	t.Helper()
	explicit, err := filter.NewIgnoreMatcher(patterns)
	if err != nil {
		t.Fatal(err)
	}
	var git *filter.GitIgnore
	if useGitIgnore {
		git = filter.NewGitIgnore()
	}
	return tree.ScanOptions{
		RootName:     rootName,
		UseGitIgnore: useGitIgnore,
		FilterPolicy: filter.NewPolicy("", useDefaults, explicit, git, includeHidden),
		GitIgnore:    git,
	}
}

func TestGitSourceCaptureSemanticsMatchCheckout(t *testing.T) {
	repo := gittest.New(t)
	repo.Write(".gitignore", "build/\n*.log\n")
	repo.Write("app.log", "log")
	repo.Write("README.md", "readme")
	repo.Write("src/.gitignore", "gen/\n")
	repo.Write("src/keep.go", "package src")
	repo.Write("src/gen/x.go", "package gen")
	repo.Write(".config/settings", "x")
	repo.Write("node_modules/dep/index.js", "x")
	repo.Write("deep/a/b/c.txt", "x")
	repo.Write("vendor/skip.txt", "x")
	repo.Git("add", "--all", "--force")
	repo.Commit("first")
	// Untracked files exist only in the checkout; remove them so the
	// checkout is a clean, lossless materialization of HEAD.
	if status := repo.Git("status", "--porcelain", "--ignored"); status != "" {
		t.Fatalf("checkout is not clean:\n%s", status)
	}

	for _, captured := range []struct {
		name                                     string
		useGitIgnore, useDefaults, includeHidden bool
		patterns                                 []string
		depth                                    *int
	}{
		{name: "defaults", useGitIgnore: true, useDefaults: true},
		{name: "no gitignore", useDefaults: true},
		{name: "hidden", useGitIgnore: true, useDefaults: true, includeHidden: true},
		{name: "explicit", useGitIgnore: true, useDefaults: true, patterns: []string{"vendor/"}},
		{name: "depth", useGitIgnore: true, depth: intPointer(2)},
	} {
		t.Run(captured.name, func(t *testing.T) {
			scan := captureScan(t, "repo", captured.useGitIgnore, captured.useDefaults, captured.includeHidden, captured.patterns...)
			scan.MaxDepth = captured.depth
			gitArt, _, err := observeGit(t, repo.Dir, "HEAD", scan)
			if err != nil {
				t.Fatal(err)
			}
			live := captureScan(t, "repo", captured.useGitIgnore, captured.useDefaults, captured.includeHidden, captured.patterns...)
			live.MaxDepth = captured.depth
			live.RootAbs = repo.Dir
			model, err := tree.NewScanner(live).Scan(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			liveArt, err := FromTree(model, FromTreeOptions{RootAbs: repo.Dir})
			if err != nil {
				t.Fatal(err)
			}
			// The checkout's .git directory is the only node a committed
			// tree cannot have.
			var want []string
			for _, line := range describe(liveArt) {
				if line != ".git directory" && !strings.HasPrefix(line, ".git/") {
					want = append(want, line)
				}
			}
			if got := describe(gitArt); !slices.Equal(got, want) {
				t.Fatalf("git:\n%s\ncheckout:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
		})
	}

	art, _, err := observeGit(t, repo.Dir, "HEAD", captureScan(t, "repo", true, true, false))
	if err != nil {
		t.Fatal(err)
	}
	requireNodes(t, art, ". directory", "README.md file", "deep directory", "deep/a directory", "deep/a/b directory",
		"deep/a/b/c.txt file", "src directory", "src/keep.go file", "vendor directory", "vendor/skip.txt file")
}

func intPointer(value int) *int { return &value }

func TestGitSourceGitIgnoreControlFiles(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("x", "x")
	repo.Write("sub/y", "y")
	repo.Git("add", "--all")
	// A link named .gitignore is not a control file, as in a working tree.
	repo.Index("120000", repo.Blob([]byte("x\n")), ".gitignore")
	repo.Index("160000", strings.Repeat("2", 40), "sub/.gitignore")
	repo.Commit("first")
	art, _, err := observeGit(t, repo.Dir, "HEAD", captureScan(t, "repo", true, false, true))
	if err != nil {
		t.Fatal(err)
	}
	requireNodes(t, art, ". directory", ".gitignore symlink -> x\n", "sub directory", "sub/.gitignore directory", "sub/y file", "x file")

	large := gittest.New(t)
	large.Write(".gitignore", strings.Repeat("#\n", 1<<19)+"#")
	large.CommitAll("large")
	_, _, err = observeGit(t, large.Dir, "HEAD", captureScan(t, "repo", true, false, true))
	requireGitCode(t, err, artifact.CodeGitTreeInvalid, `read gitignore ".gitignore"`, "1 MiB limit")
	// Without .gitignore processing the same tree is observed.
	if _, _, err := observeGit(t, large.Dir, "HEAD", captureScan(t, "repo", false, false, true)); err != nil {
		t.Fatal(err)
	}
}

func TestGitSourceLinkTargets(t *testing.T) {
	repo := gittest.New(t)
	repo.Index("120000", repo.Blob([]byte(strings.Repeat("t", 4096))), "max")
	repo.Commit("max")
	art, _, err := observeGit(t, repo.Dir, "HEAD", tree.ScanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	requireNodes(t, art, ". directory", "max symlink -> "+strings.Repeat("t", 4096))

	repo.Index("120000", repo.Blob([]byte(strings.Repeat("t", 4097))), "long")
	repo.Commit("long")
	_, _, err = observeGit(t, repo.Dir, "HEAD", tree.ScanOptions{})
	requireGitCode(t, err, artifact.CodeGitTreeInvalid, `read link "long"`, "4096 bytes limit")

	invalid := gittest.New(t)
	invalid.Index("120000", invalid.Blob([]byte("\xff")), "bad")
	invalid.Commit("bad")
	_, _, err = observeGit(t, invalid.Dir, "HEAD", tree.ScanOptions{})
	requireGitCode(t, err, artifact.CodeGitTreeInvalid, `read link "bad"`, "not valid UTF-8")

	// With dirs-only capture a link is never read.
	if _, _, err := observeGit(t, invalid.Dir, "HEAD", tree.ScanOptions{Directories: true}); err != nil {
		t.Fatal(err)
	}
}

func TestGitSourceCraftedTrees(t *testing.T) {
	repo := gittest.New(t)
	blob := repo.Blob([]byte("x"))
	empty := repo.RawTree()
	missing := strings.Repeat("d", 40)

	invalid := map[string]struct {
		entries []gittest.Entry
		code    string
		parts   []string
	}{
		"dot dot":          {[]gittest.Entry{{Mode: "40000", Name: "..", ID: empty}}, artifact.CodeGitTreeInvalid, []string{`read directory "."`, "invalid name"}},
		"unknown mode":     {[]gittest.Entry{{Mode: "100600", Name: "f", ID: blob}}, artifact.CodeGitTreeInvalid, []string{"unsupported mode"}},
		"zero padded tree": {[]gittest.Entry{{Mode: "040000", Name: "d", ID: empty}}, artifact.CodeGitTreeInvalid, []string{"unsupported mode"}},
		"duplicate":        {[]gittest.Entry{{Mode: "100644", Name: "f", ID: blob}, {Mode: "100644", Name: "f", ID: blob}}, artifact.CodeGitTreeInvalid, []string{"two entries"}},
		"tree is a blob":   {[]gittest.Entry{{Mode: "40000", Name: "d", ID: blob}}, artifact.CodeGitTreeInvalid, []string{`read directory "d"`, "expected a tree"}},
		"link is a tree":   {[]gittest.Entry{{Mode: "120000", Name: "l", ID: empty}}, artifact.CodeGitTreeInvalid, []string{`read link "l"`, "expected a blob"}},
		"missing subtree":  {[]gittest.Entry{{Mode: "40000", Name: "gone", ID: missing}}, artifact.CodeGitObjectUnavailable, []string{`read directory "gone"`, "not available"}},
		"nfc collision":    {[]gittest.Entry{{Mode: "100644", Name: "\u00e9", ID: blob}, {Mode: "100644", Name: "e\u0301", ID: blob}}, artifact.CodeCollision, nil},
	}
	for name, tc := range invalid {
		t.Run(name, func(t *testing.T) {
			_, _, err := observeGit(t, repo.Dir, repo.RawTree(tc.entries...), tree.ScanOptions{})
			requireGitCode(t, err, tc.code, tc.parts...)
		})
	}

	// A subtree that Capture Semantics excludes is never read, even when
	// its object is missing; a submodule is never read either.
	root := repo.RawTree(
		gittest.Entry{Mode: "40000", Name: "gone", ID: missing},
		gittest.Entry{Mode: "160000", Name: "module", ID: missing},
		gittest.Entry{Mode: "100644", Name: "f", ID: blob},
	)
	art, _, err := observeGit(t, repo.Dir, root, tree.ScanOptions{MaxDepth: intPointer(1)})
	if err != nil {
		t.Fatal(err)
	}
	requireNodes(t, art, ". directory", "f file", "gone directory", "module directory")
	if _, _, err := observeGit(t, repo.Dir, root, captureScan(t, "repo", false, false, true, "gone/")); err != nil {
		t.Fatal(err)
	}
	_, _, err = observeGit(t, repo.Dir, root, tree.ScanOptions{})
	requireGitCode(t, err, artifact.CodeGitObjectUnavailable)
}

func TestGitSourceRevisionAndRepositoryErrors(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("a", "a")
	repo.CommitAll("first")
	_, source, err := observeGit(t, repo.Dir, "missing-branch", tree.ScanOptions{})
	requireGitCode(t, err, artifact.CodeGitRevisionNotFound)
	if source.Tree() != "" || source.Commit() != "" {
		t.Fatal("a failed observation recorded ids")
	}
	_, _, err = observeGit(t, repo.Dir, "HEAD:a", tree.ScanOptions{})
	requireGitCode(t, err, artifact.CodeGitRevisionNotFound)
	_, _, err = observeGit(t, t.TempDir(), "HEAD", tree.ScanOptions{})
	requireGitCode(t, err, artifact.CodeGitRepositoryNotFound)
}

func TestGitSourceNeverFetchesMissingObjects(t *testing.T) {
	origin := gittest.New(t)
	origin.Write("src/main.go", "package main")
	origin.Write("docs/guide.md", "guide")
	origin.Git("add", "--all")
	target := origin.Blob([]byte("src/main.go"))
	origin.Index("120000", target, "link")
	origin.Commit("first")
	origin.Git("config", "uploadpack.allowfilter", "true")

	url := "file://" + filepath.ToSlash(origin.Dir)
	if runtime.GOOS == "windows" {
		url = "file:///" + filepath.ToSlash(origin.Dir)
	}
	clone := func(filter string) *gittest.Repo {
		dir := filepath.Join(t.TempDir(), "clone")
		origin.Git("clone", "--quiet", "--no-checkout", "--filter="+filter, url, dir)
		clone := gittest.Open(t, dir)
		// Even with the file protocol allowed, no lazy fetch may happen.
		clone.Git("config", "protocol.file.allow", "always")
		return clone
	}
	stillMissing := func(repo *gittest.Repo, id string) {
		t.Helper()
		t.Setenv("GIT_NO_LAZY_FETCH", "1")
		if _, err := repo.Run(nil, "cat-file", "-e", id); err == nil {
			t.Fatalf("object %s was fetched", id)
		}
	}

	blobless := clone("blob:none")
	_, _, err := observeGit(t, blobless.Dir, "HEAD", tree.ScanOptions{})
	requireGitCode(t, err, artifact.CodeGitObjectUnavailable, `read link "link"`)
	stillMissing(blobless, target)
	// Files are never read, so a blobless clone serves dirs-only capture.
	art, _, err := observeGit(t, blobless.Dir, "HEAD", tree.ScanOptions{Directories: true})
	if err != nil {
		t.Fatal(err)
	}
	requireNodes(t, art, ". directory", "docs directory", "src directory")

	treeless := clone("tree:1")
	subtree := origin.Git("rev-parse", "HEAD:src")
	_, _, err = observeGit(t, treeless.Dir, "HEAD", tree.ScanOptions{Directories: true})
	requireGitCode(t, err, artifact.CodeGitObjectUnavailable)
	stillMissing(treeless, subtree)
	art, _, err = observeGit(t, treeless.Dir, "HEAD", tree.ScanOptions{Directories: true, MaxDepth: intPointer(1)})
	if err != nil {
		t.Fatal(err)
	}
	requireNodes(t, art, ". directory", "docs directory", "src directory")
}

func TestGitSourceRefusesOutputPathAndCancellation(t *testing.T) {
	scan := tree.ScanOptions{FilterPolicy: filter.NewPolicy(filepath.Join(t.TempDir(), "out.txt"), false, nil, nil, true)}
	_, _, err := observeGit(t, t.TempDir(), "HEAD", scan)
	if !artifact.IsInternal(err) {
		t.Fatalf("output path: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (&Git{Revision: "HEAD"}).Observe(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled: %v", err)
	}
}

func TestFromTreePortableTargets(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "dir", "file")
	model := &tree.Node{Name: "r", Type: tree.NodeDirectory, Children: []*tree.Node{
		{Name: "abs", Path: "abs", Type: tree.NodeSymlink, Target: inside},
		{Name: "sep", Path: "sep", Type: tree.NodeSymlink, Target: `a\b`},
		{Name: "nfd", Path: "nfd", Type: tree.NodeSymlink, Target: "e\u0301"},
	}}
	art, err := FromTree(model, FromTreeOptions{RootAbs: root, PortableTargets: true})
	if err != nil {
		t.Fatal(err)
	}
	requireNodes(t, art, ". directory", "abs symlink -> "+inside, `sep symlink -> a\b`, "nfd symlink -> \u00e9")
}
