package catalog

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

var kindRegistry = buildKindRegistry()

func buildKindRegistry() map[Kind]KindDefinition {
	definitions := []KindDefinition{
		{Kind: "file", ASCII: "[FI]", Unicode: "·", Nerd: "󰈔"},
		{Kind: "source", Parent: "file", ASCII: "[SC]", Unicode: "•", Nerd: "󰅩"},
		{Kind: "manifest", Parent: "file", ASCII: "[MF]", Unicode: "◇", Nerd: "󰘦"},
		{Kind: "data", Parent: "file", ASCII: "[DT]", Unicode: "◇", Nerd: "󰆼"},
		{Kind: "document", Parent: "file", ASCII: "[DC]", Unicode: "¶", Nerd: "󰈙"},
		{Kind: "media", Parent: "file", ASCII: "[ME]", Unicode: "◆", Nerd: "󰉏"},
		{Kind: "archive", Parent: "file", ASCII: "[AR]", Unicode: "▣", Nerd: "󰀼"},
		{Kind: "font", Parent: "file", ASCII: "[FT]", Unicode: "A", Nerd: "󰛖"},
		{Kind: "binary", Parent: "file", ASCII: "[BN]", Unicode: "▪", Nerd: "󰆍"},
		{Kind: "directory", ASCII: "[DR]", Unicode: "▸", Nerd: "󰉋"},
		{Kind: "symlink", ASCII: "[LN]", Unicode: "↗", Nerd: "󰌷"},
	}
	appendChildren := func(parent Kind, names []string, asciiGlyph, unicodeGlyph, nerdGlyph string) {
		for _, name := range names {
			definitions = append(definitions, KindDefinition{
				Kind: Kind(string(parent) + "." + name), Parent: parent, ASCII: asciiGlyph, Unicode: unicodeGlyph, Nerd: nerdGlyph,
			})
		}
	}
	appendChildren("source", []string{
		"c", "cpp", "objective-c", "swift", "go", "rust", "zig", "java",
		"kotlin", "scala", "csharp", "fsharp", "dart", "solidity", "vhdl", "assembly",
		"python", "ruby", "php", "lua", "perl", "r", "julia", "elixir",
		"erlang", "clojure", "groovy", "shell", "powershell", "batch", "javascript", "typescript",
		"html", "css", "vue", "svelte", "astro", "graphql", "protobuf", "webassembly",
		"haskell", "ocaml", "nim", "d", "fortran", "gleam", "scheme", "racket",
		"elm", "v", "crystal", "nix", "hcl", "cue", "jsonnet",
	}, "[SC]", "•", "󰅩")
	appendChildren("manifest", []string{
		"node", "go", "rust", "python", "java", "dotnet", "dart", "container", "php", "generic",
		"terraform", "helm", "nix",
	}, "[MF]", "◇", "󰘦")
	appendChildren("data", []string{
		"json", "yaml", "toml", "xml", "ini", "env", "tabular", "sql", "schema", "binary", "database", "notebook",
		"properties", "plist", "certificate", "key", "localization",
	}, "[DT]", "◇", "󰆼")
	appendChildren("document", []string{"markdown", "rst", "asciidoc", "tex", "text", "pdf", "office", "ebook", "changelog", "license"}, "[DC]", "¶", "󰈙")
	appendChildren("media", []string{"image", "image.png", "image.jpeg", "image.svg", "audio", "video", "design", "model"}, "[ME]", "◆", "󰉏")
	appendChildren("archive", []string{"package", "compressed"}, "[AR]", "▣", "󰀼")
	appendChildren("font", []string{"web"}, "[FT]", "A", "󰛖")
	appendChildren("binary", []string{"executable", "library"}, "[BN]", "▪", "󰆍")

	// Unicode glyphs for existing kinds are compatibility-frozen. Nerd glyphs may
	// become more specific; values listed here are the effective pair.
	overrides := map[Kind]struct{ unicode, nerd string }{
		"source.go": {"•", "󰟓"}, "source.rust": {"•", "󱘗"},
		"source.python": {"•", "󰌠"}, "source.javascript": {"•", "󰌞"},
		"source.typescript": {"•", "󰛦"}, "source.html": {"•", "󰌝"},
		"source.css": {"•", "󰌜"}, "data.json": {"◇", "󰘦"},
		"data.yaml": {"◇", "󰈙"}, "data.toml": {"◇", "󰈙"},
		"document.markdown": {"¶", "󰍔"}, "document.pdf": {"¶", "󰈦"},
		"media.image.png": {"◆", "󰸭"}, "archive.package": {"▣", "󰏗"},
		"manifest.container": {"▣", "󰡨"},
		"source.c":           {"•", "\U000F0671"}, "source.cpp": {"•", "\U000F0672"},
		"source.csharp": {"•", "\U000F031B"}, "source.java": {"•", "\U000F0B37"},
		"source.kotlin": {"•", "\U000F1219"}, "source.php": {"•", "\U000F031F"},
		"source.ruby": {"•", "\U000F0D2D"}, "source.swift": {"•", "\U000F06E5"},
		"source.lua": {"•", "\U000F08B1"}, "source.r": {"•", "\U000F07D4"},
		"source.vue": {"•", "\U000F0844"}, "source.svelte": {"•", "\U000F059F"},
		"source.astro": {"•", "\U000F06E4"}, "source.graphql": {"•", "\U000F0877"},
		"source.protobuf": {"•", "\U000F0FD8"}, "source.haskell": {"•", "\U000F0C92"},
		"source.ocaml": {"•", "\U000F0295"}, "source.nim": {"•", "\U000F02D8"},
		"source.d": {"•", "\U000F01A6"}, "source.fortran": {"•", "\U000F121A"},
		"source.gleam": {"•", "\U000F04A0"}, "source.scheme": {"•", "\U000F0627"},
		"source.racket": {"•", "\U000F0172"}, "source.elm": {"•", "\U000F0405"},
		"source.v": {"•", "\U000F016C"}, "source.crystal": {"•", "\U000F01C8"},
		"source.nix": {"•", "\U000F1105"}, "source.hcl": {"•", "\U000F10D6"},
		"source.cue": {"•", "\U000F0168"}, "source.jsonnet": {"•", "\U000F0626"},
		"source.powershell": {"•", "\U000F0A0A"}, "source.shell": {"•", "\U000F1183"},
		"source.fsharp": {"•", "\U000F0627"}, "source.dart": {"•", "\U000F08C6"},
		"manifest.node": {"◇", "\U000F0399"}, "manifest.go": {"◇", "󰟓"},
		"manifest.rust": {"◇", "󱘗"}, "manifest.python": {"◇", "󰌠"},
		"manifest.java": {"◇", "\U000F0B37"}, "manifest.dotnet": {"◇", "\U000F0AAE"},
		"manifest.php": {"◇", "\U000F031F"}, "manifest.terraform": {"◇", "\U000F1062"},
		"manifest.helm": {"◇", "\U000F10FE"}, "manifest.nix": {"◇", "\U000F1105"},
		"data.database": {"◇", "\U000F01BC"}, "data.notebook": {"◇", "\U000F082E"},
		"data.properties": {"◇", "\U000F0493"}, "data.plist": {"◇", "\U000F05C0"},
		"data.certificate": {"◇", "\U000F0124"}, "data.key": {"◇", "\U000F0306"},
		"data.localization": {"◇", "\U000F05CA"}, "data.xml": {"◇", "\U000F05C0"},
		"media.audio": {"◆", "\U000F075A"}, "media.video": {"◆", "\U000F0567"},
		"media.design": {"◆", "\U000F03D8"}, "media.image": {"◆", "\U000F02E9"},
		"archive.compressed": {"▣", "\U000F05C4"},
	}
	for index := range definitions {
		if value, ok := overrides[definitions[index].Kind]; ok {
			definitions[index].Unicode = value.unicode
			definitions[index].Nerd = value.nerd
		}
	}
	result := make(map[Kind]KindDefinition, len(definitions))
	for _, definition := range definitions {
		result[definition.Kind] = definition
	}
	return result
}

