package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/gittest"
)

// TestOversizedLiveGitIgnoreFailsTheScan: a live .gitignore beyond 1 MiB
// fails every command that reads the tree, as an oversized .gitignore blob
// fails a Git source (D7 of the v0.4.0-rc1 plan).
func TestOversizedLiveGitIgnoreFailsTheScan(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, [][2]string{{"a.txt", "a"}})
	line := "# " + strings.Repeat("x", 61) + "\n"
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(strings.Repeat(line, (1<<20)/len(line)+1)), 0o644); err != nil {
		t.Fatal(err)
	}
	const tooLarge = "read gitignore \".gitignore\": file exceeds the 1 MiB limit"
	stdout, stderr, code := executeForTest(t, "fingerprint", root, "--no-config")
	if code != 1 || stdout != "" || stderr != "Error: "+tooLarge+"\n" {
		t.Fatalf("fingerprint = (%q, %q, %d), want exit 1 and %q", stdout, stderr, code, tooLarge)
	}
	empty := filepath.Join(t.TempDir(), "empty")
	if err := os.Mkdir(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	snap := filepath.Join(t.TempDir(), "ref.dlm.json")
	if _, stderr, code := executeForTest(t, "snapshot", empty, "--no-config", "--output", snap); code != 0 {
		t.Fatalf("snapshot = %d: %s", code, stderr)
	}
	stdout, stderr, code = executeForTest(t, "diff", "snapshot:"+snap, "live:"+root, "--format", "json")
	doc := assertDiffJSON(t, stdout, stderr, code, 5, "OBSERVATION_ERROR")
	if doc.Diagnostic == nil || doc.Diagnostic.Source != "b" || doc.Diagnostic.Code != "source_read_failure" {
		t.Fatalf("diff diagnostic = %+v", doc.Diagnostic)
	}
}

// TestHistoryStopsAtAGitBomb runs the real limits on a "Git bomb": 1,000
// files under 48 × 48 shared trees describe 2,306,305 nodes in a few hundred
// bytes of objects. history fails with git_tree_too_large, exit 5, before
// it holds more than 2,000,000 nodes (about 0.9 GiB and 4 s measured),
// instead of exhausting memory as deeper sharing would.
func TestHistoryStopsAtAGitBomb(t *testing.T) {
	if testing.Short() {
		t.Skip("observes two million nodes")
	}
	repo := gittest.New(t)
	blob := repo.GitInput([]byte("x\n"), "hash-object", "-w", "--stdin")
	entries := make([]string, 1000)
	for i := range entries {
		entries[i] = "100644 blob " + blob + "\tf" + strconv.Itoa(i)
	}
	id := repo.GitInput([]byte(strings.Join(entries, "\n")+"\n"), "mktree")
	for range 2 {
		entries = make([]string, 48)
		for i := range entries {
			entries[i] = "040000 tree " + id + "\td" + strconv.Itoa(i)
		}
		id = repo.GitInput([]byte(strings.Join(entries, "\n")+"\n"), "mktree")
	}
	repo.Git("update-ref", "refs/heads/main", repo.CommitTree(id))
	t.Chdir(repo.Dir)

	stdout, stderr, code := executeForTest(t, "history", "--limit", "1", "--format", "json")
	var doc struct {
		Status     string `json:"status"`
		Diagnostic struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"diagnostic"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil || code != 5 || stderr != "" ||
		doc.Status != "OBSERVATION_ERROR" || doc.Diagnostic.Code != "git_tree_too_large" ||
		!strings.Contains(doc.Diagnostic.Message, "more than 2000000 nodes") {
		t.Fatalf("history = (%q, %q, %d), want OBSERVATION_ERROR git_tree_too_large, exit 5", stdout, stderr, code)
	}
}
