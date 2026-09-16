package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/dirloom/dirloom/internal/filter"
	"github.com/dirloom/dirloom/internal/outputformat"
	"github.com/dirloom/dirloom/internal/presentation"
	"go.yaml.in/yaml/v3"
)

type fileDocument struct {
	SchemaVersion *int             `yaml:"schemaVersion"`
	Preset        yaml.Node        `yaml:"preset"`
	Defaults      fileDefaults     `yaml:"defaults"`
	Filters       fileFilters      `yaml:"filters"`
	Ignore        yaml.Node        `yaml:"ignore"`
	Presentation  filePresentation `yaml:"presentation"`
	Diagram       fileDiagram      `yaml:"diagram"`
}

type filePresentation struct {
	Color yaml.Node `yaml:"color"`
	Icons yaml.Node `yaml:"icons"`
	Theme yaml.Node `yaml:"theme"`
}

type fileDefaults struct {
	Depth           yaml.Node `yaml:"depth"`
	DirectoriesOnly *bool     `yaml:"dirsOnly"`
	IncludeHidden   *bool     `yaml:"hidden"`
	Format          yaml.Node `yaml:"format"`
	Style           yaml.Node `yaml:"style"`
}

type fileFilters struct {
	UseDefaultIgnores *bool `yaml:"useDefaultIgnores"`
	UseGitIgnore      *bool `yaml:"useGitignore"`
}

type fileDiagram struct {
	View      yaml.Node `yaml:"view"`
	Direction yaml.Node `yaml:"direction"`
	MaxNodes  yaml.Node `yaml:"maxNodes"`
}

type partial struct {
	Preset            PresetSelection
	Depth             DepthOverride
	DirectoriesOnly   Optional[bool]
	IncludeHidden     Optional[bool]
	Format            Optional[string]
	Style             Optional[string]
	UseDefaultIgnores Optional[bool]
	UseGitIgnore      Optional[bool]
	IgnorePatterns    []string
	Color             Optional[string]
	Icons             Optional[string]
	Theme             ThemeSelection
	DiagramView       Optional[string]
	DiagramDirection  Optional[string]
	DiagramMaxNodes   LimitOverride
}

func parseDocument(data []byte, path string) (partial, error) {
	var syntax yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&syntax); err != nil {
		if errors.Is(err, io.EOF) {
			return partial{}, invalidf("invalid config %q: file is empty", path)
		}
		return partial{}, invalidf("invalid config %q: %v", path, err)
	}
	if err := rejectUnsupportedYAML(&syntax); err != nil {
		return partial{}, invalidf("invalid config %q: %v", path, err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return partial{}, invalidf("invalid config %q: %v", path, err)
		}
		return partial{}, invalidf("invalid config %q: multiple YAML documents are not supported", path)
	}

	var document fileDocument
	strict := yaml.NewDecoder(bytes.NewReader(data))
	strict.KnownFields(true)
	if err := strict.Decode(&document); err != nil {
		return partial{}, invalidf("invalid config %q: %v", path, err)
	}
	if document.SchemaVersion == nil {
		return partial{}, invalidf("invalid config %q: schemaVersion is required", path)
	}
	if *document.SchemaVersion != SchemaVersion {
		return partial{}, invalidf("invalid config %q: unsupported schemaVersion %d (expected %d)", path, *document.SchemaVersion, SchemaVersion)
	}

	format, err := parseEnumNode(document.Defaults.Format, path, "defaults.format", outputformat.AcceptedNames())
	if err != nil {
		return partial{}, err
	}
	if format.Set {
		format.Value, _ = outputformat.Canonical(format.Value)
	}
	style, err := parseEnumNode(document.Defaults.Style, path, "defaults.style", []string{StyleUnicode, StyleASCII})
	if err != nil {
		return partial{}, err
	}
	ignore, err := parseIgnoreNode(document.Ignore, path)
	if err != nil {
		return partial{}, err
	}
	color, err := parseEnumNode(document.Presentation.Color, path, "presentation.color", presentation.ColorModes())
	if err != nil {
		return partial{}, err
	}
	icons, err := parseEnumNode(document.Presentation.Icons, path, "presentation.icons", presentation.IconModes())
	if err != nil {
		return partial{}, err
	}
	theme, err := parseThemeNode(document.Presentation.Theme, path)
	if err != nil {
		return partial{}, err
	}
	diagramView, err := parseEnumNode(document.Diagram.View, path, "diagram.view", []string{DiagramViewStructure})
	if err != nil {
		return partial{}, err
	}
	diagramDirection, err := parseEnumNode(document.Diagram.Direction, path, "diagram.direction", []string{DiagramDirectionTopDown, DiagramDirectionLeftRight})
	if err != nil {
		return partial{}, err
	}
	diagramMaxNodes, err := parsePositiveLimitNode(document.Diagram.MaxNodes, path, "diagram.maxNodes")
	if err != nil {
		return partial{}, err
	}
	result := partial{
		Preset:            PresetSelection{},
		DirectoriesOnly:   optionalBool(document.Defaults.DirectoriesOnly),
		IncludeHidden:     optionalBool(document.Defaults.IncludeHidden),
		Format:            format,
		Style:             style,
		UseDefaultIgnores: optionalBool(document.Filters.UseDefaultIgnores),
		UseGitIgnore:      optionalBool(document.Filters.UseGitIgnore),
		IgnorePatterns:    ignore,
		Color:             color,
		Icons:             icons,
		Theme:             theme,
		DiagramView:       diagramView,
		DiagramDirection:  diagramDirection,
		DiagramMaxNodes:   diagramMaxNodes,
	}
	preset, err := parsePresetNode(document.Preset, path)
	if err != nil {
		return partial{}, err
	}
	result.Preset = preset
	depth, err := parseDepthNode(document.Defaults.Depth, path)
	if err != nil {
		return partial{}, err
	}
	result.Depth = depth
	return result, nil
}

