package presentation

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/dirloom/dirloom/internal/filter"
	"go.yaml.in/yaml/v3"
)

const (
	maxThemeSize = 1 << 20
	maxPalette   = 64
	maxRules     = 512
)

type iconDocument struct {
	Unicode *string `yaml:"unicode"`
	Nerd    *string `yaml:"nerd"`
}

type tokenDocument struct {
	Color  *string       `yaml:"color"`
	Styles *[]string     `yaml:"styles"`
	Icons  *iconDocument `yaml:"icons"`
}

type matchDocument struct {
	Path      *string `yaml:"path"`
	Name      *string `yaml:"name"`
	Glob      *string `yaml:"glob"`
	Extension *string `yaml:"extension"`
	Type      *string `yaml:"type"`
}

type ruleDocument struct {
	Match  matchDocument `yaml:"match"`
	Color  *string       `yaml:"color"`
	Styles *[]string     `yaml:"styles"`
	Icons  *iconDocument `yaml:"icons"`
}

type iconsDocument struct {
	Spacing *int `yaml:"spacing"`
}

type themeDocument struct {
	SchemaVersion *int                     `yaml:"schemaVersion"`
	Name          *string                  `yaml:"name"`
	Description   string                   `yaml:"description"`
	Appearance    *string                  `yaml:"appearance"`
	Palette       map[string]string        `yaml:"palette"`
	Tokens        map[string]tokenDocument `yaml:"tokens"`
	Rules         []ruleDocument           `yaml:"rules"`
	Icons         iconsDocument            `yaml:"icons"`
}

func cloneTokenDocuments(values map[string]tokenDocument) map[string]tokenDocument {
	if values == nil {
		return nil
	}
	result := make(map[string]tokenDocument, len(values))
	for key, value := range values {
		copyValue := value
		if value.Styles != nil {
			styles := append([]string(nil), (*value.Styles)...)
			copyValue.Styles = &styles
		}
		if value.Icons != nil {
			icons := *value.Icons
			copyValue.Icons = &icons
		}
		result[key] = copyValue
	}
	return result
}

func cloneRuleDocuments(values []ruleDocument) []ruleDocument {
	if values == nil {
		return nil
	}
	result := make([]ruleDocument, len(values))
	for index, value := range values {
		result[index] = value
		if value.Styles != nil {
			styles := append([]string(nil), (*value.Styles)...)
			result[index].Styles = &styles
		}
		if value.Icons != nil {
			icons := *value.Icons
			result[index].Icons = &icons
		}
	}
	return result
}

// ReferenceContext supplies the authority used to resolve a selected theme.
type ReferenceContext struct {
	// Kind is "cli", "user", "project", or "built-in".
	Kind string
	// ConfigPath is required for a path selected by a configuration file.
	ConfigPath string
	// WorkingDirectory resolves relative CLI paths. Empty uses os.Getwd.
	WorkingDirectory string
}

// LoadReference resolves and validates a built-in name or local YAML path.
func LoadReference(reference string, context ReferenceContext) (Theme, error) {
	if reference == "" {
		return Theme{}, invalidf("theme reference must not be empty")
	}
	if theme, ok := Lookup(reference); ok {
		return theme, nil
	}
	if !IsThemePath(reference) {
		return Theme{}, invalidf("unsupported theme %q (expected %s or a .yaml/.yml path)", reference, strings.Join(ThemeNames(), ", "))
	}
	path, err := resolveThemePath(reference, context)
	if err != nil {
		return Theme{}, err
	}
	return loadFile(path)
}

func resolveThemePath(reference string, context ReferenceContext) (string, error) {
	if context.Kind != "cli" && filepath.IsAbs(reference) {
		return "", invalidf("theme path %q must be relative to its configuration file", reference)
	}
	base := context.WorkingDirectory
	if context.Kind != "cli" {
		if context.ConfigPath == "" {
			return "", invalidf("cannot resolve theme path %q without a configuration source", reference)
		}
		base = filepath.Dir(context.ConfigPath)
	}
	if base == "" {
		var err error
		base, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolve current directory for theme %q: %w", reference, err)
		}
	}
	candidate := reference
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(base, candidate)
	}
	candidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", invalidf("resolve theme path %q: %v", reference, err)
	}
	candidate = filepath.Clean(candidate)
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", invalidf("theme file %q does not exist", candidate)
		}
		return "", fmt.Errorf("resolve theme file %q: %w", candidate, err)
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("resolve theme file %q: %w", candidate, err)
	}
	if context.Kind != "cli" {
		resolvedBase, baseErr := filepath.EvalSymlinks(base)
		if baseErr != nil {
			return "", fmt.Errorf("resolve configuration directory %q: %w", base, baseErr)
		}
		resolvedBase, baseErr = filepath.Abs(resolvedBase)
		if baseErr != nil {
			return "", fmt.Errorf("resolve configuration directory %q: %w", base, baseErr)
		}
		relative, relErr := filepath.Rel(resolvedBase, resolved)
		if relErr != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
			return "", invalidf("theme path %q resolves outside its configuration directory", reference)
		}
	}
	return filepath.Clean(resolved), nil
}

