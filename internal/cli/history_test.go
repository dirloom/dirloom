package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/gittest"
)

// historyGoldenDir is absolute: history tests change into their repository.
var historyGoldenDir = func() string {
	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "history", "v1", "golden"))
	if err != nil {
		panic(err)
	}
	return dir
}()

// assertHistoryGolden compares stdout with a History Result Schema v1
// golden. Set DIRLOOM_UPDATE_GOLDEN=1 to rewrite it.
func assertHistoryGolden(t *testing.T, name, stdout string) {
	t.Helper()
	path := filepath.Join(historyGoldenDir, name)
	if os.Getenv("DIRLOOM_UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(historyGoldenDir, 0o755); err != nil {
			t.Fatal(err)
		}
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

// historyGoldenRepository builds a deterministic repository (pinned
// identities, a distinct date per commit, SHA-1):
//
//	A  root: README.md, src/a.go, old/payment/{api/handler.go,service.go}
//	B  adds docs/guide.md, removes src/a.go
//	C  rewrites README.md (contents only)
//	D  moves old/payment to payment
//	S  side branch from D: adds tools/x.go
//	M  merges S into main (first parent D)
func historyGoldenRepository(t *testing.T) (*gittest.Repo, map[string]string) {
	t.Helper()
	repo := gittest.New(t, "--object-format=sha1")
	ids := map[string]string{}
	commit := func(name string, hour int) {
		date := fmt.Sprintf("2026-03-01T%02d:00:00Z", hour)
		t.Setenv("GIT_AUTHOR_DATE", date)
		t.Setenv("GIT_COMMITTER_DATE", date)
		ids[name] = repo.CommitAll(name)
	}
	repo.Write("README.md", "readme\n")
	repo.Write("src/a.go", "package src\n")
	repo.Write("old/payment/api/handler.go", "package api\n")
	repo.Write("old/payment/service.go", "package payment\n")
	commit("A", 9)
	repo.Write("docs/guide.md", "guide\n")
	repo.Git("rm", "--quiet", "src/a.go")
	commit("B", 10)
	repo.Write("README.md", "readme, rewritten\n")
	commit("C", 11)
	repo.Git("mv", "old/payment", "payment")
	commit("D", 12)
	repo.Git("switch", "--quiet", "--create", "side")
	repo.Write("tools/x.go", "package tools\n")
	commit("S", 13)
	repo.Git("switch", "--quiet", "main")
	t.Setenv("GIT_AUTHOR_DATE", "2026-03-01T14:00:00Z")
	t.Setenv("GIT_COMMITTER_DATE", "2026-03-01T14:00:00+0200")
	repo.Git("merge", "--quiet", "--no-ff", "side", "-m", "M")
	ids["M"] = repo.Git("rev-parse", "HEAD")
	return repo, ids
}

func TestHistoryGoldens(t *testing.T) {
	repo, ids := historyGoldenRepository(t)
	t.Chdir(repo.Dir)

	cases := []struct {
		golden string
		args   []string
	}{
		{"history.json", []string{"--format", "json"}},
		{"history.txt", nil},
		{"structural-only.json", []string{"--structural-only", "--format", "json"}},
		{"structural-only.txt", []string{"--structural-only"}},
		{"no-moves.json", []string{"--no-moves", "--format", "json"}},
		{"from.json", []string{"--from", ids["B"], "--format", "json"}},
		{"limit.txt", []string{"--limit", "2"}},
		{"empty.txt", []string{"--from", "HEAD"}},
	}
	for _, tc := range cases {
		stdout, stderr, code := executeForTest(t, append([]string{"history"}, tc.args...)...)
		if code != 0 || stderr != "" {
			t.Fatalf("%s=(%q, %q, %d)", tc.golden, stdout, stderr, code)
		}
		assertHistoryGolden(t, tc.golden, stdout)
	}

	// The revision text never reaches the output.
	byID, _, _ := executeForTest(t, "history", ids["M"], "--format", "json")
	byName, _, _ := executeForTest(t, "history", "main", "--format", "json")
	if byID != byName || strings.Contains(byName, "main") {
		t.Fatal("the document depends on how the revision was written")
	}

	clone := filepath.Join(t.TempDir(), "shallow")
	repo.Git("clone", "--quiet", "--depth", "2", "--no-single-branch", fileURLForHistory(repo.Dir), clone)
	t.Chdir(clone)
	stdout, stderr, code := executeForTest(t, "history", "--format", "json")
	if code != 0 || stderr != "" {
		t.Fatalf("shallow=(%q, %q, %d)", stdout, stderr, code)
	}
	assertHistoryGolden(t, "shallow.json", stdout)
	stdout, _, code = executeForTest(t, "history")
	if code != 0 || !strings.Contains(stdout, "structure unknown (shallow boundary)") || !strings.Contains(stdout, "end: shallow") {
		t.Fatalf("shallow text=(%q, %d)", stdout, code)
	}
}

func TestHistoryDiagnosticGoldens(t *testing.T) {
	repo, ids := historyGoldenRepository(t)
	t.Chdir(repo.Dir)
	repo.Git("switch", "--quiet", "--create", "other", ids["A"])
	repo.Write("other.txt", "x")
	t.Setenv("GIT_COMMITTER_DATE", "2026-03-02T09:00:00Z")
	other := repo.CommitAll("other")
	repo.Git("switch", "--quiet", "main")

	for _, tc := range []struct {
		golden string
		args   []string
	}{
		{"revision-not-found.json", []string{"nope"}},
		{"tree-revision.json", []string{"HEAD:payment"}},
		{"from-not-reached.json", []string{"--from", other}},
	} {
		stdout, stderr, code := executeForTest(t, append(append([]string{"history"}, tc.args...), "--format", "json")...)
		if code != 5 || stderr != "" {
			t.Fatalf("%s=(%q, %q, %d)", tc.golden, stdout, stderr, code)
		}
		assertHistoryGolden(t, tc.golden, stdout)
	}

	t.Chdir(t.TempDir())
	stdout, stderr, code := executeForTest(t, "history", "--format", "json")
	if code != 5 || stderr != "" {
		t.Fatalf("outside a repository=(%q, %q, %d)", stdout, stderr, code)
	}
	assertHistoryGolden(t, "repository-not-found.json", stdout)
	_, stderr, code = executeForTest(t, "history")
	if code != 5 || !strings.Contains(stderr, "not inside a Git repository") {
		t.Fatalf("text failure=(%q, %d)", stderr, code)
	}
}

func TestHistoryUsageErrors(t *testing.T) {
	t.Chdir(t.TempDir())
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"history", "a", "b"}, "at most one revision"},
		{[]string{"history", ""}, "must not be empty"},
		{[]string{"history", "-x"}, "unknown shorthand flag"},
		{[]string{"history", "--", "-x"}, "must not start with '-'"},
		{[]string{"history", ":src/a.go"}, "index entry"},
		{[]string{"history", "--from", ""}, "--from must not be empty"},
		{[]string{"history", "--from", ":0:a"}, "index entry"},
		{[]string{"history", "--limit", "0"}, "positive integer"},
		{[]string{"history", "--limit", "-1"}, "positive integer"},
		{[]string{"history", "--limit", "many"}, "invalid argument"},
		{[]string{"history", "--format", "yaml"}, "unsupported history format"},
		{[]string{"history", "--depth", "1"}, "unknown flag"},
		{[]string{"history", "--hidden"}, "unknown flag"},
		{[]string{"history", "--color", "always"}, "unknown flag"},
		{[]string{"history", "--output", "x.json"}, "unknown flag"},
		{[]string{"history", "--config", "x.yaml"}, "not supported by history"},
		{[]string{"--no-config", "history"}, "not supported by history"},
	}
	for _, tc := range cases {
		for _, format := range []string{"text", "json"} {
			// --format goes right after "history", before any "--".
			var args []string
			for _, arg := range tc.args {
				args = append(args, arg)
				if arg == "history" && tc.want != "unsupported history format" {
					args = append(args, "--format", format)
				}
			}
			stdout, stderr, code := executeForTest(t, args...)
			if code != 2 || stdout != "" || !strings.Contains(stderr, tc.want) {
				t.Fatalf("%q (%s)=(%q, %q, %d)", tc.args, format, stdout, stderr, code)
			}
		}
	}
}

