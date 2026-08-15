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
	for _, expected := range []string{"Usage:", "Arguments:", "Flags:", "Examples:", "--dirs-only", "--no-gitignore", "--config", "--no-user-config", "--no-config", "--preset", "markdown-tree", "config", "preset"} {
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
		{"--format", "markdown-tree", "--style", "ascii"},
		{"--format", "yaml"},
		{"--style", "auto"},
		{"--ignore", "../outside"},
		{"--preset", "unknown"},
		{"--preset="},
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

func TestCLIBuiltInPresetsAndOverrides(t *testing.T) {
	root := filepath.Join(t.TempDir(), "preset project é")
	for _, directory := range []string{
		filepath.Join(root, "src", "domain", "deep"),
		filepath.Join(root, "packages", "web", "dist"),
		filepath.Join(root, "packages", "web", "build"),
	} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{
		filepath.Join(root, "main.go"),
		filepath.Join(root, "bundle.js.map"),
		filepath.Join(root, "src", "domain", "model.go"),
		filepath.Join(root, "packages", "web", "dist", "bundle.js"),
		filepath.Join(root, "packages", "web", "build", "artifact.txt"),
	} {
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	docs, stderr, code := executeForTest(t, root, "--preset", "docs", "--no-config")
	if code != 0 || stderr != "" || !strings.HasPrefix(docs, "```text\n") || !strings.Contains(docs, "main.go") {
		t.Fatalf("docs=(%q, %q, %d)", docs, stderr, code)
	}

	compact, stderr, code := executeForTest(t, root, "--preset", "compact", "--no-config")
	if code != 0 || stderr != "" || strings.HasPrefix(compact, "```") || strings.Contains(compact, "main.go") || !strings.Contains(compact, "src/") {
		t.Fatalf("compact=(%q, %q, %d)", compact, stderr, code)
	}

	monorepo, stderr, code := executeForTest(t, root, "--preset", "monorepo", "--no-config")
	if code != 0 || stderr != "" || strings.Contains(monorepo, "dist/") || strings.Contains(monorepo, "build/") || strings.Contains(monorepo, "main.go") {
		t.Fatalf("monorepo=(%q, %q, %d)", monorepo, stderr, code)
	}

	ai, stderr, code := executeForTest(t, root, "--preset", "ai", "--no-config")
	if code != 0 || stderr != "" || !strings.HasPrefix(ai, "```text\n") || !strings.Contains(ai, "main.go") || strings.Contains(ai, "bundle.js.map") || strings.Contains(ai, "dist/") || strings.Contains(ai, "build/") {
		t.Fatalf("ai=(%q, %q, %d)", ai, stderr, code)
	}

	overridden, stderr, code := executeForTest(t, root, "--preset", "compact", "--dirs-only=false", "--depth", "6", "--no-config")
	if code != 0 || stderr != "" || !strings.Contains(overridden, "main.go") || strings.HasPrefix(overridden, "```") {
		t.Fatalf("overridden compact=(%q, %q, %d)", overridden, stderr, code)
	}

	jsonOverride, stderr, code := executeForTest(t, root, "--preset", "docs", "--format", "json", "--no-config")
	if code != 0 || stderr != "" || !strings.Contains(jsonOverride, `"schemaVersion": 1`) || strings.HasPrefix(jsonOverride, "```text") {
		t.Fatalf("JSON override=(%q, %q, %d)", jsonOverride, stderr, code)
	}
}

func TestCLIPresetConfigurationAndReset(t *testing.T) {
	root := filepath.Join(t.TempDir(), "preset config é")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "visible.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	userBase := t.TempDir()
	writeCLIConfig(t, filepath.Join(userBase, "dirloom", "config.yaml"), "schemaVersion: 1\npreset: compact\n")
	writeCLIConfig(t, filepath.Join(root, ".dirloom.yaml"), "schemaVersion: 1\npreset: null\n")
	loader := configuration.NewLoader(configuration.WithUserConfigDir(func() (string, error) { return userBase, nil }))

	stdout, stderr, code := executeForTestWithLoader(t, loader, root)
	if code != 0 || stderr != "" || !strings.Contains(stdout, "visible.txt") || strings.HasPrefix(stdout, "```") {
		t.Fatalf("project reset=(%q, %q, %d)", stdout, stderr, code)
	}

	writeCLIConfig(t, filepath.Join(root, ".dirloom.yaml"), "schemaVersion: 1\npreset: ai\n")
	stdout, stderr, code = executeForTestWithLoader(t, loader, root, "--preset", "none")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "visible.txt") || strings.HasPrefix(stdout, "```") {
		t.Fatalf("CLI reset=(%q, %q, %d)", stdout, stderr, code)
	}

	stdout, stderr, code = executeForTestWithLoader(t, loader, root, "--no-config", "--preset", "ai")
	if code != 0 || stderr != "" || !strings.HasPrefix(stdout, "```text\n") {
		t.Fatalf("no-config preset=(%q, %q, %d)", stdout, stderr, code)
	}
}

func TestCLIPresetExplainAndConfigDiagnostics(t *testing.T) {
	stdout, stderr, code := executeForTest(t, "preset")
	if code != 0 || stderr != "" {
		t.Fatalf("preset help=(%q, %q, %d)", stdout, stderr, code)
	}
	for _, name := range configuration.PresetNames() {
		if !strings.Contains(stdout, name) {
			t.Errorf("preset help is missing %q\n%s", name, stdout)
		}
	}

	stdout, stderr, code = executeForTest(t, "preset", "explain", "ai")
	if code != 0 || stderr != "" {
		t.Fatalf("text explain=(%q, %q, %d)", stdout, stderr, code)
	}
	for _, want := range []string{"Preset: ai", "depth: 4", "format: markdown", "**/dist", "*.map"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("preset explanation missing %q\n%s", want, stdout)
		}
	}

	stdout, stderr, code = executeForTest(t, "preset", "explain", "compact", "--as", "json")
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"schemaVersion": 1`) || !strings.Contains(stdout, `"name": "compact"`) || !strings.Contains(stdout, `"ignore": []`) {
		t.Fatalf("JSON explain=(%q, %q, %d)", stdout, stderr, code)
	}

	root := t.TempDir()
	writeCLIConfig(t, filepath.Join(root, ".dirloom.yaml"), "schemaVersion: 1\npreset: ai\n")
	stdout, stderr, code = executeForTest(t, "config", "explain", root, "--depth", "6")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Preset: ai (project:") || !strings.Contains(stdout, "format: markdown (project preset ai:") || !strings.Contains(stdout, "depth: 6 (cli)") {
		t.Fatalf("config text explain=(%q, %q, %d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTest(t, "config", "explain", root, "--as", "json")
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"name": "ai"`) || !strings.Contains(stdout, `"preset": "ai"`) {
		t.Fatalf("config JSON explain=(%q, %q, %d)", stdout, stderr, code)
	}

	invalid := [][]string{
		{"preset", "explain"},
		{"preset", "explain", "ai", "extra"},
		{"preset", "explain", "none"},
		{"preset", "explain", "AI"},
		{"preset", "explain", "ai", "--as", "yaml"},
		{"preset", "explain", "ai", "--config", "config.yaml"},
		{"preset", "explain", "ai", "--no-config"},
		{"preset", "explain", "ai", "--no-user-config"},
	}
	for _, args := range invalid {
		stdout, stderr, code = executeForTest(t, args...)
		if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "Error: ") {
			t.Fatalf("Execute(%#v)=(%q, %q, %d)", args, stdout, stderr, code)
		}
	}
}

