package catalog

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Pinned Nerd Fonts registry used to compile Dirloom's Nerd catalog.
// Runtime code never fetches this data; the values are compiled in.
const (
	nerdFontsVersion        = "3.5.1"
	nerdFontsTag            = "v3.5.1"
	nerdFontsRepository     = "ryanoasis/nerd-fonts"
	nerdFontsGlyphnamesFile = "glyphnames.json"
	nerdFontsRetrievalDate  = "2026-09-18"
)

const (
	collectionMDI      = "Material Design Icons"
	collectionDevicons = "Devicons"

	upstreamMDI      = "https://github.com/Templarian/MaterialDesign"
	upstreamDevicons = "https://github.com/devicons/devicon"

	licenseApache20 = "Apache-2.0"
	licenseMIT      = "MIT"
)

// NerdGlyphClass is the v0.3.3 governance class for one kind projection.
type NerdGlyphClass string

const (
	nerdExactTechnology NerdGlyphClass = "exact-technology"
	nerdExactSemantic   NerdGlyphClass = "exact-semantic"
	nerdGenericSemantic NerdGlyphClass = "generic-semantic"
	nerdGenericFallback NerdGlyphClass = "generic-fallback"
)

// NerdGlyphDefinition is the auditable provenance for one compiled Nerd glyph.
type NerdGlyphDefinition struct {
	Glyph        string
	OfficialName string
	Codepoint    string
	Collection   string
	Upstream     string
	License      string
	Class        NerdGlyphClass
}

func mdi(officialName, hex string) NerdGlyphDefinition {
	return nerdGlyph(officialName, hex, collectionMDI, upstreamMDI, licenseApache20)
}

func dev(officialName, hex string) NerdGlyphDefinition {
	return nerdGlyph(officialName, hex, collectionDevicons, upstreamDevicons, licenseMIT)
}

func nerdGlyph(officialName, hex, collection, upstream, license string) NerdGlyphDefinition {
	char, err := nerdRuneFromHex(hex)
	if err != nil {
		return NerdGlyphDefinition{OfficialName: officialName, Collection: collection, Upstream: upstream, License: license}
	}
	return NerdGlyphDefinition{
		Glyph:        string(char),
		OfficialName: officialName,
		Codepoint:    fmt.Sprintf("U+%04X", char),
		Collection:   collection,
		Upstream:     upstream,
		License:      license,
	}
}

func nerdRuneFromHex(hex string) (rune, error) {
	value, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return 0, err
	}
	if value > uint64(unicode.MaxRune) {
		return 0, fmt.Errorf("nerd codepoint %s exceeds Unicode maximum", hex)
	}
	return rune(value), nil //nolint:gosec // G115: bitSize 32 parse is bounded to unicode.MaxRune.
}

func nerdGlyphFor(kind Kind) string {
	definition, ok := nerdGlyphByKind[kind]
	if !ok {
		return ""
	}
	return definition.Glyph
}

