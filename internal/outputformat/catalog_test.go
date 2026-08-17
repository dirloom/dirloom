package outputformat

import (
	"reflect"
	"strings"
	"testing"
)

func TestCatalogContract(t *testing.T) {
	want := []string{Text, Markdown, MarkdownTree, JSON, Mermaid, Graphviz, D2}
	if got := Names(); !reflect.DeepEqual(got, want) {
		t.Fatalf("names = %#v, want %#v", got, want)
	}

	seen := make(map[string]struct{})
	for _, name := range Names() {
		if _, exists := seen[name]; exists {
			t.Fatalf("duplicate canonical name %q", name)
		}
		seen[name] = struct{}{}
		descriptor, ok := Lookup(name)
		if !ok || descriptor.Name != name || descriptor.Family == "" || len(descriptor.Extensions) == 0 {
			t.Fatalf("incomplete descriptor for %q: %#v", name, descriptor)
		}
		for _, alias := range descriptor.Aliases {
			if _, exists := seen[alias]; exists {
				t.Fatalf("duplicate alias %q", alias)
			}
			seen[alias] = struct{}{}
		}
	}
}

func TestGraphvizAliasAndCapabilities(t *testing.T) {
	descriptor, ok := Lookup("dot")
	if !ok || descriptor.Name != Graphviz || descriptor.Family != FamilyDiagram {
		t.Fatalf("dot descriptor = %#v, %t", descriptor, ok)
	}
	if canonical, ok := Canonical("dot"); !ok || canonical != Graphviz {
		t.Fatalf("canonical dot = %q, %t", canonical, ok)
	}
	for _, name := range []string{Mermaid, Graphviz, "dot", D2} {
		if !IsDiagram(name) || UsesStyle(name) || UsesPresentation(name) {
			t.Errorf("unexpected diagram capabilities for %q", name)
		}
	}
	if !UsesStyle(Text) || !UsesPresentation(Text) || !UsesStyle(Markdown) || UsesPresentation(Markdown) {
		t.Fatal("text or markdown capabilities are incorrect")
	}
}

func TestLookupIsDefensiveAndDiagnosticsAreStable(t *testing.T) {
	descriptor, _ := Lookup(Graphviz)
	descriptor.Aliases[0] = "changed"
	descriptor.Extensions[0] = ".changed"
	again, _ := Lookup(Graphviz)
	if again.Aliases[0] != "dot" || again.Extensions[0] != ".dot" {
		t.Fatal("catalog was mutated through returned descriptor")
	}

	if err := Validate("yaml"); err == nil || !strings.Contains(err.Error(), `unsupported format "yaml"`) ||
		!strings.Contains(err.Error(), "dot (alias for graphviz)") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}
