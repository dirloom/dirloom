package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHelpAndVersion(t *testing.T) {
	stdout, stderr, code := executeForTest(t, "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("help code=%d stderr=%q", code, stderr)
	}
	for _, expected := range []string{"Usage:", "Arguments:", "Flags:", "Examples:", "--dirs-only", "--no-gitignore"} {
		if !strings.Contains(stdout, expected) {
			t.Errorf("help is missing %q\n%s", expected, stdout)
		}
	}

	stdout, stderr, code = executeForTest(t, "--version")
	if code != 0 || stderr != "" || stdout != "dirloom v0.1.0-test\n" {
		t.Fatalf("version=(%q, %q, %d)", stdout, stderr, code)
	}
}

func TestInvalidArgumentsReturnExitCodeTwo(t *testing.T) {
	tests := [][]string{
		{"--depth", "invalid"},
		{"--depth", "-1"},
		{"--format", "json", "--style", "ascii"},
		{"--format", "yaml"},
		{"--style", "auto"},
		{"--ignore", "../outside"},
		{"one", "two"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			stdout, stderr, code := executeForTest(t, args...)
			if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "Error: ") {
				t.Fatalf("Execute(%#v)=(stdout=%q, stderr=%q, code=%d)", args, stdout, stderr, code)
			}
		})
	}
}

func TestRuntimeErrorReturnsExitCodeOneWithoutPartialOutput(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	stdout, stderr, code := executeForTest(t, missing)
	if code != 1 || stdout != "" || !strings.Contains(stderr, "does not exist") {
		t.Fatalf("missing directory=(stdout=%q, stderr=%q, code=%d)", stdout, stderr, code)
	}
}

func TestCLIEndToEndFormatsAndRepeatedIgnores(t *testing.T) {
	root := filepath.Join(t.TempDir(), "projet espace é")
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"README.md":   "readme",
		"debug.log":   "log",
		"scratch.tmp": "tmp",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	stdout, stderr, code := executeForTest(t, root, "--style", "ascii", "--ignore", "*.log", "--ignore", "*.tmp")
	if code != 0 || stderr != "" {
		t.Fatalf("text=(stdout=%q, stderr=%q, code=%d)", stdout, stderr, code)
	}
	want := "projet espace é/\n|-- src/\n`-- README.md\n"
	if stdout != want {
		t.Fatalf("text output\n got: %q\nwant: %q", stdout, want)
	}

	stdout, stderr, code = executeForTest(t, root, "--format", "markdown", "--style", "ascii", "--depth", "0")
	if code != 0 || stderr != "" || stdout != "```text\nprojet espace é/\n```\n" {
		t.Fatalf("markdown=(stdout=%q, stderr=%q, code=%d)", stdout, stderr, code)
	}

	stdout, stderr, code = executeForTest(t, root, "--format", "json", "--depth", "0")
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"schemaVersion": 1`) || !strings.Contains(stdout, `"children": []`) {
		t.Fatalf("json=(stdout=%q, stderr=%q, code=%d)", stdout, stderr, code)
	}
}

func TestCLITransactionalOutputAndNoFormatInference(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "visible.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "structure.md")
	if err := os.WriteFile(output, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := executeForTest(t, root, "--output", output)
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("output=(stdout=%q, stderr=%q, code=%d)", stdout, stderr, code)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.HasPrefix(text, "```") || strings.Contains(text, "structure.md") || !strings.Contains(text, "visible.txt") {
		t.Fatalf("unexpected file output: %q", text)
	}

	stdout, stderr, code = executeForTest(t, root, "--output", output, "--format", "markdown")
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("markdown output=(stdout=%q, stderr=%q, code=%d)", stdout, stderr, code)
	}
	data, err = os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "```text\n") {
		t.Fatalf("expected Markdown output, got %q", data)
	}
}

func TestCLIFileOutputMatchesStdoutBytes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "visible.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := executeForTest(t, root, "--format", "json")
	if code != 0 || stderr != "" {
		t.Fatalf("stdout render=(%q, %q, %d)", stdout, stderr, code)
	}
	output := filepath.Join(t.TempDir(), "tree.json")
	fileStdout, fileStderr, fileCode := executeForTest(t, root, "--format", "json", "--output", output)
	if fileCode != 0 || fileStdout != "" || fileStderr != "" {
		t.Fatalf("file render=(%q, %q, %d)", fileStdout, fileStderr, fileCode)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, []byte(stdout)) {
		t.Fatalf("file bytes differ from stdout\nfile=%q\nout=%q", data, stdout)
	}
}

func TestCLIOutputParentMustExist(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "missing", "tree.txt")
	stdout, stderr, code := executeForTest(t, root, "--output", output)
	if code != 1 || stdout != "" || !strings.Contains(stderr, "does not exist") {
		t.Fatalf("missing output parent=(stdout=%q, stderr=%q, code=%d)", stdout, stderr, code)
	}
}

func executeForTest(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Execute(context.Background(), args, &stdout, &stderr, "v0.1.0-test")
	return stdout.String(), stderr.String(), code
}
