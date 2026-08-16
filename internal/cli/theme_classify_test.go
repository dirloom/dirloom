package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/presentation"
	"github.com/dirloom/dirloom/internal/presentation/catalog"
	"github.com/dirloom/dirloom/internal/tree"
)

func TestThemeClassifyRealFilesystemEntriesTextAndJSON(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project é")
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(root, "src", "main.go")
	if err := os.WriteFile(filePath, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := executeForTest(t, "theme", "classify", filepath.Join("src", "main.go"), "--root", root, "--theme", "vivid")
	if code != 0 || stderr != "" {
		t.Fatalf("text=(%q,%q,%d)", stdout, stderr, code)
	}
	for _, want := range []string{"Path: src/main.go", "Type: file", "Kind: source.go", "Roles: source", "Matched by: extension (.go)", "Theme: vivid (built-in)", "Text: color=#65D6BA", "Icon: unicode=\"•\" nerd=\"󰟓\" color=#65D6BA"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("text missing %q\n%s", want, stdout)
		}
	}
	if strings.ContainsRune(stdout, '\x1b') {
		t.Fatalf("classification diagnostic is decorated: %q", stdout)
	}

	stdout, stderr, code = executeForTest(t, "theme", "classify", filePath, "--root", root, "--theme", "vivid", "--as", "json")
	if code != 0 || stderr != "" {
		t.Fatalf("json=(%q,%q,%d)", stdout, stderr, code)
	}
	var document presentation.ClassifyDocument
	if err := json.Unmarshal([]byte(stdout), &document); err != nil {
		t.Fatal(err)
	}
	if document.SchemaVersion != 1 || document.Path != "src/main.go" || document.Type != tree.NodeFile || document.Classification.Kind != "source.go" || !reflectRoles(document.Classification.Roles, []catalog.Role{catalog.RoleSource}) || document.Theme.Name != "vivid" || document.Theme.Source.Path != "" || document.Style.Styles == nil {
		t.Fatalf("classification JSON = %#v", document)
	}

	stdout, stderr, code = executeForTest(t, "theme", "classify", "node_modules", "--root", root)
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Type: directory") || !strings.Contains(stdout, "Roles: vendor") {
		t.Fatalf("directory=(%q,%q,%d)", stdout, stderr, code)
	}
}

func TestThemeClassifyUsesLstatWithoutFollowingFinalSymlink(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.go")
	if err := os.WriteFile(outside, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "external.go")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	stdout, stderr, code := executeForTest(t, "theme", "classify", "external.go", "--root", root, "--as", "json")
	if code != 0 || stderr != "" {
		t.Fatalf("symlink=(%q,%q,%d)", stdout, stderr, code)
	}
	var document presentation.ClassifyDocument
	if err := json.Unmarshal([]byte(stdout), &document); err != nil {
		t.Fatal(err)
	}
	if document.Type != tree.NodeSymlink || document.Classification.Kind != "symlink" || document.Classification.Source != catalog.SourceNodeType {
		t.Fatalf("symlink classification = %#v", document)
	}

	broken := filepath.Join(root, "broken.go")
	if err := os.Symlink(filepath.Join(t.TempDir(), "missing.go"), broken); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = executeForTest(t, "theme", "classify", "broken.go", "--root", root, "--as", "json")
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"type": "symlink"`) {
		t.Fatalf("broken symlink=(%q,%q,%d)", stdout, stderr, code)
	}

	outsideDirectory := t.TempDir()
	if err := os.WriteFile(filepath.Join(outsideDirectory, "secret.go"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	parentLink := filepath.Join(root, "escape")
	if err := os.Symlink(outsideDirectory, parentLink); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = executeForTest(t, "theme", "classify", filepath.Join("escape", "secret.go"), "--root", root)
	if code != 2 || stdout != "" || !strings.Contains(stderr, "resolves outside root") {
		t.Fatalf("parent symlink escape=(%q,%q,%d)", stdout, stderr, code)
	}
}

func TestThemeClassifyConfinementValidationAndErrorContracts(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.go")
	if err := os.WriteFile(outside, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	prototype := filepath.Join(t.TempDir(), "prototype.yaml")
	writeCLIConfig(t, prototype, "schemaVersion: 1\nname: prototype\nappearance: dark\n")

	tests := [][]string{
		{"theme", "classify"},
		{"theme", "classify", "one", "two"},
		{"theme", "classify", "missing.go", "--root", root},
		{"theme", "classify", outside, "--root", root},
		{"theme", "classify", ".." + string(filepath.Separator) + filepath.Base(outside), "--root", root},
		{"theme", "classify", "missing.go", "--root", root, "--theme", prototype},
		{"theme", "classify", "missing.go", "--root", root, "--theme", "ocean"},
		{"theme", "classify", "missing.go", "--root", root, "--as", "yaml"},
		{"theme", "classify", "missing.go", "--root", root, "--no-config"},
		{"theme", "classify", "missing.go", "--root", root, "--preset", "ai"},
		{"theme", "classify", "missing.go", "--root", root, "--icons", "unicode"},
		{"theme", "classify", "missing.go", "--root", root, "--output", "x"},
	}
	for _, args := range tests {
		stdout, stderr, code := executeForTest(t, args...)
		if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "Error: ") {
			t.Errorf("%#v=(%q,%q,%d)", args, stdout, stderr, code)
		}
	}
	_, prototypeError, _ := executeForTest(t, "theme", "classify", "missing.go", "--root", root, "--theme", prototype)
	if !strings.Contains(prototypeError, "catalogVersion is required") || strings.Contains(prototypeError, "does not exist\n") {
		t.Fatalf("theme error did not precede target access: %q", prototypeError)
	}
}

func TestThemeClassifyCustomThemeAndTransactionalWriteFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	themePath := filepath.Join(t.TempDir(), "team theme.yaml")
	writeCLIConfig(t, themePath, `schemaVersion: 1
catalogVersion: 1
name: team
appearance: dark
rules:
  - match: {name: README.md}
    color: ansi:cyan
    icons: {unicode: "R", nerd: null}
`)
	stdout, stderr, code := executeForTest(t, "theme", "classify", "README.md", "--root", root, "--theme", themePath, "--as", "json")
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"name": "team"`) || !strings.Contains(stdout, `"unicode": "R"`) || strings.Contains(stdout, filepath.ToSlash(themePath)) {
		t.Fatalf("custom=(%q,%q,%d)", stdout, stderr, code)
	}

	var errorOutput bytes.Buffer
	code = Execute(context.Background(), []string{"theme", "classify", "README.md", "--root", root}, failingWriter{}, &errorOutput, "test")
	if code != 1 || !strings.Contains(errorOutput.String(), "write theme classification") {
		t.Fatalf("write failure=(%q,%d)", errorOutput.String(), code)
	}
}

func reflectRoles(got, want []catalog.Role) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}
