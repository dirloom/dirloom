package packaging_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func bashPath(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		for _, root := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
			if root == "" {
				continue
			}
			candidate := filepath.Join(root, "Git", "bin", "bash.exe")
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}
	path, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is required to test Homebrew cask patching")
	}
	return path
}

func patcher(t *testing.T) string {
	t.Helper()
	return filepath.ToSlash(filepath.Join(repoRoot(t), ".github", "scripts", "lib", "patch-homebrew-cask.sh"))
}

func runBash(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(bashPath(t), args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}

func TestPatchHomebrewCaskUpdatesReleaseFieldsOnly(t *testing.T) {
	src := filepath.Join(repoRoot(t), "internal", "packaging", "testdata", "dirloom-0.3.2.rb")
	original, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	beforePath := filepath.Join(dir, "before.rb")
	caskPath := filepath.Join(dir, "dirloom.rb")
	if err := os.WriteFile(beforePath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caskPath, original, 0o644); err != nil {
		t.Fatal(err)
	}

	darwinArm := "1111111111111111111111111111111111111111111111111111111111111111"
	darwinX64 := "2222222222222222222222222222222222222222222222222222222222222222"
	linuxArm := "3333333333333333333333333333333333333333333333333333333333333333"
	linuxX64 := "4444444444444444444444444444444444444444444444444444444444444444"
	out, err := runBash(t, patcher(t),
		filepath.ToSlash(caskPath),
		"0.3.3",
		darwinArm, darwinX64, linuxArm, linuxX64,
	)
	if err != nil {
		t.Fatalf("patch failed: %v\n%s", err, out)
	}

	got, err := os.ReadFile(caskPath)
	if err != nil {
		t.Fatal(err)
	}
	gotText := string(got)
	want := string(original)
	want = strings.Replace(want, `version "0.3.2"`, `version "0.3.3"`, 1)
	want = strings.Replace(want, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", darwinArm, 1)
	want = strings.Replace(want, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", darwinX64, 1)
	want = strings.Replace(want, "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", linuxArm, 1)
	want = strings.Replace(want, "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", linuxX64, 1)
	if gotText != want {
		t.Fatalf("patched cask mismatch\n--- got ---\n%s\n--- want ---\n%s", gotText, want)
	}

	postflight := original[bytes.Index(original, []byte("postflight do")):]
	if !bytes.HasSuffix(got, postflight) {
		t.Fatal("postflight and trailing stanzas must remain byte-for-byte identical")
	}
	if bytes.Contains(got, []byte("generate_completions_from_executable")) {
		t.Fatal("publisher must not introduce generate_completions_from_executable")
	}

	assertOut, err := runBash(t, patcher(t), "--assert-fields-only",
		filepath.ToSlash(beforePath), filepath.ToSlash(caskPath))
	if err != nil {
		t.Fatalf("field-only assert failed: %v\n%s", err, assertOut)
	}
}

func TestAssertHomebrewCaskReleaseFieldsRejectsPostflightDrift(t *testing.T) {
	src := filepath.Join(repoRoot(t), "internal", "packaging", "testdata", "dirloom-0.3.2.rb")
	before, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	beforePath := filepath.Join(dir, "before.rb")
	afterPath := filepath.Join(dir, "after.rb")
	after := strings.Replace(string(before), "postflight do", "generate_completions_from_executable \"dirloom\"\n  postflight do", 1)
	if err := os.WriteFile(beforePath, before, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(afterPath, []byte(after), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runBash(t, patcher(t), "--assert-fields-only",
		filepath.ToSlash(beforePath), filepath.ToSlash(afterPath))
	if err == nil {
		t.Fatalf("expected field-only assert to reject postflight drift\n%s", out)
	}
}

func TestHomebrewPublisherDoesNotRewriteCask(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "scripts", "update-homebrew.sh"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if strings.Contains(text, "cat > Casks/dirloom.rb") {
		t.Fatal("update-homebrew.sh still rewrites Casks/dirloom.rb")
	}
	if !strings.Contains(text, "patch_homebrew_cask") {
		t.Fatal("update-homebrew.sh must patch release fields through patch_homebrew_cask")
	}
	if !strings.Contains(text, "Deleting orphan automation branch") {
		t.Fatal("update-homebrew.sh must delete orphan dirloom-VERSION branches")
	}
}

func TestHomebrewTapUpdateWorkflowIsManualOnly(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "packaging", "homebrew-tap", ".github", "workflows", "update.yml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if strings.Contains(text, "cron:") || regexp.MustCompile(`(?m)^[[:space:]]*schedule:`).MatchString(text) {
		t.Fatal("homebrew-tap Update cask must not run on a schedule")
	}
	if !strings.Contains(text, "workflow_dispatch:") {
		t.Fatal("homebrew-tap Update cask should remain a manual recovery tool")
	}
	if strings.Contains(text, "cat > Casks/dirloom.rb") {
		t.Fatal("homebrew-tap Update cask must not rewrite the cask")
	}
	if !strings.Contains(text, "update-homebrew.sh") {
		t.Fatal("homebrew-tap recovery must delegate to the official publisher")
	}
}