func TestCLIPresetErrorsPreserveOutputAndClassifyWriteFailure(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(t.TempDir(), "tree.txt")
	if err := os.WriteFile(output, []byte("preserved"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := executeForTest(t, root, "--preset", "unknown", "--output", output)
	if code != 2 || stdout != "" || !strings.Contains(stderr, "unsupported preset") {
		t.Fatalf("invalid preset=(%q, %q, %d)", stdout, stderr, code)
	}
	data, err := os.ReadFile(output)
	if err != nil || string(data) != "preserved" {
		t.Fatalf("output changed after preset error: data=%q err=%v", data, err)
	}

	loader := configuration.NewLoader(configuration.WithUserConfigDir(func() (string, error) { return "", errors.New("must not be called") }))
	var errorOutput bytes.Buffer
	code = executeWithLoader(context.Background(), []string{"preset", "explain", "ai"}, failingWriter{}, &errorOutput, "v0.1.0-test", loader)
	if code != 1 || !strings.Contains(errorOutput.String(), "write preset explanation") {
		t.Fatalf("write failure=(stderr=%q, code=%d)", errorOutput.String(), code)
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

	stdout, stderr, code = executeForTest(t, root, "--format", "markdown-tree", "--depth", "1", "--ignore", "*.log", "--ignore", "*.tmp")
	if code != 0 || stderr != "" || stdout != "- `projet espace é/`\n  - `src/`\n  - `README.md`\n" {
		t.Fatalf("markdown-tree=(stdout=%q, stderr=%q, code=%d)", stdout, stderr, code)
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

func TestCLIStyleConflictIncludesConfiguredSemanticMarkdown(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, filepath.Join(root, ".dirloom.yaml"), "schemaVersion: 1\ndefaults:\n  format: markdown-tree\n  style: ascii\n")
	stdout, stderr, code := executeForTest(t, root)
	if code != 0 || stderr != "" || !strings.HasPrefix(stdout, "- `") {
		t.Fatalf("configured inactive style=(%q, %q, %d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTest(t, root, "--style", "ascii")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "--style cannot be used with --format markdown-tree") {
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
	presetDocumentation, err := os.ReadFile(filepath.Join("..", "..", "docs", "presets.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "docs/presets.md") {
		t.Fatal("README does not link to docs/presets.md")
	}
	markdownTreeDocumentation, err := os.ReadFile(filepath.Join("..", "..", "docs", "markdown-tree.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "docs/markdown-tree.md") || !strings.Contains(string(markdownTreeDocumentation), "dirloom --format markdown-tree") {
		t.Fatal("README or semantic Markdown documentation is missing")
	}
	stdout, stderr, code := executeForTest(t, "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("help=(%q, %q, %d)", stdout, stderr, code)
	}
	for _, option := range []string{"--config", "--no-user-config", "--no-config", "--preset", "--depth", "--dirs-only", "--hidden", "--format", "--style", "--no-default-ignore", "--no-gitignore"} {
		if !strings.Contains(stdout, option) {
			t.Errorf("CLI help is missing documented option %q", option)
		}
		if !strings.Contains(string(documentation), option) {
			t.Errorf("public configuration documentation is missing option %q", option)
		}
	}
	if !strings.Contains(stdout, "preset") || !strings.Contains(string(presetDocumentation), "dirloom preset explain") {
		t.Fatal("preset help or public documentation is missing")
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

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("synthetic write failure")
}
