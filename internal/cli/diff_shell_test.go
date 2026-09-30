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

// TestDiffShellExitCodes proves the diff exit-code contract through a real
// operating-system shell: bash on POSIX, PowerShell on Windows. Exit 6 is
// covered internally by TestDiffInternalRendering.
func TestDiffShellExitCodes(t *testing.T) {
	bin := buildVerifyBinary(t)
	work := t.TempDir()
	out := t.TempDir()
	if err := os.Mkdir(filepath.Join(work, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "src", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		runPowerShellDiffSmoke(t, bin, work, out)
		return
	}
	runBashDiffSmoke(t, bin, work, out)
	assertDiffShellJSON(t, filepath.Join(out, "same.json"), "NO_DIFFERENCES")
	assertDiffShellJSON(t, filepath.Join(out, "diff.json"), "DIFFERENCES")
}

func runBashDiffSmoke(t *testing.T, bin, work, out string) {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal("bash is required to prove POSIX $? for diff")
	}
	script := `
set -euo pipefail
cd "$1"
bin=$2
out=$3
"$bin" snapshot --no-config --output architecture.dlm.json
set +e
"$bin" diff snapshot:architecture.dlm.json snapshot:architecture.dlm.json --format json >"$out/same.json" 2>"$out/same.err"
code=$?
set -e
test "$code" -eq 0
test ! -s "$out/same.err"
printf 'package extra\n' > src/extra.go
set +e
"$bin" diff snapshot:architecture.dlm.json live:. --format json >"$out/diff.json" 2>"$out/diff.err"
code=$?
set -e
test "$code" -eq 1
test ! -s "$out/diff.err"
set +e
"$bin" diff live:. live:. >"$out/usage.out" 2>"$out/usage.err"
code=$?
set -e
test "$code" -eq 2
printf '{}\n' > bad.dlm.json
set +e
"$bin" diff snapshot:bad.dlm.json snapshot:architecture.dlm.json >"$out/bad.out" 2>"$out/bad.err"
code=$?
set -e
test "$code" -eq 3
sed 's/"schemaVersion": 1/"schemaVersion": 2/' architecture.dlm.json > future.dlm.json
set +e
"$bin" diff snapshot:future.dlm.json snapshot:architecture.dlm.json >"$out/future.out" 2>"$out/future.err"
code=$?
set -e
test "$code" -eq 4
set +e
"$bin" diff snapshot:missing.dlm.json snapshot:architecture.dlm.json >"$out/missing.out" 2>"$out/missing.err"
code=$?
set -e
test "$code" -eq 5
`
	cmd := exec.Command(bash, "-c", script, "bash", work, bin, out)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash diff smoke: %v\n%s", err, output)
	}
}

func runPowerShellDiffSmoke(t *testing.T, bin, work, out string) {
	t.Helper()
	shell, err := exec.LookPath("pwsh")
	if err != nil {
		shell, err = exec.LookPath("powershell")
	}
	if err != nil {
		t.Fatal("pwsh or powershell is required to prove $LASTEXITCODE for diff")
	}
	script := `
param(
  [Parameter(Mandatory = $true)][string]$Work,
  [Parameter(Mandatory = $true)][string]$Bin,
  [Parameter(Mandatory = $true)][string]$Out
)
$ErrorActionPreference = 'Stop'
Set-Location $Work
& $Bin snapshot --no-config --output architecture.dlm.json
if ($LASTEXITCODE -ne 0) { throw "snapshot LASTEXITCODE=$LASTEXITCODE" }
& $Bin diff snapshot:architecture.dlm.json snapshot:architecture.dlm.json --format json > (Join-Path $Out 'same.json') 2> (Join-Path $Out 'same.err')
if ($LASTEXITCODE -ne 0) { throw "same LASTEXITCODE=$LASTEXITCODE" }
if ((Get-Item (Join-Path $Out 'same.err')).Length -ne 0) { throw "same stderr not empty" }
Set-Content -Encoding ascii -Path (Join-Path $Work 'src/extra.go') -Value "package extra"
& $Bin diff snapshot:architecture.dlm.json live:. --format json > (Join-Path $Out 'diff.json') 2> (Join-Path $Out 'diff.err')
if ($LASTEXITCODE -ne 1) { throw "diff LASTEXITCODE=$LASTEXITCODE" }
$diffErr = Get-Content -Raw (Join-Path $Out 'diff.err') -ErrorAction SilentlyContinue
if ($diffErr) { throw "diff stderr not empty: $diffErr" }
& $Bin diff live:. live:. > (Join-Path $Out 'usage.out') 2> (Join-Path $Out 'usage.err')
if ($LASTEXITCODE -ne 2) { throw "usage LASTEXITCODE=$LASTEXITCODE" }
Set-Content -Encoding ascii -Path (Join-Path $Work 'bad.dlm.json') -Value '{}'
& $Bin diff snapshot:bad.dlm.json snapshot:architecture.dlm.json > (Join-Path $Out 'bad.out') 2> (Join-Path $Out 'bad.err')
if ($LASTEXITCODE -ne 3) { throw "invalid LASTEXITCODE=$LASTEXITCODE" }
(Get-Content -Raw architecture.dlm.json) -replace '"schemaVersion": 1', '"schemaVersion": 2' | Set-Content -Encoding ascii -Path future.dlm.json
& $Bin diff snapshot:future.dlm.json snapshot:architecture.dlm.json > (Join-Path $Out 'future.out') 2> (Join-Path $Out 'future.err')
if ($LASTEXITCODE -ne 4) { throw "unsupported LASTEXITCODE=$LASTEXITCODE" }
& $Bin diff snapshot:missing.dlm.json snapshot:architecture.dlm.json > (Join-Path $Out 'missing.out') 2> (Join-Path $Out 'missing.err')
if ($LASTEXITCODE -ne 5) { throw "missing LASTEXITCODE=$LASTEXITCODE" }
`
	scriptPath := filepath.Join(t.TempDir(), "diff-smoke.ps1")
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(shell, "-NoProfile", "-File", scriptPath, work, bin, out)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("powershell diff smoke: %v\n%s", err, output)
	}
	assertDiffShellJSON(t, filepath.Join(out, "same.json"), "NO_DIFFERENCES")
	assertDiffShellJSON(t, filepath.Join(out, "diff.json"), "DIFFERENCES")
}

func assertDiffShellJSON(t *testing.T, path, status string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	var doc struct {
		SchemaVersion int    `json:"schemaVersion"`
		Status        string `json:"status"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("%s: %v\n%s", path, err, data)
	}
	if doc.SchemaVersion != 1 || doc.Status != status {
		t.Fatalf("%s = %+v", path, doc)
	}
}
