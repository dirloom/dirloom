package presentation

import (
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/presentation/catalog"
	"github.com/dirloom/dirloom/internal/render"
	"github.com/dirloom/dirloom/internal/tree"
)

func TestASCIIThemeChannelAndLegacyThemesRemainCompatible(t *testing.T) {
	legacy, err := parseTheme([]byte(`schemaVersion: 1
catalogVersion: 1
name: legacy
appearance: dark
kinds:
  source.go:
    icons:
      unicode: "G"
      nerd: "N"
`), "legacy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := Compile(legacy)
	if err != nil {
		t.Fatal(err)
	}
	got := compiled.Inspect("main.go", "main.go", tree.NodeFile)
	if got.Icons.ASCII != "[SC]" || got.Icons.Unicode != "G" || got.Icons.Nerd != "N" {
		t.Fatalf("legacy theme = %#v", got.Icons)
	}

	theme, err := parseTheme([]byte(`schemaVersion: 1
catalogVersion: 1
name: ascii
appearance: dark
kinds:
  source:
    icons:
      ascii: "[X]"
  source.go:
    icons:
      ascii: null
roles:
  test:
    icons:
      ascii: "[T]"
rules:
  - match: {name: README.md}
    icons:
      ascii: "[R]"
`), "ascii.yaml")
	if err != nil {
		t.Fatal(err)
	}
	compiled, err = Compile(theme)
	if err != nil {
		t.Fatal(err)
	}
	source := compiled.Inspect("lib.rs", "lib.rs", tree.NodeFile)
	if source.Icons.ASCII != "[X]" {
		t.Fatalf("kind ascii override = %#v", source.Icons)
	}
	reset := compiled.Inspect("main.go", "main.go", tree.NodeFile)
	if reset.Icons.ASCII != "" {
		t.Fatalf("ascii null must clear the inherited glyph: %#v", reset.Icons)
	}
	testFile := compiled.Inspect("user_test.go", "user_test.go", tree.NodeFile)
	if testFile.Icons.ASCII != "[T]" {
		t.Fatalf("role ascii override = %#v", testFile.Icons)
	}
	readme := compiled.Inspect("README.md", "README.md", tree.NodeFile)
	if readme.Icons.ASCII != "[R]" {
		t.Fatalf("rule ascii override = %#v", readme.Icons)
	}

	for _, glyph := range []string{"→", "📁"} {
		document := "schemaVersion: 1\ncatalogVersion: 1\nname: bad\nappearance: dark\nkinds:\n  source:\n    icons:\n      ascii: \"" + glyph + "\"\n"
		if _, parseErr := parseTheme([]byte(document), "bad.yaml"); parseErr == nil || !IsInvalid(parseErr) {
			t.Fatalf("%q ascii glyph unexpectedly accepted: %v", glyph, parseErr)
		}
	}
}

func TestDecoratorASCIIAndStyleOrthogonality(t *testing.T) {
	theme, _ := Lookup(ThemeDefault)
	compiled, err := Compile(theme)
	if err != nil {
		t.Fatal(err)
	}
	context := render.NodeContext{Path: "src/main.go", Name: "main.go", Display: "main.go", Type: tree.NodeFile}
	asciiDecorator := NewDecorator(compiled, false, IconsASCII, ProfileANSI16)
	unicodeDecorator := NewDecorator(compiled, false, IconsUnicode, ProfileANSI16)
	nerdDecorator := NewDecorator(compiled, false, IconsNerd, ProfileANSI16)
	if got := asciiDecorator.Node(context); got != "[SC] main.go" {
		t.Fatalf("ascii node = %q", got)
	}
	if got := unicodeDecorator.Node(context); !strings.HasPrefix(got, catalog.Glyphs("source.go").Unicode+" ") {
		t.Fatalf("unicode node = %q", got)
	}
	if asciiDecorator.Node(context) == unicodeDecorator.Node(context) || unicodeDecorator.Node(context) == nerdDecorator.Node(context) {
		t.Fatal("icon modes must change the glyph without sharing the same prefix")
	}
	if asciiDecorator.Edge("|-- ") != "|-- " || unicodeDecorator.Edge("├── ") != "├── " {
		t.Fatal("icon mode must not restyle tree edges")
	}
}
