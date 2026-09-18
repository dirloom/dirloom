package catalog

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestASCIIAndUnicodeCatalogsRemainFrozen(t *testing.T) {
	if got := len(Kinds()); got != KindCount {
		t.Fatalf("kinds = %d", got)
	}
	for _, definition := range Kinds() {
		glyphs := Glyphs(definition.Kind)
		wantASCII := frozenASCIIFor(definition.Kind)
		wantUnicode := frozenUnicodeFor(definition.Kind)
		if glyphs.ASCII != wantASCII || definition.ASCII != wantASCII {
			t.Errorf("kind %s ascii = %q, want %q", definition.Kind, glyphs.ASCII, wantASCII)
		}
		if glyphs.Unicode != wantUnicode || definition.Unicode != wantUnicode {
			t.Errorf("kind %s unicode = %q, want %q", definition.Kind, glyphs.Unicode, wantUnicode)
		}
		if err := validateASCIICatalogGlyph(glyphs.ASCII); err != nil {
			t.Errorf("kind %s ascii: %v", definition.Kind, err)
		}
		for _, char := range glyphs.ASCII {
			if char < 0x20 || char > 0x7E {
				t.Errorf("kind %s ascii has non-ASCII U+%04X", definition.Kind, char)
			}
		}
		if utf8.RuneCountInString(glyphs.Unicode) == 0 {
			t.Errorf("kind %s unicode is empty", definition.Kind)
		}
	}
}

func frozenASCIIFor(kind Kind) string {
	switch kind {
	case "file":
		return "[FI]"
	case "directory":
		return "[DR]"
	case "symlink":
		return "[LN]"
	}
	switch familyOf(kind) {
	case "source":
		return "[SC]"
	case "manifest":
		return "[MF]"
	case "data":
		return "[DT]"
	case "document":
		return "[DC]"
	case "media":
		return "[ME]"
	case "archive":
		return "[AR]"
	case "font":
		return "[FT]"
	case "binary":
		return "[BN]"
	default:
		return ""
	}
}

func frozenUnicodeFor(kind Kind) string {
	switch kind {
	case "file":
		return "·"
	case "directory":
		return "▸"
	case "symlink":
		return "↗"
	case "manifest.container":
		return "▣"
	}
	switch familyOf(kind) {
	case "source":
		return "•"
	case "manifest", "data":
		return "◇"
	case "document":
		return "¶"
	case "media":
		return "◆"
	case "archive":
		return "▣"
	case "font":
		return "A"
	case "binary":
		return "▪"
	default:
		return ""
	}
}

func familyOf(kind Kind) string {
	family, _, _ := strings.Cut(string(kind), ".")
	return family
}
