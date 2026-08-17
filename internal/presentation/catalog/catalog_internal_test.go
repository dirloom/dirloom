package catalog

import (
	"strings"
	"testing"
)

func TestCatalogValidationRejectsBrokenInvariants(t *testing.T) {
	withManifest := func(t *testing.T, mutate func([]Entry) []Entry, want string) {
		t.Helper()
		original := manifest
		copyManifest := make([]Entry, len(original))
		for index, entry := range original {
			copyManifest[index] = cloneEntry(entry)
		}
		manifest = mutate(copyManifest)
		defer func() { manifest = original }()
		if err := Validate(); err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("validation error = %v, want %q", err, want)
		}
	}
	withKinds := func(t *testing.T, mutate func(map[Kind]KindDefinition), want string) {
		t.Helper()
		original := kindRegistry
		copyRegistry := make(map[Kind]KindDefinition, len(original))
		for key, value := range original {
			copyRegistry[key] = value
		}
		kindRegistry = copyRegistry
		defer func() { kindRegistry = original }()
		mutate(kindRegistry)
		if err := Validate(); err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("validation error = %v, want %q", err, want)
		}
	}

	t.Run("entry count", func(t *testing.T) {
		withManifest(t, func(values []Entry) []Entry { return values[:len(values)-1] }, "entries")
	})
	t.Run("role count", func(t *testing.T) {
		original := canonicalRoles
		canonicalRoles = canonicalRoles[:len(canonicalRoles)-1]
		defer func() { canonicalRoles = original }()
		if err := Validate(); err == nil || !strings.Contains(err.Error(), "roles") {
			t.Fatalf("validation error = %v", err)
		}
	})
	t.Run("kind count", func(t *testing.T) {
		withKinds(t, func(values map[Kind]KindDefinition) { delete(values, "source.go") }, "kinds")
	})
	t.Run("invalid kind identifier", func(t *testing.T) {
		withKinds(t, func(values map[Kind]KindDefinition) {
			definition := values["source.go"]
			delete(values, "source.go")
			definition.Kind = "Source.Go"
			values[definition.Kind] = definition
		}, "invalid kind identifier")
	})
	t.Run("unknown parent", func(t *testing.T) {
		withKinds(t, func(values map[Kind]KindDefinition) {
			definition := values["source.go"]
			definition.Parent = "missing"
			values["source.go"] = definition
		}, "unknown parent")
	})
	t.Run("cycle", func(t *testing.T) {
		withKinds(t, func(values map[Kind]KindDefinition) {
			definition := values["source.go"]
			definition.Parent = "source.go"
			values["source.go"] = definition
		}, "inheritance chain")
	})
	t.Run("invalid glyph", func(t *testing.T) {
		withKinds(t, func(values map[Kind]KindDefinition) {
			definition := values["source.go"]
			definition.Unicode = ""
			values["source.go"] = definition
		}, "unicode glyph")
	})
	t.Run("unknown matcher kind", func(t *testing.T) {
		withManifest(t, func(values []Entry) []Entry { values[0].Kind = "missing"; return values }, "unknown kind")
	})
	t.Run("empty matcher roles", func(t *testing.T) {
		withManifest(t, func(values []Entry) []Entry { values[0].Roles = nil; return values }, "has no role")
	})
	t.Run("unknown matcher role", func(t *testing.T) {
		withManifest(t, func(values []Entry) []Entry { values[0].Roles = []Role{"future"}; return values }, "unknown role")
	})
	t.Run("duplicate matcher", func(t *testing.T) {
		withManifest(t, func(values []Entry) []Entry { values[len(values)-1] = cloneEntry(values[0]); return values }, "duplicate")
	})
	t.Run("group count", func(t *testing.T) {
		withManifest(t, func(values []Entry) []Entry {
			values[0].Matcher = Matcher{Source: SourceExtension, Value: ".fixture-unique"}
			return values
		}, "matchers")
	})
}

func TestCatalogInternalFallbackAndValidationBranches(t *testing.T) {
	if !IsKind("source.go") || IsKind("source.future") {
		t.Fatal("kind membership contract changed")
	}
	if chain := KindChain("source.future"); chain != nil {
		t.Fatalf("unknown chain = %#v", chain)
	}
	if unicodeGlyph, nerdGlyph := Glyphs("source.future"); unicodeGlyph != "" || nerdGlyph != "" {
		t.Fatalf("unknown glyphs = %q/%q", unicodeGlyph, nerdGlyph)
	}
	original := kindRegistry["source.go"]
	withoutGlyph := original
	withoutGlyph.Unicode, withoutGlyph.Nerd = "", ""
	kindRegistry["source.go"] = withoutGlyph
	unicodeGlyph, nerdGlyph := Glyphs("source.go")
	kindRegistry["source.go"] = original
	if unicodeGlyph != "•" || nerdGlyph == "" {
		t.Fatalf("parent fallback = %q/%q", unicodeGlyph, nerdGlyph)
	}

	if got := normalizeRoles([]Role{RoleSource, "future", RoleSecurity, RoleSource}); len(got) != 2 || got[0] != RoleSecurity || got[1] != RoleSource {
		t.Fatalf("normalized roles = %#v", got)
	}
	if got := normalizeRoles(nil); len(got) != 1 || got[0] != RoleGeneric {
		t.Fatalf("empty roles = %#v", got)
	}
	for name, glyph := range map[string]string{
		"empty": "", "control": "\x1b", "bidi": "\u202e", "too-many-runes": "abcde", "too-many-bytes": strings.Repeat("a", 65),
	} {
		if err := validateCatalogGlyph(glyph); err == nil {
			t.Errorf("%s glyph unexpectedly valid", name)
		}
	}
	if err := validateCatalogGlyph("•"); err != nil {
		t.Fatal(err)
	}

	definition := kindRegistry["source.go"]
	definition.Parent = "source.go"
	kindRegistry["source.go"] = definition
	if chain := KindChain("source.go"); chain != nil {
		t.Fatalf("cycle chain = %#v", chain)
	}
	kindRegistry["source.go"] = original
}
