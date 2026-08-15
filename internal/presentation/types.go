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
)

const (
	SchemaVersion = 1

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

	AppearanceUniversal = "universal"
	AppearanceLight     = "light"
	AppearanceDark      = "dark"
)

var activeTokens = map[string]struct{}{
	"tree.edge":      {},
	"node.directory": {},
	"node.file":      {},
	"node.symlink":   {},
}

// IconPair defines portable and Nerd Font glyphs for one visual token.
type IconPair struct {
	Unicode string `json:"unicode,omitempty" yaml:"unicode"`
	Nerd    string `json:"nerd,omitempty" yaml:"nerd"`
}

// Token defines the presentation of one base tree segment.
type Token struct {
	Color  string   `json:"color,omitempty" yaml:"color"`
	Styles []string `json:"styles" yaml:"styles"`
	Icons  IconPair `json:"icons" yaml:"icons"`
}

// Match identifies exactly one rule matcher.
type Match struct {
	Path      string `json:"path,omitempty" yaml:"path"`
	Name      string `json:"name,omitempty" yaml:"name"`
	Glob      string `json:"glob,omitempty" yaml:"glob"`
	Extension string `json:"extension,omitempty" yaml:"extension"`
	Type      string `json:"type,omitempty" yaml:"type"`
}

// Rule applies presentation overrides to matching nodes.
type Rule struct {
	Match  Match    `json:"match" yaml:"match"`
	Color  string   `json:"color,omitempty" yaml:"color"`
	Styles []string `json:"styles" yaml:"styles"`
	Icons  IconPair `json:"icons" yaml:"icons"`
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

// Theme is the normalized, inspectable public theme definition.
type Theme struct {
	SchemaVersion int               `json:"schemaVersion"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Appearance    string            `json:"appearance"`
	Palette       map[string]string `json:"palette"`
	Tokens        map[string]Token  `json:"tokens"`
	Rules         []Rule            `json:"rules"`
	Icons         IconSettings      `json:"icons"`
	Source        Source            `json:"source"`
	Warnings      []Warning         `json:"warnings"`
	customTokens  map[string]tokenDocument
	customRules   []ruleDocument
}

// ValidationResult is emitted by theme validate.
type ValidationResult struct {
	SchemaVersion int       `json:"schemaVersion"`
	Valid         bool      `json:"valid"`
	Source        Source    `json:"source"`
	Name          string    `json:"name,omitempty"`
	Warnings      []Warning `json:"warnings"`
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
	names := []string{ThemeDefault, ThemeMidnight, ThemeDaylight}
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
	return strings.ContainsAny(value, `/\\`) || strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml")
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
	result.Palette = make(map[string]string, len(theme.Palette))
	for key, value := range theme.Palette {
		result.Palette[key] = value
	}
	result.Tokens = make(map[string]Token, len(theme.Tokens))
	for key, value := range theme.Tokens {
		value.Styles = append([]string(nil), value.Styles...)
		if value.Styles == nil {
			value.Styles = []string{}
		}
		result.Tokens[key] = value
	}
	result.Rules = cloneRules(theme.Rules)
	result.Warnings = append([]Warning(nil), theme.Warnings...)
	result.customTokens = cloneTokenDocuments(theme.customTokens)
	result.customRules = cloneRuleDocuments(theme.customRules)
	if result.Rules == nil {
		result.Rules = []Rule{}
	}
	if result.Warnings == nil {
		result.Warnings = []Warning{}
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

// WriteJSON writes a normalized theme definition.
func (theme Theme) WriteJSON(writer io.Writer) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(cloneTheme(theme))
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
	if _, err := fmt.Fprintln(writer, "\n\nPalette:"); err != nil {
		return err
	}
	keys := make([]string, 0, len(theme.Palette))
	for key := range theme.Palette {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, err := fmt.Fprintf(writer, "  %s: %s\n", key, theme.Palette[key]); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(writer, "\nTokens:"); err != nil {
		return err
	}
	keys = keys[:0]
	for key := range theme.Tokens {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		token := theme.Tokens[key]
		if _, err := fmt.Fprintf(writer, "  %s: color=%s styles=%s unicode=%q nerd=%q\n", key, token.Color, formatStyles(token.Styles), token.Icons.Unicode, token.Icons.Nerd); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(writer, "\nRules: %d\nIcon spacing: %d\n", len(theme.Rules), theme.Icons.Spacing); err != nil {
		return err
	}
	return nil
}

func formatStyles(styles []string) string {
	if len(styles) == 0 {
		return "none"
	}
	return strings.Join(styles, ",")
}
