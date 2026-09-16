package presentation

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dirloom/dirloom/internal/filter"
	"github.com/dirloom/dirloom/internal/presentation/catalog"
	"github.com/dirloom/dirloom/internal/tree"
)

type compiledToken struct {
	color           colorSpec
	iconColor       colorSpec
	iconFollowsText bool
	styles          []string
	icons           IconPair
}

type compiledBinding struct {
	color          *colorSpec
	iconColor      *colorSpec
	iconColorReset bool
	styles         []string
	stylesSet      bool
	unicodeIcon    string
	unicodeIconSet bool
	nerdIcon       string
	nerdIconSet    bool
}

type compiledRule struct {
	document ruleDocument
	matcher  *filter.IgnoreMatcher
	binding  compiledBinding
	kind     *catalog.Kind
	role     *catalog.Role
}

// CompiledTheme is an immutable renderer-ready theme.
type CompiledTheme struct {
	theme       Theme
	tokens      map[string]compiledToken
	kinds       map[catalog.Kind]compiledBinding
	roles       map[catalog.Role]compiledBinding
	rules       []compiledRule
	iconSpacing int
}

// StyleOrigins explains the source of every semantic presentation choice.
type StyleOrigins struct {
	Kind      string `json:"kind"`
	Role      string `json:"role"`
	TextColor string `json:"textColor"`
	IconColor string `json:"iconColor"`
	Icons     string `json:"icons"`
}

// NodeStyle is the complete presentation selected for one tree node.
type NodeStyle struct {
	color           colorSpec
	iconColor       colorSpec
	iconFollowsText bool
	styles          []string
	icons           IconPair
	classification  catalog.Classification
	visualRole      catalog.Role
	origins         StyleOrigins
}

// StyleInspection is the stable, serializable result used by theme classify.
type StyleInspection struct {
	Classification catalog.Classification `json:"classification"`
	VisualRole     catalog.Role           `json:"visualRole"`
	TextColor      string                 `json:"textColor"`
	IconColor      string                 `json:"iconColor"`
	Styles         []string               `json:"styles"`
	Icons          IconPair               `json:"icons"`
	Origins        StyleOrigins           `json:"origins"`
}

// Compile validates all palette references and creates deterministic matchers.
func Compile(theme Theme) (*CompiledTheme, error) {
	if theme.CatalogVersion != catalog.Version {
		return nil, invalidf("theme %q targets catalogVersion %d (expected %d)", theme.Name, theme.CatalogVersion, catalog.Version)
	}
	compiled := &CompiledTheme{
		theme: cloneTheme(theme), tokens: make(map[string]compiledToken),
		kinds: make(map[catalog.Kind]compiledBinding), roles: make(map[catalog.Role]compiledBinding),
		iconSpacing: theme.Icons.Spacing,
	}
	for name, token := range theme.Tokens {
		color, err := resolveColor(token.Color, theme.Palette)
		if err != nil {
			return nil, invalidf("theme %q token %s: %v", theme.Name, name, err)
		}
		iconColor := color
		iconFollowsText := token.IconColor == ""
		if token.IconColor != "" {
			iconColor, err = resolveColor(token.IconColor, theme.Palette)
			if err != nil {
				return nil, invalidf("theme %q token %s iconColor: %v", theme.Name, name, err)
			}
		}
		compiled.tokens[name] = compiledToken{color: color, iconColor: iconColor, iconFollowsText: iconFollowsText, styles: append([]string(nil), token.Styles...), icons: token.Icons}
	}
	for name, value := range theme.Kinds {
		binding, err := compileBinding(value, theme.Palette)
		if err != nil {
			return nil, invalidf("theme %q kind %s: %v", theme.Name, name, err)
		}
		compiled.kinds[catalog.Kind(name)] = binding
	}
	for name, value := range theme.Roles {
		binding, err := compileBinding(value, theme.Palette)
		if err != nil {
			return nil, invalidf("theme %q role %s: %v", theme.Name, name, err)
		}
		compiled.roles[catalog.Role(name)] = binding
	}
	rules := theme.customRules
	if rules == nil && len(theme.Rules) > 0 {
		rules = make([]ruleDocument, 0, len(theme.Rules))
		for _, value := range theme.Rules {
			rules = append(rules, ruleDocumentFromPublic(value))
		}
	}
	for _, document := range rules {
		value, err := compileRule(document, theme.Palette)
		if err != nil {
			return nil, invalidf("theme %q rule: %v", theme.Name, err)
		}
		compiled.rules = append(compiled.rules, value)
	}
	return compiled, nil
}

