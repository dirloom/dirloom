package packaging_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// check-module-zip.sh guards anomaly A14 of the v0.4.0-rc1 plan: a tag whose
// module zip Go refuses cannot be installed with `go install`. These tests run
// it on throwaway repositories, offline: HTTP(S)_PROXY points at a closed local
// port, so a fetch from GitHub would fail.

const zipFixtureModule = "github.com/dirloom/module-zip-fixture"

func zipFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	repo := t.TempDir()
	for name, body := range files {
		path := filepath.Join(repo, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"add", "-A"},
		{"commit", "-q", "-m", "fixture"},
		{"checkout", "-q", "--detach"}, // as a pull request merge commit
	} {
		cmd := exec.Command("git", append([]string{"-c", "core.autocrlf=false"}, args...)...)
		cmd.Dir = repo
		cmd.Env = zipEnv(t)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	return repo
}

func zipEnv(t *testing.T) []string {
	t.Helper()
	empty := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		u := strings.ToUpper(k)
		if strings.HasPrefix(u, "GIT_") || strings.HasSuffix(u, "_PROXY") || u == "GOFLAGS" || u == "GOTOOLCHAIN" {
			continue
		}
		env = append(env, kv)
	}
	return append(env,
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+empty,
		"GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid",
		"GOTOOLCHAIN=local",
		"HTTP_PROXY=http://127.0.0.1:9", "HTTPS_PROXY=http://127.0.0.1:9",
		"http_proxy=http://127.0.0.1:9", "https_proxy=http://127.0.0.1:9",
	)
}

func runModuleZipCheck(t *testing.T, repo string) (string, error) {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go is required to build a module zip")
	}
	script := filepath.ToSlash(filepath.Join(repoRoot(t), ".github", "scripts", "check-module-zip.sh"))
	cmd := exec.Command(bashPath(t), script)
	cmd.Dir = repo
	cmd.Env = zipEnv(t)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestCheckModuleZipAcceptsAValidModule(t *testing.T) {
	repo := zipFixture(t, map[string]string{
		"go.mod":  "module " + zipFixtureModule + "\n\ngo 1.22\n",
		"main.go": "package main\n\nfunc main() {}\n",
		// A letter outside ASCII is a valid module file name.
		".plans/v0.2.0-Confort-d-utilisation-complétions.plan.md": "# plan\n",
	})
	out, err := runModuleZipCheck(t, repo)
	if err != nil {
		t.Fatalf("check-module-zip.sh failed on a valid module: %v\n%s", err, out)
	}
	if !strings.Contains(out, `"Sum": "h1:`) {
		t.Fatalf("no module sum in the output:\n%s", out)
	}
	if refs, _ := exec.Command("git", "-C", repo, "for-each-ref", "refs/heads/module-zip-check-*").Output(); len(refs) != 0 {
		t.Fatalf("the temporary ref was not removed: %s", refs)
	}
}

func TestCheckModuleZipRefusesAFileNameThatModulesRefuse(t *testing.T) {
	repo := zipFixture(t, map[string]string{
		"go.mod":  "module " + zipFixtureModule + "\n\ngo 1.22\n",
		"main.go": "package main\n\nfunc main() {}\n",
		// The name that made seven Dirloom tags unzippable: U+2014 is refused.
		"plans/Plan v0.3.0 — Presentation Richness.md": "# plan\n",
	})
	out, err := runModuleZipCheck(t, repo)
	if err == nil {
		t.Fatalf("check-module-zip.sh accepted an unzippable module:\n%s", out)
	}
	if !strings.Contains(out, "malformed file path") {
		t.Fatalf("the failure does not name the refused file:\n%s", out)
	}
}
