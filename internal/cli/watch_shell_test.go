package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// TestWatchShellExitCodes proves the watch exit statuses through a real
// shell: $? in bash, $LASTEXITCODE in PowerShell.
func TestWatchShellExitCodes(t *testing.T) {
	bin := buildVerifyBinary(t)
	work := t.TempDir()
	if err := os.Mkdir(filepath.Join(work, "tree"), 0o755); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		runPowerShellWatchSmoke(t, bin, work)
		return
	}
	runBashWatchSmoke(t, bin, work)
}

func runBashWatchSmoke(t *testing.T, bin, work string) {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal("bash is required to prove POSIX $? for watch")
	}
	script := `
set -euo pipefail
cd "$1"
bin=$2
status() { code=0; "$@" >out.txt 2>err.txt || code=$?; echo "$code"; }
test "$(status "$bin" watch --no-config --format json)" -eq 2
grep -q '^Error: ' err.txt
test "$(status "$bin" watch --no-config tree --poll 0s)" -eq 2
test "$(status "$bin" watch --no-config git:HEAD)" -eq 2
test "$(status "$bin" watch --no-config missing --format ndjson)" -eq 5
test ! -s err.txt
test "$(wc -l < out.txt)" -eq 2
tail -n 1 out.txt | grep -qx '{"schemaVersion":1,"event":"watch.stopped","reason":"error"}'
"$bin" watch --no-config tree --format ndjson --poll 100ms >stream.ndjson 2>stream.err &
pid=$!
for _ in $(seq 150); do grep -q '"event":"watch.started"' stream.ndjson && break; sleep 0.1; done
kill -TERM "$pid"
code=0
wait "$pid" || code=$?
test "$code" -eq 0
test ! -s stream.err
tail -n 1 stream.ndjson | grep -q '^{"schemaVersion":1,"event":"watch.stopped","generation":0,"reason":"interrupted"'
`
	cmd := exec.Command(bash, "-c", script, "bash", work, bin)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash smoke: %v\n%s", err, output)
	}
}

func runPowerShellWatchSmoke(t *testing.T, bin, work string) {
	t.Helper()
	shell, err := exec.LookPath("pwsh")
	if err != nil {
		shell, err = exec.LookPath("powershell")
	}
	if err != nil {
		t.Fatal("pwsh or powershell is required to prove $LASTEXITCODE for watch")
	}
	// A nonzero exit status is expected here: never let it stop the script.
	script := `
param(
  [Parameter(Mandatory = $true)][string]$Work,
  [Parameter(Mandatory = $true)][string]$Bin
)
$ErrorActionPreference = 'Continue'
$PSNativeCommandUseErrorActionPreference = $false
Set-Location $Work
& $Bin watch --no-config --format json > out.txt 2> err.txt
if ($LASTEXITCODE -ne 2) { throw "json LASTEXITCODE=$LASTEXITCODE" }
& $Bin watch --no-config --color always > out.txt 2> err.txt
if ($LASTEXITCODE -ne 2) { throw "color LASTEXITCODE=$LASTEXITCODE" }
& $Bin watch --no-config tree --poll 0s > out.txt 2> err.txt
if ($LASTEXITCODE -ne 2) { throw "poll LASTEXITCODE=$LASTEXITCODE" }
$lines = & $Bin watch --no-config missing --format ndjson 2> err.txt
if ($LASTEXITCODE -ne 5) { throw "missing root LASTEXITCODE=$LASTEXITCODE" }
if ((Get-Item err.txt).Length -ne 0) { throw "missing root stderr not empty" }
if ($lines.Count -ne 2 -or $lines[1] -ne '{"schemaVersion":1,"event":"watch.stopped","reason":"error"}') { throw "startup failure stream: $lines" }
exit 0
`
	scriptPath := filepath.Join(t.TempDir(), "watch-smoke.ps1")
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(shell, "-NoProfile", "-File", scriptPath, work, bin)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("powershell smoke: %v\n%s", err, output)
	}
}