// Kinds returns definitions ordered lexically and defensively.
func Kinds() []KindDefinition {
	result := make([]KindDefinition, 0, len(kindRegistry))
	for _, definition := range kindRegistry {
		result = append(result, definition)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Kind < result[right].Kind })
	return result
}

// LookupKind returns one immutable kind definition.
func LookupKind(kind Kind) (KindDefinition, bool) {
	value, ok := kindRegistry[kind]
	return value, ok
}

// IsKind reports whether a public kind exists.
func IsKind(value string) bool {
	_, ok := kindRegistry[Kind(value)]
	return ok
}

// KindChain returns the inheritance chain from the most generic parent to kind.
func KindChain(kind Kind) []Kind {
	var reversed []Kind
	seen := map[Kind]struct{}{}
	for kind != "" {
		if _, duplicate := seen[kind]; duplicate {
			return nil
		}
		seen[kind] = struct{}{}
		definition, ok := kindRegistry[kind]
		if !ok {
			return nil
		}
		reversed = append(reversed, kind)
		kind = definition.Parent
	}
	result := make([]Kind, len(reversed))
	for index := range reversed {
		result[len(reversed)-1-index] = reversed[index]
	}
	return result
}

// Glyphs returns the effective three-channel catalog glyphs for a kind.
func Glyphs(kind Kind) GlyphSet {
	var result GlyphSet
	for current := kind; current != ""; {
		definition, ok := kindRegistry[current]
		if !ok {
			break
		}
		if result.ASCII == "" {
			result.ASCII = definition.ASCII
		}
		if result.Unicode == "" {
			result.Unicode = definition.Unicode
		}
		if result.Nerd == "" {
			result.Nerd = definition.Nerd
		}
		if result.ASCII != "" && result.Unicode != "" && result.Nerd != "" {
			return result
		}
		current = definition.Parent
	}
	return result
}

