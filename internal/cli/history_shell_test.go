package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// TestHistoryShellExitCodes proves the history exit-code contract through a
// real operating-system shell: bash on POSIX, PowerShell on Windows. There is
// no exit 1, 3 or 4 in history; exit 6 is covered in-process by
// TestHistoryInternalGolden.
func TestHistoryShellExitCodes(t *testing.T) {
	bin := buildVerifyBinary(t)
	repo, _ := historyGoldenRepository(t)
	outside := t.TempDir()
	out := t.TempDir()
	if runtime.GOOS == "windows" {
		runPowerShellHistorySmoke(t, bin, repo.Dir, outside, out)
	} else {
		runBashHistorySmoke(t, bin, repo.Dir, outside, out)
	}
	assertHistoryShellJSON(t, filepath.Join(out, "ok.json"), "OK", "")
	assertHistoryShellJSON(t, filepath.Join(out, "missing.json"), "OBSERVATION_ERROR", "git_revision_not_found")
	assertHistoryShellJSON(t, filepath.Join(out, "outside.json"), "OBSERVATION_ERROR", "git_repository_not_found")
}

func runBashHistorySmoke(t *testing.T, bin, work, outside, out string) {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal("bash is required to prove POSIX $? for history")
	}
	script := `
set -euo pipefail
cd "$1"
bin=$2
outside=$3
out=$4
set +e
"$bin" history --format json >"$out/ok.json" 2>"$out/ok.err"
code=$?
set -e
test "$code" -eq 0
test ! -s "$out/ok.err"
set +e
"$bin" history --structural-only >"$out/ok.txt" 2>"$out/ok-text.err"
code=$?
set -e
test "$code" -eq 0
test ! -s "$out/ok-text.err"
grep -q '^Structural History (first parent, ' "$out/ok.txt"
set +e
"$bin" history --limit 0 --format json >"$out/usage.out" 2>"$out/usage.err"
code=$?
set -e
test "$code" -eq 2
test ! -s "$out/usage.out"
grep -q '^Error: ' "$out/usage.err"
set +e
"$bin" history --depth 1 >"$out/refused.out" 2>"$out/refused.err"
code=$?
set -e
test "$code" -eq 2
test ! -s "$out/refused.out"
grep -q '^Error: ' "$out/refused.err"
set +e
"$bin" history nope --format json >"$out/missing.json" 2>"$out/missing.err"
code=$?
set -e
test "$code" -eq 5
test ! -s "$out/missing.err"
cd "$outside"
set +e
"$bin" history --format json >"$out/outside.json" 2>"$out/outside.err"
code=$?
set -e
test "$code" -eq 5
test ! -s "$out/outside.err"
set +e
"$bin" history >"$out/outside.txt" 2>"$out/outside-text.err"
code=$?
set -e
test "$code" -eq 5
test ! -s "$out/outside.txt"
grep -q '^Error: ' "$out/outside-text.err"
`
	cmd := exec.Command(bash, "-c", script, "history-smoke", work, bin, outside, out)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bash history smoke: %v\n%s", err, output)
	}
}

