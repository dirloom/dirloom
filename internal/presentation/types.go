// Package presentation defines and renders Dirloom's terminal-only visual layer.
// It never changes the canonical tree model or machine-readable formats.
package presentation

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/dirloom/dirloom/internal/presentation/catalog"
)

const (
	ThemeFileSchemaVersion     = 1
	ThemeListSchemaVersion     = 1
	ThemeExplainSchemaVersion  = 1
	ThemeValidateSchemaVersion = 1
	ThemeClassifySchemaVersion = 1

	// SchemaVersion remains an alias for source compatibility inside v0.2.
	SchemaVersion = ThemeFileSchemaVersion

	ColorNever  = "never"
	ColorAlways = "always"
	ColorAuto   = "auto"

	IconsNever   = "never"
	IconsUnicode = "unicode"
	IconsNerd    = "nerd"
	IconsAuto    = "auto"

	ThemeDefault  = "default"
	ThemeMidnight = "midnight"
	ThemeDaylight = "daylight"
	ThemeVivid    = "vivid"

	AppearanceUniversal = "universal"
	AppearanceLight     = "light"
	AppearanceDark      = "dark"
)

var activeTokens = map[string]struct{}{
	"tree.edge": {}, "node.directory": {}, "node.file": {}, "node.symlink": {},
}

// IconPair defines portable and Nerd Font glyphs for one visual token.
type IconPair struct {
	Unicode string `json:"unicode" yaml:"unicode"`
	Nerd    string `json:"nerd" yaml:"nerd"`
}

// Token defines the presentation of one base tree segment.
type Token struct {
	Color     string   `json:"color,omitempty" yaml:"color"`
	IconColor string   `json:"iconColor,omitempty" yaml:"iconColor,omitempty"`
	Styles    []string `json:"styles" yaml:"styles"`
	Icons     IconPair `json:"icons" yaml:"icons"`
}

// Binding styles a semantic kind family or structural role.
type Binding struct {
	Color     string   `json:"color,omitempty" yaml:"color,omitempty"`
	IconColor string   `json:"iconColor,omitempty" yaml:"iconColor,omitempty"`
	Styles    []string `json:"styles,omitempty" yaml:"styles,omitempty"`
	Icons     IconPair `json:"icons,omitempty" yaml:"icons,omitempty"`

	colorSet       bool
	iconColorSet   bool
	stylesSet      bool
	unicodeIconSet bool
	nerdIconSet    bool
}

// Match identifies exactly one rule matcher.
type Match struct {
	Path      string `json:"path,omitempty" yaml:"path"`
	Name      string `json:"name,omitempty" yaml:"name"`
	Glob      string `json:"glob,omitempty" yaml:"glob"`
	Extension string `json:"extension,omitempty" yaml:"extension"`
	Type      string `json:"type,omitempty" yaml:"type"`
}

// Rule applies semantic and presentation overrides to matching nodes.
type Rule struct {
	Match     Match    `json:"match" yaml:"match"`
	Kind      string   `json:"kind,omitempty" yaml:"kind,omitempty"`
	Role      string   `json:"role,omitempty" yaml:"role,omitempty"`
	Color     string   `json:"color,omitempty" yaml:"color,omitempty"`
	IconColor string   `json:"iconColor,omitempty" yaml:"iconColor,omitempty"`
	Styles    []string `json:"styles,omitempty" yaml:"styles,omitempty"`
	Icons     IconPair `json:"icons,omitempty" yaml:"icons,omitempty"`
}

// IconSettings controls spacing between an icon and a displayed node name.
type IconSettings struct {
	Spacing int `json:"spacing" yaml:"spacing"`
}

// Source records whether a theme is compiled in or loaded from a local file.
type Source struct {
	Kind string `json:"kind"`
	Path string `json:"path,omitempty"`
}

// Warning is a stable non-blocking validation result.
type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// CatalogSummary exposes the catalog contract without dumping all matchers.
type CatalogSummary struct {
	Version    int `json:"version"`
	EntryCount int `json:"entryCount"`
	KindCount  int `json:"kindCount"`
	RoleCount  int `json:"roleCount"`
}

func semanticCatalogSummary() CatalogSummary {
	return CatalogSummary{Version: catalog.Version, EntryCount: catalog.EntryCount, KindCount: catalog.KindCount, RoleCount: catalog.RoleCount}
}

