package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
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

// TestHistoryShellExitCodes proves the history exit codes through a real
// shell: bash on POSIX, PowerShell on Windows.
func TestHistoryShellExitCodes(t *testing.T) {
	bin := buildVerifyBinary(t)
	repo, _ := historyGoldenRepository(t)
	out := t.TempDir()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		shell, err := exec.LookPath("pwsh")
		if err != nil {
			shell, err = exec.LookPath("powershell")
		}
		if err != nil {
			t.Fatal("pwsh or powershell is required to prove $LASTEXITCODE for history")
		}
		script := `
param(
  [Parameter(Mandatory = $true)][string]$Work,
  [Parameter(Mandatory = $true)][string]$Bin,
  [Parameter(Mandatory = $true)][string]$Out
)
$ErrorActionPreference = 'Stop'
Set-Location $Work
& $Bin history --format json > (Join-Path $Out 'history.json') 2> (Join-Path $Out 'history.err')
if ($LASTEXITCODE -ne 0) { throw "history LASTEXITCODE=$LASTEXITCODE" }
& $Bin history nope --format json > (Join-Path $Out 'missing.json') 2> (Join-Path $Out 'missing.err')
if ($LASTEXITCODE -ne 5) { throw "missing LASTEXITCODE=$LASTEXITCODE" }
$ErrorActionPreference = 'Continue'
& $Bin history --limit 0 --format json > (Join-Path $Out 'usage.out') 2> (Join-Path $Out 'usage.err')
if ($LASTEXITCODE -ne 2) { throw "usage LASTEXITCODE=$LASTEXITCODE" }
`
		scriptPath := filepath.Join(t.TempDir(), "history-smoke.ps1")
		if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
			t.Fatal(err)
		}
		cmd = exec.Command(shell, "-NoProfile", "-File", scriptPath, repo.Dir, bin, out)
	} else {
		bash, err := exec.LookPath("bash")
		if err != nil {
			t.Fatal("bash is required to prove POSIX $? for history")
		}
		script := `
set -euo pipefail
cd "$1"
bin=$2
out=$3
set +e
"$bin" history --format json >"$out/history.json" 2>"$out/history.err"
code=$?
set -e
test "$code" -eq 0
test ! -s "$out/history.err"
set +e
"$bin" history nope --format json >"$out/missing.json" 2>"$out/missing.err"
code=$?
set -e
test "$code" -eq 5
test ! -s "$out/missing.err"
set +e
"$bin" history --limit 0 --format json >"$out/usage.out" 2>"$out/usage.err"
code=$?
set -e
test "$code" -eq 2
test ! -s "$out/usage.out"
`
		cmd = exec.Command(bash, "-c", script, "history-smoke", repo.Dir, bin, out)
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("history shell smoke: %v\n%s", err, output)
	}
	for name, status := range map[string]string{"history.json": "OK", "missing.json": "OBSERVATION_ERROR"} {
		data, err := os.ReadFile(filepath.Join(out, name)) // #nosec G304 -- test-owned path.
		if err != nil {
			t.Fatal(err)
		}
		data = trimBOM(data)
		if doc := readHistoryDocument(t, data); doc["status"] != status || doc["schemaVersion"] != float64(historyResultSchemaVersion) {
			t.Fatalf("%s: %v", name, doc)
		}
	}
}

func trimBOM(data []byte) []byte {
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return data[3:]
	}
	return data
}
