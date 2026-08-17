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
		{Kind: "file", Unicode: "·", Nerd: "󰈔"},
		{Kind: "source", Parent: "file", Unicode: "•", Nerd: "󰅩"},
		{Kind: "manifest", Parent: "file", Unicode: "◇", Nerd: "󰘦"},
		{Kind: "data", Parent: "file", Unicode: "◇", Nerd: "󰆼"},
		{Kind: "document", Parent: "file", Unicode: "¶", Nerd: "󰈙"},
		{Kind: "media", Parent: "file", Unicode: "◆", Nerd: "󰉏"},
		{Kind: "archive", Parent: "file", Unicode: "▣", Nerd: "󰀼"},
		{Kind: "font", Parent: "file", Unicode: "A", Nerd: "󰛖"},
		{Kind: "binary", Parent: "file", Unicode: "▪", Nerd: "󰆍"},
		{Kind: "directory", Unicode: "▸", Nerd: "󰉋"},
		{Kind: "symlink", Unicode: "↗", Nerd: "󰌷"},
	}
	appendChildren := func(parent Kind, names []string, unicodeGlyph, nerdGlyph string) {
		for _, name := range names {
			definitions = append(definitions, KindDefinition{
				Kind: Kind(string(parent) + "." + name), Parent: parent, Unicode: unicodeGlyph, Nerd: nerdGlyph,
			})
		}
	}
	appendChildren("source", []string{
		"c", "cpp", "objective-c", "swift", "go", "rust", "zig", "java",
		"kotlin", "scala", "csharp", "fsharp", "dart", "solidity", "vhdl", "assembly",
		"python", "ruby", "php", "lua", "perl", "r", "julia", "elixir",
		"erlang", "clojure", "groovy", "shell", "powershell", "batch", "javascript", "typescript",
		"html", "css", "vue", "svelte", "astro", "graphql", "protobuf", "webassembly",
	}, "•", "󰅩")
	appendChildren("manifest", []string{"node", "go", "rust", "python", "java", "dotnet", "dart", "container", "php", "generic"}, "◇", "󰘦")
	appendChildren("data", []string{"json", "yaml", "toml", "xml", "ini", "env", "tabular", "sql", "schema", "binary", "database", "notebook"}, "◇", "󰆼")
	appendChildren("document", []string{"markdown", "rst", "asciidoc", "tex", "text", "pdf", "office", "ebook", "changelog", "license"}, "¶", "󰈙")
	appendChildren("media", []string{"image", "image.png", "image.jpeg", "image.svg", "audio", "video", "design", "model"}, "◆", "󰉏")
	appendChildren("archive", []string{"package", "compressed"}, "▣", "󰀼")
	appendChildren("font", []string{"web"}, "A", "󰛖")
	appendChildren("binary", []string{"executable", "library"}, "▪", "󰆍")

	overrides := map[Kind]struct{ unicode, nerd string }{
		"source.go": {"•", "󰟓"}, "source.rust": {"•", "󱘗"},
		"source.python": {"•", "󰌠"}, "source.javascript": {"•", "󰌞"},
		"source.typescript": {"•", "󰛦"}, "source.html": {"•", "󰌝"},
		"source.css": {"•", "󰌜"}, "data.json": {"◇", "󰘦"},
		"data.yaml": {"◇", "󰈙"}, "data.toml": {"◇", "󰈙"},
		"document.markdown": {"¶", "󰍔"}, "document.pdf": {"¶", "󰈦"},
		"media.image.png": {"◆", "󰸭"}, "archive.package": {"▣", "󰏗"},
		"manifest.container": {"▣", "󰡨"},
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

// Glyphs returns the effective catalog glyph pair for a kind.
func Glyphs(kind Kind) (string, string) {
	for current := kind; current != ""; {
		definition, ok := kindRegistry[current]
		if !ok {
			break
		}
		if definition.Unicode != "" || definition.Nerd != "" {
			return definition.Unicode, definition.Nerd
		}
		current = definition.Parent
	}
	return "", ""
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
		if err := validateCatalogGlyph(definition.Unicode); err != nil {
			return fmt.Errorf("kind %q unicode glyph: %w", kind, err)
		}
		if err := validateCatalogGlyph(definition.Nerd); err != nil {
			return fmt.Errorf("kind %q nerd glyph: %w", kind, err)
		}
	}
	return nil
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
