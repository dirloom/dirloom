package gitsource

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/gittest"
)

// The test binary doubles as a fake git when fakeGitMode is set.
const (
	fakeGitMode = "DIRLOOM_FAKE_GIT"
	fakeGitLog  = "DIRLOOM_FAKE_GIT_LOG"
)

func TestMain(m *testing.M) {
	if mode := os.Getenv(fakeGitMode); mode != "" {
		os.Exit(runFakeGit(mode, os.Args[1:]))
	}
	os.Exit(gittest.Main(m))
}

// runFakeGit answers `git version` and, depending on mode, records its
// invocation or hangs on `cat-file --batch`. Modes: "version:<line>",
// "record", "hang".
func runFakeGit(mode string, args []string) int {
	command := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "-c" {
			i++
			continue
		}
		command = args[i]
		break
	}
	if mode == "record" {
		line := strings.Join(args, " ") + " |"
		for _, key := range []string{"GIT_NO_LAZY_FETCH", "GIT_NO_REPLACE_OBJECTS", "GIT_TERMINAL_PROMPT", "GIT_OPTIONAL_LOCKS"} {
			line += " " + key + "=" + os.Getenv(key)
		}
		// #nosec G304 -- test-owned log path.
		file, err := os.OpenFile(os.Getenv(fakeGitLog), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return 3
		}
		_, _ = fmt.Fprintln(file, line)
		_ = file.Close()
	}
	switch {
	case command == "version" && strings.HasPrefix(mode, "version:"):
		fmt.Println(strings.TrimPrefix(mode, "version:"))
	case command == "version":
		fmt.Println("git version 2.50.1")
	case command == "cat-file" && mode == "hang":
		time.Sleep(30 * time.Second)
	}
	return 0
}

// fakeRunner returns a runner whose git is this test binary in mode.
func fakeRunner(t *testing.T, mode string) *Runner {
	t.Helper()
	t.Setenv(fakeGitMode, mode)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return &Runner{executable: executable, dir: t.TempDir()}
}

// realRunner returns a runner on the real git for repo.
func realRunner(t *testing.T, repo *gittest.Repo) *Runner {
	t.Helper()
	runner, err := NewRunner(context.Background(), repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	return runner
}

func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	var typed *artifact.Error
	if !errors.As(err, &typed) {
		t.Fatalf("error %v (%T), want code %s", err, err, code)
	}
	if typed.Code != code || typed.Internal {
		t.Fatalf("code %s internal %v (%s), want %s", typed.Code, typed.Internal, typed.Message, code)
	}
}

func requireMessage(t *testing.T, err error, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(err.Error(), part) {
			t.Fatalf("message %q lacks %q", err.Error(), part)
		}
	}
}

func TestValidateRevision(t *testing.T) {
	for _, revision := range []string{"HEAD", "main", "v1.0", "HEAD~2", "HEAD:src", "a:b:c", "@{-1}", ":/fix", "HEAD^{tree}"} {
		if err := ValidateRevision(revision); err != nil {
			t.Fatalf("%q: %v", revision, err)
		}
	}
	for _, revision := range []string{"", "-x", "--output=x", "a\nb", "a\x00b", ":a.txt", ":0:a.txt", ":src", ":"} {
		if err := ValidateRevision(revision); err == nil {
			t.Fatalf("%q accepted", revision)
		}
	}
}

func TestParseVersion(t *testing.T) {
	accepted := map[string]Version{
		"git version 2.56.0.windows.2\n":       {2, 56, "2.56.0.windows.2"},
		"git version 2.39.5 (Apple Git-154)\n": {2, 39, "2.39.5"},
		"git version 2.45.0\r\n":               {2, 45, "2.45.0"},
		"git version 2.45":                     {2, 45, "2.45"},
		"git version 2.45.0-rc1\n":             {2, 45, "2.45.0-rc1"},
		"git version 3.0.0\n":                  {3, 0, "3.0.0"},
	}
	for output, want := range accepted {
		got, ok := ParseVersion(output)
		if !ok || got != want {
			t.Fatalf("%q: %+v %v, want %+v", output, got, ok, want)
		}
	}
	for _, output := range []string{
		"", "git version", "git version \n", "git version x.y\n", "git version 2\n", "git version 2.\n",
		"git version 02.45\n", "version 2.45\n", "git version 2.45abc\n", "git version 2.45.0\ngit version 9.9\n",
		"git version 1234567890.1\n", "git version 2.4\x005\n", "Git version 2.45\n",
	} {
		if got, ok := ParseVersion(output); ok {
			t.Fatalf("%q accepted as %+v", output, got)
		}
	}
}