func loadFile(path string) (Theme, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Theme{}, invalidf("theme file %q does not exist", path)
		}
		return Theme{}, fmt.Errorf("inspect theme %q: %w", path, err)
	}
	if info.IsDir() {
		return Theme{}, invalidf("theme path %q is a directory", path)
	}
	if info.Size() > maxThemeSize {
		return Theme{}, invalidf("invalid theme %q: file exceeds the 1 MiB limit", path)
	}
	// #nosec G304 -- path is explicitly selected and confined above when config-backed.
	data, err := os.ReadFile(path)
	if err != nil {
		return Theme{}, fmt.Errorf("read theme %q: %w", path, err)
	}
	if len(data) > maxThemeSize {
		return Theme{}, invalidf("invalid theme %q: file exceeds the 1 MiB limit", path)
	}
	return parseTheme(data, path)
}

func parseTheme(data []byte, path string) (Theme, error) {
	if !bytes.Equal(bytes.ToValidUTF8(data, []byte("\uFFFD")), data) {
		return Theme{}, invalidf("invalid theme %q: file must contain valid UTF-8", path)
	}
	var syntax yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&syntax); err != nil {
		if errors.Is(err, io.EOF) {
			return Theme{}, invalidf("invalid theme %q: file is empty", path)
		}
		return Theme{}, invalidf("invalid theme %q: %v", path, err)
	}
	if err := rejectUnsupportedYAML(&syntax); err != nil {
		return Theme{}, invalidf("invalid theme %q: %v", path, err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return Theme{}, invalidf("invalid theme %q: %v", path, err)
		}
		return Theme{}, invalidf("invalid theme %q: multiple YAML documents are not supported", path)
	}
	var document themeDocument
	strict := yaml.NewDecoder(bytes.NewReader(data))
	strict.KnownFields(true)
	if err := strict.Decode(&document); err != nil {
		return Theme{}, invalidf("invalid theme %q: %v", path, err)
	}
	if document.SchemaVersion == nil {
		return Theme{}, invalidf("invalid theme %q: schemaVersion is required", path)
	}
	if *document.SchemaVersion != SchemaVersion {
		return Theme{}, invalidf("invalid theme %q: unsupported schemaVersion %d (expected %d)", path, *document.SchemaVersion, SchemaVersion)
	}
	if document.Name == nil || strings.TrimSpace(*document.Name) == "" {
		return Theme{}, invalidf("invalid theme %q: name is required and must not be empty", path)
	}
	if document.Appearance == nil {
		return Theme{}, invalidf("invalid theme %q: appearance is required", path)
	}
	switch *document.Appearance {
	case AppearanceUniversal, AppearanceLight, AppearanceDark:
	default:
		return Theme{}, invalidf("invalid theme %q: appearance must be universal, light, or dark", path)
	}
	if len(document.Palette) > maxPalette {
		return Theme{}, invalidf("invalid theme %q: palette exceeds the %d-color limit", path, maxPalette)
	}
	for name, value := range document.Palette {
		if strings.TrimSpace(name) == "" {
			return Theme{}, invalidf("invalid theme %q: palette names must not be empty", path)
		}
		if _, err := parseLiteralColor(value); err != nil {
			return Theme{}, invalidf("invalid theme %q: palette.%s: %v", path, name, err)
		}
	}
	baseTheme, _ := Lookup(ThemeDefault)
	validationPalette := make(map[string]string, len(baseTheme.Palette)+len(document.Palette))
	for name, value := range baseTheme.Palette {
		validationPalette[name] = value
	}
	for name, value := range document.Palette {
		validationPalette[name] = value
	}
	if len(document.Rules) > maxRules {
		return Theme{}, invalidf("invalid theme %q: rules exceed the %d-rule limit", path, maxRules)
	}
	warnings := make([]Warning, 0)
	for name, token := range document.Tokens {
		if _, active := activeTokens[name]; !active {
			warnings = append(warnings, Warning{Code: "unknown-token", Message: fmt.Sprintf("token %q is not active in theme schema v1 and was ignored", name)})
			continue
		}
		if err := validateTokenDocument(token, validationPalette); err != nil {
			return Theme{}, invalidf("invalid theme %q: token %s: %v", path, name, err)
		}
	}
	seenMatchers := make(map[string]struct{}, len(document.Rules))
	for index, rule := range document.Rules {
		identity, err := validateRuleDocument(rule, validationPalette)
		if err != nil {
			return Theme{}, invalidf("invalid theme %q: rule %d: %v", path, index+1, err)
		}
		if _, exists := seenMatchers[identity]; exists {
			return Theme{}, invalidf("invalid theme %q: rule %d duplicates matcher %s", path, index+1, identity)
		}
		seenMatchers[identity] = struct{}{}
	}
	spacing := 1
	if document.Icons.Spacing != nil {
		spacing = *document.Icons.Spacing
	}
	if spacing < 0 || spacing > 4 {
		return Theme{}, invalidf("invalid theme %q: icons.spacing must be between 0 and 4", path)
	}
	theme := baseTheme
	theme.Name = *document.Name
	theme.Description = document.Description
	theme.Appearance = *document.Appearance
	theme.Source = Source{Kind: "file", Path: path}
	theme.Warnings = warnings
	theme.Icons.Spacing = spacing
	for key, value := range document.Palette {
		theme.Palette[key] = value
	}
	for key, value := range document.Tokens {
		if _, active := activeTokens[key]; !active {
			continue
		}
		token := theme.Tokens[key]
		theme.Tokens[key] = mergeToken(token, value)
	}
	customRules := make([]Rule, 0, len(document.Rules))
	for _, value := range document.Rules {
		customRules = append(customRules, publicRule(value))
	}
	theme.Rules = append(customRules, theme.Rules...)
	theme.customTokens = cloneTokenDocuments(document.Tokens)
	theme.customRules = cloneRuleDocuments(document.Rules)
	return cloneTheme(theme), nil
}

