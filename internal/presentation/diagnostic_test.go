package presentation

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestThemeDiagnosticsAreStableAndNonNull(t *testing.T) {
	var textOutput bytes.Buffer
	if err := WriteListText(&textOutput); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(textOutput.String(), "Built-in themes:\n  daylight:") || !strings.Contains(textOutput.String(), "\n  default:") || !strings.HasSuffix(textOutput.String(), "\n") {
		t.Fatalf("list text = %q", textOutput.String())
	}
	var jsonOutput bytes.Buffer
	if err := WriteListJSON(&jsonOutput); err != nil {
		t.Fatal(err)
	}
	var list ListDocument
	if err := json.Unmarshal(jsonOutput.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if list.SchemaVersion != 1 || len(list.Themes) != 4 || list.Themes[0].Name != "daylight" || list.Themes[1].Name != "default" || list.Themes[2].Name != "midnight" || list.Themes[3].Name != "vivid" {
		t.Fatalf("list = %#v", list)
	}
	if list.Themes[0].Rules == nil || list.Themes[0].Warnings == nil {
		t.Fatal("list contains null arrays")
	}

	theme, _ := Lookup("midnight")
	textOutput.Reset()
	if err := theme.WriteText(&textOutput); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Theme: midnight", "Appearance: dark", "Source: built-in", "node.directory", "Rules:", "Icon spacing: 1"} {
		if !strings.Contains(textOutput.String(), want) {
			t.Errorf("theme text missing %q\n%s", want, textOutput.String())
		}
	}
	jsonOutput.Reset()
	if err := theme.WriteJSON(&jsonOutput); err != nil {
		t.Fatal(err)
	}
	var decoded ExplainDocument
	if err := json.Unmarshal(jsonOutput.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SchemaVersion != 1 || decoded.ThemeSchemaVersion != 1 || decoded.Catalog.EntryCount != 256 || decoded.Theme.Name != "midnight" || decoded.Theme.Rules == nil || decoded.Theme.Warnings == nil {
		t.Fatalf("theme JSON = %#v", decoded)
	}
}

func TestValidationDiagnosticsPreserveWarnings(t *testing.T) {
	theme, _ := Lookup("default")
	theme.Warnings = []Warning{{Code: "unknown-token", Message: "future token ignored"}}
	result := Validation(theme)
	if !result.Valid || result.SchemaVersion != 1 || len(result.Warnings) != 1 {
		t.Fatalf("result = %#v", result)
	}
	var textOutput bytes.Buffer
	if err := result.WriteText(&textOutput); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(textOutput.String(), "Valid theme: default") || !strings.Contains(textOutput.String(), "[unknown-token]") {
		t.Fatalf("text = %q", textOutput.String())
	}
	var jsonOutput bytes.Buffer
	if err := result.WriteJSON(&jsonOutput); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(jsonOutput.String(), `"warnings": [`) || strings.Contains(jsonOutput.String(), `"warnings": null`) {
		t.Fatalf("JSON = %s", jsonOutput.String())
	}

	noWarnings := Validation(theme)
	noWarnings.Warnings = nil
	textOutput.Reset()
	if err := noWarnings.WriteText(&textOutput); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(textOutput.String(), "Warnings: none") {
		t.Fatalf("no-warning text = %q", textOutput.String())
	}
}

func TestPublicModeCatalogs(t *testing.T) {
	if got, want := ColorModes(), []string{"never", "always", "auto"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("colors = %#v", got)
	}
	if got, want := IconModes(), []string{"never", "unicode", "nerd", "auto"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("icons = %#v", got)
	}
	if !IsBuiltIn("default") || IsBuiltIn("ocean") {
		t.Fatal("built-in lookup contract changed")
	}
}
