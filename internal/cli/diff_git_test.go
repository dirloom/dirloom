package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/gittest"
)

// fakeGitVersion makes the test binary answer `git version` with its value
// when it runs as a copied git executable.
const fakeGitVersion = "DIRLOOM_FAKE_GIT_VERSION"

func TestMain(m *testing.M) {
	if version := os.Getenv(fakeGitVersion); version != "" {
		for _, arg := range os.Args[1:] {
			if arg == "version" {
				_, _ = os.Stdout.WriteString("git version " + version + "\n")
				os.Exit(0)
			}
		}
		os.Exit(1)
	}
	os.Exit(gittest.Main(m))
}

// gitGoldenDir is absolute: Git tests change into their repository.
var gitGoldenDir = func() string {
	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "diff", diffGoldenV3, "golden"))
	if err != nil {
		panic(err)
	}
	return dir
}()

// assertGitDiffGolden compares stdout with a v3 golden. Set
// DIRLOOM_UPDATE_GOLDEN=1 to rewrite it.
func assertGitDiffGolden(t *testing.T, name, stdout string) {
	t.Helper()
	path := filepath.Join(gitGoldenDir, name)
	if os.Getenv("DIRLOOM_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, []byte(stdout), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path) // #nosec G304 -- fixed test golden path.
	if err != nil {
		t.Fatal(err)
	}
	if stdout != string(want) {
		t.Fatalf("%s\n got %q\nwant %q", name, stdout, want)
	}
}

// TestDiffV3GoldensReplayFrozenV2 checks that every frozen v2 golden has a v3
// counterpart equal to it except schemaVersion. Since the current output is
// compared with the v3 goldens, snapshot: and live: documents are v2 documents
// with "schemaVersion": 3.
func TestDiffV3GoldensReplayFrozenV2(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("..", "..", "testdata", "diff", diffGoldenV2, "golden"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no v2 goldens")
	}
	for _, entry := range entries {
		name := entry.Name()
		frozen := readDiffGolden(t, diffGoldenV2, name)
		current := readDiffGolden(t, diffGoldenV3, name)
		if !strings.HasSuffix(name, ".json") {
			if current != frozen {
				t.Fatalf("%s: text golden changed", name)
			}
			continue
		}
		if want := strings.Replace(frozen, "\n  \"schemaVersion\": 2,\n", "\n  \"schemaVersion\": 3,\n", 1); current != want {
			t.Fatalf("%s: v3 golden is not the v2 golden with schemaVersion 3", name)
		}
		v2, v3 := decodeDiffDocument(t, frozen), decodeDiffDocument(t, current)
		if v2["schemaVersion"] != float64(2) || v3["schemaVersion"] != float64(3) {
			t.Fatalf("%s schema versions %v, %v", name, v2["schemaVersion"], v3["schemaVersion"])
		}
		delete(v2, "schemaVersion")
		delete(v3, "schemaVersion")
		if !reflect.DeepEqual(v2, v3) {
			t.Fatalf("%s documents differ beyond schemaVersion", name)
		}
	}
}

// gitGoldenRepository builds a deterministic repository: author, committer,
// dates and object format are pinned, so its ids are the same on every
// platform. HEAD~1 holds src/old/payment and a link; HEAD moved the directory
// to src/payment and added docs/guide.md. The snapshot captures the work tree
// of HEAD~1, where the link (added to the index only) is not materialized; it
// ignores the link so that the Git side, under the same capture, does too.
func gitGoldenRepository(t *testing.T) (repo *gittest.Repo, snapshotPath string) {
	t.Helper()
	repo = gittest.New(t, "--object-format=sha1")
	repo.Write("README.md", "readme\n")
	repo.Write("src/old/payment/api/handler.go", "package api\n")
	repo.Write("src/old/payment/service.go", "package payment\n")
	repo.Git("add", "--all")
	repo.Index("120000", repo.Blob([]byte("../README.md")), "src/readme-link")
	repo.Commit("first")

	snapshotPath = filepath.Join(t.TempDir(), "first.dlm.json")
	if stdout, stderr, code := executeForTest(t, "snapshot", repo.Dir, "--no-config", "--ignore", "readme-link", "--output", snapshotPath); code != 0 {
		t.Fatalf("snapshot=(%q, %q, %d)", stdout, stderr, code)
	}

	repo.Git("mv", "src/old/payment", "src/payment")
	repo.Write("docs/guide.md", "guide\n")
	repo.Git("add", "docs/guide.md")
	repo.Commit("second")
	return repo, snapshotPath
}

func TestDiffGitGoldens(t *testing.T) {
	repo, snapshotPath := gitGoldenRepository(t)
	t.Chdir(repo.Dir)

	stdout, stderr, code := executeForTest(t, "diff", "git:HEAD~1", "git:HEAD", "--format", "json")
	if code != 1 || stderr != "" {
		t.Fatalf("git moved=(%q, %q, %d)", stdout, stderr, code)
	}
	assertGitDiffGolden(t, "git-moved.json", stdout)
	explicit, _, _ := executeForTest(t, "diff", "git:"+repo.Git("rev-parse", "HEAD~1"), "git:"+repo.Git("rev-parse", "main"), "--format", "json")
	if explicit != stdout {
		t.Fatal("the document depends on how the revisions were written")
	}

	stdout, stderr, code = executeForTest(t, "diff", "git:HEAD~1", "git:HEAD")
	if code != 1 || stderr != "" {
		t.Fatalf("git moved text=(%q, %q, %d)", stdout, stderr, code)
	}
	assertGitDiffGolden(t, "git-moved.txt", stdout)

	stdout, stderr, code = executeForTest(t, "diff", "git:HEAD~1:src", "git:HEAD:src", "--format", "json")
	if code != 1 || stderr != "" {
		t.Fatalf("git subtree=(%q, %q, %d)", stdout, stderr, code)
	}
	assertGitDiffGolden(t, "git-subtree.json", stdout)

	stdout, stderr, code = executeForTest(t, "diff", "snapshot:"+snapshotPath, "git:HEAD", "--format", "json")
	if code != 1 || stderr != "" {
		t.Fatalf("snapshot against git=(%q, %q, %d)", stdout, stderr, code)
	}
	assertGitDiffGolden(t, "git-snapshot.json", stdout)

	stdout, stderr, code = executeForTest(t, "diff", "git:HEAD", "git:HEAD", "--format", "json")
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"status": "NO_DIFFERENCES"`) {
		t.Fatalf("same revision=(%q, %q, %d)", stdout, stderr, code)
	}
}

func TestDiffGitDiagnosticGoldens(t *testing.T) {
	repo, _ := gitGoldenRepository(t)
	blob := repo.Blob([]byte("x"))
	missingSubtree := repo.RawTree(gittest.Entry{Mode: "40000", Name: "gone", ID: strings.Repeat("d", 40)})
	unknownMode := repo.RawTree(gittest.Entry{Mode: "100600", Name: "file", ID: blob})
	t.Chdir(repo.Dir)

	cases := []struct {
		golden string
		a, b   string
	}{
		{"git-revision-not-found.json", "git:nope", "git:HEAD"},
		{"git-object-unavailable.json", "git:HEAD", "git:" + missingSubtree},
		{"git-tree-invalid.json", "git:" + unknownMode, "git:HEAD"},
	}
	for _, tc := range cases {
		stdout, stderr, code := executeForTest(t, "diff", tc.a, tc.b, "--format", "json")
		if code != 5 || stderr != "" {
			t.Fatalf("%s=(%q, %q, %d)", tc.golden, stdout, stderr, code)
		}
		assertGitDiffGolden(t, tc.golden, stdout)
	}

	t.Chdir(t.TempDir())
	stdout, stderr, code := executeForTest(t, "diff", "git:HEAD", "git:HEAD", "--format", "json")
	if code != 5 || stderr != "" {
		t.Fatalf("outside a repository=(%q, %q, %d)", stdout, stderr, code)
	}
	assertGitDiffGolden(t, "git-repository-not-found.json", stdout)

	t.Setenv("PATH", t.TempDir())
	stdout, stderr, code = executeForTest(t, "diff", "git:HEAD", "git:HEAD", "--format", "json")
	if code != 5 || stderr != "" {
		t.Fatalf("no git=(%q, %q, %d)", stdout, stderr, code)
	}
	assertGitDiffGolden(t, "git-unavailable.json", stdout)
}

func TestDiffGitUsageErrors(t *testing.T) {
	cases := []struct {
		a, b string
		want string
	}{
		{"git:HEAD", "live:.", "git: against live: is not supported"},
		{"live:.", "git:HEAD", "git: against live: is not supported"},
		{"git:", "git:HEAD", "must not be empty"},
		{"git:-x", "git:HEAD", "must not start with '-'"},
		{"git:HEAD", "git::src/main.go", "index entry"},
		{"git:HEAD", "git::0:src/main.go", "index entry"},
	}
	for _, tc := range cases {
		for _, format := range []string{"text", "json"} {
			stdout, stderr, code := executeForTest(t, "diff", tc.a, tc.b, "--format", format)
			if code != 2 || stdout != "" || !strings.Contains(stderr, tc.want) {
				t.Fatalf("%s %s (%s)=(%q, %q, %d)", tc.a, tc.b, format, stdout, stderr, code)
			}
		}
	}
	_, stderr, code := executeForTest(t, "diff", "git:HEAD")
	if code != 2 || !strings.Contains(stderr, "git:<revision>") {
		t.Fatalf("one source=(%q, %d)", stderr, code)
	}
}

func TestDiffGitHelpAndCompletion(t *testing.T) {
	stdout, stderr, code := executeForTest(t, "diff", "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("help=(%q, %q, %d)", stdout, stderr, code)
	}
	for _, want := range []string{"git:<revision>", "not git diff", "git 2.45 or newer", "dirloom diff git:main git:feature", "git: against live: is\nrefused"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("help lacks %q:\n%s", want, stdout)
		}
	}
	stdout, _, code = executeForTest(t, "__complete", "diff", "")
	if code != 0 || !strings.Contains(stdout, "git:") {
		t.Fatalf("source completion=%q code=%d", stdout, code)
	}
	stdout, _, code = executeForTest(t, "__complete", "diff", "git:")
	if code != 0 || strings.Contains(stdout, "snapshot:") || !strings.Contains(stdout, ":4") {
		t.Fatalf("revision completion=%q code=%d", stdout, code)
	}
}
