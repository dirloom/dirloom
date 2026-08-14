package config

import (
	"strings"
	"testing"
)

func TestParseDocumentAcceptsCompleteSchema(t *testing.T) {
	values, err := parseDocument([]byte(`schemaVersion: 1
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
`), ".dirloom.yaml")
	if err != nil {
		t.Fatal(err)
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
