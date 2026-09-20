package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configuration "github.com/dirloom/dirloom/internal/config"
	"github.com/dirloom/dirloom/internal/identity"
)

func TestFingerprintHelp(t *testing.T) {
	stdout, stderr, code := executeForTest(t, "fingerprint", "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("help=(%q,%q,%d)", stdout, stderr, code)
	}
	for _, want := range []string{
		"structural view",
		"does not hash file contents",
		"--format",
		"--root",
		"--ignore",
		"--depth",
	} {
		if !strings.Contains(strings.ToLower(stdout), strings.ToLower(want)) && !strings.Contains(stdout, want) {
			t.Errorf("help missing %q\n%s", want, stdout)
		}
	}
}

func TestFingerprintTextJSONAndRoot(t *testing.T) {
	root := writeFingerprintFixture(t)
	text, stderr, code := executeForTest(t, "fingerprint", root, "--no-config")
	if code != 0 || stderr != "" {
		t.Fatalf("text=(%q,%q,%d)", text, stderr, code)
	}
	if !strings.HasPrefix(text, "dlm:v1:sha256:") || !strings.HasSuffix(text, "\n") || strings.Count(text, "\n") != 1 {
		t.Fatalf("text output %q", text)
	}
	if strings.ContainsRune(text, '\x1b') {
		t.Fatal("ANSI in text fingerprint")
	}
	fp, err := identity.Parse(strings.TrimSpace(text))
	if err != nil {
		t.Fatal(err)
	}

	viaRoot, stderr, code := executeForTest(t, "fingerprint", "--root", root, "--no-config")
	if code != 0 || stderr != "" || viaRoot != text {
		t.Fatalf("root flag=(%q,%q,%d)", viaRoot, stderr, code)
	}

	jsonOut, stderr, code := executeForTest(t, "fingerprint", root, "--no-config", "--format", "json")
	if code != 0 || stderr != "" {
		t.Fatalf("json=(%q,%q,%d)", jsonOut, stderr, code)
	}
	var document identity.JSONDocument
	if err := json.Unmarshal([]byte(jsonOut), &document); err != nil {
		t.Fatal(err)
	}
	if document.SchemaVersion != 1 || document.IdentityVersion != 1 || document.Algorithm != "sha256" || document.Fingerprint != fp.String() || document.NodeCount < 2 {
		t.Fatalf("%#v", document)
	}
	if strings.ContainsRune(jsonOut, '\x1b') {
		t.Fatal("ANSI in JSON")
	}
}

func TestFingerprintPresentationAndEnvInvariance(t *testing.T) {
	root := writeFingerprintFixture(t)
	baseline, stderr, code := executeForTest(t, "fingerprint", root, "--no-config")
	if code != 0 || stderr != "" {
		t.Fatal(stderr)
	}
	variants := [][]string{
		{root, "--no-config", "--theme", "midnight"},
		{root, "--no-config", "--theme", "vivid"},
		{root, "--no-config", "--icons", "never"},
		{root, "--no-config", "--icons", "unicode"},
		{root, "--no-config", "--icons", "nerd"},
		{root, "--no-config", "--color", "never"},
		{root, "--no-config", "--color", "always"},
		{root, "--no-config", "--format", "text"},
	}
	for _, args := range variants {
		stdout, stderr, code := executeForTest(t, append([]string{"fingerprint"}, args...)...)
		if code != 0 || stderr != "" || stdout != baseline {
			t.Fatalf("%v=(%q,%q,%d) want %q", args, stdout, stderr, code, baseline)
		}
	}
}

