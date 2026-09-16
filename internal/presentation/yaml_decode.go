package presentation

import (
	"fmt"

	"go.yaml.in/yaml/v3"
)

func decodeNullableString(node *yaml.Node) (nullableStringDocument, error) {
	result := nullableStringDocument{Present: true}
	if node.ShortTag() == "!!null" {
		result.Null = true
		return result, nil
	}
	if node.Kind != yaml.ScalarNode || node.ShortTag() != "!!str" {
		return nullableStringDocument{}, fmt.Errorf("line %d, column %d: must be a string or null", node.Line, node.Column)
	}
	result.Value = node.Value
	return result, nil
}

func requireMapping(node *yaml.Node, typeName string) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d, column %d: %s must be a mapping", node.Line, node.Column, typeName)
	}
	return nil
}

func unknownYAMLField(key *yaml.Node, typeName string) error {
	return fmt.Errorf("line %d, column %d: field %s not found in type presentation.%s", key.Line, key.Column, key.Value, typeName)
}

func decodeString(node *yaml.Node) (*string, error) {
	if node.Kind != yaml.ScalarNode || node.ShortTag() != "!!str" {
		return nil, fmt.Errorf("line %d, column %d: must be a string", node.Line, node.Column)
	}
	value := node.Value
	return &value, nil
}

func decodeStringSlice(node *yaml.Node) (*[]string, error) {
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("line %d, column %d: must be a sequence of strings", node.Line, node.Column)
	}
	var values []string
	if err := node.Decode(&values); err != nil {
		return nil, err
	}
	return &values, nil
}

func (document *iconDocument) UnmarshalYAML(node *yaml.Node) error {
	if err := requireMapping(node, "iconDocument"); err != nil {
		return err
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		key, value := node.Content[index], node.Content[index+1]
		decoded, err := decodeNullableString(value)
		if err != nil {
			return err
		}
		switch key.Value {
		case "unicode":
			document.Unicode = decoded
		case "nerd":
			document.Nerd = decoded
		default:
			return unknownYAMLField(key, "iconDocument")
		}
	}
	return nil
}

func decodeTokenLike(node *yaml.Node, typeName string, color **string, iconColor *nullableStringDocument, styles **[]string, icons **iconDocument) error {
	if err := requireMapping(node, typeName); err != nil {
		return err
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		key, value := node.Content[index], node.Content[index+1]
		var err error
		switch key.Value {
		case "color":
			*color, err = decodeString(value)
		case "iconColor":
			*iconColor, err = decodeNullableString(value)
		case "styles":
			*styles, err = decodeStringSlice(value)
		case "icons":
			var decoded iconDocument
			err = value.Decode(&decoded)
			if err == nil {
				*icons = &decoded
			}
		default:
			return unknownYAMLField(key, typeName)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (document *tokenDocument) UnmarshalYAML(node *yaml.Node) error {
	return decodeTokenLike(node, "tokenDocument", &document.Color, &document.IconColor, &document.Styles, &document.Icons)
}

func (document *bindingDocument) UnmarshalYAML(node *yaml.Node) error {
	return decodeTokenLike(node, "bindingDocument", &document.Color, &document.IconColor, &document.Styles, &document.Icons)
}

func (document *matchDocument) UnmarshalYAML(node *yaml.Node) error {
	if err := requireMapping(node, "matchDocument"); err != nil {
		return err
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		key, value := node.Content[index], node.Content[index+1]
		decoded, err := decodeString(value)
		if err != nil {
			return err
		}
		switch key.Value {
		case "path":
			document.Path = decoded
		case "name":
			document.Name = decoded
		case "glob":
			document.Glob = decoded
		case "extension":
			document.Extension = decoded
		case "type":
			document.Type = decoded
		default:
			return unknownYAMLField(key, "matchDocument")
		}
	}
	return nil
}

func (document *ruleDocument) UnmarshalYAML(node *yaml.Node) error {
	if err := requireMapping(node, "ruleDocument"); err != nil {
		return err
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		key, value := node.Content[index], node.Content[index+1]
		var err error
		switch key.Value {
		case "match":
			err = value.Decode(&document.Match)
		case "kind":
			document.Kind, err = decodeString(value)
		case "role":
			document.Role, err = decodeString(value)
		case "color":
			document.Color, err = decodeString(value)
		case "iconColor":
			document.IconColor, err = decodeNullableString(value)
		case "styles":
			document.Styles, err = decodeStringSlice(value)
		case "icons":
			var decoded iconDocument
			err = value.Decode(&decoded)
			if err == nil {
				document.Icons = &decoded
			}
		default:
			return unknownYAMLField(key, "ruleDocument")
		}
		if err != nil {
			return err
		}
	}
	return nil
}