func validateKinds() error {
	if len(kindRegistry) != KindCount {
		return fmt.Errorf("catalog has %d kinds; expected %d", len(kindRegistry), KindCount)
	}
	for kind, definition := range kindRegistry {
		if string(kind) != strings.ToLower(string(kind)) || strings.TrimSpace(string(kind)) == "" {
			return fmt.Errorf("invalid kind identifier %q", kind)
		}
		if definition.Parent != "" {
			if _, ok := kindRegistry[definition.Parent]; !ok {
				return fmt.Errorf("kind %q has unknown parent %q", kind, definition.Parent)
			}
		}
		chain := KindChain(kind)
		if len(chain) == 0 || len(chain) > 4 {
			return fmt.Errorf("kind %q has an invalid or too deep inheritance chain", kind)
		}
		if err := validateASCIICatalogGlyph(definition.ASCII); err != nil {
			return fmt.Errorf("kind %q ascii glyph: %w", kind, err)
		}
		if err := validateUnicodeCatalogGlyph(definition.Unicode); err != nil {
			return fmt.Errorf("kind %q unicode glyph: %w", kind, err)
		}
		if err := validateCatalogGlyph(definition.Nerd); err != nil {
			return fmt.Errorf("kind %q nerd glyph: %w", kind, err)
		}
	}
	return nil
}

func validateASCIICatalogGlyph(value string) error {
	if err := validateCatalogGlyph(value); err != nil {
		return err
	}
	for _, char := range value {
		if char < 0x20 || char > 0x7E {
			return fmt.Errorf("must contain only printable ASCII (U+0020 to U+007E)")
		}
	}
	return nil
}

func validateUnicodeCatalogGlyph(value string) error {
	if err := validateCatalogGlyph(value); err != nil {
		return err
	}
	for _, char := range value {
		if isPrivateUseArea(char) {
			return fmt.Errorf("contains Private Use Area character U+%04X", char)
		}
		if char == '\u200D' {
			return fmt.Errorf("contains a ZWJ sequence")
		}
		if isEmojiVariationSelector(char) {
			return fmt.Errorf("contains an emoji variation selector U+%04X", char)
		}
	}
	return nil
}

func isPrivateUseArea(char rune) bool {
	return char >= 0xE000 && char <= 0xF8FF ||
		char >= 0xF0000 && char <= 0xFFFFD ||
		char >= 0x100000 && char <= 0x10FFFD
}

func isEmojiVariationSelector(char rune) bool {
	return char == 0xFE0E || char == 0xFE0F || char >= 0xE0100 && char <= 0xE01EF
}

func validateCatalogGlyph(value string) error {
	if value == "" {
		return fmt.Errorf("must not be empty")
	}
	if len(value) > 64 || !utf8.ValidString(value) {
		return fmt.Errorf("must be valid UTF-8 and at most 64 bytes")
	}
	count := 0
	for _, char := range value {
		count++
		if unicode.IsControl(char) || char == '\x1b' || char == '\u061c' || char == '\u200e' || char == '\u200f' ||
			char >= '\u202a' && char <= '\u202e' || char >= '\u2066' && char <= '\u2069' {
			return fmt.Errorf("contains forbidden character U+%04X", char)
		}
	}
	if count > 4 {
		return fmt.Errorf("exceeds four runes")
	}
	return nil
}
