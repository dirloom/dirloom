package config

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/filter"
)

func TestPresetCatalogContract(t *testing.T) {
	wantNames := []string{"ai", "compact", "docs", "monorepo"}
	if got := PresetNames(); !reflect.DeepEqual(got, wantNames) {
		t.Fatalf("PresetNames() = %#v, want %#v", got, wantNames)
	}

	tests := map[string]struct {
		depth          int
		directories    bool
		format         string
		ignorePatterns []string
	}{
		"docs":     {depth: 4, format: FormatMarkdown, ignorePatterns: []string{}},
		"compact":  {depth: 3, directories: true, format: FormatText, ignorePatterns: []string{}},
		"monorepo": {depth: 4, directories: true, format: FormatText, ignorePatterns: []string{"**/dist", "**/build"}},
		"ai":       {depth: 4, format: FormatMarkdown, ignorePatterns: []string{"**/dist", "**/build", "*.map"}},
	}
	for name, want := range tests {
		definition, ok := LookupPreset(name)
		if !ok {
			t.Fatalf("LookupPreset(%q) was not found", name)
		}
		if definition.SchemaVersion != 1 || definition.Name != name || definition.Description == "" {
			t.Errorf("%s identity = %#v", name, definition)
		}
		if definition.Defaults.Depth != want.depth || definition.Defaults.DirectoriesOnly != want.directories || definition.Defaults.IncludeHidden || definition.Defaults.Format != want.format || definition.Defaults.Style != StyleUnicode {
			t.Errorf("%s defaults = %#v", name, definition.Defaults)
		}
		if !definition.Filters.UseDefaultIgnores || !definition.Filters.UseGitIgnore {
			t.Errorf("%s filters = %#v", name, definition.Filters)
		}
		if !reflect.DeepEqual(definition.Ignore, want.ignorePatterns) {
			t.Errorf("%s ignore = %#v, want %#v", name, definition.Ignore, want.ignorePatterns)
		}
		if _, err := filter.NewIgnoreMatcher(definition.Ignore); err != nil {
			t.Errorf("%s contains an invalid ignore pattern: %v", name, err)
		}
		seen := map[string]struct{}{}
		for _, pattern := range definition.Ignore {
			if _, duplicate := seen[pattern]; duplicate {
				t.Errorf("%s contains duplicate ignore pattern %q", name, pattern)
			}
			seen[pattern] = struct{}{}
		}
	}
	if _, ok := LookupPreset("none"); ok {
		t.Fatal("none must remain a CLI control value, not a preset")
	}
	if _, ok := LookupPreset("AI"); ok {
		t.Fatal("preset names must be case-sensitive")
	}
}

func TestLookupPresetReturnsDefensiveCopies(t *testing.T) {
	definition, ok := LookupPreset("ai")
	if !ok {
		t.Fatal("ai preset not found")
	}
	definition.Ignore[0] = "mutated"
	names := PresetNames()
	names[0] = "mutated"

	again, ok := LookupPreset("ai")
	if !ok || again.Ignore[0] != "**/dist" {
		t.Fatalf("catalog was mutated: %#v", again)
	}
	if got := PresetNames()[0]; got != "ai" {
		t.Fatalf("preset names were mutated: %q", got)
	}
}

func TestPresetDiagnostics(t *testing.T) {
	definition, ok := LookupPreset("ai")
	if !ok {
		t.Fatal("ai preset not found")
	}
	var textOutput bytes.Buffer
	if err := definition.WriteText(&textOutput); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Preset: ai",
		"Purpose: Produce a concise Markdown structure for AI-assisted workflows.",
		"depth: 4",
		"dirsOnly: false",
		"format: markdown",
		"**/dist",
		"*.map",
	} {
		if !strings.Contains(textOutput.String(), want) {
			t.Errorf("text diagnostic missing %q\n%s", want, textOutput.String())
		}
	}
	if !strings.HasSuffix(textOutput.String(), "\n") {
		t.Fatalf("text diagnostic has no final LF: %q", textOutput.String())
	}

	compact, _ := LookupPreset("compact")
	var jsonOutput bytes.Buffer
	if err := compact.WriteJSON(&jsonOutput); err != nil {
		t.Fatal(err)
	}
	var document struct {
		SchemaVersion int      `json:"schemaVersion"`
		Name          string   `json:"name"`
		Ignore        []string `json:"ignore"`
	}
	if err := json.Unmarshal(jsonOutput.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if document.SchemaVersion != 1 || document.Name != "compact" || document.Ignore == nil || len(document.Ignore) != 0 {
		t.Fatalf("JSON diagnostic = %#v\n%s", document, jsonOutput.String())
	}
}
