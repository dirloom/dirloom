package config

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

const (
	PresetNone              = "none"
	presetDiagnosticVersion = 1
)

// PresetDefaults contains the scalar inspection defaults supplied by a preset.
type PresetDefaults struct {
	Depth           int    `json:"depth"`
	DirectoriesOnly bool   `json:"dirsOnly"`
	IncludeHidden   bool   `json:"hidden"`
	Format          string `json:"format"`
	Style           string `json:"style"`
}

// PresetFilters contains the filtering switches supplied by a preset.
type PresetFilters struct {
	UseDefaultIgnores bool `json:"useDefaultIgnores"`
	UseGitIgnore      bool `json:"useGitignore"`
}

// PresetDefinition is the stable public description of one built-in preset.
type PresetDefinition struct {
	SchemaVersion int            `json:"schemaVersion"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	Defaults      PresetDefaults `json:"defaults"`
	Filters       PresetFilters  `json:"filters"`
	Ignore        []string       `json:"ignore"`
}

var presetCatalog = map[string]PresetDefinition{
	"docs": newPreset(
		"docs",
		"Produce a Markdown tree for documentation and reviews.",
		4,
		false,
		FormatMarkdown,
		nil,
	),
	"compact": newPreset(
		"compact",
		"Show a shallow directory-only overview.",
		3,
		true,
		FormatText,
		nil,
	),
	"monorepo": newPreset(
		"monorepo",
		"Show a directory-first workspace overview without repeated build outputs.",
		4,
		true,
		FormatText,
		[]string{"**/dist", "**/build"},
	),
	"ai": newPreset(
		"ai",
		"Produce a concise Markdown structure for AI-assisted workflows.",
		4,
		false,
		FormatMarkdown,
		[]string{"**/dist", "**/build", "*.map"},
	),
}

func newPreset(name, description string, depth int, directoriesOnly bool, format string, ignore []string) PresetDefinition {
	return PresetDefinition{
		SchemaVersion: presetDiagnosticVersion,
		Name:          name,
		Description:   description,
		Defaults: PresetDefaults{
			Depth:           depth,
			DirectoriesOnly: directoriesOnly,
			IncludeHidden:   false,
			Format:          format,
			Style:           StyleUnicode,
		},
		Filters: PresetFilters{
			UseDefaultIgnores: true,
			UseGitIgnore:      true,
		},
		Ignore: append([]string{}, ignore...),
	}
}

// PresetNames returns the canonical preset names in stable lexical order.
func PresetNames() []string {
	names := make([]string, 0, len(presetCatalog))
	for name := range presetCatalog {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// LookupPreset returns a defensive copy of a built-in preset definition.
func LookupPreset(name string) (PresetDefinition, bool) {
	definition, ok := presetCatalog[name]
	if !ok {
		return PresetDefinition{}, false
	}
	definition.Ignore = append([]string{}, definition.Ignore...)
	return definition, true
}

// WriteText writes the human-readable preset contract.
func (definition PresetDefinition) WriteText(writer io.Writer) error {
	if _, err := fmt.Fprintf(writer, "Preset: %s\nPurpose: %s\n\n", definition.Name, definition.Description); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "Defaults:\n  depth: %d\n  dirsOnly: %t\n  hidden: %t\n  format: %s\n  style: %s\n\n",
		definition.Defaults.Depth,
		definition.Defaults.DirectoriesOnly,
		definition.Defaults.IncludeHidden,
		definition.Defaults.Format,
		definition.Defaults.Style,
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "Filters:\n  useDefaultIgnores: %t\n  useGitignore: %t\n\n",
		definition.Filters.UseDefaultIgnores,
		definition.Filters.UseGitIgnore,
	); err != nil {
		return err
	}
	if len(definition.Ignore) == 0 {
		_, err := fmt.Fprintln(writer, "Ignore: none")
		return err
	}
	if _, err := fmt.Fprintln(writer, "Ignore:"); err != nil {
		return err
	}
	for _, pattern := range definition.Ignore {
		if _, err := fmt.Fprintf(writer, "  %s\n", pattern); err != nil {
			return err
		}
	}
	return nil
}

// WriteJSON writes the versioned machine-readable preset contract.
func (definition PresetDefinition) WriteJSON(writer io.Writer) error {
	definition.Ignore = append([]string{}, definition.Ignore...)
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(definition)
}

func presetPartial(definition PresetDefinition) partial {
	return partial{
		Depth:             DepthOverride{Set: true, Value: definition.Defaults.Depth},
		DirectoriesOnly:   Optional[bool]{Set: true, Value: definition.Defaults.DirectoriesOnly},
		IncludeHidden:     Optional[bool]{Set: true, Value: definition.Defaults.IncludeHidden},
		Format:            Optional[string]{Set: true, Value: definition.Defaults.Format},
		Style:             Optional[string]{Set: true, Value: definition.Defaults.Style},
		UseDefaultIgnores: Optional[bool]{Set: true, Value: definition.Filters.UseDefaultIgnores},
		UseGitIgnore:      Optional[bool]{Set: true, Value: definition.Filters.UseGitIgnore},
		IgnorePatterns:    append([]string{}, definition.Ignore...),
	}
}
