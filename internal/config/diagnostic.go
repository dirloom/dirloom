package config

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

const configurationDiagnosticVersion = 1

type diagnosticDocument struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Root          string              `json:"root"`
	Sources       []Source            `json:"sources"`
	Preset        ResolvedPreset      `json:"preset"`
	Effective     diagnosticEffective `json:"effective"`
	Provenance    map[string]Origin   `json:"provenance"`
	Inactive      []string            `json:"inactive"`
}

type diagnosticEffective struct {
	Depth             *int         `json:"depth"`
	DirectoriesOnly   bool         `json:"dirsOnly"`
	IncludeHidden     bool         `json:"hidden"`
	Format            string       `json:"format"`
	Style             string       `json:"style"`
	UseDefaultIgnores bool         `json:"useDefaultIgnores"`
	UseGitIgnore      bool         `json:"useGitignore"`
	Ignore            []IgnoreRule `json:"ignore"`
}

// WriteJSON writes the stable machine-readable configuration diagnostic.
func (resolution Resolution) WriteJSON(writer io.Writer) error {
	document := resolution.diagnostic()
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(document)
}

// WriteText writes a concise human-readable configuration diagnostic.
func (resolution Resolution) WriteText(writer io.Writer) error {
	if _, err := fmt.Fprintf(writer, "Root: %s\n\nSources:\n", resolution.Root); err != nil {
		return err
	}
	for _, source := range resolution.Sources {
		if _, err := fmt.Fprintf(writer, "  %s: %s", source.Kind, source.Status); err != nil {
			return err
		}
		if source.Path != "" {
			if _, err := fmt.Fprintf(writer, " (%s)", source.Path); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(writer); err != nil {
			return err
		}
	}
	preset := "none"
	if resolution.Preset.Name != nil {
		preset = *resolution.Preset.Name
	}
	if _, err := fmt.Fprintf(writer, "\nPreset: %s (%s)\n", preset, formatOrigin(resolution.Preset.Origin)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, "\nEffective:"); err != nil {
		return err
	}
	depth := "unlimited"
	if resolution.Effective.MaxDepth != nil {
		depth = strconv.Itoa(*resolution.Effective.MaxDepth)
	}
	values := []struct {
		name  string
		value string
	}{
		{"depth", depth},
		{"dirsOnly", strconv.FormatBool(resolution.Effective.DirectoriesOnly)},
		{"hidden", strconv.FormatBool(resolution.Effective.IncludeHidden)},
		{"format", resolution.Effective.Format},
		{"style", resolution.Effective.Style},
		{"useDefaultIgnores", strconv.FormatBool(resolution.Effective.UseDefaultIgnores)},
		{"useGitignore", strconv.FormatBool(resolution.Effective.UseGitIgnore)},
	}
	for _, value := range values {
		inactive := ""
		if value.name == "style" && resolution.Effective.Format == FormatJSON {
			inactive = "; inactive for json"
		}
		if _, err := fmt.Fprintf(writer, "  %s: %s (%s%s)\n", value.name, value.value, formatOrigin(resolution.Provenance[value.name]), inactive); err != nil {
			return err
		}
	}
	if len(resolution.Ignores) == 0 {
		_, err := fmt.Fprintln(writer, "\nIgnore: none")
		return err
	}
	if _, err := fmt.Fprintln(writer, "\nIgnore:"); err != nil {
		return err
	}
	for _, rule := range resolution.Ignores {
		if _, err := fmt.Fprintf(writer, "  - %s (%s)\n", rule.Pattern, formatOrigin(rule.Origin)); err != nil {
			return err
		}
	}
	return nil
}

func (resolution Resolution) diagnostic() diagnosticDocument {
	inactive := make([]string, 0, 1)
	if resolution.Effective.Format == FormatJSON {
		inactive = append(inactive, "style")
	}
	ignores := append([]IgnoreRule(nil), resolution.Ignores...)
	if ignores == nil {
		ignores = []IgnoreRule{}
	}
	sources := append([]Source(nil), resolution.Sources...)
	if sources == nil {
		sources = []Source{}
	}
	return diagnosticDocument{
		SchemaVersion: configurationDiagnosticVersion,
		Root:          resolution.Root,
		Sources:       sources,
		Preset:        resolution.Preset,
		Effective: diagnosticEffective{
			Depth:             resolution.Effective.MaxDepth,
			DirectoriesOnly:   resolution.Effective.DirectoriesOnly,
			IncludeHidden:     resolution.Effective.IncludeHidden,
			Format:            resolution.Effective.Format,
			Style:             resolution.Effective.Style,
			UseDefaultIgnores: resolution.Effective.UseDefaultIgnores,
			UseGitIgnore:      resolution.Effective.UseGitIgnore,
			Ignore:            ignores,
		},
		Provenance: resolution.Provenance,
		Inactive:   inactive,
	}
}

func formatOrigin(origin Origin) string {
	label := string(origin.Source)
	if origin.Preset != "" {
		label += " preset " + origin.Preset
	}
	if origin.Path == "" {
		return label
	}
	return fmt.Sprintf("%s: %s", label, origin.Path)
}