func resolveColor(value string, palette map[string]string) (colorSpec, error) {
	if literal, ok := palette[value]; ok {
		value = literal
	}
	return parseLiteralColor(value)
}

func compileBinding(value Binding, palette map[string]string) (compiledBinding, error) {
	result := compiledBinding{
		styles: append([]string(nil), value.Styles...), stylesSet: value.stylesSet,
		unicodeIcon: value.Icons.Unicode, unicodeIconSet: value.unicodeIconSet,
		nerdIcon: value.Icons.Nerd, nerdIconSet: value.nerdIconSet,
	}
	if value.colorSet || value.Color != "" {
		color, err := resolveColor(value.Color, palette)
		if err != nil {
			return compiledBinding{}, err
		}
		result.color = &color
	}
	if value.iconColorSet {
		if value.IconColor == "" {
			result.iconColorReset = true
		} else {
			color, err := resolveColor(value.IconColor, palette)
			if err != nil {
				return compiledBinding{}, err
			}
			result.iconColor = &color
		}
	}
	return result, nil
}

func compileRule(document ruleDocument, palette map[string]string) (compiledRule, error) {
	result := compiledRule{document: document}
	if document.Match.Glob != nil {
		matcher, err := filter.NewIgnoreMatcher([]string{*document.Match.Glob})
		if err != nil {
			return compiledRule{}, err
		}
		result.matcher = matcher
	}
	value := Binding{}
	if document.Color != nil {
		value.Color, value.colorSet = *document.Color, true
	}
	if document.IconColor.Present {
		value.iconColorSet = true
		if !document.IconColor.Null {
			value.IconColor = document.IconColor.Value
		}
	}
	if document.Styles != nil {
		value.Styles, value.stylesSet = append([]string(nil), (*document.Styles)...), true
	}
	if document.Icons != nil {
		if document.Icons.Unicode.Present {
			value.unicodeIconSet = true
			if !document.Icons.Unicode.Null {
				value.Icons.Unicode = document.Icons.Unicode.Value
			}
		}
		if document.Icons.Nerd.Present {
			value.nerdIconSet = true
			if !document.Icons.Nerd.Null {
				value.Icons.Nerd = document.Icons.Nerd.Value
			}
		}
	}
	binding, err := compileBinding(value, palette)
	if err != nil {
		return compiledRule{}, err
	}
	result.binding = binding
	if document.Kind != nil {
		kind := catalog.Kind(*document.Kind)
		result.kind = &kind
	}
	if document.Role != nil {
		role := catalog.Role(*document.Role)
		result.role = &role
	}
	return result, nil
}

func ruleDocumentFromPublic(rule Rule) ruleDocument {
	result := ruleDocument{Match: matchDocumentFromPublic(rule.Match)}
	if rule.Kind != "" {
		value := rule.Kind
		result.Kind = &value
	}
	if rule.Role != "" {
		value := rule.Role
		result.Role = &value
	}
	if rule.Color != "" {
		value := rule.Color
		result.Color = &value
	}
	if rule.IconColor != "" {
		result.IconColor = nullableStringDocument{Present: true, Value: rule.IconColor}
	}
	if rule.Styles != nil {
		styles := append([]string(nil), rule.Styles...)
		result.Styles = &styles
	}
	if rule.Icons.Unicode != "" || rule.Icons.Nerd != "" {
		icons := iconDocument{}
		if rule.Icons.Unicode != "" {
			icons.Unicode = nullableStringDocument{Present: true, Value: rule.Icons.Unicode}
		}
		if rule.Icons.Nerd != "" {
			icons.Nerd = nullableStringDocument{Present: true, Value: rule.Icons.Nerd}
		}
		result.Icons = &icons
	}
	return result
}

