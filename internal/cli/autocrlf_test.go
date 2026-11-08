package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/gittest"
)

// TestCRLFCheckoutKeepsGitIgnoreAndGitEquivalence pins the LOT 0 facts of
// the v0.4.0-rc1 plan on core.autocrlf: a checkout that writes the
// .gitignore with CRLF line endings applies the same rules as an LF one,
// and the structure of the checkout matches git:HEAD (Git Tree Source v1).
func TestCRLFCheckoutKeepsGitIgnoreAndGitEquivalence(t *testing.T) {
	repo := gittest.New(t)
	repo.Write(".gitignore", "ignored.txt\nbuild/\n*.log\n")
	repo.Write("app/main.go", "package main\n")
	repo.CommitAll("A")

	fingerprints := map[string]string{}
	for _, autocrlf := range []string{"false", "true"} {
		clone := filepath.Join(t.TempDir(), "clone")
		repo.Git("clone", "--quiet", "-c", "core.autocrlf="+autocrlf, fileURLForHistory(repo.Dir), clone)
		data, err := os.ReadFile(filepath.Join(clone, ".gitignore"))
		if err != nil {
			t.Fatal(err)
		}
		if crlf := bytes.Contains(data, []byte("\r\n")); crlf != (autocrlf == "true") {
			t.Fatalf("core.autocrlf=%s wrote the .gitignore as %q", autocrlf, data)
		}
		writeFiles(t, clone, [][2]string{{"build/out.bin", "x"}, {"ignored.txt", "x"}, {"app/debug.log", "x"}})

		stdout, stderr, code := executeForTest(t, clone, "--no-config")
		if code != 0 || !strings.Contains(stdout, "main.go") ||
			strings.Contains(stdout, "out.bin") || strings.Contains(stdout, "ignored.txt") || strings.Contains(stdout, "debug.log") {
			t.Fatalf("core.autocrlf=%s: tree = (%q, %q, %d), want the .gitignore rules applied", autocrlf, stdout, stderr, code)
		}
		fingerprints[autocrlf], stderr, code = executeForTest(t, "fingerprint", clone, "--no-config")
		if code != 0 {
			t.Fatalf("core.autocrlf=%s: fingerprint = %d: %s", autocrlf, code, stderr)
		}

		snap := filepath.Join(t.TempDir(), "checkout.dlm.json")
		if _, stderr, code := executeForTest(t, "snapshot", clone, "--no-config", "--output", snap); code != 0 {
			t.Fatalf("core.autocrlf=%s: snapshot = %d: %s", autocrlf, code, stderr)
		}
		t.Chdir(clone)
		stdout, stderr, code = executeForTest(t, "diff", "snapshot:"+snap, "git:HEAD")
		if code != 0 || !strings.Contains(stdout, "No structural differences.") {
			t.Fatalf("core.autocrlf=%s: diff snapshot: git:HEAD = (%q, %q, %d), want no difference", autocrlf, stdout, stderr, code)
		}
	}
	if fingerprints["true"] != fingerprints["false"] {
		t.Fatalf("CRLF and LF checkouts have different structures: %q, %q", fingerprints["true"], fingerprints["false"])
	}
}