func TestVersionGate(t *testing.T) {
	ctx := context.Background()
	for _, accepted := range []string{"git version 2.45.0", "git version 2.56.0.windows.2", "git version 3.1.0"} {
		if err := fakeRunner(t, "version:"+accepted).checkVersion(ctx); err != nil {
			t.Fatalf("%s: %v", accepted, err)
		}
	}
	err := fakeRunner(t, "version:git version 2.44.9").checkVersion(ctx)
	requireCode(t, err, artifact.CodeGitUnavailable)
	requireMessage(t, err, "2.44.9", "2.45")

	err = fakeRunner(t, "version:not git").checkVersion(ctx)
	requireCode(t, err, artifact.CodeGitUnavailable)
	requireMessage(t, err, "cannot be parsed", "2.45")
}

func TestNewRunnerWithoutGit(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := NewRunner(context.Background(), t.TempDir())
	requireCode(t, err, artifact.CodeGitUnavailable)
	requireMessage(t, err, "not found", "2.45")
}

func TestNewRunnerRefusesCurrentDirectory(t *testing.T) {
	dir := t.TempDir()
	name := "git"
	if os.PathSeparator == '\\' {
		name = "git.exe"
	}
	if err := os.WriteFile(dir+string(os.PathSeparator)+name, nil, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv("PATH", ".")
	_, err := NewRunner(context.Background(), dir)
	requireCode(t, err, artifact.CodeGitUnavailable)
	requireMessage(t, err, "current directory")
}

func TestEveryInvocationIsHardened(t *testing.T) {
	log := t.TempDir() + string(os.PathSeparator) + "log"
	runner := fakeRunner(t, "record")
	t.Setenv(fakeGitLog, log)
	// Inherited values must lose to the hardened ones.
	t.Setenv("GIT_NO_LAZY_FETCH", "0")
	t.Setenv("GIT_NO_REPLACE_OBJECTS", "0")
	t.Setenv("GIT_TERMINAL_PROMPT", "1")
	ctx := context.Background()
	if err := runner.checkVersion(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.output(ctx, "rev-parse", "--show-object-format"); err != nil {
		t.Fatal(err)
	}
	batch, err := runner.StartBatch(ctx, SHA1)
	if err != nil {
		t.Fatal(err)
	}
	_ = batch.Close()

	data, err := os.ReadFile(log) // #nosec G304 -- test-owned path.
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	wantCommands := []string{"version", "rev-parse --show-object-format", "cat-file --batch"}
	if len(lines) != len(wantCommands) {
		t.Fatalf("invocations %q", lines)
	}
	const environment = "| GIT_NO_LAZY_FETCH=1 GIT_NO_REPLACE_OBJECTS=1 GIT_TERMINAL_PROMPT=0 GIT_OPTIONAL_LOCKS=0"
	for i, line := range lines {
		want := "-c protocol.allow=never " + wantCommands[i] + " " + environment
		if strings.TrimRight(line, "\r") != want {
			t.Fatalf("invocation %d = %q, want %q", i, line, want)
		}
	}
}

func TestResolve(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("a.txt", "a")
	repo.Write("src/main.go", "package main")
	commit := repo.CommitAll("first")
	tree := repo.Git("rev-parse", "HEAD^{tree}")
	src := repo.Git("rev-parse", "HEAD:src")
	blob := repo.Git("rev-parse", "HEAD:a.txt")
	repo.Git("tag", "--annotate", "--message", "v1", "v1", commit)
	repo.Git("tag", "lightweight", commit)
	repo.Git("tag", "--annotate", "--message", "nested", "nested", "v1")
	repo.Git("tag", "--annotate", "--message", "tree", "treetag", tree)
	repo.Git("tag", "--annotate", "--message", "tree", "treetagtag", "treetag")
	repo.Git("tag", "--annotate", "--message", "blob", "blobtag", blob)
	runner := realRunner(t, repo)
	ctx := context.Background()

	found := map[string]Resolved{
		"HEAD":         {Tree: tree, Commit: commit},
		"main":         {Tree: tree, Commit: commit},
		commit:         {Tree: tree, Commit: commit},
		commit[:12]:    {Tree: tree, Commit: commit},
		"v1":           {Tree: tree, Commit: commit},
		"lightweight":  {Tree: tree, Commit: commit},
		"nested":       {Tree: tree, Commit: commit},
		"treetag":      {Tree: tree},
		"treetagtag":   {Tree: tree},
		"HEAD^{tree}":  {Tree: tree},
		"HEAD:src":     {Tree: src},
		"HEAD:src/":    {Tree: src},
		tree:           {Tree: tree},
		"main:./src/.": {Tree: src},
	}
	for revision, want := range found {
		got, err := runner.Resolve(ctx, revision)
		if err != nil {
			t.Fatalf("%s: %v", revision, err)
		}
		want.Format = SHA1
		if got != want {
			t.Fatalf("%s: %+v, want %+v", revision, got, want)
		}
	}

	for _, revision := range []string{"HEAD:a.txt", "blobtag", blob, "nope", "HEAD~1", strings.Repeat("f", 40), "HEAD:missing"} {
		_, err := runner.Resolve(ctx, revision)
		requireCode(t, err, artifact.CodeGitRevisionNotFound)
		if strings.Contains(err.Error(), revision) && revision != blob && len(revision) != 40 {
			t.Fatalf("message %q repeats the revision text", err.Error())
		}
	}

	_, err := runner.Resolve(ctx, "-x")
	if !artifact.IsInternal(err) {
		t.Fatalf("unchecked revision: %v", err)
	}
}

func TestResolveOutsideRepository(t *testing.T) {
	gittest.Require(t)
	runner, err := NewRunner(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = runner.Resolve(context.Background(), "HEAD")
	requireCode(t, err, artifact.CodeGitRepositoryNotFound)
}

func TestResolveIgnoresReplaceRefs(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("recorded.txt", "1")
	recorded := repo.CommitAll("recorded")
	recordedTree := repo.Git("rev-parse", "HEAD^{tree}")
	repo.Git("rm", "--quiet", "recorded.txt")
	repo.Write("replacement.txt", "2")
	replacement := repo.CommitAll("replacement")
	repo.Git("replace", recorded, replacement)
	if substituted := repo.Git("rev-parse", recorded+"^{tree}"); substituted == recordedTree {
		t.Fatal("fixture: git replace did not take effect")
	}
	got, err := realRunner(t, repo).Resolve(context.Background(), recorded)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tree != recordedTree || got.Commit != recorded {
		t.Fatalf("%+v, want recorded tree %s", got, recordedTree)
	}
}

func TestResolveSHA256(t *testing.T) {
	repo := gittest.New(t, "--object-format=sha256")
	repo.Write("a.txt", "a")
	commit := repo.CommitAll("first")
	got, err := realRunner(t, repo).Resolve(context.Background(), "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if got.Format != SHA256 || got.Commit != commit || len(got.Tree) != 64 || got.Tree != repo.Git("rev-parse", "HEAD^{tree}") {
		t.Fatalf("%+v", got)
	}
}

func TestResolveFromSubdirectoryBareAndWorktree(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("src/main.go", "package main")
	commit := repo.CommitAll("first")
	tree := repo.Git("rev-parse", "HEAD^{tree}")

	bare := t.TempDir()
	repo.Git("clone", "--quiet", "--bare", repo.Dir, bare)
	worktree := t.TempDir() + string(os.PathSeparator) + "linked"
	repo.Git("worktree", "add", "--quiet", "--detach", worktree, commit)

	for _, dir := range []string{repo.Dir + string(os.PathSeparator) + "src", bare, worktree} {
		runner, err := NewRunner(context.Background(), dir)
		if err != nil {
			t.Fatal(err)
		}
		got, err := runner.Resolve(context.Background(), commit)
		if err != nil {
			t.Fatalf("%s: %v", dir, err)
		}
		if got.Tree != tree || got.Commit != commit {
			t.Fatalf("%s: %+v", dir, got)
		}
	}
}

func TestBatchReadsTreesAndBlobs(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("a.txt", "alpha")
	repo.CommitAll("first")
	runner := realRunner(t, repo)
	batch, err := runner.StartBatch(context.Background(), SHA1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = batch.Close() }()

	data, err := batch.read(repo.Git("rev-parse", "HEAD^{tree}"), treeObject)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := ParseTree(data, SHA1)
	if err != nil || len(entries) != 1 || entries[0].Name != "a.txt" || entries[0].Mode != ModeFile {
		t.Fatalf("%+v %v", entries, err)
	}
	blob, err := batch.read(entries[0].ID, linkObject)
	if err != nil || string(blob) != "alpha" {
		t.Fatalf("%q %v", blob, err)
	}

	small := objectKind{typ: "blob", limit: 4, label: "test blob"}
	_, err = batch.read(entries[0].ID, small)
	requireCode(t, err, artifact.CodeGitTreeInvalid)
	requireMessage(t, err, "5 bytes", "4 bytes limit")
	// The protocol position is lost: every later read fails the same way.
	_, again := batch.read(entries[0].ID, linkObject)
	if !errors.Is(again, err) {
		t.Fatalf("sticky error %v, want %v", again, err)
	}
	if err := batch.Close(); err != nil {
		t.Fatalf("close after a protocol error: %v", err)
	}
	if batch.cmd.ProcessState == nil {
		t.Fatal("cat-file was not reaped")
	}
}

func TestBatchMissingAndWrongType(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("a.txt", "alpha")
	repo.CommitAll("first")
	runner := realRunner(t, repo)
	ctx := context.Background()

	batch, err := runner.StartBatch(ctx, SHA1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = batch.read(strings.Repeat("0", 40), treeObject)
	requireCode(t, err, artifact.CodeGitObjectUnavailable)
	_ = batch.Close()

	batch, err = runner.StartBatch(ctx, SHA1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = batch.read(repo.Git("rev-parse", "HEAD:a.txt"), treeObject)
	requireCode(t, err, artifact.CodeGitTreeInvalid)
	requireMessage(t, err, "expected a tree")
	_ = batch.Close()

	batch, err = runner.StartBatch(ctx, SHA1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = batch.read("HEAD", treeObject)
	if !artifact.IsInternal(err) {
		t.Fatalf("non-id request: %v", err)
	}
	_ = batch.Close()
	if _, err := batch.read(strings.Repeat("0", 40), treeObject); err == nil {
		t.Fatal("read after close succeeded")
	}
}

func TestBatchCancelledBeforeRead(t *testing.T) {
	repo := gittest.New(t)
	repo.CommitAll("empty")
	ctx, cancel := context.WithCancel(context.Background())
	batch, err := realRunner(t, repo).StartBatch(ctx, SHA1)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if _, err := batch.read(repo.Git("rev-parse", "HEAD^{tree}"), treeObject); !errors.Is(err, context.Canceled) {
		t.Fatalf("read after cancel: %v", err)
	}
	_ = batch.Close()
	if batch.cmd.ProcessState == nil {
		t.Fatal("cat-file was not reaped")
	}
}

func TestBatchCancelledWhileBlocked(t *testing.T) {
	setWaitDelay(t, 200*time.Millisecond)
	runner := fakeRunner(t, "hang")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	batch, err := runner.StartBatch(ctx, SHA1)
	if err != nil {
		t.Fatal(err)
	}
	time.AfterFunc(100*time.Millisecond, cancel)
	start := time.Now()
	_, err = batch.read(strings.Repeat("a", 40), treeObject)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("blocked read: %v", err)
	}
	_ = batch.Close()
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("cancellation took %v", elapsed)
	}
	if batch.cmd.ProcessState == nil {
		t.Fatal("cat-file was not reaped")
	}
}

func TestBatchCloseKillsAfterGrace(t *testing.T) {
	setWaitDelay(t, 200*time.Millisecond)
	batch, err := fakeRunner(t, "hang").StartBatch(context.Background(), SHA1)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	err = batch.Close()
	requireCode(t, err, artifact.CodeGitObjectUnavailable)
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("close took %v", elapsed)
	}
	if batch.cmd.ProcessState == nil || batch.cmd.ProcessState.Success() {
		t.Fatalf("process state %v", batch.cmd.ProcessState)
	}
	if again := batch.Close(); !errors.Is(again, err) {
		t.Fatalf("second close %v", again)
	}
}

// TestBatchCloseAfterRefusedObjectIsPrompt refuses an object larger than the
// pipe buffer: git is still writing it when Close runs, and must not hold
// Close for the grace period.
func TestBatchCloseAfterRefusedObjectIsPrompt(t *testing.T) {
	setWaitDelay(t, 10*time.Second)
	repo := gittest.New(t)
	large := repo.Blob([]byte(strings.Repeat("#\n", 1<<20)))
	batch, err := realRunner(t, repo).StartBatch(context.Background(), SHA1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = batch.read(large, gitIgnoreObject)
	requireCode(t, err, artifact.CodeGitTreeInvalid)
	start := time.Now()
	if err := batch.Close(); err != nil {
		t.Fatalf("close after a refused object: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("close took %v", elapsed)
	}
	if batch.cmd.ProcessState == nil {
		t.Fatal("cat-file was not reaped")
	}
}

func setWaitDelay(t *testing.T, delay time.Duration) {
	t.Helper()
	previous := waitDelay
	waitDelay = delay
	t.Cleanup(func() { waitDelay = previous })
}

func TestObjectFormatIDs(t *testing.T) {
	if !SHA1.isObjectID(strings.Repeat("a", 40)) || SHA1.isObjectID(strings.Repeat("A", 40)) || SHA1.isObjectID(strings.Repeat("a", 64)) {
		t.Fatal("sha1 ids")
	}
	if !SHA256.isObjectID(strings.Repeat("0", 64)) || SHA256.isObjectID(strings.Repeat("g", 64)) || (ObjectFormat{}).isObjectID("") {
		t.Fatal("sha256 ids")
	}
	if got := []string{formatLimit(treeObject.limit), formatLimit(gitIgnoreObject.limit), formatLimit(linkObject.limit)}; !slices.Equal(got, []string{"32 MiB", "1 MiB", "4096 bytes"}) {
		t.Fatal(got)
	}
}
