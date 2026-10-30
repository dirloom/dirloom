package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/gittest"
)

// TestDiffGitRepositoryLayouts runs the golden comparison from a
// subdirectory, a bare clone and a linked worktree: Git discovers the same
// repository and the document is byte-identical.
func TestDiffGitRepositoryLayouts(t *testing.T) {
	repo, _ := gitGoldenRepository(t)
	bare := filepath.Join(t.TempDir(), "bare.git")
	repo.Git("clone", "--quiet", "--bare", repo.Dir, bare)
	worktree := filepath.Join(t.TempDir(), "linked")
	repo.Git("worktree", "add", "--quiet", "--detach", worktree, "HEAD")

	for name, dir := range map[string]string{
		"subdirectory":    filepath.Join(repo.Dir, "src", "payment", "api"),
		"bare repository": bare,
		"linked worktree": worktree,
	} {
		t.Run(name, func(t *testing.T) {
			t.Chdir(dir)
			stdout, stderr, code := executeForTest(t, "diff", "git:HEAD~1", "git:HEAD", "--format", "json")
			if code != 1 || stderr != "" {
				t.Fatalf("=(%q, %q, %d)", stdout, stderr, code)
			}
			assertGitDiffGolden(t, "git-moved.json", stdout)
		})
	}
}

// TestDiffGitTooOld puts a git reporting 2.44 first on PATH: the test binary
// copied as git.
func TestDiffGitTooOld(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(executable) // #nosec G304 -- this test binary.
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	name := "git"
	if runtime.GOOS == "windows" {
		name = "git.exe"
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv(fakeGitVersion, "2.44.0")

	stdout, stderr, code := executeForTest(t, "diff", "git:HEAD", "git:HEAD", "--format", "json")
	if code != 5 || stderr != "" {
		t.Fatalf("=(%q, %q, %d)", stdout, stderr, code)
	}
	var doc diffJSONDocument
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Status != "OBSERVATION_ERROR" || doc.Diagnostic == nil || doc.Diagnostic.Code != "git_unavailable" || doc.Diagnostic.Source != "a" ||
		doc.Diagnostic.Message != "git 2.44.0 is installed; Dirloom requires git 2.45 or newer" {
		t.Fatalf("%+v", doc.Diagnostic)
	}
}

// TestDiffGitPartialCloneIsNeverFetched observes a blobless clone whose link
// blob is missing: exit 5, and the blob is still missing afterwards.
func TestDiffGitPartialCloneIsNeverFetched(t *testing.T) {
	origin := gittest.New(t)
	origin.Write("src/main.go", "package main")
	origin.Git("add", "--all")
	target := origin.Blob([]byte("src/main.go"))
	origin.Index("120000", target, "link")
	origin.Commit("first")
	origin.Git("config", "uploadpack.allowfilter", "true")
	url := "file://" + filepath.ToSlash(origin.Dir)
	if runtime.GOOS == "windows" {
		url = "file:///" + filepath.ToSlash(origin.Dir)
	}
	dir := filepath.Join(t.TempDir(), "clone")
	origin.Git("clone", "--quiet", "--no-checkout", "--filter=blob:none", url, dir)
	clone := gittest.Open(t, dir)
	clone.Git("config", "protocol.file.allow", "always")
	t.Chdir(dir)

	stdout, stderr, code := executeForTest(t, "diff", "git:HEAD", "git:HEAD", "--format", "json")
	if code != 5 || stderr != "" || !strings.Contains(stdout, `"code": "git_object_unavailable"`) || !strings.Contains(stdout, `read link \"link\"`) {
		t.Fatalf("=(%q, %q, %d)", stdout, stderr, code)
	}
	t.Setenv("GIT_NO_LAZY_FETCH", "1")
	if _, err := clone.Run(nil, "cat-file", "-e", target); err == nil {
		t.Fatal("the missing blob was fetched")
	}
}

// TestDiffShellExitCodesGit proves the Git exit codes through a real shell:
// bash on POSIX, PowerShell on Windows. CI runs it with the other diff shell
// smoke tests.
func TestDiffShellExitCodesGit(t *testing.T) {
	bin := buildVerifyBinary(t)
	repo, _ := gitGoldenRepository(t)
	out := t.TempDir()
	if runtime.GOOS == "windows" {
		runPowerShellGitDiffSmoke(t, bin, repo.Dir, out)
	} else {
		runBashGitDiffSmoke(t, bin, repo.Dir, out)
	}
	assertDiffShellJSON(t, filepath.Join(out, "git-diff.json"), "DIFFERENCES")
	assertDiffShellJSON(t, filepath.Join(out, "git-missing.json"), "OBSERVATION_ERROR")
}

func runBashGitDiffSmoke(t *testing.T, bin, work, out string) {
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
set +e
"$bin" diff git:HEAD~1 git:HEAD --format json >"$out/git-diff.json" 2>"$out/git-diff.err"
code=$?
set -e
test "$code" -eq 1
test ! -s "$out/git-diff.err"
set +e
"$bin" diff git:HEAD git:HEAD >"$out/git-same.out" 2>"$out/git-same.err"
code=$?
set -e
test "$code" -eq 0
set +e
"$bin" diff git:nope git:HEAD --format json >"$out/git-missing.json" 2>"$out/git-missing.err"
code=$?
set -e
test "$code" -eq 5
test ! -s "$out/git-missing.err"
set +e
"$bin" diff git:HEAD live:. --format json >"$out/git-usage.out" 2>"$out/git-usage.err"
code=$?
set -e
test "$code" -eq 2
test ! -s "$out/git-usage.out"
`
	cmd := exec.Command(bash, "-c", script, "git-diff-smoke", work, bin, out)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bash git diff smoke: %v\n%s", err, output)
	}
}

func runPowerShellGitDiffSmoke(t *testing.T, bin, work, out string) {
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
& $Bin diff git:HEAD~1 git:HEAD --format json > (Join-Path $Out 'git-diff.json') 2> (Join-Path $Out 'git-diff.err')
if ($LASTEXITCODE -ne 1) { throw "git diff LASTEXITCODE=$LASTEXITCODE" }
& $Bin diff git:HEAD git:HEAD > (Join-Path $Out 'git-same.out') 2> (Join-Path $Out 'git-same.err')
if ($LASTEXITCODE -ne 0) { throw "git same LASTEXITCODE=$LASTEXITCODE" }
& $Bin diff git:nope git:HEAD --format json > (Join-Path $Out 'git-missing.json') 2> (Join-Path $Out 'git-missing.err')
if ($LASTEXITCODE -ne 5) { throw "git missing LASTEXITCODE=$LASTEXITCODE" }
$ErrorActionPreference = 'Continue'
& $Bin diff git:HEAD live:. --format json > (Join-Path $Out 'git-usage.out') 2> (Join-Path $Out 'git-usage.err')
if ($LASTEXITCODE -ne 2) { throw "git usage LASTEXITCODE=$LASTEXITCODE" }
`
	scriptPath := filepath.Join(t.TempDir(), "git-diff-smoke.ps1")
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(shell, "-NoProfile", "-File", scriptPath, work, bin, out)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("powershell git diff smoke: %v\n%s", err, output)
	}
}
