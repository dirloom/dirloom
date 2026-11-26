package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func readHistoryDocument(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("%v\n%s", err, data)
	}
	return doc
}

// TestHistoryStructuralOnlyKeepsRange checks the golden pair: hiding
// non-structural entries changes entries only, never the range.
func TestHistoryStructuralOnlyKeepsRange(t *testing.T) {
	read := func(name string) map[string]any {
		data, err := os.ReadFile(filepath.Join(historyGoldenDir, name)) // #nosec G304 -- fixed test golden path.
		if err != nil {
			t.Fatal(err)
		}
		return readHistoryDocument(t, data)
	}
	full, filtered := read("history.json"), read("structural-only.json")
	if !reflect.DeepEqual(full["range"], filtered["range"]) || !reflect.DeepEqual(full["metadata"], filtered["metadata"]) {
		t.Fatalf("range or metadata changed: %v vs %v", full["range"], filtered["range"])
	}
	var kept []any
	for _, entry := range full["entries"].([]any) {
		if entry.(map[string]any)["structural"] == true {
			kept = append(kept, entry)
		}
	}
	if !reflect.DeepEqual(kept, filtered["entries"]) {
		t.Fatal("structural-only entries are not the structural entries of the full history")
	}
}

// TestHistoryFailureMidWalkIsOneDocument corrupts a tree deep in the history:
// the result is one failure document, never a partial history.
func TestHistoryFailureMidWalkIsOneDocument(t *testing.T) {
	repo, ids := historyGoldenRepository(t)
	tree := repo.Git("rev-parse", ids["B"]+"^{tree}")
	path := filepath.Join(repo.Dir, ".git", "objects", tree[:2], tree[2:])
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not a zlib stream"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo.Dir)
	stdout, stderr, code := executeForTest(t, "history", "--format", "json")
	if code != 5 || stderr != "" {
		t.Fatalf("=(%q, %q, %d)", stdout, stderr, code)
	}
	doc := readHistoryDocument(t, []byte(stdout))
	for _, key := range []string{"metadata", "range", "entries"} {
		if _, ok := doc[key]; ok {
			t.Fatalf("failure document carries %s: %s", key, stdout)
		}
	}
	diagnostic := doc["diagnostic"].(map[string]any)
	if doc["status"] != "OBSERVATION_ERROR" || diagnostic["code"] != "git_object_unavailable" {
		t.Fatalf("%s", stdout)
	}
	// Stopping before the corrupt tree is a normal history.
	if _, _, code := executeForTest(t, "history", "--limit", "1"); code != 0 {
		t.Fatalf("limit 1 exit %d", code)
	}
}

// TestHistoryFingerprintMatchesLosslessCheckout checks the documented
// equivalence: the history fingerprint of HEAD equals the fingerprint of a
// lossless checkout under a capture that keeps every committed entry and
// drops .git.
func TestHistoryFingerprintMatchesLosslessCheckout(t *testing.T) {
	repo, _ := historyGoldenRepository(t)
	t.Chdir(repo.Dir)
	stdout, _, code := executeForTest(t, "history", "--limit", "1", "--format", "json")
	if code != 0 {
		t.Fatalf("history exit %d", code)
	}
	entries := readHistoryDocument(t, []byte(stdout))["entries"].([]any)
	historyFingerprint := entries[0].(map[string]any)["fingerprint"]

	// The work tree that built the repository is not a clean checkout:
	// `git mv` left an empty old/ directory behind. A fresh clone is.
	checkout := filepath.Join(t.TempDir(), "checkout")
	repo.Git("clone", "--quiet", repo.Dir, checkout)
	stdout, stderr, code := executeForTest(t, "fingerprint", checkout, "--no-config", "--hidden", "--no-default-ignore", "--no-gitignore", "--ignore", ".git", "--format", "json")
	if code != 0 {
		t.Fatalf("fingerprint=(%q, %q, %d)", stdout, stderr, code)
	}
	var fingerprint struct {
		Fingerprint string `json:"fingerprint"`
	}
	if err := json.Unmarshal([]byte(stdout), &fingerprint); err != nil {
		t.Fatal(err)
	}
	if fingerprint.Fingerprint == "" || fingerprint.Fingerprint != historyFingerprint {
		t.Fatalf("checkout %s, history %v", fingerprint.Fingerprint, historyFingerprint)
	}
}