func TestHistoryHelpAndCompletion(t *testing.T) {
	stdout, stderr, code := executeForTest(t, "history", "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("help=(%q, %q, %d)", stdout, stderr, code)
	}
	for _, want := range []string{"history [revision]", "first-parent", "not git log", "--structural-only", "--from", "--limit", "--no-moves", "git 2.45 or newer", "never shown"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("help lacks %q:\n%s", want, stdout)
		}
	}
	root, _, _ := executeForTest(t, "--help")
	if !strings.Contains(root, "history") {
		t.Fatal("root help does not list history")
	}
	stdout, _, code = executeForTest(t, "__complete", "history", "--format", "")
	if code != 0 || !strings.Contains(stdout, "text") || !strings.Contains(stdout, "json") {
		t.Fatalf("format completion=%q", stdout)
	}
	stdout, _, code = executeForTest(t, "__complete", "history", "")
	if code != 0 || !strings.Contains(stdout, ":4") {
		t.Fatalf("revision completion=%q", stdout)
	}
}

func TestHistoryJSONContract(t *testing.T) {
	for _, name := range []string{"history.json", "structural-only.json", "no-moves.json", "from.json", "shallow.json"} {
		data, err := os.ReadFile(filepath.Join(historyGoldenDir, name)) // #nosec G304 -- fixed test golden path.
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			SchemaVersion int    `json:"schemaVersion"`
			Status        string `json:"status"`
			Range         struct {
				To    string `json:"to"`
				Limit int    `json:"limit"`
				End   string `json:"end"`
			} `json:"range"`
			Entries []map[string]any `json:"entries"`
		}
		if err := json.Unmarshal(data, &doc); err != nil {
			t.Fatal(err)
		}
		if doc.SchemaVersion != historyResultSchemaVersion || doc.Status != "OK" || doc.Entries == nil {
			t.Fatalf("%s: %+v", name, doc)
		}
		for _, entry := range doc.Entries {
			_, structural := entry["structural"]
			_, summary := entry["summary"]
			if structural != summary {
				t.Fatalf("%s: structural and summary must be both present or both absent: %v", name, entry)
			}
			for _, field := range []string{"author", "committer", "message", "subject"} {
				if _, leaked := entry[field]; leaked {
					t.Fatalf("%s: entry carries %s", name, field)
				}
			}
		}
	}
}

func fileURLForHistory(dir string) string {
	slashed := filepath.ToSlash(dir)
	if !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed
	}
	return "file://" + slashed
}