var (
	glyphFile         = mdi("nf-md-file", "f0214")
	glyphSource       = mdi("nf-md-code-braces", "f0169")
	glyphManifest     = mdi("nf-md-code-json", "f0626")
	glyphData         = mdi("nf-md-database", "f01bc")
	glyphDocument     = mdi("nf-md-file-document", "f0219")
	glyphMedia        = mdi("nf-md-image", "f02e9")
	glyphArchive      = mdi("nf-md-archive", "f003c")
	glyphFont         = mdi("nf-md-format-font", "f06d6")
	glyphBinary       = mdi("nf-md-console", "f018d")
	glyphDirectory    = mdi("nf-md-folder", "f024b")
	glyphSymlink      = mdi("nf-md-link", "f0337")
	glyphGo           = mdi("nf-md-language-go", "f07d3")
	glyphRust         = mdi("nf-md-language-rust", "f1617")
	glyphPython       = mdi("nf-md-language-python", "f0320")
	glyphJavaScript   = mdi("nf-md-language-javascript", "f031e")
	glyphTypeScript   = mdi("nf-md-language-typescript", "f06e6")
	glyphHTML         = mdi("nf-md-language-html5", "f031d")
	glyphCSS          = mdi("nf-md-language-css3", "f031c")
	glyphC            = mdi("nf-md-language-c", "f0671")
	glyphCpp          = mdi("nf-md-language-cpp", "f0672")
	glyphCSharp       = mdi("nf-md-language-csharp", "f031b")
	glyphJava         = mdi("nf-md-language-java", "f0b37")
	glyphKotlin       = mdi("nf-md-language-kotlin", "f1219")
	glyphPHP          = mdi("nf-md-language-php", "f031f")
	glyphRuby         = mdi("nf-md-language-ruby", "f0d2d")
	glyphSwift        = mdi("nf-md-language-swift", "f06e5")
	glyphLua          = mdi("nf-md-language-lua", "f08b1")
	glyphR            = mdi("nf-md-language-r", "f07d4")
	glyphVue          = mdi("nf-md-vuejs", "f0844")
	glyphGraphQL      = mdi("nf-md-graphql", "f0877")
	glyphHaskell      = mdi("nf-md-language-haskell", "f0c92")
	glyphFortran      = mdi("nf-md-language-fortran", "f121a")
	glyphNix          = mdi("nf-md-nix", "f1105")
	glyphTerraform    = mdi("nf-md-terraform", "f1062")
	glyphDocker       = mdi("nf-md-docker", "f0868")
	glyphPowerShell   = mdi("nf-md-powershell", "f0a0a")
	glyphShell        = mdi("nf-md-bash", "f1183")
	glyphNode         = mdi("nf-md-nodejs", "f0399")
	glyphDotNet       = mdi("nf-md-dot-net", "f0aae")
	glyphMarkdown     = mdi("nf-md-language-markdown", "f0354")
	glyphPDF          = mdi("nf-md-file-pdf-box", "f0226")
	glyphPNG          = mdi("nf-md-file-png-box", "f0e2d")
	glyphJPEG         = mdi("nf-md-file-jpg-box", "f0225")
	glyphSVG          = mdi("nf-md-svg", "f0721")
	glyphPackage      = mdi("nf-md-package-variant-closed", "f03d7")
	glyphZip          = mdi("nf-md-zip-box", "f05c4")
	glyphCertificate  = mdi("nf-md-certificate", "f0124")
	glyphKey          = mdi("nf-md-key", "f0306")
	glyphLocalization = mdi("nf-md-translate", "f05ca")
	glyphNotebook     = mdi("nf-md-notebook", "f082e")
	glyphXML          = mdi("nf-md-xml", "f05c0")
	glyphCog          = mdi("nf-md-cog", "f0493")
	glyphMusic        = mdi("nf-md-music", "f075a")
	glyphPalette      = mdi("nf-md-palette", "f03d8")
	glyphVideo        = mdi("nf-md-video", "f0567")
	glyphProtocol     = mdi("nf-md-protocol", "f0fd8")
	glyphHCL          = mdi("nf-md-code-braces-box", "f10d6")
	glyphLambda       = mdi("nf-md-lambda", "f0627")
	glyphParentheses  = mdi("nf-md-code-parentheses", "f0172")
	glyphLicense      = mdi("nf-md-license", "f0fc3")
	glyphChangelog    = mdi("nf-md-history", "f02da")
	glyphSvelte       = dev("nf-dev-svelte", "e8b7")
	glyphAstro        = dev("nf-dev-astro", "e735")
	glyphDart         = dev("nf-dev-dart", "e798")
	glyphHelm         = dev("nf-dev-helm", "e7fb")
	glyphOCaml        = dev("nf-dev-ocaml", "e84e")
	glyphNim          = dev("nf-dev-nim", "e841")
	glyphD            = dev("nf-dev-dlang", "e7af")
	glyphGleam        = dev("nf-dev-gleam", "e914")
	glyphElm          = dev("nf-dev-elm", "e7ce")
	glyphCrystal      = dev("nf-dev-crystal", "e7ac")
	glyphFSharp       = dev("nf-dev-fsharp", "e7a7")
)

var nerdGlyphByKind = buildNerdGlyphByKind()