func matchDocumentFromPublic(match Match) matchDocument {
	result := matchDocument{}
	if match.Path != "" {
		value := match.Path
		result.Path = &value
	}
	if match.Name != "" {
		value := match.Name
		result.Name = &value
	}
	if match.Glob != "" {
		value := match.Glob
		result.Glob = &value
	}
	if match.Extension != "" {
		value := match.Extension
		result.Extension = &value
	}
	if match.Type != "" {
		value := match.Type
		result.Type = &value
	}
	return result
}

func (theme *CompiledTheme) resolve(pathValue, name string, nodeType tree.NodeType) NodeStyle {
	classification := catalog.Classify(name, pathValue, nodeType)
	rule := theme.bestRule(pathValue, name, nodeType)
	effectiveKind := classification.Kind
	kindOrigin := "catalog"
	if rule != nil && rule.kind != nil {
		effectiveKind = *rule.kind
		kindOrigin = "theme-rule"
		classification.Kind = effectiveKind
	}
	visualRole := catalog.RoleGeneric
	for _, role := range classification.Roles {
		if _, ok := theme.roles[role]; ok {
			visualRole = role
			break
		}
	}
	roleOrigin := "catalog"
	if rule != nil && rule.role != nil {
		visualRole = *rule.role
		roleOrigin = "theme-rule"
	}

	tokenName := "node.file"
	switch nodeType {
	case tree.NodeDirectory:
		tokenName = "node.directory"
	case tree.NodeSymlink:
		tokenName = "node.symlink" //nolint:gosec // Public semantic token identifier, not a credential.
	}
	result := theme.baseStyle(tokenName)
	result.classification = classification
	result.visualRole = visualRole
	result.origins.Kind = kindOrigin
	result.origins.Role = roleOrigin
	unicodeGlyph, nerdGlyph := catalog.Glyphs(effectiveKind)
	if unicodeGlyph != "" {
		result.icons.Unicode = unicodeGlyph
	}
	if nerdGlyph != "" {
		result.icons.Nerd = nerdGlyph
	}
	result.origins.Icons = "catalog-kind"

	for _, kind := range catalog.KindChain(effectiveKind) {
		if binding, ok := theme.kinds[kind]; ok {
			result = applyCompiledBinding(result, binding, "theme-kind")
		}
	}
	if binding, ok := theme.roles[visualRole]; ok {
		result = applyCompiledBinding(result, binding, "theme-role")
	}
	if rule != nil {
		result = applyCompiledBinding(result, rule.binding, "theme-rule")
	}
	return result
}

func applyCompiledBinding(result NodeStyle, binding compiledBinding, origin string) NodeStyle {
	if binding.color != nil {
		result.color = *binding.color
		result.origins.TextColor = origin
		if result.iconFollowsText {
			result.iconColor = result.color
			result.origins.IconColor = origin
		}
	}
	if binding.iconColorReset {
		result.iconColor = result.color
		result.iconFollowsText = true
		result.origins.IconColor = origin
	} else if binding.iconColor != nil {
		result.iconColor = *binding.iconColor
		result.iconFollowsText = false
		result.origins.IconColor = origin
	}
	if binding.stylesSet {
		result.styles = append([]string(nil), binding.styles...)
	}
	if binding.unicodeIconSet {
		result.icons.Unicode = binding.unicodeIcon
		result.origins.Icons = origin
	}
	if binding.nerdIconSet {
		result.icons.Nerd = binding.nerdIcon
		result.origins.Icons = origin
	}
	return result
}

