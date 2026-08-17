package presentation

import (
	"reflect"
	"testing"

	"github.com/dirloom/dirloom/internal/presentation/catalog"
	"github.com/dirloom/dirloom/internal/tree"
)

func TestCompilePublicRulesAndEveryMatcherShape(t *testing.T) {
	theme, _ := Lookup(ThemeDefault)
	theme.Rules = []Rule{
		{Match: Match{Path: "src/main.go"}, Kind: "source.go", Role: "test", Color: "ansi:cyan", IconColor: "ansi:magenta", Styles: []string{}, Icons: IconPair{Unicode: "G", Nerd: "N"}},
		{Match: Match{Name: "README.md"}, Color: "ansi:yellow"},
		{Match: Match{Glob: "generated/**"}, Role: "generated"},
		{Match: Match{Extension: ".txt"}, Kind: "document.text"},
		{Match: Match{Type: "directory"}, Role: "source"},
	}
	compiled, err := Compile(theme)
	if err != nil {
		t.Fatal(err)
	}
	pathRule := compiled.Inspect("src/main.go", "main.go", tree.NodeFile)
	if pathRule.Classification.Kind != "source.go" || pathRule.VisualRole != catalog.RoleTest || pathRule.TextColor != "ansi:cyan" || pathRule.IconColor != "ansi:magenta" || pathRule.Icons != (IconPair{Unicode: "G", Nerd: "N"}) {
		t.Fatalf("path rule = %#v", pathRule)
	}
	if got := compiled.Inspect("README.md", "README.md", tree.NodeFile); got.TextColor != "ansi:yellow" {
		t.Fatalf("name rule = %#v", got)
	}
	if got := compiled.Inspect("generated/value.go", "value.go", tree.NodeFile); got.VisualRole != catalog.RoleGenerated {
		t.Fatalf("glob rule = %#v", got)
	}
	if got := compiled.Inspect("notes.txt", "notes.txt", tree.NodeFile); got.Classification.Kind != "document.text" {
		t.Fatalf("extension rule = %#v", got)
	}
	if got := compiled.Inspect("src", "src", tree.NodeDirectory); got.VisualRole != catalog.RoleSource {
		t.Fatalf("type rule = %#v", got)
	}
}

func TestCompileRejectsInvalidProgrammaticContractsAndUsesNeutralFallbacks(t *testing.T) {
	theme, _ := Lookup(ThemeDefault)
	invalid := []Theme{}
	wrongCatalog := cloneTheme(theme)
	wrongCatalog.CatalogVersion = 2
	invalid = append(invalid, wrongCatalog)
	badToken := cloneTheme(theme)
	badToken.Tokens["node.file"] = Token{Color: "not-a-color", Styles: []string{}}
	invalid = append(invalid, badToken)
	badKind := cloneTheme(theme)
	badKind.Kinds["file"] = Binding{Color: "not-a-color"}
	invalid = append(invalid, badKind)
	badRole := cloneTheme(theme)
	badRole.Roles["generic"] = Binding{Color: "not-a-color"}
	invalid = append(invalid, badRole)
	badRule := cloneTheme(theme)
	badRule.Rules = []Rule{{Match: Match{Name: "x"}, Color: "not-a-color"}}
	invalid = append(invalid, badRule)
	for index, candidate := range invalid {
		if _, err := Compile(candidate); err == nil || !IsInvalid(err) {
			t.Errorf("invalid theme %d error = %v", index, err)
		}
	}

	fallback := cloneTheme(theme)
	delete(fallback.Tokens, "node.file")
	delete(fallback.Tokens, "tree.edge")
	compiled, err := Compile(fallback)
	if err != nil {
		t.Fatal(err)
	}
	style := compiled.resolve("unknown", "unknown", tree.NodeFile)
	if style.color.kind != colorDefault || style.iconColor.kind != colorDefault || style.styles == nil {
		t.Fatalf("fallback style = %#v", style)
	}
	edge := compiled.edge()
	if edge.color.kind != colorDefault || edge.styles == nil {
		t.Fatalf("fallback edge = %#v", edge)
	}
	if got := formatColor(colorSpec{kind: colorANSI, ansiIndex: ansiNames["red"]}); got != "ansi:red" {
		t.Fatalf("ANSI color = %q", got)
	}
	for _, glyph := range []string{"", "\xff", "abcde", "\u202e"} {
		if err := validateGlyph(glyph); err == nil {
			t.Errorf("glyph %q unexpectedly valid", glyph)
		}
	}
}

func TestCloneThemeNormalizesPublicCollections(t *testing.T) {
	theme := Theme{Palette: map[string]string{}, Tokens: map[string]Token{"node.file": {}}, Kinds: nil, Roles: nil, Rules: nil, Warnings: nil}
	clone := cloneTheme(theme)
	if clone.Kinds == nil || clone.Roles == nil || clone.Rules == nil || clone.Warnings == nil || clone.Tokens["node.file"].Styles == nil {
		t.Fatalf("clone collections = %#v", clone)
	}
	if !reflect.DeepEqual(clone.Rules, []Rule{}) || !reflect.DeepEqual(clone.Warnings, []Warning{}) {
		t.Fatalf("normalized arrays = %#v/%#v", clone.Rules, clone.Warnings)
	}
}
