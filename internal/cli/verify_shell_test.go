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

func TestVerifyShellExitCodes(t *testing.T) {
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
		runPowerShellVerifySmoke(t, bin, work, out)
		return
	}
	runBashVerifySmoke(t, bin, work, out)
	assertShellJSON(t, filepath.Join(out, "match.json"), "MATCH")
	assertShellJSON(t, filepath.Join(out, "mismatch.json"), "MISMATCH")
}

func buildVerifyBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "dirloom")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-trimpath", "-o", bin, ".")
	cmd.Dir = filepath.Join("..", "..", "cmd", "dirloom")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build dirloom: %v\n%s", err, output)
	}
	return bin
}

func runBashVerifySmoke(t *testing.T, bin, work, out string) {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal("bash is required to prove POSIX $? for verify")
	}
	script := `
set -euo pipefail
cd "$1"
bin=$2
out=$3
"$bin" snapshot --no-config --output architecture.dlm.json
set +e
"$bin" verify architecture.dlm.json --format json >"$out/match.json" 2>"$out/match.err"
code=$?
set -e
test "$code" -eq 0
test ! -s "$out/match.err"
printf 'package extra\n' > src/extra.go
set +e
"$bin" verify architecture.dlm.json --format json >"$out/mismatch.json" 2>"$out/mismatch.err"
code=$?
set -e
test "$code" -eq 1
test ! -s "$out/mismatch.err"
printf '{}\n' > bad.dlm.json
set +e
"$bin" verify bad.dlm.json >"$out/bad.out" 2>"$out/bad.err"
code=$?
set -e
test "$code" -eq 3
`
	cmd := exec.Command(bash, "-c", script, "bash", work, bin, out)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash smoke: %v\n%s", err, output)
	}
}

func runPowerShellVerifySmoke(t *testing.T, bin, work, out string) {
	t.Helper()
	shell, err := exec.LookPath("pwsh")
	if err != nil {
		shell, err = exec.LookPath("powershell")
	}
	if err != nil {
		t.Fatal("pwsh or powershell is required to prove $LASTEXITCODE for verify")
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
& $Bin verify architecture.dlm.json --format json > (Join-Path $Out 'match.json') 2> (Join-Path $Out 'match.err')
if ($LASTEXITCODE -ne 0) { throw "match LASTEXITCODE=$LASTEXITCODE" }
if ((Get-Item (Join-Path $Out 'match.err')).Length -ne 0) { throw "match stderr not empty" }
Set-Content -Encoding ascii -Path (Join-Path $Work 'src/extra.go') -Value "package extra"
& $Bin verify architecture.dlm.json --format json > (Join-Path $Out 'mismatch.json') 2> (Join-Path $Out 'mismatch.err')
if ($LASTEXITCODE -ne 1) { throw "mismatch LASTEXITCODE=$LASTEXITCODE" }
$mismatchErr = Get-Content -Raw (Join-Path $Out 'mismatch.err') -ErrorAction SilentlyContinue
if ($mismatchErr -match 'Error:') { throw "mismatch stderr: $mismatchErr" }
if ($mismatchErr) { throw "mismatch stderr not empty: $mismatchErr" }
Set-Content -Encoding ascii -Path (Join-Path $Work 'bad.dlm.json') -Value '{}'
& $Bin verify bad.dlm.json > (Join-Path $Out 'bad.out') 2> (Join-Path $Out 'bad.err')
if ($LASTEXITCODE -ne 3) { throw "invalid LASTEXITCODE=$LASTEXITCODE" }
`
	scriptPath := filepath.Join(t.TempDir(), "verify-smoke.ps1")
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(shell, "-NoProfile", "-File", scriptPath, work, bin, out)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("powershell smoke: %v\n%s", err, output)
	}
	assertShellJSON(t, filepath.Join(out, "match.json"), "MATCH")
	assertShellJSON(t, filepath.Join(out, "mismatch.json"), "MISMATCH")
}

func assertShellJSON(t *testing.T, path, status string) {
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
