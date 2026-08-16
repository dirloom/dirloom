package presentation

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/presentation/catalog"
	"github.com/dirloom/dirloom/internal/tree"
)

func TestClassificationDiagnosticContractsAndPathRedaction(t *testing.T) {
	theme, _ := Lookup(ThemeVivid)
	theme.Name = "team"
	theme.Source = Source{Kind: "file", Path: "/private/themes/team.yaml"}
	compiled, err := Compile(theme)
	if err != nil {
		t.Fatal(err)
	}
	document := NewClassifyDocument("src/main.go", "main.go", tree.NodeFile, theme, compiled)
	if document.SchemaVersion != ThemeClassifySchemaVersion || document.Theme.Source.Path != "" || document.Style.Styles == nil || document.Classification.Kind != "source.go" || document.VisualRole != catalog.RoleSource {
		t.Fatalf("document = %#v", document)
	}
	var textOutput bytes.Buffer
	if err := document.WriteText(&textOutput); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Path: src/main.go", "Kind: source.go", "Theme: team (file)", "nerd=\"󰟓\""} {
		if !strings.Contains(textOutput.String(), want) {
			t.Errorf("text missing %q\n%s", want, textOutput.String())
		}
	}
	var jsonOutput bytes.Buffer
	if err := document.WriteJSON(&jsonOutput); err != nil {
		t.Fatal(err)
	}
	var decoded ClassifyDocument
	if err := json.Unmarshal(jsonOutput.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SchemaVersion != 1 || decoded.Style.Styles == nil || decoded.Classification.Roles == nil {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestClassificationDiagnosticNormalizesNilArraysAndEscapesGlyphQuotes(t *testing.T) {
	document := ClassifyDocument{
		SchemaVersion:  ThemeClassifySchemaVersion,
		Classification: catalog.Classification{Kind: "file"},
		Style:          ClassifyStyle{Icons: IconPair{Unicode: `"`, Nerd: `\`}},
	}
	var output bytes.Buffer
	if err := document.WriteJSON(&output); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), `"roles": null`) || strings.Contains(output.String(), `"styles": null`) {
		t.Fatalf("null arrays = %s", output.String())
	}
	output.Reset()
	if err := document.WriteText(&output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `unicode="\""`) || !strings.Contains(output.String(), `nerd="\\"`) {
		t.Fatalf("quoted glyphs = %q", output.String())
	}
	if err := document.WriteText(errorWriter{}); err == nil {
		t.Fatal("text write failure was ignored")
	}
	if err := document.WriteJSON(errorWriter{}); err == nil {
		t.Fatal("JSON write failure was ignored")
	}
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) { return 0, errors.New("synthetic write failure") }
