package config

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestResolutionDiagnostics(t *testing.T) {
	resolved := defaultResolution("/workspace")
	resolved.Sources = []Source{
		{Kind: SourceUser, Path: "/user/config.yaml", Status: StatusMissing},
		{Kind: SourceProject, Path: "/workspace/.dirloom.yaml", Status: StatusLoaded},
	}
	if err := applyPartial(&resolved, partial{
		Format:         Optional[string]{Set: true, Value: FormatJSON},
		IgnorePatterns: []string{"generated/**"},
	}, Origin{Source: SourceProject, Path: "/workspace/.dirloom.yaml"}); err != nil {
		t.Fatal(err)
	}

	var textOutput bytes.Buffer
	if err := resolved.WriteText(&textOutput); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Root: /workspace",
		"user: missing (/user/config.yaml)",
		"Preset: none (built-in)",
		"format: json (project: /workspace/.dirloom.yaml)",
		"style: unicode (built-in; inactive for json)",
		"generated/** (project: /workspace/.dirloom.yaml)",
	} {
		if !strings.Contains(textOutput.String(), want) {
			t.Errorf("text diagnostic missing %q\n%s", want, textOutput.String())
		}
	}
	if !strings.HasSuffix(textOutput.String(), "\n") {
		t.Fatalf("text diagnostic has no final LF: %q", textOutput.String())
	}

	var jsonOutput bytes.Buffer
	if err := resolved.WriteJSON(&jsonOutput); err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(jsonOutput.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if document["schemaVersion"] != float64(1) || document["root"] != "/workspace" {
		t.Fatalf("JSON diagnostic = %#v", document)
	}
	preset, ok := document["preset"].(map[string]any)
	if !ok || preset["name"] != nil {
		t.Fatalf("preset = %#v", document["preset"])
	}
	inactive, ok := document["inactive"].([]any)
	if !ok || len(inactive) != 1 || inactive[0] != "style" {
		t.Fatalf("inactive = %#v", document["inactive"])
	}
}

func TestResolutionDiagnosticsExposePresetProvenance(t *testing.T) {
	resolved := defaultResolution("/workspace")
	path := "/workspace/.dirloom.yaml"
	name := "ai"
	resolved.Preset = ResolvedPreset{Name: &name, Origin: Origin{Source: SourceProject, Path: path}}
	if err := applyNamedPreset(&resolved, name, Origin{Source: SourceProject, Path: path}); err != nil {
		t.Fatal(err)
	}
	if err := applyPartial(&resolved, partial{Depth: DepthOverride{Set: true, Value: 6}}, Origin{Source: SourceProject, Path: path}); err != nil {
		t.Fatal(err)
	}

	var textOutput bytes.Buffer
	if err := resolved.WriteText(&textOutput); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Preset: ai (project: /workspace/.dirloom.yaml)",
		"format: markdown (project preset ai: /workspace/.dirloom.yaml)",
		"depth: 6 (project: /workspace/.dirloom.yaml)",
		"**/build (project preset ai: /workspace/.dirloom.yaml)",
	} {
		if !strings.Contains(textOutput.String(), want) {
			t.Errorf("text diagnostic missing %q\n%s", want, textOutput.String())
		}
	}

	var jsonOutput bytes.Buffer
	if err := resolved.WriteJSON(&jsonOutput); err != nil {
		t.Fatal(err)
	}
	var document struct {
		Preset     ResolvedPreset    `json:"preset"`
		Provenance map[string]Origin `json:"provenance"`
	}
	if err := json.Unmarshal(jsonOutput.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if document.Preset.Name == nil || *document.Preset.Name != "ai" || document.Preset.Origin.Source != SourceProject {
		t.Fatalf("preset = %#v", document.Preset)
	}
	if document.Provenance["format"].Preset != "ai" || document.Provenance["depth"].Preset != "" {
		t.Fatalf("provenance = %#v", document.Provenance)
	}
}