func (theme *CompiledTheme) baseStyle(tokenName string) NodeStyle {
	token, ok := theme.tokens[tokenName]
	if !ok {
		return NodeStyle{color: colorSpec{kind: colorDefault}, iconColor: colorSpec{kind: colorDefault}, iconFollowsText: true, styles: []string{}, origins: StyleOrigins{TextColor: "theme-token", IconColor: "theme-token", Icons: "theme-token"}}
	}
	return NodeStyle{
		color: token.color, iconColor: token.iconColor, iconFollowsText: token.iconFollowsText, styles: append([]string(nil), token.styles...), icons: token.icons,
		origins: StyleOrigins{TextColor: "theme-token", IconColor: "theme-token", Icons: "theme-token"},
	}
}

func (theme *CompiledTheme) bestRule(pathValue, name string, kind tree.NodeType) *compiledRule {
	for rank := 0; rank < 5; rank++ {
		for index := range theme.rules {
			rule := &theme.rules[index]
			if matcherRank(rule.document.Match) == rank && ruleMatches(*rule, pathValue, name, kind) {
				return rule
			}
		}
	}
	return nil
}

func matcherRank(match matchDocument) int {
	switch {
	case match.Path != nil:
		return 0
	case match.Name != nil:
		return 1
	case match.Glob != nil:
		return 2
	case match.Extension != nil:
		return 3
	default:
		return 4
	}
}

func ruleMatches(rule compiledRule, pathValue, name string, kind tree.NodeType) bool {
	match := rule.document.Match
	switch {
	case match.Path != nil:
		return pathValue == *match.Path
	case match.Name != nil:
		return name == *match.Name
	case match.Glob != nil:
		return rule.matcher.Match(pathValue, kind == tree.NodeDirectory)
	case match.Extension != nil:
		return filepath.Ext(name) == *match.Extension
	case match.Type != nil:
		return string(kind) == *match.Type
	default:
		return false
	}
}

func (theme *CompiledTheme) edge() compiledToken {
	if token, ok := theme.tokens["tree.edge"]; ok {
		return token
	}
	return compiledToken{color: colorSpec{kind: colorDefault}, iconColor: colorSpec{kind: colorDefault}, styles: []string{}}
}

// Inspect resolves one entry without decorating or touching the filesystem.
func (theme *CompiledTheme) Inspect(pathValue, name string, nodeType tree.NodeType) StyleInspection {
	style := theme.resolve(pathValue, name, nodeType)
	return StyleInspection{
		Classification: style.classification, VisualRole: style.visualRole,
		TextColor: formatColor(style.color), IconColor: formatColor(style.iconColor),
		Styles: append([]string(nil), style.styles...), Icons: style.icons, Origins: style.origins,
	}
}

func formatColor(value colorSpec) string {
	switch value.kind {
	case colorDefault:
		return "default"
	case colorANSI:
		for name, index := range ansiNames {
			if index == value.ansiIndex {
				return "ansi:" + name
			}
		}
	}
	return fmt.Sprintf("#%02X%02X%02X", value.r, value.g, value.b)
}

func validateGlyph(value string) error {
	if value == "" {
		return fmt.Errorf("must not be empty")
	}
	if len(value) > 64 {
		return fmt.Errorf("exceeds the 64-byte limit")
	}
	if !utf8.ValidString(value) {
		return fmt.Errorf("must contain valid UTF-8")
	}
	count := 0
	for _, char := range value {
		count++
		if unicode.IsControl(char) || char == '\x1b' || isBidiControl(char) {
			return fmt.Errorf("contains a forbidden control character U+%04X", char)
		}
	}
	if count > 4 {
		return fmt.Errorf("exceeds the 4-rune limit")
	}
	return nil
}

func isBidiControl(char rune) bool {
	return char == '\u061c' || char == '\u200e' || char == '\u200f' ||
		char >= '\u202a' && char <= '\u202e' || char >= '\u2066' && char <= '\u2069'
}

// EscapeTerminalText replaces controls in decorated output only.
func EscapeTerminalText(value string) string {
	var builder strings.Builder
	for _, char := range value {
		if unicode.IsControl(char) || char == '\x1b' || isBidiControl(char) {
			_, _ = fmt.Fprintf(&builder, "\\u{%04X}", char)
			continue
		}
		builder.WriteRune(char)
	}
	return builder.String()
}
