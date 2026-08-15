package config

import (
	"strings"
	"testing"
)

func TestParseDocumentAcceptsCompleteSchema(t *testing.T) {
	values, err := parseDocument([]byte(`schemaVersion: 1
preset: docs
defaults:
  depth: 0
  dirsOnly: false
  hidden: true
  format: markdown
  style: ascii
filters:
  useDefaultIgnores: false
  useGitignore: true
ignore:
  - generated/**
  - "*.log"
presentation:
  color: always
  icons: nerd
  theme: .dirloom/themes/team.yaml
`), ".dirloom.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !values.Preset.Set || values.Preset.Disabled || values.Preset.Name != "docs" {
		t.Fatalf("preset = %#v", values.Preset)
	}
	if !values.Depth.Set || values.Depth.Unlimited || values.Depth.Value != 0 {
		t.Fatalf("depth = %#v", values.Depth)
	}
	if !values.DirectoriesOnly.Set || values.DirectoriesOnly.Value {
		t.Fatalf("dirsOnly = %#v", values.DirectoriesOnly)
	}
	if !values.IncludeHidden.Set || !values.IncludeHidden.Value {
		t.Fatalf("hidden = %#v", values.IncludeHidden)
	}
	if !values.Format.Set || values.Format.Value != FormatMarkdown || !values.Style.Set || values.Style.Value != StyleASCII {
		t.Fatalf("format/style = %#v / %#v", values.Format, values.Style)
	}
	if !values.UseDefaultIgnores.Set || values.UseDefaultIgnores.Value || !values.UseGitIgnore.Set || !values.UseGitIgnore.Value {
		t.Fatalf("filters = %#v / %#v", values.UseDefaultIgnores, values.UseGitIgnore)
	}
	if strings.Join(values.IgnorePatterns, ",") != "generated/**,*.log" {
		t.Fatalf("ignore = %#v", values.IgnorePatterns)
	}
	if !values.Color.Set || values.Color.Value != "always" || !values.Icons.Set || values.Icons.Value != "nerd" || !values.Theme.Set || values.Theme.Reset || values.Theme.Value != ".dirloom/themes/team.yaml" {
		t.Fatalf("presentation = color:%#v icons:%#v theme:%#v", values.Color, values.Icons, values.Theme)
	}
}

func TestParseDocumentDistinguishesThemeInheritanceAndReset(t *testing.T) {
	absent, err := parseDocument([]byte("schemaVersion: 1\n"), "absent.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if absent.Theme.Set {
		t.Fatalf("absent theme = %#v", absent.Theme)
	}
	reset, err := parseDocument([]byte("schemaVersion: 1\npresentation:\n  theme: null\n"), "reset.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !reset.Theme.Set || !reset.Theme.Reset || reset.Theme.Value != "" {
		t.Fatalf("reset theme = %#v", reset.Theme)
	}
	for _, value := range []string{"default", "midnight", "daylight", "themes/team.yaml", "team.yml"} {
		parsed, parseErr := parseDocument([]byte("schemaVersion: 1\npresentation:\n  theme: "+value+"\n"), "theme.yaml")
		if parseErr != nil || !parsed.Theme.Set || parsed.Theme.Value != value {
			t.Errorf("theme %q = %#v err=%v", value, parsed.Theme, parseErr)
		}
	}
}

func TestParseDocumentDistinguishesPresetInheritanceAndReset(t *testing.T) {
	absent, err := parseDocument([]byte("schemaVersion: 1\n"), "absent.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if absent.Preset.Set {
		t.Fatalf("absent preset = %#v", absent.Preset)
	}

	reset, err := parseDocument([]byte("schemaVersion: 1\npreset: null\n"), "reset.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !reset.Preset.Set || !reset.Preset.Disabled || reset.Preset.Name != "" {
		t.Fatalf("reset preset = %#v", reset.Preset)
	}

	for _, name := range PresetNames() {
		values, err := parseDocument([]byte("schemaVersion: 1\npreset: "+name+"\n"), name+".yaml")
		if err != nil {
			t.Errorf("preset %q: %v", name, err)
			continue
		}
		if !values.Preset.Set || values.Preset.Disabled || values.Preset.Name != name {
			t.Errorf("preset %q parsed as %#v", name, values.Preset)
		}
	}
}