func TestFingerprintFilterAndConfig(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, filepath.Join(root, ".dirloom.yaml"), "schemaVersion: 1\nignore:\n  - generated\n")
	if err := os.Mkdir(filepath.Join(root, "generated"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "generated", "out.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "keep.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	configured, stderr, code := executeForTest(t, "fingerprint", root)
	if code != 0 || stderr != "" {
		t.Fatalf("configured=(%q,%d)", stderr, code)
	}
	ignored, _, code := executeForTest(t, "fingerprint", root, "--no-config", "--ignore", "generated")
	if code != 0 || ignored != configured {
		t.Fatalf("same artifact via different mechanisms\n%s\n%s", configured, ignored)
	}
	unfiltered, _, code := executeForTest(t, "fingerprint", root, "--no-config")
	if code != 0 || unfiltered == configured {
		t.Fatal("ignore did not change fingerprint")
	}
	userDir := t.TempDir()
	writeCLIConfig(t, filepath.Join(userDir, "dirloom", "config.yaml"), "schemaVersion: 1\nignore:\n  - generated\n")
	loader := configuration.NewLoader(configuration.WithUserConfigDir(func() (string, error) { return userDir, nil }))
	fromUser, stderr, code := executeForTestWithLoader(t, loader, "fingerprint", root)
	if code != 0 || stderr != "" || fromUser != configured {
		t.Fatalf("user config=(%q,%q,%d)", fromUser, stderr, code)
	}
	cliOverride, _, code := executeForTest(t, "fingerprint", root, "--no-config", "--ignore", "keep.go")
	if code != 0 || cliOverride == unfiltered {
		t.Fatal("CLI ignore did not apply")
	}
}

func TestFingerprintRelocationContentAndStructure(t *testing.T) {
	payload := [][2]string{{"src/main.go", "package main\n"}, {"README.md", "hi\n"}}
	a := t.TempDir()
	b := filepath.Join(t.TempDir(), "completely-different-name")
	writeFiles(t, a, payload)
	writeFiles(t, b, payload)
	left, _, code := executeForTest(t, "fingerprint", a, "--no-config")
	if code != 0 {
		t.Fatal(left)
	}
	right, _, code := executeForTest(t, "fingerprint", b, "--no-config")
	if code != 0 || left != right {
		t.Fatalf("relocation %q vs %q", left, right)
	}
	if err := os.WriteFile(filepath.Join(a, "src", "main.go"), []byte("package main\n// edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	content, _, code := executeForTest(t, "fingerprint", a, "--no-config")
	if code != 0 || content != left {
		t.Fatal("content-only change altered fingerprint")
	}
	if err := os.Mkdir(filepath.Join(a, "src", "payments"), 0o755); err != nil {
		t.Fatal(err)
	}
	structural, _, code := executeForTest(t, "fingerprint", a, "--no-config")
	if code != 0 || structural == left {
		t.Fatal("structural addition kept fingerprint")
	}
}

func TestFingerprintDepthHiddenGitignore(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, [][2]string{
		{"src/main.go", "x"},
		{".hidden", "x"},
		{"skip.log", "x"},
		{".gitignore", "*.log\n"},
	})
	full, _, code := executeForTest(t, "fingerprint", root, "--no-config", "--hidden")
	if code != 0 {
		t.Fatal(full)
	}
	noHidden, _, code := executeForTest(t, "fingerprint", root, "--no-config")
	if code != 0 || noHidden == full {
		t.Fatal("hidden visibility should change fingerprint")
	}
	depth, _, code := executeForTest(t, "fingerprint", root, "--no-config", "--depth", "0")
	if code != 0 || depth == noHidden {
		t.Fatal("depth should change fingerprint")
	}
	noGit, _, code := executeForTest(t, "fingerprint", root, "--no-config", "--no-gitignore")
	if code != 0 || noGit == noHidden {
		t.Fatal("gitignore should change fingerprint")
	}
}

func TestFingerprintRejectsBadUsage(t *testing.T) {
	root := t.TempDir()
	stdout, stderr, code := executeForTest(t, "fingerprint", root, "--format", "yaml")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "unsupported fingerprint format") {
		t.Fatalf("format=(%q,%q,%d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTest(t, "fingerprint", root, "--root", root)
	if code != 2 || stdout != "" {
		t.Fatalf("both roots=(%q,%q,%d)", stdout, stderr, code)
	}
}

func TestFingerprintGoldenBasicFixture(t *testing.T) {
	cases := []struct {
		fixture, golden string
	}{
		{"basic", "basic.fingerprint"},
		{"nested", "nested.fingerprint"},
	}
	for _, test := range cases {
		root := filepath.Join("..", "..", "testdata", "identity", test.fixture)
		stdout, stderr, code := executeForTest(t, "fingerprint", root, "--no-config", "--no-default-ignore", "--no-gitignore")
		if code != 0 || stderr != "" {
			t.Fatalf("%s: %q %q %d", test.fixture, stdout, stderr, code)
		}
		goldenPath := filepath.Join("..", "..", "testdata", "identity", "golden", test.golden)
		want, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatal(err)
		}
		if stdout != string(want) {
			t.Fatalf("%s golden drift\n got %q\nwant %q", test.fixture, stdout, want)
		}
	}
}

func TestFingerprintDoesNotPanicOnMalformedCanonicalInputs(t *testing.T) {
	if _, err := identity.Parse("::::"); err == nil {
		t.Fatal("expected parse error")
	}
	root := filepath.Join(t.TempDir(), "missing")
	stdout, stderr, code := executeForTest(t, "fingerprint", root, "--no-config")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "Error:") {
		t.Fatalf("missing=(%q,%q,%d)", stdout, stderr, code)
	}
}

func TestFingerprintCorpusFixtures(t *testing.T) {
	fixtures := []string{"basic", "nested", "spaces", "special-chars", "unicode-nfc", "hidden-files", "gitignore", "case-variants", "content"}
	for _, name := range fixtures {
		root := filepath.Join("..", "..", "testdata", "identity", name)
		stdout, stderr, code := executeForTest(t, "fingerprint", root, "--no-config", "--no-default-ignore", "--no-gitignore", "--hidden")
		if code != 0 || stderr != "" {
			t.Fatalf("%s=(%q,%q,%d)", name, stdout, stderr, code)
		}
		if _, err := identity.Parse(strings.TrimSpace(stdout)); err != nil {
			t.Fatalf("%s parse: %v", name, err)
		}
	}
}

func TestPublicFingerprintDocumentation(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "reference", "fingerprint.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"dirloom fingerprint",
		"dlm:v1:sha256:",
		"schemaVersion",
		"does not hash file contents",
		"--format json",
		"--root",
		"Ignore rules",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("fingerprint guide missing %q", want)
		}
	}
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "docs/reference/fingerprint.md") {
		t.Fatal("README does not link fingerprint reference")
	}
}

func writeFingerprintFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFiles(t, root, [][2]string{{"src/index.ts", "export {}\n"}, {"README.md", "hi\n"}})
	return root
}

func writeFiles(t *testing.T, root string, files [][2]string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		path := filepath.Join(root, filepath.FromSlash(file[0]))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(file[1]), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