func parseThemeNode(node yaml.Node, path string) (ThemeSelection, error) {
	if node.Kind == 0 {
		return ThemeSelection{}, nil
	}
	if node.Kind != yaml.ScalarNode {
		return ThemeSelection{}, invalidf("invalid config %q: line %d, column %d: presentation.theme must be a built-in name, relative YAML path, or null", path, node.Line, node.Column)
	}
	if node.ShortTag() == "!!null" {
		return ThemeSelection{Set: true, Reset: true}, nil
	}
	if node.ShortTag() != "!!str" || node.Value == "" {
		return ThemeSelection{}, invalidf("invalid config %q: line %d, column %d: presentation.theme must be a non-empty built-in name, relative YAML path, or null", path, node.Line, node.Column)
	}
	if presentation.IsBuiltIn(node.Value) {
		return ThemeSelection{Set: true, Value: node.Value}, nil
	}
	if !presentation.IsThemePath(node.Value) {
		return ThemeSelection{}, invalidf("invalid config %q: line %d, column %d: unsupported theme %q (expected %s or a relative .yaml/.yml path)", path, node.Line, node.Column, node.Value, joinExpected(presentation.ThemeNames()))
	}
	if filepath.IsAbs(node.Value) {
		return ThemeSelection{}, invalidf("invalid config %q: line %d, column %d: presentation.theme path must be relative to the configuration file", path, node.Line, node.Column)
	}
	cleaned := filepath.Clean(node.Value)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return ThemeSelection{}, invalidf("invalid config %q: line %d, column %d: presentation.theme path must remain inside the configuration directory", path, node.Line, node.Column)
	}
	return ThemeSelection{Set: true, Value: node.Value}, nil
}

func parsePresetNode(node yaml.Node, path string) (PresetSelection, error) {
	if node.Kind == 0 {
		return PresetSelection{}, nil
	}
	if node.Kind != yaml.ScalarNode {
		return PresetSelection{}, invalidf("invalid config %q: line %d, column %d: preset must be a built-in preset name or null", path, node.Line, node.Column)
	}
	if node.ShortTag() == "!!null" {
		return PresetSelection{Set: true, Disabled: true}, nil
	}
	if node.ShortTag() != "!!str" {
		return PresetSelection{}, invalidf("invalid config %q: line %d, column %d: preset must be a built-in preset name or null", path, node.Line, node.Column)
	}
	if _, ok := LookupPreset(node.Value); !ok {
		allowed := append(PresetNames(), "null")
		return PresetSelection{}, invalidf("invalid config %q: line %d, column %d: unsupported preset %q (expected %s)", path, node.Line, node.Column, node.Value, joinExpected(allowed))
	}
	return PresetSelection{Set: true, Name: node.Value}, nil
}

func optionalBool(value *bool) Optional[bool] {
	if value == nil {
		return Optional[bool]{}
	}
	return Optional[bool]{Set: true, Value: *value}
}