func buildNerdGlyphByKind() map[Kind]NerdGlyphDefinition {
	result := make(map[Kind]NerdGlyphDefinition, KindCount)
	put := func(class NerdGlyphClass, glyph NerdGlyphDefinition, kinds ...Kind) {
		glyph.Class = class
		for _, kind := range kinds {
			result[kind] = glyph
		}
	}

	// Semantic generic glyphs and family fallbacks.
	put(nerdExactSemantic, glyphFile, "file")
	put(nerdGenericSemantic, glyphSource, "source")
	put(nerdGenericSemantic, glyphManifest, "manifest")
	put(nerdGenericSemantic, glyphData, "data")
	put(nerdGenericSemantic, glyphDocument, "document")
	put(nerdGenericSemantic, glyphMedia, "media")
	put(nerdExactSemantic, glyphArchive, "archive")
	put(nerdExactSemantic, glyphFont, "font")
	put(nerdGenericSemantic, glyphBinary, "binary")
	put(nerdExactSemantic, glyphDirectory, "directory")
	put(nerdExactSemantic, glyphSymlink, "symlink")

	// Technology-specific glyphs with approved Nerd Fonts collection provenance.
	put(nerdExactTechnology, glyphGo, "source.go", "manifest.go")
	put(nerdExactTechnology, glyphRust, "source.rust", "manifest.rust")
	put(nerdExactTechnology, glyphPython, "source.python", "manifest.python")
	put(nerdExactTechnology, glyphJava, "source.java", "manifest.java")
	put(nerdExactTechnology, glyphPHP, "source.php", "manifest.php")
	put(nerdExactTechnology, glyphDart, "source.dart", "manifest.dart")
	put(nerdExactTechnology, glyphNix, "source.nix", "manifest.nix")
	put(nerdExactTechnology, glyphJavaScript, "source.javascript")
	put(nerdExactTechnology, glyphTypeScript, "source.typescript")
	put(nerdExactTechnology, glyphHTML, "source.html")
	put(nerdExactTechnology, glyphCSS, "source.css")
	put(nerdExactTechnology, glyphC, "source.c")
	put(nerdExactTechnology, glyphCpp, "source.cpp")
	put(nerdExactTechnology, glyphCSharp, "source.csharp")
	put(nerdExactTechnology, glyphFSharp, "source.fsharp")
	put(nerdExactTechnology, glyphKotlin, "source.kotlin")
	put(nerdExactTechnology, glyphRuby, "source.ruby")
	put(nerdExactTechnology, glyphSwift, "source.swift")
	put(nerdExactTechnology, glyphLua, "source.lua")
	put(nerdExactTechnology, glyphR, "source.r")
	put(nerdExactTechnology, glyphVue, "source.vue")
	put(nerdExactTechnology, glyphSvelte, "source.svelte")
	put(nerdExactTechnology, glyphAstro, "source.astro")
	put(nerdExactTechnology, glyphGraphQL, "source.graphql")
	put(nerdExactTechnology, glyphHaskell, "source.haskell")
	put(nerdExactTechnology, glyphOCaml, "source.ocaml")
	put(nerdExactTechnology, glyphNim, "source.nim")
	put(nerdExactTechnology, glyphD, "source.d")
	put(nerdExactTechnology, glyphFortran, "source.fortran")
	put(nerdExactTechnology, glyphGleam, "source.gleam")
	put(nerdExactTechnology, glyphElm, "source.elm")
	put(nerdExactTechnology, glyphCrystal, "source.crystal")
	put(nerdExactTechnology, glyphPowerShell, "source.powershell")
	put(nerdExactTechnology, glyphShell, "source.shell")
	put(nerdExactTechnology, glyphNode, "manifest.node")
	put(nerdExactTechnology, glyphDotNet, "manifest.dotnet")
	put(nerdExactTechnology, glyphDocker, "manifest.container")
	put(nerdExactTechnology, glyphTerraform, "manifest.terraform")
	put(nerdExactTechnology, glyphHelm, "manifest.helm")

	// Exact semantic glyphs for documents, media, and data identities.
	put(nerdExactSemantic, glyphMarkdown, "document.markdown")
	put(nerdExactSemantic, glyphPDF, "document.pdf")
	put(nerdExactSemantic, glyphLicense, "document.license")
	put(nerdExactSemantic, glyphChangelog, "document.changelog")
	put(nerdExactSemantic, glyphJPEG, "media.image.jpeg")
	put(nerdExactSemantic, glyphPNG, "media.image.png")
	put(nerdExactSemantic, glyphSVG, "media.image.svg")
	put(nerdExactSemantic, glyphMedia, "media.image")
	put(nerdExactSemantic, glyphMusic, "media.audio")
	put(nerdExactSemantic, glyphVideo, "media.video")
	put(nerdExactSemantic, glyphPalette, "media.design")
	put(nerdExactSemantic, glyphPackage, "archive.package")
	put(nerdExactSemantic, glyphZip, "archive.compressed")
	put(nerdExactSemantic, glyphCertificate, "data.certificate")
	put(nerdExactSemantic, glyphKey, "data.key")
	put(nerdExactSemantic, glyphLocalization, "data.localization")
	put(nerdExactSemantic, glyphNotebook, "data.notebook")
	put(nerdExactSemantic, glyphXML, "data.xml", "data.plist")
	put(nerdGenericSemantic, glyphCog, "data.properties")
	put(nerdGenericSemantic, glyphProtocol, "source.protobuf")
	put(nerdGenericSemantic, glyphHCL, "source.hcl")
	put(nerdGenericSemantic, glyphLambda, "source.scheme")
	put(nerdGenericSemantic, glyphParentheses, "source.racket")
	put(nerdGenericSemantic, glyphManifest, "data.json", "source.jsonnet")
	put(nerdGenericSemantic, glyphDocument, "data.yaml", "data.toml")
	put(nerdGenericSemantic, glyphData, "data.database")

	// Conservative generic fallbacks: no approved technology glyph, so the
	// family identity is used instead of a metaphor that could be mistaken
	// for a logo.
	put(nerdGenericFallback, glyphSource,
		"source.assembly", "source.batch", "source.clojure", "source.elixir",
		"source.erlang", "source.groovy", "source.julia", "source.objective-c",
		"source.perl", "source.scala", "source.solidity", "source.vhdl",
		"source.webassembly", "source.zig", "source.cue", "source.v",
	)
	put(nerdGenericFallback, glyphManifest, "manifest.generic")
	put(nerdGenericFallback, glyphData,
		"data.binary", "data.env", "data.ini", "data.schema", "data.sql", "data.tabular",
	)
	put(nerdGenericFallback, glyphDocument,
		"document.asciidoc", "document.ebook", "document.office", "document.rst",
		"document.tex", "document.text",
	)
	put(nerdGenericFallback, glyphMedia, "media.model")
	put(nerdGenericFallback, glyphFont, "font.web")
	put(nerdGenericFallback, glyphBinary, "binary.executable", "binary.library")

	return result
}

