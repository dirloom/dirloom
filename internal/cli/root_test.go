package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configuration "github.com/dirloom/dirloom/internal/config"
	"github.com/dirloom/dirloom/internal/presentation"
)

func TestHelpAndVersion(t *testing.T) {
	stdout, stderr, code := executeForTest(t, "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("help code=%d stderr=%q", code, stderr)
	}
	for _, expected := range []string{"Usage:", "Arguments:", "Flags:", "Examples:", "--dirs-only", "--no-gitignore", "--config", "--no-user-config", "--no-config", "--preset", "--color", "--icons", "--theme", "markdown-tree", "mermaid", "graphviz", "d2", "--diagram-view", "--diagram-direction", "--diagram-max-nodes", "config", "preset", "theme"} {
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

func TestVisualPresentationCanonicalAndInteractiveContracts(t *testing.T) {
	root := filepath.Join(t.TempDir(), "visual project é")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"README.md", "main.go", "data.json"} {
		if err := os.WriteFile(filepath.Join(root, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	automatic, stderr, code := executeForTest(t, root, "--no-config")
	if code != 0 || stderr != "" {
		t.Fatalf("auto=(%q,%q,%d)", automatic, stderr, code)
	}
	neutral, stderr, code := executeForTest(t, root, "--no-config", "--color", "never", "--icons", "never")
	if code != 0 || stderr != "" || neutral != automatic || strings.ContainsRune(neutral, '\x1b') {
		t.Fatalf("neutral differs\nauto=%q\nneutral=%q\nstderr=%q code=%d", automatic, neutral, stderr, code)
	}

	evaluator := presentation.NewEvaluator(
		presentation.WithEnvironment(func(name string) (string, bool) {
			if name == "COLORTERM" {
				return "truecolor", true
			}
			return "", false
		}),
		presentation.WithTerminalDetection(func(io.Writer) bool { return true }),
		presentation.WithANSIPreparation(func(io.Writer) (func() error, error) { return func() error { return nil }, nil }),
		presentation.WithWindowsTerminalCompatibility(false),
	)
	loader := configuration.NewLoader(configuration.WithUserConfigDir(func() (string, error) { return "", errors.New("disabled") }))
	interactive, stderr, code := executeForTestWithDependencies(t, loader, evaluator, root, "--no-config", "--theme", "midnight", "--icons", "unicode")
	if code != 0 || stderr != "" || !strings.ContainsRune(interactive, '\x1b') || !strings.Contains(interactive, "¶") || !strings.Contains(interactive, "README.md") || !strings.Contains(interactive, "•") || !strings.Contains(interactive, "main.go") || !strings.Contains(interactive, "◇") || !strings.Contains(interactive, "data.json") {
		t.Fatalf("interactive=(%q,%q,%d)", interactive, stderr, code)
	}
	for _, line := range strings.Split(strings.TrimSuffix(interactive, "\n"), "\n") {
		if !strings.HasSuffix(line, "\x1b[0m") {
			t.Errorf("style leaked or reset missing in %q", line)
		}
	}
	vividOnly, stderr, code := executeForTestWithDependencies(t, loader, evaluator, root, "--no-config", "--theme", "vivid")
	if code != 0 || stderr != "" || !strings.ContainsRune(vividOnly, '\x1b') || strings.Contains(vividOnly, "•") || strings.Contains(vividOnly, "¶") || strings.Contains(vividOnly, "◇") {
		t.Fatalf("vivid alone must keep icons disabled=(%q,%q,%d)", vividOnly, stderr, code)
	}
	autoIcons, stderr, code := executeForTestWithDependencies(t, loader, evaluator, root, "--no-config", "--theme", "vivid", "--icons", "auto")
	if code != 0 || stderr != "" || !strings.Contains(autoIcons, "•") || !strings.Contains(autoIcons, "¶") || !strings.Contains(autoIcons, "◇") {
		t.Fatalf("explicit auto icons on TTY=(%q,%q,%d)", autoIcons, stderr, code)
	}

	outputPath := filepath.Join(t.TempDir(), "colored tree.txt")
	fileStdout, fileStderr, fileCode := executeForTest(t, root, "--no-config", "--color", "always", "--icons", "unicode", "--theme", "daylight", "--output", outputPath)
	data, err := os.ReadFile(outputPath)
	if err != nil || fileCode != 0 || fileStdout != "" || fileStderr != "" || !bytes.Contains(data, []byte("\x1b[")) || !bytes.Contains(data, []byte("¶")) || !bytes.Contains(data, []byte("README.md")) {
		t.Fatalf("forced file stdout=%q stderr=%q code=%d data=%q err=%v", fileStdout, fileStderr, fileCode, data, err)
	}
}

func TestVisualPresentationNoColorAndForcedModes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	loader := configuration.NewLoader(configuration.WithUserConfigDir(func() (string, error) { return "", errors.New("disabled") }))
	newEvaluator := func(noColor bool) *presentation.Evaluator {
		return presentation.NewEvaluator(
			presentation.WithEnvironment(func(name string) (string, bool) {
				if name == "NO_COLOR" && noColor {
					return "1", true
				}
				return "", false
			}),
			presentation.WithTerminalDetection(func(io.Writer) bool { return true }),
			presentation.WithANSIPreparation(func(io.Writer) (func() error, error) { return func() error { return nil }, nil }),
			presentation.WithWindowsTerminalCompatibility(false),
		)
	}
	withoutColor, stderr, code := executeForTestWithDependencies(t, loader, newEvaluator(true), root, "--no-config", "--icons", "unicode")
	if code != 0 || stderr != "" || strings.ContainsRune(withoutColor, '\x1b') || !strings.Contains(withoutColor, "• main.go") {
		t.Fatalf("NO_COLOR=(%q,%q,%d)", withoutColor, stderr, code)
	}
	writeCLIConfig(t, filepath.Join(root, ".dirloom.yaml"), "schemaVersion: 1\npresentation:\n  color: always\n  icons: unicode\n")
	configured, stderr, code := executeForTestWithDependencies(t, loader, newEvaluator(true), root)
	if code != 0 || stderr != "" || strings.ContainsRune(configured, '\x1b') || !strings.Contains(configured, "• main.go") {
		t.Fatalf("configured NO_COLOR=(%q,%q,%d)", configured, stderr, code)
	}
	forced, stderr, code := executeForTestWithDependencies(t, loader, newEvaluator(true), root, "--no-config", "--color", "always", "--icons", "nerd")
	if code != 0 || stderr != "" || !strings.ContainsRune(forced, '\x1b') || !strings.Contains(forced, "󰟓") || !strings.Contains(forced, "main.go") {
		t.Fatalf("forced=(%q,%q,%d)", forced, stderr, code)
	}
}

func TestVisualOptionsPreserveMachineFormatsAndRejectExplicitDecoration(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "visible.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	writeCLIConfig(t, filepath.Join(root, ".dirloom.yaml"), `schemaVersion: 1
presentation:
  color: always
  icons: nerd
  theme: midnight
`)
	configuredJSON, stderr, code := executeForTest(t, root, "--format", "json")
	if code != 0 || stderr != "" || strings.ContainsRune(configuredJSON, '\x1b') || strings.Contains(configuredJSON, "󰈔") {
		t.Fatalf("configured JSON=(%q,%q,%d)", configuredJSON, stderr, code)
	}
	canonicalJSON, stderr, code := executeForTest(t, root, "--no-config", "--format", "json")
	if code != 0 || stderr != "" || configuredJSON != canonicalJSON {
		t.Fatalf("JSON changed\nconfigured=%q\ncanonical=%q", configuredJSON, canonicalJSON)
	}
	configuredMarkdown, stderr, code := executeForTest(t, root, "--format", "markdown")
	canonicalMarkdown, _, _ := executeForTest(t, root, "--no-config", "--format", "markdown")
	if code != 0 || stderr != "" || configuredMarkdown != canonicalMarkdown || strings.ContainsRune(configuredMarkdown, '\x1b') {
		t.Fatalf("Markdown changed=(%q,%q,%d)", configuredMarkdown, stderr, code)
	}
	configuredSemanticMarkdown, stderr, code := executeForTest(t, root, "--format", "markdown-tree")
	canonicalSemanticMarkdown, _, _ := executeForTest(t, root, "--no-config", "--format", "markdown-tree")
	if code != 0 || stderr != "" || configuredSemanticMarkdown != canonicalSemanticMarkdown || strings.ContainsRune(configuredSemanticMarkdown, '\x1b') || strings.Contains(configuredSemanticMarkdown, "󰈔") {
		t.Fatalf("semantic Markdown changed=(%q,%q,%d)", configuredSemanticMarkdown, stderr, code)
	}

	invalid := [][]string{
		{root, "--no-config", "--format", "json", "--color", "auto"},
		{root, "--no-config", "--format", "json", "--color", "always"},
		{root, "--no-config", "--format", "markdown", "--icons", "auto"},
		{root, "--no-config", "--format", "markdown", "--icons", "unicode"},
		{root, "--no-config", "--format", "markdown-tree", "--color", "auto"},
		{root, "--no-config", "--format", "markdown-tree", "--icons", "unicode"},
		{root, "--no-config", "--format", "markdown-tree", "--theme", "default"},
		{root, "--no-config", "--format", "json", "--icons", "nerd"},
		{root, "--no-config", "--format", "json", "--theme", "default"},
	}
	for _, args := range invalid {
		stdout, errorOutput, exitCode := executeForTest(t, args...)
		if exitCode != 2 || stdout != "" || !strings.HasPrefix(errorOutput, "Error: ") {
			t.Fatalf("%#v=(%q,%q,%d)", args, stdout, errorOutput, exitCode)
		}
	}
	for _, args := range [][]string{{root, "--no-config", "--format", "json", "--color", "never", "--icons", "never"}, {root, "--no-config", "--format", "markdown", "--color", "never", "--icons", "never"}, {root, "--no-config", "--format", "markdown-tree", "--color", "never", "--icons", "never"}} {
		_, errorOutput, exitCode := executeForTest(t, args...)
		if exitCode != 0 || errorOutput != "" {
			t.Fatalf("neutral machine %#v=(%q,%d)", args, errorOutput, exitCode)
		}
	}
}

func TestThemeCommandsAndCustomThemeLifecycle(t *testing.T) {
	themePath := filepath.Join(t.TempDir(), "team theme é.yaml")
	writeCLIConfig(t, themePath, `schemaVersion: 1
catalogVersion: 1
name: team
description: Team theme
appearance: dark
tokens:
  node.future:
    color: default
rules:
  - match: {extension: .go}
    color: ansi:cyan
    icons: {unicode: "•", nerd: "󰟓"}
`)

	stdout, stderr, code := executeForTest(t, "theme", "list")
	if code != 0 || stderr != "" || strings.Index(stdout, "daylight") > strings.Index(stdout, "default") || strings.Index(stdout, "default") > strings.Index(stdout, "midnight") {
		t.Fatalf("list=(%q,%q,%d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTest(t, "theme", "list", "--as", "json")
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"schemaVersion": 1`) || !strings.Contains(stdout, `"themes": [`) || strings.Contains(stdout, `"themes": null`) {
		t.Fatalf("list JSON=(%q,%q,%d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTest(t, "theme", "explain", "midnight")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Theme: midnight") || !strings.Contains(stdout, "Appearance: dark") {
		t.Fatalf("explain=(%q,%q,%d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTest(t, "theme", "explain", themePath, "--as", "json")
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"name": "team"`) || !strings.Contains(stdout, `"kind": "file"`) {
		t.Fatalf("custom explain=(%q,%q,%d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTest(t, "theme", "validate", themePath)
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Valid theme: team") || !strings.Contains(stdout, "unknown-token") {
		t.Fatalf("validate=(%q,%q,%d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTest(t, "theme", "validate", themePath, "--as", "json")
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"valid": true`) || !strings.Contains(stdout, `"warnings": [`) {
		t.Fatalf("validate JSON=(%q,%q,%d)", stdout, stderr, code)
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = executeForTest(t, root, "--no-config", "--color", "never", "--icons", "unicode", "--theme", themePath)
	if code != 0 || stderr != "" || !strings.Contains(stdout, "• main.go") {
		t.Fatalf("custom render=(%q,%q,%d)", stdout, stderr, code)
	}

	invalid := [][]string{
		{"theme", "list", "extra"}, {"theme", "list", "--as", "yaml"},
		{"theme", "explain"}, {"theme", "explain", "ocean"}, {"theme", "explain", "midnight", "extra"},
		{"theme", "validate", "midnight"}, {"theme", "validate", filepath.Join(t.TempDir(), "missing.yaml")},
		{"theme", "list", "--no-config"}, {"theme", "list", "--no-user-config"}, {"theme", "list", "--config", "x.yaml"},
	}
	for _, args := range invalid {
		stdout, stderr, code = executeForTest(t, args...)
		if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "Error: ") {
			t.Fatalf("%#v=(%q,%q,%d)", args, stdout, stderr, code)
		}
	}
}

func TestThemeErrorsPrecedeScanPreserveOutputAndMaskedPathIsNotRead(t *testing.T) {
	root := t.TempDir()
	outputPath := filepath.Join(t.TempDir(), "tree.txt")
	if err := os.WriteFile(outputPath, []byte("preserved"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "missing.yaml")
	stdout, stderr, code := executeForTest(t, root, "--no-config", "--theme", missing, "--output", outputPath)
	if code != 2 || stdout != "" || !strings.Contains(stderr, "does not exist") {
		t.Fatalf("missing=(%q,%q,%d)", stdout, stderr, code)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil || string(data) != "preserved" {
		t.Fatalf("output=%q err=%v", data, err)
	}

	userBase := t.TempDir()
	writeCLIConfig(t, filepath.Join(userBase, "dirloom", "config.yaml"), "schemaVersion: 1\npresentation:\n  theme: missing.yaml\n")
	writeCLIConfig(t, filepath.Join(root, ".dirloom.yaml"), "schemaVersion: 1\npresentation:\n  theme: daylight\n")
	loader := configuration.NewLoader(configuration.WithUserConfigDir(func() (string, error) { return userBase, nil }))
	stdout, stderr, code = executeForTestWithLoader(t, loader, root, "--color", "never", "--icons", "never")
	if code != 0 || stderr != "" || stdout == "" {
		t.Fatalf("masked theme=(%q,%q,%d)", stdout, stderr, code)
	}
}

func TestConfigExplainIncludesPresentationProvenanceWithoutTTYResolution(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, filepath.Join(root, ".dirloom.yaml"), `schemaVersion: 1
presentation:
  color: auto
  icons: nerd
  theme: midnight
`)
	stdout, stderr, code := executeForTest(t, "config", "explain", root)
	if code != 0 || stderr != "" {
		t.Fatalf("text=(%q,%q,%d)", stdout, stderr, code)
	}
	for _, want := range []string{"color: auto (project:", "resolved at output time", "icons: nerd (project:", "theme: midnight (project:", "theme source: built-in"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("text missing %q\n%s", want, stdout)
		}
	}
	stdout, stderr, code = executeForTest(t, "config", "explain", root, "--as", "json", "--format", "markdown")
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"presentation": {`) || !strings.Contains(stdout, `"appearance": "dark"`) || !strings.Contains(stdout, `"inactive": [`) || !strings.Contains(stdout, `"theme"`) {
		t.Fatalf("JSON=(%q,%q,%d)", stdout, stderr, code)
	}
}

func TestInvalidArgumentsReturnExitCodeTwo(t *testing.T) {
	tests := [][]string{
		{"--depth", "invalid"},
		{"--depth", "-1"},
		{"--format", "json", "--style", "ascii"},
		{"--format", "markdown-tree", "--style", "ascii"},
		{"--format", "mermaid", "--style", "ascii"},
		{"--format", "text", "--diagram-direction", "left-right"},
		{"--format", "yaml"},
		{"--style", "auto"},
		{"--ignore", "../outside"},
		{"--preset", "unknown"},
		{"--preset="},
		{"--color", "sometimes"},
		{"--color="},
		{"--icons", "font"},
		{"--icons="},
		{"--theme", "ocean"},
		{"--theme="},
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

func TestCLIInspectionErrorsPreserveOutputAndClassifyWriteFailure(t *testing.T) {
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
	errorOutput.Reset()
	code = executeWithLoader(context.Background(), []string{"theme", "list"}, failingWriter{}, &errorOutput, "v0.1.0-test", loader)
	if code != 1 || !strings.Contains(errorOutput.String(), "write theme list") {
		t.Fatalf("theme write failure=(stderr=%q, code=%d)", errorOutput.String(), code)
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
	themeDocumentation, err := os.ReadFile(filepath.Join("..", "..", "docs", "themes.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "docs/themes.md") {
		t.Fatal("README does not link to docs/themes.md")
	}
	graphDocumentation, err := os.ReadFile(filepath.Join("..", "..", "docs", "graph-exports.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "docs/graph-exports.md") || !strings.Contains(string(graphDocumentation), "dirloom --format mermaid") {
		t.Fatal("README or graphical export documentation is missing")
	}
	stdout, stderr, code := executeForTest(t, "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("help=(%q, %q, %d)", stdout, stderr, code)
	}
	for _, option := range []string{"--config", "--no-user-config", "--no-config", "--preset", "--depth", "--dirs-only", "--hidden", "--format", "--style", "--no-default-ignore", "--no-gitignore", "--diagram-view", "--diagram-direction", "--diagram-max-nodes"} {
		if !strings.Contains(stdout, option) {
			t.Errorf("CLI help is missing documented option %q", option)
		}
		if !strings.Contains(string(documentation), option) {
			t.Errorf("public configuration documentation is missing option %q", option)
		}
	}
	for _, option := range []string{"--color", "--icons", "--theme"} {
		if !strings.Contains(stdout, option) {
			t.Errorf("CLI help is missing documented option %q", option)
		}
		if !strings.Contains(string(themeDocumentation), option) {
			t.Errorf("public theme documentation is missing option %q", option)
		}
	}
	if !strings.Contains(stdout, "preset") || !strings.Contains(string(presetDocumentation), "dirloom preset explain") {
		t.Fatal("preset help or public documentation is missing")
	}
	if !strings.Contains(stdout, "theme") || !strings.Contains(string(themeDocumentation), "dirloom theme validate") {
		t.Fatal("theme help or public documentation is missing")
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

func executeForTestWithDependencies(t *testing.T, loader *configuration.Loader, evaluator *presentation.Evaluator, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := executeWithDependencies(context.Background(), args, &stdout, &stderr, "v0.1.0-test", loader, evaluator)
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
