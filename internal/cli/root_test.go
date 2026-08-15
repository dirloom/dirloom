package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configuration "github.com/dirloom/dirloom/internal/config"
)

func TestHelpAndVersion(t *testing.T) {
	stdout, stderr, code := executeForTest(t, "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("help code=%d stderr=%q", code, stderr)
	}
	for _, expected := range []string{"Usage:", "Arguments:", "Flags:", "Examples:", "--dirs-only", "--no-gitignore", "--config", "--no-user-config", "--no-config", "config"} {
		if !strings.Contains(stdout, expected) {
			t.Errorf("help is missing %q\n%s", expected, stdout)
		}
	}
	if strings.Contains(stdout, "completion  Generate") {
		t.Fatalf("help exposes the out-of-scope completion command\n%s", stdout)
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

func TestCLIProjectConfigurationAndExplicitOverrides(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project configuration é")
	if err := os.MkdirAll(filepath.Join(root, "src", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(root, "README.md"),
		filepath.Join(root, "src", "visible.go"),
		filepath.Join(root, "src", "ignored.log"),
	} {
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeCLIConfig(t, filepath.Join(root, ".dirloom.yaml"), `schemaVersion: 1
defaults:
  depth: 1
  dirsOnly: true
  format: markdown
  style: ascii
ignore:
  - "*.log"
`)

	stdout, stderr, code := executeForTest(t, root)
	if code != 0 || stderr != "" || stdout != "```text\nproject configuration é/\n`-- src/\n```\n" {
		t.Fatalf("configured output=(%q, %q, %d)", stdout, stderr, code)
	}

	stdout, stderr, code = executeForTest(t, root, "--dirs-only=false", "--format", "text", "--style", "unicode", "--depth", "unlimited")
	if code != 0 || stderr != "" {
		t.Fatalf("overridden output=(%q, %q, %d)", stdout, stderr, code)
	}
	for _, want := range []string{"README.md", "src/", "visible.go", "nested/"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("overridden output missing %q\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "ignored.log") {
		t.Fatalf("project ignore was not applied:\n%s", stdout)
	}
}

func TestCLIUserConfigurationCanBeDisabled(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "visible.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	userBase := t.TempDir()
	writeCLIConfig(t, filepath.Join(userBase, "dirloom", "config.yaml"), "schemaVersion: 1\ndefaults:\n  depth: 0\n")
	loader := configuration.NewLoader(configuration.WithUserConfigDir(func() (string, error) { return userBase, nil }))

	stdout, stderr, code := executeForTestWithLoader(t, loader, root)
	if code != 0 || stderr != "" || strings.Contains(stdout, "visible.txt") {
		t.Fatalf("user-configured output=(%q, %q, %d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTestWithLoader(t, loader, root, "--no-user-config")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "visible.txt") {
		t.Fatalf("no-user-config output=(%q, %q, %d)", stdout, stderr, code)
	}
}

func TestCLIConfigurationControlsAndErrors(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "explicit.yaml")
	writeCLIConfig(t, configPath, "schemaVersion: 1\ndefaults:\n  depth: 0\n")

	stdout, stderr, code := executeForTest(t, root, "--config", configPath)
	if code != 0 || stderr != "" || stdout == "" {
		t.Fatalf("explicit config=(%q, %q, %d)", stdout, stderr, code)
	}
	ciOutput := filepath.Join(t.TempDir(), "structure.json")
	stdout, stderr, code = executeForTest(t, root, "--no-user-config", "--config", configPath, "--format", "json", "--output", ciOutput)
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("documented CI command=(%q, %q, %d)", stdout, stderr, code)
	}
	data, err := os.ReadFile(ciOutput)
	if err != nil || !strings.Contains(string(data), `"schemaVersion": 1`) {
		t.Fatalf("CI output=%q err=%v", data, err)
	}

	tests := [][]string{
		{root, "--config", filepath.Join(t.TempDir(), "missing.yaml")},
		{root, "--config="},
		{root, "--no-config", "--no-user-config"},
		{root, "--no-config", "--config", configPath},
	}
	for _, args := range tests {
		stdout, stderr, code = executeForTest(t, args...)
		if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "Error: ") {
			t.Fatalf("Execute(%#v)=(%q, %q, %d)", args, stdout, stderr, code)
		}
	}

	invalidPath := filepath.Join(root, ".dirloom.yaml")
	writeCLIConfig(t, invalidPath, "schemaVersion: 1\ndefaults:\n  format: yaml\n")
	outputPath := filepath.Join(t.TempDir(), "tree.txt")
	if err := os.WriteFile(outputPath, []byte("preserved"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = executeForTest(t, root, "--output", outputPath)
	if code != 2 || stdout != "" || !strings.Contains(stderr, "unsupported defaults.format") {
		t.Fatalf("invalid auto config=(%q, %q, %d)", stdout, stderr, code)
	}
	data, err = os.ReadFile(outputPath)
	if err != nil || string(data) != "preserved" {
		t.Fatalf("output changed after config error: data=%q err=%v", data, err)
	}
	stdout, stderr, code = executeForTest(t, root, "--no-config", "--depth", "0")
	if code != 0 || stdout == "" || stderr != "" {
		t.Fatalf("no-config with invalid automatic file=(%q, %q, %d)", stdout, stderr, code)
	}

	stdout, stderr, code = executeForTest(t, "--config", invalidPath, "--help")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Usage:") {
		t.Fatalf("help with invalid config=(%q, %q, %d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTest(t, "--config", invalidPath, "--version")
	if code != 0 || stderr != "" || stdout != "dirloom v0.1.0-test\n" {
		t.Fatalf("version with invalid config=(%q, %q, %d)", stdout, stderr, code)
	}
}

func TestCLIConfigExplainTextAndJSON(t *testing.T) {
	root := filepath.Join(t.TempDir(), "workspace é")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	writeCLIConfig(t, filepath.Join(root, ".dirloom.yaml"), `schemaVersion: 1
defaults:
  depth: 6
  format: json
ignore:
  - generated/**
`)

	stdout, stderr, code := executeForTest(t, "config", "explain", root, "--hidden=false")
	if code != 0 || stderr != "" {
		t.Fatalf("text explain=(%q, %q, %d)", stdout, stderr, code)
	}
	for _, want := range []string{"Root: " + root, "project: loaded", "depth: 6 (project:", "hidden: false (cli)", "style: unicode (built-in; inactive for json)", "generated/** (project:"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("text explanation missing %q\n%s", want, stdout)
		}
	}

	stdout, stderr, code = executeForTest(t, "config", "explain", root, "--as", "json", "--depth", "unlimited")
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"schemaVersion": 1`) || !strings.Contains(stdout, `"depth": null`) || !strings.Contains(stdout, `"source": "cli"`) {
		t.Fatalf("JSON explain=(%q, %q, %d)", stdout, stderr, code)
	}

	stdout, stderr, code = executeForTest(t, "config", "explain", root, "--as", "yaml")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "unsupported explanation format") {
		t.Fatalf("invalid explain format=(%q, %q, %d)", stdout, stderr, code)
	}

	stdout, stderr, code = executeForTest(t, "config", "unknown")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "expected a config subcommand") {
		t.Fatalf("invalid config subcommand=(%q, %q, %d)", stdout, stderr, code)
	}
}

func TestCLIStyleConflictIncludesConfiguredJSON(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, filepath.Join(root, ".dirloom.yaml"), "schemaVersion: 1\ndefaults:\n  format: json\n")
	stdout, stderr, code := executeForTest(t, root, "--style", "ascii")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "--style cannot be used with --format json") {
		t.Fatalf("style conflict=(%q, %q, %d)", stdout, stderr, code)
	}
}

func TestPublicConfigurationDocumentationMatchesCLI(t *testing.T) {
	documentationPath := filepath.Join("..", "..", "docs", "configuration.md")
	documentation, err := os.ReadFile(documentationPath)
	if err != nil {
		t.Fatal(err)
	}
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "docs/configuration.md") {
		t.Fatal("README does not link to docs/configuration.md")
	}
	stdout, stderr, code := executeForTest(t, "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("help=(%q, %q, %d)", stdout, stderr, code)
	}
	for _, option := range []string{"--config", "--no-user-config", "--no-config", "--depth", "--dirs-only", "--hidden", "--format", "--style", "--no-default-ignore", "--no-gitignore"} {
		if !strings.Contains(stdout, option) {
			t.Errorf("CLI help is missing documented option %q", option)
		}
		if !strings.Contains(string(documentation), option) {
			t.Errorf("public configuration documentation is missing option %q", option)
		}
	}
}

func executeForTest(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	loader := configuration.NewLoader(configuration.WithUserConfigDir(func() (string, error) {
		return "", errors.New("user configuration disabled in tests")
	}))
	return executeForTestWithLoader(t, loader, args...)
}

func executeForTestWithLoader(t *testing.T, loader *configuration.Loader, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := executeWithLoader(context.Background(), args, &stdout, &stderr, "v0.1.0-test", loader)
	return stdout.String(), stderr.String(), code
}

func writeCLIConfig(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