func validateTokenDocument(document tokenDocument, palette map[string]string) error {
	if document.Color != nil {
		if err := validateColorReference(*document.Color, palette); err != nil {
			return err
		}
	}
	if document.Styles != nil {
		if err := validateStyles(*document.Styles); err != nil {
			return err
		}
	}
	return validateIconDocument(document.Icons)
}

func validateRuleDocument(document ruleDocument, palette map[string]string) (string, error) {
	identity, err := validateMatch(document.Match)
	if err != nil {
		return "", err
	}
	if document.Color != nil {
		if err := validateColorReference(*document.Color, palette); err != nil {
			return "", err
		}
	}
	if document.Styles != nil {
		if err := validateStyles(*document.Styles); err != nil {
			return "", err
		}
	}
	if err := validateIconDocument(document.Icons); err != nil {
		return "", err
	}
	return identity, nil
}

func validateColorReference(value string, palette map[string]string) error {
	if _, ok := palette[value]; ok {
		return nil
	}
	_, err := parseLiteralColor(value)
	return err
}

func validateStyles(styles []string) error {
	seen := make(map[string]struct{}, len(styles))
	for _, style := range styles {
		switch style {
		case "bold", "dim", "italic", "underline":
		default:
			return fmt.Errorf("unsupported style %q (expected bold, dim, italic, or underline)", style)
		}
		if _, exists := seen[style]; exists {
			return fmt.Errorf("duplicate style %q", style)
		}
		seen[style] = struct{}{}
	}
	return nil
}

func validateIconDocument(document *iconDocument) error {
	if document == nil {
		return nil
	}
	if document.Unicode != nil {
		if err := validateGlyph(*document.Unicode); err != nil {
			return fmt.Errorf("unicode icon: %w", err)
		}
	}
	if document.Nerd != nil {
		if err := validateGlyph(*document.Nerd); err != nil {
			return fmt.Errorf("nerd icon: %w", err)
		}
	}
	return nil
}

