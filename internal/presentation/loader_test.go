package presentation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/tree"
)

const validCustomTheme = `schemaVersion: 1
name: team
description: Team terminal theme
appearance: dark
palette:
  accent2: "#7DCFFF"
tokens:
  tree.edge:
    color: accent2
    styles: [dim]
  node.file:
    color: file
rules:
  - match:
      name: README.md
    color: accent2
    styles: [bold]
    icons:
      unicode: "¶"
      nerd: "󰍔"
  - match:
      extension: .go
    icons:
      nerd: "󰟓"
icons:
  spacing: 2
`

func TestLoadCustomThemeAndRuleFallback(t *testing.T) {
	path := writeTheme(t, "team theme.yaml", validCustomTheme)
	theme, err := LoadReference(path, ReferenceContext{Kind: "cli"})
	if err != nil {
		t.Fatal(err)
	}
	if theme.Name != "team" || theme.Source.Kind != "file" || theme.Source.Path != path || theme.Icons.Spacing != 2 || len(theme.Warnings) != 0 {
		t.Fatalf("theme = %#v", theme)
	}
	compiled, err := Compile(theme)
	if err != nil {
		t.Fatal(err)
	}
	readme := compiled.resolve("README.md", "README.md", tree.NodeFile)
	if readme.icons.Unicode != "¶" || readme.icons.Nerd != "󰍔" || !reflectStyles(readme.styles, []string{"bold"}) {
		t.Fatalf("README style = %#v", readme)
	}
	goFile := compiled.resolve("src/main.go", "main.go", tree.NodeFile)
	if goFile.icons.Nerd != "󰟓" || goFile.icons.Unicode != "•" {
		t.Fatalf("Go fallback = %#v", goFile.icons)
	}
}

func TestThemeParserRejectsUnsafeAndAmbiguousDocuments(t *testing.T) {
	tests := map[string]string{
		"empty":           "",
		"version":         "schemaVersion: 2\nname: x\nappearance: dark\n",
		"unknown-field":   "schemaVersion: 1\nname: x\nappearance: dark\nfuture: true\n",
		"duplicate-key":   "schemaVersion: 1\nname: x\nname: y\nappearance: dark\n",
		"anchor":          "schemaVersion: 1\nname: x\nappearance: dark\npalette: &p {x: default}\n",
		"multiple":        "schemaVersion: 1\nname: x\nappearance: dark\n---\nschemaVersion: 1\nname: y\nappearance: light\n",
		"bad-color":       "schemaVersion: 1\nname: x\nappearance: dark\npalette: {x: red}\n",
		"bad-style":       "schemaVersion: 1\nname: x\nappearance: dark\ntokens: {node.file: {styles: [blink]}}\n",
		"bad-icon":        "schemaVersion: 1\nname: x\nappearance: dark\ntokens: {node.file: {icons: {unicode: \"\\u001b[31m\"}}}\n",
		"two-matchers":    "schemaVersion: 1\nname: x\nappearance: dark\nrules: [{match: {name: a, extension: .go}}]\n",
		"duplicate-match": "schemaVersion: 1\nname: x\nappearance: dark\nrules: [{match: {name: a}}, {match: {name: a}}]\n",
		"bad-spacing":     "schemaVersion: 1\nname: x\nappearance: dark\nicons: {spacing: 5}\n",
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			path := writeTheme(t, name+".yaml", content)
			if _, err := LoadReference(path, ReferenceContext{Kind: "cli"}); err == nil || !IsInvalid(err) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestUnknownTokenWarnsAndIsIgnored(t *testing.T) {
	path := writeTheme(t, "future.yaml", "schemaVersion: 1\nname: future\nappearance: universal\ntokens:\n  node.future:\n    color: default\n")
	theme, err := LoadReference(path, ReferenceContext{Kind: "cli"})
	if err != nil {
		t.Fatal(err)
	}
	if len(theme.Warnings) != 1 || theme.Warnings[0].Code != "unknown-token" {
		t.Fatalf("warnings = %#v", theme.Warnings)
	}
	if _, exists := theme.Tokens["node.future"]; exists {
		t.Fatal("future token was activated")
	}
}

func TestRulePrecedenceAndFirstDeclaration(t *testing.T) {
	path := writeTheme(t, "rules.yaml", `schemaVersion: 1
name: rules
appearance: universal
rules:
  - match: {type: file}
    icons: {unicode: "T"}
  - match: {extension: .go}
    icons: {unicode: "E"}
  - match: {glob: "src/**"}
    icons: {unicode: "G"}
  - match: {name: main.go}
    icons: {unicode: "N"}
  - match: {path: src/main.go}
    icons: {unicode: "P"}
  - match: {extension: .txt}
    icons: {unicode: "1"}
  - match: {extension: .md}
    icons: {unicode: "2"}
`)
	theme, err := LoadReference(path, ReferenceContext{Kind: "cli"})
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := Compile(theme)
	if err != nil {
		t.Fatal(err)
	}
	if got := compiled.resolve("src/main.go", "main.go", tree.NodeFile).icons.Unicode; got != "P" {
		t.Fatalf("path = %q", got)
	}
	if got := compiled.resolve("other/main.go", "main.go", tree.NodeFile).icons.Unicode; got != "N" {
		t.Fatalf("name = %q", got)
	}
	if got := compiled.resolve("src/other.go", "other.go", tree.NodeFile).icons.Unicode; got != "G" {
		t.Fatalf("glob = %q", got)
	}
	if got := compiled.resolve("other.go", "other.go", tree.NodeFile).icons.Unicode; got != "E" {
		t.Fatalf("extension = %q", got)
	}
	if got := compiled.resolve("plain.bin", "plain.bin", tree.NodeFile).icons.Unicode; got != "T" {
		t.Fatalf("type = %q", got)
	}
}

func TestConfigurationThemePathIsConfined(t *testing.T) {
	configDirectory := t.TempDir()
	inside := filepath.Join(configDirectory, "themes", "team.yaml")
	if err := os.MkdirAll(filepath.Dir(inside), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inside, []byte(validCustomTheme), 0o644); err != nil {
		t.Fatal(err)
	}
	context := ReferenceContext{Kind: "project", ConfigPath: filepath.Join(configDirectory, ".dirloom.yaml")}
	if _, err := LoadReference(filepath.Join("themes", "team.yaml"), context); err != nil {
		t.Fatal(err)
	}

	outside := writeTheme(t, "outside.yaml", validCustomTheme)
	relative, err := filepath.Rel(configDirectory, outside)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LoadReference(relative, context); err == nil || !IsInvalid(err) || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("outside error = %v", err)
	}
	if _, err := LoadReference(outside, context); err == nil || !IsInvalid(err) {
		t.Fatalf("absolute config path error = %v", err)
	}
	symlink := filepath.Join(configDirectory, "themes", "outside-link.yaml")
	if err := os.Symlink(outside, symlink); err != nil {
		t.Logf("symlink confinement case skipped: %v", err)
	} else if _, err := LoadReference(filepath.Join("themes", "outside-link.yaml"), context); err == nil || !IsInvalid(err) || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("symlink escape error = %v", err)
	}
}

func TestThemeLimits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.yaml")
	if err := os.WriteFile(path, make([]byte, maxThemeSize+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadReference(path, ReferenceContext{Kind: "cli"}); err == nil || !IsInvalid(err) {
		t.Fatalf("large error = %v", err)
	}
}

func writeTheme(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func reflectStyles(got, want []string) bool {
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