func validateNerdGlyphs() error {
	if len(nerdGlyphByKind) != KindCount {
		return fmt.Errorf("nerd catalog has %d kind projections; expected %d", len(nerdGlyphByKind), KindCount)
	}
	seen := make(map[Kind]struct{}, KindCount)
	for kind, definition := range nerdGlyphByKind {
		if _, ok := kindRegistry[kind]; !ok {
			return fmt.Errorf("nerd projection for unknown kind %q", kind)
		}
		seen[kind] = struct{}{}
		if err := validateNerdGlyphDefinition(kind, definition); err != nil {
			return err
		}
	}
	for kind := range kindRegistry {
		if _, ok := seen[kind]; !ok {
			return fmt.Errorf("kind %q has no nerd provenance", kind)
		}
	}
	return nil
}

func validateNerdGlyphDefinition(kind Kind, definition NerdGlyphDefinition) error {
	if definition.OfficialName == "" || definition.Codepoint == "" || definition.Collection == "" || definition.Upstream == "" || definition.License == "" {
		return fmt.Errorf("kind %q nerd provenance is incomplete", kind)
	}
	switch definition.Class {
	case nerdExactTechnology, nerdExactSemantic, nerdGenericSemantic, nerdGenericFallback:
	default:
		return fmt.Errorf("kind %q has unknown nerd class %q", kind, definition.Class)
	}
	if err := validateCatalogGlyph(definition.Glyph); err != nil {
		return fmt.Errorf("kind %q nerd glyph: %w", kind, err)
	}
	if !strings.HasPrefix(definition.OfficialName, "nf-") {
		return fmt.Errorf("kind %q official nerd name %q is not a Nerd Fonts CSS name", kind, definition.OfficialName)
	}
	if !nerdCodepointPattern.MatchString(definition.Codepoint) {
		return fmt.Errorf("kind %q nerd codepoint %q is not U+XXXX or U+XXXXX", kind, definition.Codepoint)
	}
	expected, err := nerdRuneFromHex(strings.TrimPrefix(definition.Codepoint, "U+"))
	if err != nil {
		return fmt.Errorf("kind %q nerd codepoint %q: %w", kind, definition.Codepoint, err)
	}
	runes := []rune(definition.Glyph)
	if len(runes) != 1 || runes[0] != expected {
		return fmt.Errorf("kind %q nerd glyph does not match codepoint %s", kind, definition.Codepoint)
	}
	return nil
}

var nerdCodepointPattern = regexp.MustCompile(`^U\+[0-9A-F]{4,5}$`)
