// Package outputformat defines Dirloom's public output format catalog.
package outputformat

import (
	"fmt"
	"strings"
)

const (
	Text         = "text"
	Markdown     = "markdown"
	MarkdownTree = "markdown-tree"
	JSON         = "json"
	Mermaid      = "mermaid"
	Graphviz     = "graphviz"
	D2           = "d2"
)

// Family groups formats by their public purpose.
type Family string

const (
	FamilyText     Family = "text"
	FamilyDocument Family = "document"
	FamilyMachine  Family = "machine"
	FamilyDiagram  Family = "diagram"
)

// Descriptor records stable capabilities for one public format.
type Descriptor struct {
	Name             string
	Aliases          []string
	Family           Family
	Extensions       []string
	UsesStyle        bool
	UsesPresentation bool
}

var catalog = []Descriptor{
	{Name: Text, Family: FamilyText, Extensions: []string{".txt"}, UsesStyle: true, UsesPresentation: true},
	{Name: Markdown, Family: FamilyDocument, Extensions: []string{".md"}, UsesStyle: true},
	{Name: MarkdownTree, Family: FamilyDocument, Extensions: []string{".md"}},
	{Name: JSON, Family: FamilyMachine, Extensions: []string{".json"}},
	{Name: Mermaid, Family: FamilyDiagram, Extensions: []string{".mmd", ".mermaid"}},
	{Name: Graphviz, Aliases: []string{"dot"}, Family: FamilyDiagram, Extensions: []string{".dot", ".gv"}},
	{Name: D2, Family: FamilyDiagram, Extensions: []string{".d2"}},
}

// Lookup returns a defensive copy for a canonical name or alias.
func Lookup(name string) (Descriptor, bool) {
	for _, descriptor := range catalog {
		if descriptor.Name == name {
			return clone(descriptor), true
		}
		for _, alias := range descriptor.Aliases {
			if alias == name {
				return clone(descriptor), true
			}
		}
	}
	return Descriptor{}, false
}

// Canonical returns the canonical public name for a name or alias.
func Canonical(name string) (string, bool) {
	descriptor, ok := Lookup(name)
	if !ok {
		return "", false
	}
	return descriptor.Name, true
}

// Names returns canonical names in stable public order.
func Names() []string {
	names := make([]string, 0, len(catalog))
	for _, descriptor := range catalog {
		names = append(names, descriptor.Name)
	}
	return names
}

// AcceptedNames returns canonical names followed by aliases in stable order.
func AcceptedNames() []string {
	values := Names()
	for _, descriptor := range catalog {
		values = append(values, descriptor.Aliases...)
	}
	return values
}

// Expected formats the accepted canonical names and aliases for diagnostics.
func Expected() string {
	values := Names()
	for _, descriptor := range catalog {
		for _, alias := range descriptor.Aliases {
			values = append(values, alias+" (alias for "+descriptor.Name+")")
		}
	}
	if len(values) == 1 {
		return values[0]
	}
	return strings.Join(values[:len(values)-1], ", ") + ", or " + values[len(values)-1]
}

// Validate returns an actionable error for an unsupported format.
func Validate(name string) error {
	if _, ok := Lookup(name); ok {
		return nil
	}
	return fmt.Errorf("unsupported format %q (expected %s)", name, Expected())
}

// IsDiagram reports whether a name or alias selects a diagram source format.
func IsDiagram(name string) bool {
	descriptor, ok := Lookup(name)
	return ok && descriptor.Family == FamilyDiagram
}

// UsesStyle reports whether drawing style affects the format.
func UsesStyle(name string) bool {
	descriptor, ok := Lookup(name)
	return ok && descriptor.UsesStyle
}

// UsesPresentation reports whether terminal presentation affects the format.
func UsesPresentation(name string) bool {
	descriptor, ok := Lookup(name)
	return ok && descriptor.UsesPresentation
}

func clone(descriptor Descriptor) Descriptor {
	descriptor.Aliases = append([]string(nil), descriptor.Aliases...)
	descriptor.Extensions = append([]string(nil), descriptor.Extensions...)
	return descriptor
}
