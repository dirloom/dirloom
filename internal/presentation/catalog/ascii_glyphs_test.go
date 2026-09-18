package catalog

import (
	"testing"
	"unicode"
)

func TestEveryKindResolvesThreeGlyphChannels(t *testing.T) {
	wantASCII := map[Kind]string{
		"file": "[FI]", "source": "[SC]", "manifest": "[MF]", "data": "[DT]",
		"document": "[DC]", "media": "[ME]", "archive": "[AR]", "font": "[FT]",
		"binary": "[BN]", "directory": "[DR]", "symlink": "[LN]",
		"source.go": "[SC]", "source.rust": "[SC]", "source.python": "[SC]",
		"manifest.container": "[MF]", "document.markdown": "[DC]",
	}
	for _, definition := range Kinds() {
		if definition.ASCII == "" || definition.Unicode == "" || definition.Nerd == "" {
			t.Errorf("kind %s is missing a compiled glyph channel: %#v", definition.Kind, definition)
		}
		glyphs := Glyphs(definition.Kind)
		if glyphs.ASCII == "" || glyphs.Unicode == "" || glyphs.Nerd == "" {
			t.Errorf("kind %s does not resolve all glyph channels: %#v", definition.Kind, glyphs)
		}
		if err := validateASCIICatalogGlyph(definition.ASCII); err != nil {
			t.Errorf("kind %s ascii: %v", definition.Kind, err)
		}
		if err := validateUnicodeCatalogGlyph(definition.Unicode); err != nil {
			t.Errorf("kind %s unicode: %v", definition.Kind, err)
		}
		if err := validateCatalogGlyph(definition.Nerd); err != nil {
			t.Errorf("kind %s nerd: %v", definition.Kind, err)
		}
		if want, ok := wantASCII[definition.Kind]; ok && glyphs.ASCII != want {
			t.Errorf("kind %s ascii = %q, want %q", definition.Kind, glyphs.ASCII, want)
		}
	}
}

func TestUnicodeCatalogGlyphsStayOutsidePrivateUseAndEmojiSelectors(t *testing.T) {
	for _, definition := range Kinds() {
		for _, char := range definition.Unicode {
			if isPrivateUseArea(char) {
				t.Errorf("kind %s unicode uses PUA U+%04X", definition.Kind, char)
			}
			if char == '\u200D' || isEmojiVariationSelector(char) || unicode.IsControl(char) {
				t.Errorf("kind %s unicode contains forbidden rune U+%04X", definition.Kind, char)
			}
		}
	}
	if err := validateUnicodeCatalogGlyph("\uE000"); err == nil {
		t.Fatal("BMP PUA unexpectedly accepted")
	}
	if err := validateUnicodeCatalogGlyph("\U000F0000"); err == nil {
		t.Fatal("plane 15 PUA unexpectedly accepted")
	}
	if err := validateUnicodeCatalogGlyph("\U00100000"); err == nil {
		t.Fatal("plane 16 PUA unexpectedly accepted")
	}
}

func TestASCIICatalogGlyphsRejectUnicode(t *testing.T) {
	for _, glyph := range []string{"é", "→", "📁", "\uE000"} {
		if err := validateASCIICatalogGlyph(glyph); err == nil {
			t.Errorf("%q unexpectedly accepted as ASCII", glyph)
		}
	}
	if err := validateASCIICatalogGlyph("[SC]"); err != nil {
		t.Fatal(err)
	}
}