func validateMatch(match matchDocument) (string, error) {
	values := []struct {
		kind  string
		value *string
	}{{"path", match.Path}, {"name", match.Name}, {"glob", match.Glob}, {"extension", match.Extension}, {"type", match.Type}}
	var selected *struct {
		kind  string
		value *string
	}
	for index := range values {
		if values[index].value == nil {
			continue
		}
		if selected != nil {
			return "", fmt.Errorf("match must define exactly one of path, name, glob, extension, or type")
		}
		selected = &values[index]
	}
	if selected == nil || *selected.value == "" {
		return "", fmt.Errorf("match must define exactly one non-empty matcher")
	}
	value := *selected.value
	switch selected.kind {
	case "path":
		if err := validateRelativeSlashPath(value); err != nil {
			return "", fmt.Errorf("path: %w", err)
		}
	case "name":
		if strings.ContainsAny(value, `/\\`) {
			return "", fmt.Errorf("name %q must not contain a path separator", value)
		}
	case "glob":
		if _, err := filter.NewIgnoreMatcher([]string{value}); err != nil {
			return "", fmt.Errorf("glob: %w", err)
		}
	case "extension":
		if !strings.HasPrefix(value, ".") || len(value) == 1 || strings.ContainsAny(value, `/\\`) {
			return "", fmt.Errorf("extension %q must start with '.' and contain no path separator", value)
		}
	case "type":
		if value != "directory" && value != "file" && value != "symlink" {
			return "", fmt.Errorf("type must be directory, file, or symlink")
		}
	}
	return selected.kind + ":" + value, nil
}

func validateRelativeSlashPath(value string) error {
	if filepath.IsAbs(value) || strings.Contains(value, `\`) || strings.HasPrefix(value, "/") {
		return fmt.Errorf("%q must be relative and use '/' separators", value)
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("%q contains an invalid path segment", value)
		}
	}
	return nil
}

func mergeToken(base Token, document tokenDocument) Token {
	if document.Color != nil {
		base.Color = *document.Color
	}
	if document.Styles != nil {
		base.Styles = append([]string(nil), (*document.Styles)...)
	}
	if document.Icons != nil {
		if document.Icons.Unicode != nil {
			base.Icons.Unicode = *document.Icons.Unicode
		}
		if document.Icons.Nerd != nil {
			base.Icons.Nerd = *document.Icons.Nerd
		}
	}
	return base
}

func publicRule(document ruleDocument) Rule {
	result := Rule{Match: publicMatch(document.Match), Styles: []string{}}
	if document.Color != nil {
		result.Color = *document.Color
	}
	if document.Styles != nil {
		result.Styles = append([]string(nil), (*document.Styles)...)
	}
	if document.Icons != nil {
		if document.Icons.Unicode != nil {
			result.Icons.Unicode = *document.Icons.Unicode
		}
		if document.Icons.Nerd != nil {
			result.Icons.Nerd = *document.Icons.Nerd
		}
	}
	return result
}

func publicMatch(document matchDocument) Match {
	result := Match{}
	if document.Path != nil {
		result.Path = *document.Path
	}
	if document.Name != nil {
		result.Name = *document.Name
	}
	if document.Glob != nil {
		result.Glob = *document.Glob
	}
	if document.Extension != nil {
		result.Extension = *document.Extension
	}
	if document.Type != nil {
		result.Type = *document.Type
	}
	return result
}

func rejectUnsupportedYAML(root *yaml.Node) error {
	seen := make(map[*yaml.Node]struct{})
	var walk func(*yaml.Node) error
	walk = func(node *yaml.Node) error {
		if node == nil {
			return nil
		}
		if _, ok := seen[node]; ok {
			return nil
		}
		seen[node] = struct{}{}
		if node.Anchor != "" || node.Kind == yaml.AliasNode || node.Alias != nil {
			return fmt.Errorf("line %d, column %d: YAML anchors and aliases are not supported", node.Line, node.Column)
		}
		if node.Tag != "" && !isStandardTag(node.Tag) {
			return fmt.Errorf("line %d, column %d: custom YAML tag %q is not supported", node.Line, node.Column, node.Tag)
		}
		if node.Kind == yaml.MappingNode {
			keys := make(map[string]struct{}, len(node.Content)/2)
			for index := 0; index+1 < len(node.Content); index += 2 {
				key := node.Content[index]
				if key.Value == "<<" || key.Tag == "!!merge" {
					return fmt.Errorf("line %d, column %d: YAML merge keys are not supported", key.Line, key.Column)
				}
				identity := key.Tag + "\x00" + key.Value
				if _, exists := keys[identity]; exists {
					return fmt.Errorf("line %d, column %d: duplicate key %q", key.Line, key.Column, key.Value)
				}
				keys[identity] = struct{}{}
			}
		}
		for _, child := range node.Content {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(root)
}

func isStandardTag(tag string) bool {
	const prefix = "tag:yaml.org,2002:"
	if strings.HasPrefix(tag, prefix) {
		return true
	}
	switch tag {
	case "!!map", "!!seq", "!!str", "!!int", "!!bool", "!!null", "!!float", "!!timestamp", "!!binary", "!!merge":
		return true
	default:
		return false
	}
}
