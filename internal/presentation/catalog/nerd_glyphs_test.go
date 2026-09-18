package catalog

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

func TestEveryKindHasGovernedNerdProjection(t *testing.T) {
	if err := Validate(); err != nil {
		t.Fatal(err)
	}
	if got := len(nerdGlyphByKind); got != KindCount {
		t.Fatalf("nerd projections = %d, want %d", got, KindCount)
	}
	for _, definition := range Kinds() {
		nerd, ok := nerdGlyphByKind[definition.Kind]
		if !ok {
			t.Errorf("kind %s missing nerd provenance", definition.Kind)
			continue
		}
		glyphs := Glyphs(definition.Kind)
		if glyphs.Nerd != nerd.Glyph || definition.Nerd != nerd.Glyph {
			t.Errorf("kind %s nerd = %q compiled=%q provenance=%q", definition.Kind, glyphs.Nerd, definition.Nerd, nerd.Glyph)
		}
		if err := validateCatalogGlyph(glyphs.Nerd); err != nil {
			t.Errorf("kind %s nerd glyph: %v", definition.Kind, err)
		}
		for _, char := range glyphs.Nerd {
			if unicode.IsControl(char) || char == '\x1b' {
				t.Errorf("kind %s nerd contains control U+%04X", definition.Kind, char)
			}
		}
		switch nerd.Class {
		case nerdExactTechnology, nerdExactSemantic, nerdGenericSemantic, nerdGenericFallback:
		default:
			t.Errorf("kind %s has invalid nerd class %q", definition.Kind, nerd.Class)
		}
	}
}

func TestNerdOverridesHaveCompleteProvenance(t *testing.T) {
	for kind, definition := range nerdGlyphByKind {
		if err := validateNerdGlyphDefinition(kind, definition); err != nil {
			t.Error(err)
		}
		if utf8.RuneCountInString(definition.Glyph) != 1 {
			t.Errorf("kind %s nerd glyph is not a single rune: %q", kind, definition.Glyph)
		}
	}
}

func TestNerdGlyphsMatchPinnedContractFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/nerd-fonts-v3.5.1-contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Version    string `json:"nerd_fonts_version"`
		Repository string `json:"source_repository"`
		Tag        string `json:"source_tag"`
		Source     string `json:"glyphnames_source"`
		Retrieved  string `json:"retrieval_date"`
		Glyphs     map[string]struct {
			Key  string `json:"glyphnames_key"`
			Char string `json:"char"`
			Code string `json:"code"`
		} `json:"glyphs"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != nerdFontsVersion || fixture.Tag != nerdFontsTag || fixture.Repository != nerdFontsRepository || fixture.Source != nerdFontsGlyphnamesFile || fixture.Retrieved != nerdFontsRetrievalDate {
		t.Fatalf("pinned nerd fonts metadata = %#v", fixture)
	}
	used := map[string]struct{}{}
	for kind, definition := range nerdGlyphByKind {
		entry, ok := fixture.Glyphs[definition.OfficialName]
		if !ok {
			t.Errorf("kind %s official name %s missing from pinned fixture", kind, definition.OfficialName)
			continue
		}
		used[definition.OfficialName] = struct{}{}
		if entry.Char != definition.Glyph {
			t.Errorf("kind %s glyph %q != fixture %q", kind, definition.Glyph, entry.Char)
		}
		wantCode := strings.ToUpper(entry.Code)
		gotCode := strings.TrimPrefix(definition.Codepoint, "U+")
		if gotCode != wantCode {
			t.Errorf("kind %s codepoint %s != fixture U+%s", kind, definition.Codepoint, wantCode)
		}
	}
	if len(used) != len(fixture.Glyphs) {
		t.Fatalf("pinned fixture has %d glyphs, catalog uses %d", len(fixture.Glyphs), len(used))
	}
}

func TestNerdCatalogDoesNotUseKnownMisleadingMappings(t *testing.T) {
	forbiddenNames := map[Kind][]string{
		"source.svelte":      {"nf-md-web", "nf-md-hexagon"},
		"source.astro":       {"nf-md-infinity"},
		"source.dart":        {"nf-md-application"},
		"source.ocaml":       {"nf-md-function"},
		"source.nim":         {"nf-md-hexagon", "nf-md-pine-tree"},
		"source.d":           {"nf-md-cube"},
		"source.gleam":       {"nf-md-sigma"},
		"source.elm":         {"nf-md-pine-tree"},
		"source.v":           {"nf-md-code-greater-than"},
		"source.crystal":     {"nf-md-diamond-stone", "nf-md-diamond"},
		"source.cue":         {"nf-md-code-array"},
		"source.fsharp":      {"nf-md-lambda"},
		"manifest.helm":      {"nf-md-kubernetes", "nf-md-ship-wheel"},
		"media.image.jpeg":   {"nf-md-folder-image"},
		"media.image.svg":    {"nf-md-folder-image"},
		"media":              {"nf-md-folder-image"},
		"media.model":        {"nf-md-folder-image"},
		"document.license":   {"nf-md-file-document"},
		"document.changelog": {"nf-md-file-document"},
	}
	for kind, names := range forbiddenNames {
		definition, ok := nerdGlyphByKind[kind]
		if !ok {
			t.Errorf("kind %s missing from nerd catalog", kind)
			continue
		}
		for _, name := range names {
			if definition.OfficialName == name {
				t.Errorf("kind %s reintroduced misleading glyph %s", kind, name)
			}
		}
	}
	for kind, definition := range nerdGlyphByKind {
		if definition.OfficialName == "nf-md-folder-image" && kind != "directory" && !strings.HasPrefix(string(kind), "directory.") {
			t.Errorf("file kind %s uses a folder glyph", kind)
		}
	}
}

func TestRequiredNerdFidelityMappings(t *testing.T) {
	tests := map[Kind]string{
		"media.image.jpeg":   "nf-md-file-jpg-box",
		"media.image.svg":    "nf-md-svg",
		"document.license":   "nf-md-license",
		"document.changelog": "nf-md-history",
		"source.svelte":      "nf-dev-svelte",
		"source.astro":       "nf-dev-astro",
		"source.dart":        "nf-dev-dart",
		"manifest.dart":      "nf-dev-dart",
		"manifest.helm":      "nf-dev-helm",
		"source.ocaml":       "nf-dev-ocaml",
		"source.nim":         "nf-dev-nim",
		"source.d":           "nf-dev-dlang",
		"source.gleam":       "nf-dev-gleam",
		"source.elm":         "nf-dev-elm",
		"source.v":           "nf-md-code-braces",
		"source.crystal":     "nf-dev-crystal",
		"source.cue":         "nf-md-code-braces",
		"source.go":          "nf-md-language-go",
		"manifest.go":        "nf-md-language-go",
		"source.rust":        "nf-md-language-rust",
		"manifest.rust":      "nf-md-language-rust",
		"source.python":      "nf-md-language-python",
		"manifest.python":    "nf-md-language-python",
		"source.nix":         "nf-md-nix",
		"manifest.nix":       "nf-md-nix",
		"manifest.terraform": "nf-md-terraform",
	}
	for kind, want := range tests {
		got := nerdGlyphByKind[kind].OfficialName
		if got != want {
			t.Errorf("kind %s official name = %s, want %s", kind, got, want)
		}
	}
}

func TestNerdFallbackStaysDeterministic(t *testing.T) {
	original := kindRegistry["source.zig"]
	without := original
	without.Nerd = ""
	kindRegistry["source.zig"] = without
	glyphs := Glyphs("source.zig")
	kindRegistry["source.zig"] = original
	if glyphs.Nerd != nerdGlyphByKind["source"].Glyph {
		t.Fatalf("missing nerd must fall back to the source family glyph, got %#v", glyphs)
	}
}