// Theme is the normalized, inspectable public theme definition.
type Theme struct {
	SchemaVersion  int                `json:"schemaVersion"`
	CatalogVersion int                `json:"catalogVersion"`
	Name           string             `json:"name"`
	Description    string             `json:"description"`
	Appearance     string             `json:"appearance"`
	Palette        map[string]string  `json:"palette"`
	Tokens         map[string]Token   `json:"tokens"`
	Kinds          map[string]Binding `json:"kinds"`
	Roles          map[string]Binding `json:"roles"`
	Rules          []Rule             `json:"rules"`
	Icons          IconSettings       `json:"icons"`
	Source         Source             `json:"source"`
	Warnings       []Warning          `json:"warnings"`
	Catalog        CatalogSummary     `json:"catalog"`

	customRules []ruleDocument
}

// ThemeDefinition is the normalized theme payload embedded in diagnostics.
type ThemeDefinition struct {
	CatalogVersion int                `json:"catalogVersion"`
	Name           string             `json:"name"`
	Description    string             `json:"description"`
	Appearance     string             `json:"appearance"`
	Palette        map[string]string  `json:"palette"`
	Tokens         map[string]Token   `json:"tokens"`
	Kinds          map[string]Binding `json:"kinds"`
	Roles          map[string]Binding `json:"roles"`
	Rules          []Rule             `json:"rules"`
	Icons          IconSettings       `json:"icons"`
	Source         Source             `json:"source"`
	Warnings       []Warning          `json:"warnings"`
}

// ExplainDocument is the independently versioned machine contract for theme explain.
type ExplainDocument struct {
	SchemaVersion      int             `json:"schemaVersion"`
	ThemeSchemaVersion int             `json:"themeSchemaVersion"`
	Catalog            CatalogSummary  `json:"catalog"`
	Theme              ThemeDefinition `json:"theme"`
}

// ValidationResult is emitted by theme validate.
type ValidationResult struct {
	SchemaVersion      int       `json:"schemaVersion"`
	ThemeSchemaVersion int       `json:"themeSchemaVersion"`
	CatalogVersion     int       `json:"catalogVersion"`
	Valid              bool      `json:"valid"`
	Source             Source    `json:"source"`
	Name               string    `json:"name,omitempty"`
	Warnings           []Warning `json:"warnings"`
}

// ListDocument is the stable machine contract for theme list.
type ListDocument struct {
	SchemaVersion int     `json:"schemaVersion"`
	Themes        []Theme `json:"themes"`
}

// InvalidError marks invalid user-controlled theme input.
type InvalidError struct{ Err error }

func (e *InvalidError) Error() string { return e.Err.Error() }
func (e *InvalidError) Unwrap() error { return e.Err }

func invalidf(format string, args ...any) error {
	return &InvalidError{Err: fmt.Errorf(format, args...)}
}

// IsInvalid reports whether an error should be classified as CLI usage error 2.
func IsInvalid(err error) bool {
	var invalid *InvalidError
	return errors.As(err, &invalid)
}

// ColorModes returns the public color values in canonical order.
func ColorModes() []string { return []string{ColorNever, ColorAlways, ColorAuto} }

// IconModes returns the public icon values in canonical order.
func IconModes() []string { return []string{IconsNever, IconsUnicode, IconsNerd, IconsAuto} }

// ThemeNames returns the built-in names in lexical order.
func ThemeNames() []string {
	names := []string{ThemeDefault, ThemeMidnight, ThemeDaylight, ThemeVivid}
	sort.Strings(names)
	return names
}

// IsBuiltIn reports whether a reference names a compiled-in theme.
func IsBuiltIn(name string) bool {
	_, ok := builtInCatalog[name]
	return ok
}

