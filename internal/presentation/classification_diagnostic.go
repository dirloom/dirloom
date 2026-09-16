package presentation

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/dirloom/dirloom/internal/presentation/catalog"
	"github.com/dirloom/dirloom/internal/tree"
)

// ClassifyTheme identifies the theme used to resolve a classification.
type ClassifyTheme struct {
	Name   string `json:"name"`
	Source Source `json:"source"`
}

// ClassifyStyle is the normalized style selected for one real entry.
type ClassifyStyle struct {
	TextColor string   `json:"textColor"`
	IconColor string   `json:"iconColor"`
	Styles    []string `json:"styles"`
	Icons     IconPair `json:"icons"`
}

// ClassifyDocument is the independent JSON v1 contract for theme classify.
type ClassifyDocument struct {
	SchemaVersion  int                    `json:"schemaVersion"`
	Path           string                 `json:"path"`
	Type           tree.NodeType          `json:"type"`
	Classification catalog.Classification `json:"classification"`
	VisualRole     catalog.Role           `json:"visualRole"`
	Theme          ClassifyTheme          `json:"theme"`
	Style          ClassifyStyle          `json:"style"`
	Origins        StyleOrigins           `json:"origins"`
}

// NewClassifyDocument resolves a pure semantic/style diagnostic.
func NewClassifyDocument(pathValue, name string, nodeType tree.NodeType, theme Theme, compiled *CompiledTheme) ClassifyDocument {
	inspection := compiled.Inspect(pathValue, name, nodeType)
	source := theme.Source
	// A classification never leaks a host-absolute theme path.
	if source.Kind == "file" {
		source.Path = ""
	}
	styles := append([]string(nil), inspection.Styles...)
	if styles == nil {
		styles = []string{}
	}
	return ClassifyDocument{
		SchemaVersion: ThemeClassifySchemaVersion, Path: pathValue, Type: nodeType,
		Classification: inspection.Classification, VisualRole: inspection.VisualRole,
		Theme:   ClassifyTheme{Name: theme.Name, Source: source},
		Style:   ClassifyStyle{TextColor: inspection.TextColor, IconColor: inspection.IconColor, Styles: styles, Icons: inspection.Icons},
		Origins: inspection.Origins,
	}
}

// WriteJSON writes the stable classification diagnostic.
func (document ClassifyDocument) WriteJSON(writer io.Writer) error {
	if document.Classification.Roles == nil {
		document.Classification.Roles = []catalog.Role{}
	}
	if document.Style.Styles == nil {
		document.Style.Styles = []string{}
	}
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(document)
}

// WriteText writes a stable, undecorated classification diagnostic.
func (document ClassifyDocument) WriteText(writer io.Writer) error {
	roles := make([]string, len(document.Classification.Roles))
	for index, role := range document.Classification.Roles {
		roles[index] = string(role)
	}
	if _, err := fmt.Fprintf(writer, "Path: %s\nType: %s\nKind: %s\nRoles: %s\nVisual role: %s\nMatched by: %s (%s)\nTheme: %s (%s)\nText: color=%s styles=%s\nIcon: unicode=%s nerd=%s color=%s\n",
		document.Path, document.Type, document.Classification.Kind, strings.Join(roles, ", "),
		document.VisualRole, document.Classification.Source, document.Classification.MatcherKey,
		document.Theme.Name, document.Theme.Source.Kind, document.Style.TextColor,
		formatStyles(document.Style.Styles), quoteGlyph(document.Style.Icons.Unicode), quoteGlyph(document.Style.Icons.Nerd),
		document.Style.IconColor,
	); err != nil {
		return err
	}
	return nil
}

func quoteGlyph(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}