func runPowerShellHistorySmoke(t *testing.T, bin, work, outside, out string) {
	t.Helper()
	shell, err := exec.LookPath("pwsh")
	if err != nil {
		shell, err = exec.LookPath("powershell")
	}
	if err != nil {
		t.Fatal("pwsh or powershell is required to prove $LASTEXITCODE for history")
	}
	// Usage errors and failures write stderr: never let that stop the script.
	script := `
param(
  [Parameter(Mandatory = $true)][string]$Work,
  [Parameter(Mandatory = $true)][string]$Bin,
  [Parameter(Mandatory = $true)][string]$Outside,
  [Parameter(Mandatory = $true)][string]$Out
)
$ErrorActionPreference = 'Continue'
$PSNativeCommandUseErrorActionPreference = $false
function Assert-Empty([string]$Name) {
  if ((Get-Item (Join-Path $Out $Name)).Length -ne 0) { throw "$Name is not empty" }
}
function Assert-Line([string]$Name, [string]$Pattern) {
  if (-not (Select-String -Quiet -Pattern $Pattern -Path (Join-Path $Out $Name))) { throw "$Name lacks $Pattern" }
}
Set-Location $Work
& $Bin history --format json > (Join-Path $Out 'ok.json') 2> (Join-Path $Out 'ok.err')
if ($LASTEXITCODE -ne 0) { throw "ok LASTEXITCODE=$LASTEXITCODE" }
Assert-Empty 'ok.err'
& $Bin history --structural-only > (Join-Path $Out 'ok.txt') 2> (Join-Path $Out 'ok-text.err')
if ($LASTEXITCODE -ne 0) { throw "ok text LASTEXITCODE=$LASTEXITCODE" }
Assert-Empty 'ok-text.err'
Assert-Line 'ok.txt' '^Structural History \(first parent, '
& $Bin history --limit 0 --format json > (Join-Path $Out 'usage.out') 2> (Join-Path $Out 'usage.err')
if ($LASTEXITCODE -ne 2) { throw "usage LASTEXITCODE=$LASTEXITCODE" }
Assert-Empty 'usage.out'
Assert-Line 'usage.err' '^Error: '
& $Bin history --depth 1 > (Join-Path $Out 'refused.out') 2> (Join-Path $Out 'refused.err')
if ($LASTEXITCODE -ne 2) { throw "refused LASTEXITCODE=$LASTEXITCODE" }
Assert-Empty 'refused.out'
Assert-Line 'refused.err' '^Error: '
& $Bin history nope --format json > (Join-Path $Out 'missing.json') 2> (Join-Path $Out 'missing.err')
if ($LASTEXITCODE -ne 5) { throw "missing LASTEXITCODE=$LASTEXITCODE" }
Assert-Empty 'missing.err'
Set-Location $Outside
& $Bin history --format json > (Join-Path $Out 'outside.json') 2> (Join-Path $Out 'outside.err')
if ($LASTEXITCODE -ne 5) { throw "outside LASTEXITCODE=$LASTEXITCODE" }
Assert-Empty 'outside.err'
& $Bin history > (Join-Path $Out 'outside.txt') 2> (Join-Path $Out 'outside-text.err')
if ($LASTEXITCODE -ne 5) { throw "outside text LASTEXITCODE=$LASTEXITCODE" }
Assert-Empty 'outside.txt'
Assert-Line 'outside-text.err' '^Error: '
exit 0
`
	scriptPath := filepath.Join(t.TempDir(), "history-smoke.ps1")
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(shell, "-NoProfile", "-File", scriptPath, work, bin, outside, out)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("powershell history smoke: %v\n%s", err, output)
	}
}

// assertHistoryShellJSON checks a History Result v1 document written through
// a shell: an OK history of the golden repository's five first-parent
// commits, or a failure with its machine code.
func assertHistoryShellJSON(t *testing.T, path, status, code string) {
	t.Helper()
	data, err := os.ReadFile(path) // #nosec G304 -- test output path.
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	var doc struct {
		SchemaVersion int    `json:"schemaVersion"`
		Status        string `json:"status"`
		Range         *struct {
			End string `json:"end"`
		} `json:"range"`
		Entries    []json.RawMessage `json:"entries"`
		Diagnostic *struct {
			Code string `json:"code"`
		} `json:"diagnostic"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("%s: %v\n%s", path, err, data)
	}
	if doc.SchemaVersion != historyResultSchemaVersion || doc.Status != status {
		t.Fatalf("%s = %s", path, data)
	}
	if status == "OK" {
		if doc.Range == nil || doc.Range.End != "root" || len(doc.Entries) != 5 || doc.Diagnostic != nil {
			t.Fatalf("%s = %s", path, data)
		}
		return
	}
	if doc.Diagnostic == nil || doc.Diagnostic.Code != code || doc.Range != nil || doc.Entries != nil {
		t.Fatalf("%s = %s", path, data)
	}
}