// IsThemePath reports whether a theme reference is explicitly path-like.
func IsThemePath(value string) bool {
	lower := strings.ToLower(value)
	return strings.ContainsAny(value, `/\`) || strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml")
}

// Lookup returns a defensive copy of one built-in theme.
func Lookup(name string) (Theme, bool) {
	theme, ok := builtInCatalog[name]
	if !ok {
		return Theme{}, false
	}
	return cloneTheme(theme), true
}

// BuiltIns returns defensive copies in lexical name order.
func BuiltIns() []Theme {
	names := ThemeNames()
	result := make([]Theme, 0, len(names))
	for _, name := range names {
		theme, _ := Lookup(name)
		result = append(result, theme)
	}
	return result
}

func cloneTheme(theme Theme) Theme {
	result := theme
	result.Palette = cloneStringMap(theme.Palette)
	result.Tokens = make(map[string]Token, len(theme.Tokens))
	for key, value := range theme.Tokens {
		value.Styles = append([]string(nil), value.Styles...)
		if value.Styles == nil {
			value.Styles = []string{}
		}
		result.Tokens[key] = value
	}
	result.Kinds = cloneBindings(theme.Kinds)
	result.Roles = cloneBindings(theme.Roles)
	result.Rules = cloneRules(theme.Rules)
	result.Warnings = append([]Warning(nil), theme.Warnings...)
	result.customRules = cloneRuleDocuments(theme.customRules)
	if result.Rules == nil {
		result.Rules = []Rule{}
	}
	if result.Warnings == nil {
		result.Warnings = []Warning{}
	}
	if result.Kinds == nil {
		result.Kinds = map[string]Binding{}
	}
	if result.Roles == nil {
		result.Roles = map[string]Binding{}
	}
	return result
}

func cloneStringMap(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func cloneBindings(values map[string]Binding) map[string]Binding {
	result := make(map[string]Binding, len(values))
	for key, value := range values {
		value.Styles = append([]string(nil), value.Styles...)
		result[key] = value
	}
	return result
}

func cloneRules(rules []Rule) []Rule {
	result := make([]Rule, len(rules))
	for index, rule := range rules {
		result[index] = rule
		result[index].Styles = append([]string(nil), rule.Styles...)
		if result[index].Styles == nil {
			result[index].Styles = []string{}
		}
	}
	return result
}

// Explanation returns the independently versioned normalized theme diagnostic.
func (theme Theme) Explanation() ExplainDocument {
	normalized := cloneTheme(theme)
	return ExplainDocument{
		SchemaVersion: ThemeExplainSchemaVersion, ThemeSchemaVersion: ThemeFileSchemaVersion,
		Catalog: normalized.Catalog,
		Theme: ThemeDefinition{
			CatalogVersion: normalized.CatalogVersion, Name: normalized.Name,
			Description: normalized.Description, Appearance: normalized.Appearance,
			Palette: normalized.Palette, Tokens: normalized.Tokens, Kinds: normalized.Kinds,
			Roles: normalized.Roles, Rules: normalized.Rules, Icons: normalized.Icons,
			Source: normalized.Source, Warnings: normalized.Warnings,
		},
	}
}

// WriteJSON writes the stable theme explanation contract.
func (theme Theme) WriteJSON(writer io.Writer) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(theme.Explanation())
}

// WriteText writes a concise, stable theme explanation.
func (theme Theme) WriteText(writer io.Writer) error {
	if _, err := fmt.Fprintf(writer, "Theme: %s\nDescription: %s\nAppearance: %s\nSource: %s", theme.Name, theme.Description, theme.Appearance, theme.Source.Kind); err != nil {
		return err
	}
	if theme.Source.Path != "" {
		if _, err := fmt.Fprintf(writer, " (%s)", theme.Source.Path); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(writer, "\nCatalog: v%d (%d matchers, %d kinds, %d roles)\n\nPalette:\n", theme.Catalog.Version, theme.Catalog.EntryCount, theme.Catalog.KindCount, theme.Catalog.RoleCount); err != nil {
		return err
	}
	keys := sortedKeys(theme.Palette)
	for _, key := range keys {
		if _, err := fmt.Fprintf(writer, "  %s: %s\n", key, theme.Palette[key]); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(writer, "\nTokens:"); err != nil {
		return err
	}
	for _, key := range sortedKeys(theme.Tokens) {
		token := theme.Tokens[key]
		if _, err := fmt.Fprintf(writer, "  %s: color=%s iconColor=%s styles=%s unicode=%q nerd=%q\n", key, token.Color, displayOptional(token.IconColor, "text"), formatStyles(token.Styles), token.Icons.Unicode, token.Icons.Nerd); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(writer, "\nKind bindings: %d\nRole bindings: %d\nRules: %d\nIcon spacing: %d\n", len(theme.Kinds), len(theme.Roles), len(theme.Rules), theme.Icons.Spacing); err != nil {
		return err
	}
	return nil
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func displayOptional(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func formatStyles(styles []string) string {
	if len(styles) == 0 {
		return "none"
	}
	return strings.Join(styles, ",")
}
