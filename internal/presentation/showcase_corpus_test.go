package presentation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/dirloom/dirloom/internal/tree"
)

const showcaseGoldenSchemaVersion = 1

type showcaseGoldenDocument struct {
	SchemaVersion int                  `json:"schemaVersion"`
	Cases         []showcaseGoldenCase `json:"cases"`
}

type showcaseGoldenCase struct {
	Path   string                     `json:"path"`
	Name   string                     `json:"name"`
	Type   tree.NodeType              `json:"type"`
	Themes map[string]StyleInspection `json:"themes"`
}

func TestShowcaseClassificationsAndStyleInspectionsMatchGolden(t *testing.T) {
	actual := buildShowcaseGolden(t)
	goldenPath := filepath.Join("testdata", "showcase-styles-v1.json")
	if os.Getenv("DIRLOOM_WRITE_SHOWCASE_GOLDEN") == "1" {
		data, err := json.MarshalIndent(actual, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, '\n')
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	data, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	var expected showcaseGoldenDocument
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatal(err)
	}
	if expected.SchemaVersion != showcaseGoldenSchemaVersion {
		t.Fatalf("showcase golden schema = %d, want %d", expected.SchemaVersion, showcaseGoldenSchemaVersion)
	}
	if len(actual.Cases) != len(expected.Cases) {
		t.Fatalf("showcase cases = %d, want %d", len(actual.Cases), len(expected.Cases))
	}
	for index := range actual.Cases {
		if !reflect.DeepEqual(actual.Cases[index], expected.Cases[index]) {
			t.Errorf("showcase case %d (%s) changed\nactual: %#v\nexpected: %#v", index, actual.Cases[index].Path, actual.Cases[index], expected.Cases[index])
		}
	}
}

func buildShowcaseGolden(t *testing.T) showcaseGoldenDocument {
	t.Helper()
	compiledThemes := make(map[string]*CompiledTheme, len(ThemeNames()))
	for _, themeName := range ThemeNames() {
		theme, ok := Lookup(themeName)
		if !ok {
			t.Fatalf("missing built-in theme %s", themeName)
		}
		compiled, err := Compile(theme)
		if err != nil {
			t.Fatalf("compile %s: %v", themeName, err)
		}
		compiledThemes[themeName] = compiled
	}

	root := filepath.Join("..", "..", "testdata", "showcase")
	scenarios, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	document := showcaseGoldenDocument{SchemaVersion: showcaseGoldenSchemaVersion, Cases: []showcaseGoldenCase{}}
	for _, scenario := range scenarios {
		if !scenario.IsDir() {
			continue
		}
		entries, err := os.ReadDir(filepath.Join(root, scenario.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			nodeType := tree.NodeFile
			if entry.IsDir() {
				nodeType = tree.NodeDirectory
			} else if entry.Type()&os.ModeSymlink != 0 {
				nodeType = tree.NodeSymlink
			}
			pathValue := filepath.ToSlash(filepath.Join(scenario.Name(), entry.Name()))
			goldenCase := showcaseGoldenCase{
				Path: pathValue, Name: entry.Name(), Type: nodeType,
				Themes: make(map[string]StyleInspection, len(compiledThemes)),
			}
			for _, themeName := range ThemeNames() {
				goldenCase.Themes[themeName] = compiledThemes[themeName].Inspect(pathValue, entry.Name(), nodeType)
			}
			document.Cases = append(document.Cases, goldenCase)
		}
	}
	if len(document.Cases) == 0 {
		t.Fatal("showcase corpus is empty")
	}
	return document
}