func TestParseDocumentDistinguishesAbsentAndUnlimitedDepth(t *testing.T) {
	absent, err := parseDocument([]byte("schemaVersion: 1\n"), "absent.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if absent.Depth.Set {
		t.Fatalf("absent depth = %#v", absent.Depth)
	}

	unlimited, err := parseDocument([]byte("schemaVersion: 1\ndefaults:\n  depth: null\n"), "unlimited.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !unlimited.Depth.Set || !unlimited.Depth.Unlimited {
		t.Fatalf("unlimited depth = %#v", unlimited.Depth)
	}
}

func TestParseDocumentRejectsInvalidYAMLContracts(t *testing.T) {
	tests := []struct {
		name     string
		document string
		want     string
	}{
		{"empty", "", "file is empty"},
		{"missing-version", "defaults: {}\n", "schemaVersion is required"},
		{"unsupported-version", "schemaVersion: 2\n", "unsupported schemaVersion 2"},
		{"empty-preset", "schemaVersion: 1\npreset: \"\"\n", "line 2, column 9: unsupported preset"},
		{"unknown-preset", "schemaVersion: 1\npreset: unknown\n", "line 2, column 9: unsupported preset"},
		{"uppercase-preset", "schemaVersion: 1\npreset: AI\n", "line 2, column 9: unsupported preset"},
		{"boolean-preset", "schemaVersion: 1\npreset: true\n", "line 2, column 9: preset must be"},
		{"sequence-preset", "schemaVersion: 1\npreset: [docs]\n", "line 2, column 9: preset must be"},
		{"mapping-preset", "schemaVersion: 1\npreset: {name: docs}\n", "line 2, column 9: preset must be"},
		{"unknown-field", "schemaVersion: 1\nunknown: true\n", "field unknown not found"},
		{"unknown-nested-field", "schemaVersion: 1\ndefaults:\n  unknown: true\n", "field unknown not found"},
		{"duplicate-key", "schemaVersion: 1\ndefaults:\n  hidden: true\n  hidden: false\n", "duplicate key \"hidden\""},
		{"multiple-documents", "schemaVersion: 1\n---\nschemaVersion: 1\n", "multiple YAML documents"},
		{"anchor", "schemaVersion: 1\ndefaults: &settings\n  hidden: true\n", "anchors and aliases"},
		{"alias", "schemaVersion: 1\ndefaults: &settings\n  hidden: true\nfilters: *settings\n", "anchors and aliases"},
		{"merge", "schemaVersion: 1\ndefaults:\n  <<: {hidden: true}\n", "merge keys"},
		{"custom-tag", "schemaVersion: 1\ndefaults:\n  format: !env FORMAT\n", "custom YAML tag"},
		{"negative-depth", "schemaVersion: 1\ndefaults:\n  depth: -1\n", "line 3, column 10: defaults.depth"},
		{"string-depth", "schemaVersion: 1\ndefaults:\n  depth: unlimited\n", "line 3, column 10: defaults.depth"},
		{"bad-format", "schemaVersion: 1\ndefaults:\n  format: yaml\n", "line 3, column 11: unsupported defaults.format"},
		{"bad-style", "schemaVersion: 1\ndefaults:\n  style: auto\n", "line 3, column 10: unsupported defaults.style"},
		{"null-ignore", "schemaVersion: 1\nignore: null\n", "ignore must be a sequence"},
		{"non-string-ignore", "schemaVersion: 1\nignore:\n  - 42\n", "ignore entries must be strings"},
		{"invalid-ignore", "schemaVersion: 1\nignore:\n  - ../outside\n", "line 3, column 5: ignore pattern"},
		{"bad-color", "schemaVersion: 1\npresentation:\n  color: \"yes\"\n", "unsupported presentation.color"},
		{"bad-icons", "schemaVersion: 1\npresentation:\n  icons: font\n", "unsupported presentation.icons"},
		{"empty-theme", "schemaVersion: 1\npresentation:\n  theme: \"\"\n", "presentation.theme must be a non-empty"},
		{"unknown-theme", "schemaVersion: 1\npresentation:\n  theme: ocean\n", "unsupported theme"},
		{"escaping-theme", "schemaVersion: 1\npresentation:\n  theme: ../outside.yaml\n", "must remain inside"},
		{"theme-number", "schemaVersion: 1\npresentation:\n  theme: 42\n", "presentation.theme must be"},
		{"unknown-presentation-field", "schemaVersion: 1\npresentation:\n  future: true\n", "field future not found"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseDocument([]byte(test.document), "config.yaml")
			if err == nil || !IsInvalid(err) || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want invalid containing %q", err, test.want)
			}
		})
	}
}
