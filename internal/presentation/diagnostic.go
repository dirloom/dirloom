package presentation

import (
	"encoding/json"
	"fmt"
	"io"
)

// WriteListText writes built-in themes in lexical order.
func WriteListText(writer io.Writer) error {
	if _, err := fmt.Fprintln(writer, "Built-in themes:"); err != nil {
		return err
	}
	for _, theme := range BuiltIns() {
		if _, err := fmt.Fprintf(writer, "  %s: %s\n", theme.Name, theme.Description); err != nil {
			return err
		}
	}
	return nil
}

// WriteListJSON writes the stable built-in theme list contract.
func WriteListJSON(writer io.Writer) error {
	document := ListDocument{SchemaVersion: SchemaVersion, Themes: BuiltIns()}
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(document)
}

// Validation creates a successful result for a fully parsed theme.
func Validation(theme Theme) ValidationResult {
	warnings := append([]Warning(nil), theme.Warnings...)
	if warnings == nil {
		warnings = []Warning{}
	}
	return ValidationResult{SchemaVersion: SchemaVersion, Valid: true, Source: theme.Source, Name: theme.Name, Warnings: warnings}
}

// WriteJSON writes the stable validation contract.
func (result ValidationResult) WriteJSON(writer io.Writer) error {
	if result.Warnings == nil {
		result.Warnings = []Warning{}
	}
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(result)
}

// WriteText writes a stable human validation result.
func (result ValidationResult) WriteText(writer io.Writer) error {
	if _, err := fmt.Fprintf(writer, "Valid theme: %s\nSource: %s", result.Name, result.Source.Kind); err != nil {
		return err
	}
	if result.Source.Path != "" {
		if _, err := fmt.Fprintf(writer, " (%s)", result.Source.Path); err != nil {
			return err
		}
	}
	if len(result.Warnings) == 0 {
		_, err := fmt.Fprintln(writer, "\nWarnings: none")
		return err
	}
	if _, err := fmt.Fprintln(writer, "\nWarnings:"); err != nil {
		return err
	}
	for _, warning := range result.Warnings {
		if _, err := fmt.Fprintf(writer, "  - [%s] %s\n", warning.Code, warning.Message); err != nil {
			return err
		}
	}
	return nil
}
