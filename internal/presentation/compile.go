package presentation

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dirloom/dirloom/internal/filter"
	"github.com/dirloom/dirloom/internal/tree"
)

type compiledToken struct {
	color  colorSpec
	styles []string
	icons  IconPair
}

type compiledRule struct {
	document ruleDocument
	matcher  *filter.IgnoreMatcher
	color    *colorSpec
}

// CompiledTheme is an immutable renderer-ready theme.
type CompiledTheme struct {
	theme       Theme
	tokens      map[string]compiledToken
	rules       []compiledRule
	fallback    *CompiledTheme
	iconSpacing int
}

// NodeStyle is the complete presentation selected for one tree node.
type NodeStyle struct {
	color  colorSpec
	styles []string
	icons  IconPair
}

// Compile validates all palette references and creates deterministic matchers.
func Compile(theme Theme) (*CompiledTheme, error) {
	compiled := &CompiledTheme{theme: cloneTheme(theme), tokens: make(map[string]compiledToken), iconSpacing: theme.Icons.Spacing}
	for name, token := range theme.Tokens {
		color, err := resolveColor(token.Color, theme.Palette)
		if err != nil {
			return nil, invalidf("theme %q token %s: %v", theme.Name, name, err)
		}
		compiled.tokens[name] = compiledToken{color: color, styles: append([]string(nil), token.Styles...), icons: token.Icons}
	}
	if theme.Source.Kind == "file" {
		base, _ := Lookup(ThemeDefault)
		base.Palette = make(map[string]string, len(theme.Palette))
		for name, value := range theme.Palette {
			base.Palette[name] = value
		}
		fallback, err := Compile(base)
		if err != nil {
			return nil, err
		}
		compiled.fallback = fallback
		for name, document := range theme.customTokens {
			if _, active := activeTokens[name]; !active {
				continue
			}
			baseToken := compiled.tokens[name]
			if document.Color != nil {
				color, colorErr := resolveColor(*document.Color, theme.Palette)
				if colorErr != nil {
					return nil, colorErr
				}
				baseToken.color = color
			}
			if document.Styles != nil {
				baseToken.styles = append([]string(nil), (*document.Styles)...)
			}
			if document.Icons != nil {
				if document.Icons.Unicode != nil {
					baseToken.icons.Unicode = *document.Icons.Unicode
				}
				if document.Icons.Nerd != nil {
					baseToken.icons.Nerd = *document.Icons.Nerd
				}
			}
			compiled.tokens[name] = baseToken
		}
		for _, rule := range theme.customRules {
			value, err := compileRule(rule, theme.Palette)
			if err != nil {
				return nil, err
			}
			compiled.rules = append(compiled.rules, value)
		}
		return compiled, nil
	}
	for _, rule := range theme.Rules {
		document := ruleDocumentFromPublic(rule)
		value, err := compileRule(document, theme.Palette)
		if err != nil {
			return nil, err
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

func compileRule(document ruleDocument, palette map[string]string) (compiledRule, error) {
	result := compiledRule{document: document}
	if document.Color != nil {
		color, err := resolveColor(*document.Color, palette)
		if err != nil {
			return compiledRule{}, err
		}
		result.color = &color
	}
	if document.Match.Glob != nil {
		matcher, err := filter.NewIgnoreMatcher([]string{*document.Match.Glob})
		if err != nil {
			return compiledRule{}, err
		}
		result.matcher = matcher
	}
	return result, nil
}

func ruleDocumentFromPublic(rule Rule) ruleDocument {
	result := ruleDocument{Match: matchDocumentFromPublic(rule.Match)}
	if rule.Color != "" {
		color := rule.Color
		result.Color = &color
	}
	if len(rule.Styles) > 0 {
		styles := append([]string(nil), rule.Styles...)
		result.Styles = &styles
	}
	if rule.Icons.Unicode != "" || rule.Icons.Nerd != "" {
		icons := iconDocument{}
		if rule.Icons.Unicode != "" {
			value := rule.Icons.Unicode
			icons.Unicode = &value
		}
		if rule.Icons.Nerd != "" {
			value := rule.Icons.Nerd
			icons.Nerd = &value
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

func (theme *CompiledTheme) resolve(pathValue, name string, kind tree.NodeType) NodeStyle {
	semanticRole := "node.file"
	switch kind {
	case tree.NodeDirectory:
		semanticRole = "node.directory"
	case tree.NodeSymlink:
		semanticRole = "node.symlink"
	}
	var result NodeStyle
	if theme.fallback != nil {
		result = theme.fallback.baseStyle(semanticRole)
		if document, customized := theme.theme.customTokens[semanticRole]; customized {
			token := theme.tokens[semanticRole]
			if document.Color != nil {
				result.color = token.color
			}
			if document.Styles != nil {
				result.styles = append([]string(nil), token.styles...)
			}
			if document.Icons != nil {
				if document.Icons.Unicode != nil {
					result.icons.Unicode = token.icons.Unicode
				}
				if document.Icons.Nerd != nil {
					result.icons.Nerd = token.icons.Nerd
				}
			}
		}
	} else if token, ok := theme.tokens[semanticRole]; ok {
		result = NodeStyle{color: token.color, styles: append([]string(nil), token.styles...), icons: token.icons}
	}
	rule := theme.bestRule(pathValue, name, kind)
	var inheritedRule *compiledRule
	if theme.fallback != nil {
		fallbackRule := theme.fallback.bestRule(pathValue, name, kind)
		if rule == nil || fallbackRule != nil && matcherRank(fallbackRule.document.Match) < matcherRank(rule.document.Match) {
			rule = fallbackRule
		} else if rule != nil && fallbackRule != nil && sameMatcher(rule.document.Match, fallbackRule.document.Match) {
			inheritedRule = fallbackRule
		}
	}
	if inheritedRule != nil {
		result = applyCompiledRule(result, inheritedRule)
	}
	if rule != nil {
		result = applyCompiledRule(result, rule)
	}
	return result
}

func applyCompiledRule(result NodeStyle, rule *compiledRule) NodeStyle {
	if rule.color != nil {
		result.color = *rule.color
	}
	if rule.document.Styles != nil {
		result.styles = append([]string(nil), (*rule.document.Styles)...)
	}
	if rule.document.Icons != nil {
		if rule.document.Icons.Unicode != nil {
			result.icons.Unicode = *rule.document.Icons.Unicode
		}
		if rule.document.Icons.Nerd != nil {
			result.icons.Nerd = *rule.document.Icons.Nerd
		}
	}
	return result
}

func sameMatcher(left, right matchDocument) bool {
	switch {
	case left.Path != nil && right.Path != nil:
		return *left.Path == *right.Path
	case left.Name != nil && right.Name != nil:
		return *left.Name == *right.Name
	case left.Glob != nil && right.Glob != nil:
		return *left.Glob == *right.Glob
	case left.Extension != nil && right.Extension != nil:
		return *left.Extension == *right.Extension
	case left.Type != nil && right.Type != nil:
		return *left.Type == *right.Type
	default:
		return false
	}
}

func (theme *CompiledTheme) baseStyle(tokenName string) NodeStyle {
	if token, ok := theme.tokens[tokenName]; ok {
		return NodeStyle{color: token.color, styles: append([]string(nil), token.styles...), icons: token.icons}
	}
	return NodeStyle{color: colorSpec{kind: colorDefault}}
}

func (theme *CompiledTheme) bestRule(pathValue, name string, kind tree.NodeType) *compiledRule {
	for rank := 0; rank < 5; rank++ {
		for index := range theme.rules {
			rule := &theme.rules[index]
			if matcherRank(rule.document.Match) != rank {
				continue
			}
			if ruleMatches(*rule, pathValue, name, kind) {
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
	if theme.fallback != nil {
		result := theme.fallback.edge()
		if document, customized := theme.theme.customTokens["tree.edge"]; customized {
			token := theme.tokens["tree.edge"]
			if document.Color != nil {
				result.color = token.color
			}
			if document.Styles != nil {
				result.styles = append([]string(nil), token.styles...)
			}
		}
		return result
	}
	if token, ok := theme.tokens["tree.edge"]; ok {
		return token
	}
	return compiledToken{color: colorSpec{kind: colorDefault}}
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

// EscapeTerminalText replaces control and bidirectional formatting characters
// in decorated output. Canonical neutral output remains byte-for-byte unchanged.
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
