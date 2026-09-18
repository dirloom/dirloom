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

func TestDecoratorNerdSpacingAndIconModeOrthogonality(t *testing.T) {
	theme, _ := Lookup(ThemeDefault)
	context := render.NodeContext{Path: "src/main.go", Name: "main.go", Display: "main.go", Type: tree.NodeFile}
	nerdGlyph := catalog.Glyphs("source.go").Nerd
	unicodeGlyph := catalog.Glyphs("source.go").Unicode
	asciiGlyph := catalog.Glyphs("source.go").ASCII
	for _, spacing := range []int{0, 1, 4} {
		theme.Icons.Spacing = spacing
		compiled, err := Compile(theme)
		if err != nil {
			t.Fatal(err)
		}
		gap := strings.Repeat(" ", spacing)
		nerdGot := NewDecorator(compiled, false, IconsNerd, ProfileANSI16).Node(context)
		if nerdGot != nerdGlyph+gap+"main.go" {
			t.Errorf("nerd spacing %d = %q", spacing, nerdGot)
		}
		asciiGot := NewDecorator(compiled, false, IconsASCII, ProfileANSI16).Node(context)
		if asciiGot != asciiGlyph+gap+"main.go" {
			t.Errorf("ascii spacing %d = %q", spacing, asciiGot)
		}
		unicodeGot := NewDecorator(compiled, false, IconsUnicode, ProfileANSI16).Node(context)
		if unicodeGot != unicodeGlyph+gap+"main.go" {
			t.Errorf("unicode spacing %d = %q", spacing, unicodeGot)
		}
		neverGot := NewDecorator(compiled, false, IconsNever, ProfileANSI16).Node(context)
		if neverGot != "main.go" {
			t.Errorf("never spacing %d = %q", spacing, neverGot)
		}
		if NewDecorator(compiled, false, IconsNerd, ProfileANSI16).Edge("|-- ") != "|-- " {
			t.Fatal("nerd icons must not change ASCII connectors")
		}
		if NewDecorator(compiled, false, IconsNerd, ProfileANSI16).Edge("├── ") != "├── " {
			t.Fatal("nerd icons must not change Unicode connectors")
		}
	}
}

func TestDecoratorStyleAndNerdIconsStayIndependent(t *testing.T) {
	theme, _ := Lookup(ThemeVivid)
	compiled, err := Compile(theme)
	if err != nil {
		t.Fatal(err)
	}
	context := render.NodeContext{Path: "src/main.go", Name: "main.go", Display: "main.go", Type: tree.NodeFile}
	nerd := NewDecorator(compiled, false, IconsNerd, ProfileANSI16)
	if !strings.HasPrefix(nerd.Node(context), catalog.Glyphs("source.go").Nerd) {
		t.Fatalf("vivid nerd node = %q", nerd.Node(context))
	}
	if nerd.Edge("|-- ") != "|-- " || nerd.Edge("├── ") != "├── " {
		t.Fatal("theme must not couple style connectors to nerd icons")
	}
}