func parseDepthNode(node yaml.Node, path string) (DepthOverride, error) {
	if node.Kind == 0 {
		return DepthOverride{}, nil
	}
	if node.Kind != yaml.ScalarNode {
		return DepthOverride{}, invalidf("invalid config %q: line %d, column %d: defaults.depth must be a non-negative integer or null", path, node.Line, node.Column)
	}
	if node.ShortTag() == "!!null" {
		return DepthOverride{Set: true, Unlimited: true}, nil
	}
	if node.ShortTag() != "!!int" {
		return DepthOverride{}, invalidf("invalid config %q: line %d, column %d: defaults.depth must be a non-negative integer or null", path, node.Line, node.Column)
	}
	var value int
	if err := node.Decode(&value); err != nil || value < 0 {
		return DepthOverride{}, invalidf("invalid config %q: line %d, column %d: defaults.depth must be a non-negative integer or null", path, node.Line, node.Column)
	}
	return DepthOverride{Set: true, Value: value}, nil
}

func parsePositiveLimitNode(node yaml.Node, path, field string) (LimitOverride, error) {
	if node.Kind == 0 {
		return LimitOverride{}, nil
	}
	if node.Kind != yaml.ScalarNode {
		return LimitOverride{}, invalidf("invalid config %q: line %d, column %d: %s must be a positive integer or null", path, node.Line, node.Column, field)
	}
	if node.ShortTag() == "!!null" {
		return LimitOverride{Set: true, Unlimited: true}, nil
	}
	if node.ShortTag() != "!!int" {
		return LimitOverride{}, invalidf("invalid config %q: line %d, column %d: %s must be a positive integer or null", path, node.Line, node.Column, field)
	}
	var value int
	if err := node.Decode(&value); err != nil || value <= 0 {
		return LimitOverride{}, invalidf("invalid config %q: line %d, column %d: %s must be a positive integer or null", path, node.Line, node.Column, field)
	}
	return LimitOverride{Set: true, Value: value}, nil
}

func parseEnumNode(node yaml.Node, path, field string, allowed []string) (Optional[string], error) {
	if node.Kind == 0 {
		return Optional[string]{}, nil
	}
	if node.Kind != yaml.ScalarNode || node.ShortTag() != "!!str" {
		return Optional[string]{}, invalidf("invalid config %q: line %d, column %d: %s must be a string", path, node.Line, node.Column, field)
	}
	for _, value := range allowed {
		if node.Value == value {
			return Optional[string]{Set: true, Value: node.Value}, nil
		}
	}
	return Optional[string]{}, invalidf("invalid config %q: line %d, column %d: unsupported %s %q (expected %s)", path, node.Line, node.Column, field, node.Value, joinExpected(allowed))
}

func parseIgnoreNode(node yaml.Node, path string) ([]string, error) {
	if node.Kind == 0 {
		return nil, nil
	}
	if node.Kind != yaml.SequenceNode {
		return nil, invalidf("invalid config %q: line %d, column %d: ignore must be a sequence of relative patterns", path, node.Line, node.Column)
	}
	patterns := make([]string, 0, len(node.Content))
	for _, item := range node.Content {
		if item.Kind != yaml.ScalarNode || item.ShortTag() != "!!str" {
			return nil, invalidf("invalid config %q: line %d, column %d: ignore entries must be strings", path, item.Line, item.Column)
		}
		if _, err := filter.NewIgnoreMatcher([]string{item.Value}); err != nil {
			return nil, invalidf("invalid config %q: line %d, column %d: %v", path, item.Line, item.Column, err)
		}
		patterns = append(patterns, item.Value)
	}
	return patterns, nil
}

func joinExpected(values []string) string {
	if len(values) == 0 {
		return ""
	}
	if len(values) == 1 {
		return values[0]
	}
	result := ""
	for index, value := range values {
		if index > 0 {
			if index == len(values)-1 {
				result += ", or "
			} else {
				result += ", "
			}
		}
		result += value
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
	const standardPrefix = "tag:yaml.org,2002:"
	if len(tag) >= len(standardPrefix) && tag[:len(standardPrefix)] == standardPrefix {
		return true
	}
	switch tag {
	case "!!map", "!!seq", "!!str", "!!int", "!!bool", "!!null", "!!float", "!!timestamp", "!!binary", "!!merge":
		return true
	default:
		return false
	}
}
