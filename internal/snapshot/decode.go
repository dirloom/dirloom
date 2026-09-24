package snapshot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"unicode/utf8"
)

// knownRequiredFeatures is empty for Snapshot Schema v1 writers/readers.
// Future mandatory features are listed here when implemented.
var knownRequiredFeatures = map[string]struct{}{}

// Decode reads exactly one Snapshot Schema v1 JSON document.
func Decode(r io.Reader) (Document, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Document{}, readFailure(err)
	}
	return DecodeBytes(data)
}

// DecodeBytes decodes snapshot JSON from a byte slice.
func DecodeBytes(data []byte) (Document, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return Document{}, invalidJSON("empty snapshot document")
	}
	if !utf8.Valid(trimmed) {
		return Document{}, invalidJSON("snapshot JSON is not valid UTF-8")
	}
	if err := rejectDuplicateKeys(trimmed); err != nil {
		return Document{}, err
	}
	if err := rejectTrailingTokens(trimmed); err != nil {
		return Document{}, err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return Document{}, invalidJSON("invalid snapshot JSON: %v", err)
	}

	schemaVersion, err := requireInt(raw, "schemaVersion")
	if err != nil {
		return Document{}, err
	}
	artifactVersion, err := requireInt(raw, "artifactVersion")
	if err != nil {
		return Document{}, err
	}
	fingerprint, err := requireString(raw, "fingerprint")
	if err != nil {
		return Document{}, err
	}
	features, err := requireStringArray(raw, "requiredFeatures")
	if err != nil {
		return Document{}, err
	}
	captureRaw, ok := raw["capture"]
	if !ok {
		return Document{}, missingField("capture")
	}
	artifactRaw, ok := raw["artifact"]
	if !ok {
		return Document{}, missingField("artifact")
	}

	capture, err := decodeCapture(captureRaw)
	if err != nil {
		return Document{}, err
	}
	proj, err := decodeArtifact(artifactRaw)
	if err != nil {
		return Document{}, err
	}

	if err := validateFeatureList(features); err != nil {
		return Document{}, err
	}

	return Document{
		SchemaVersion:    schemaVersion,
		ArtifactVersion:  artifactVersion,
		Fingerprint:      fingerprint,
		RequiredFeatures: features,
		Capture:          capture,
		Artifact:         proj,
	}, nil
}

func decodeCapture(raw json.RawMessage) (CaptureV1, error) {
	if err := rejectDuplicateKeys(raw); err != nil {
		return CaptureV1{}, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return CaptureV1{}, invalidField("capture must be an object: %v", err)
	}
	for _, name := range []string{"depth", "dirsOnly", "hidden", "useDefaultIgnores", "useGitignore", "ignore"} {
		if _, ok := fields[name]; !ok {
			return CaptureV1{}, missingField("capture." + name)
		}
	}
	depth, err := decodeNullableInt(fields["depth"], "capture.depth")
	if err != nil {
		return CaptureV1{}, err
	}
	dirsOnly, err := decodeBool(fields["dirsOnly"], "capture.dirsOnly")
	if err != nil {
		return CaptureV1{}, err
	}
	hidden, err := decodeBool(fields["hidden"], "capture.hidden")
	if err != nil {
		return CaptureV1{}, err
	}
	useDefault, err := decodeBool(fields["useDefaultIgnores"], "capture.useDefaultIgnores")
	if err != nil {
		return CaptureV1{}, err
	}
	useGit, err := decodeBool(fields["useGitignore"], "capture.useGitignore")
	if err != nil {
		return CaptureV1{}, err
	}
	ignore, err := decodeStringArray(fields["ignore"], "capture.ignore")
	if err != nil {
		return CaptureV1{}, err
	}
	return CaptureV1{
		Depth:             depth,
		DirsOnly:          dirsOnly,
		Hidden:            hidden,
		UseDefaultIgnores: useDefault,
		UseGitignore:      useGit,
		Ignore:            ignore,
	}, nil
}

func decodeArtifact(raw json.RawMessage) (ArtifactV1, error) {
	if err := rejectDuplicateKeys(raw); err != nil {
		return ArtifactV1{}, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return ArtifactV1{}, invalidField("artifact must be an object: %v", err)
	}
	nodesRaw, ok := fields["nodes"]
	if !ok {
		return ArtifactV1{}, missingField("artifact.nodes")
	}
	var nodeMessages []json.RawMessage
	if err := json.Unmarshal(nodesRaw, &nodeMessages); err != nil {
		return ArtifactV1{}, invalidField("artifact.nodes must be an array: %v", err)
	}
	nodes := make([]NodeV1, 0, len(nodeMessages))
	for i, nodeRaw := range nodeMessages {
		node, err := decodeNode(nodeRaw, i)
		if err != nil {
			return ArtifactV1{}, err
		}
		nodes = append(nodes, node)
	}
	return ArtifactV1{Nodes: nodes}, nil
}

func decodeNode(raw json.RawMessage, index int) (NodeV1, error) {
	if err := rejectDuplicateKeys(raw); err != nil {
		return NodeV1{}, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return NodeV1{}, invalidField("artifact.nodes[%d] must be an object: %v", index, err)
	}
	path, err := requireStringFrom(fields, "path", fmt.Sprintf("artifact.nodes[%d].path", index))
	if err != nil {
		return NodeV1{}, err
	}
	kind, err := requireStringFrom(fields, "kind", fmt.Sprintf("artifact.nodes[%d].kind", index))
	if err != nil {
		return NodeV1{}, err
	}
	node := NodeV1{Path: path, Kind: kind}
	if targetRaw, ok := fields["target"]; ok {
		if isJSONNull(targetRaw) {
			return NodeV1{}, invalidField("artifact.nodes[%d].target must not be null", index)
		}
		var target string
		if err := json.Unmarshal(targetRaw, &target); err != nil {
			return NodeV1{}, invalidField("artifact.nodes[%d].target must be a string: %v", index, err)
		}
		node.Target = &target
	}
	return node, nil
}

func validateFeatureList(features []string) error {
	seen := make(map[string]struct{}, len(features))
	for _, feature := range features {
		if feature == "" {
			return invalidField("requiredFeatures entries must be non-empty strings")
		}
		if _, exists := seen[feature]; exists {
			return invalidField("duplicate requiredFeature %q", feature)
		}
		seen[feature] = struct{}{}
	}
	return nil
}

func checkSupportedFeatures(features []string) error {
	for _, feature := range features {
		if _, ok := knownRequiredFeatures[feature]; !ok {
			return unsupportedFeature(feature)
		}
	}
	return nil
}

func requireInt(raw map[string]json.RawMessage, name string) (int, error) {
	value, ok := raw[name]
	if !ok {
		return 0, missingField(name)
	}
	if isJSONNull(value) {
		return 0, invalidField("%s must not be null", name)
	}
	var number json.Number
	dec := json.NewDecoder(bytes.NewReader(value))
	dec.UseNumber()
	if err := dec.Decode(&number); err != nil {
		return 0, invalidField("%s must be an integer: %v", name, err)
	}
	if err := rejectTrailingTokens(value); err != nil {
		return 0, invalidField("%s must be an integer", name)
	}
	parsed, err := number.Int64()
	if err != nil {
		return 0, invalidField("%s must be an integer: %v", name, err)
	}
	if int64(int(parsed)) != parsed {
		return 0, invalidField("%s is out of range", name)
	}
	return int(parsed), nil
}

func requireString(raw map[string]json.RawMessage, name string) (string, error) {
	return requireStringFrom(raw, name, name)
}

func requireStringFrom(raw map[string]json.RawMessage, key, label string) (string, error) {
	value, ok := raw[key]
	if !ok {
		return "", missingField(label)
	}
	if isJSONNull(value) {
		return "", invalidField("%s must not be null", label)
	}
	var out string
	if err := json.Unmarshal(value, &out); err != nil {
		return "", invalidField("%s must be a string: %v", label, err)
	}
	return out, nil
}

func requireStringArray(raw map[string]json.RawMessage, name string) ([]string, error) {
	value, ok := raw[name]
	if !ok {
		return nil, missingField(name)
	}
	return decodeStringArray(value, name)
}

func decodeStringArray(raw json.RawMessage, label string) ([]string, error) {
	if isJSONNull(raw) {
		return nil, invalidField("%s must not be null", label)
	}
	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, invalidField("%s must be an array: %v", label, err)
	}
	out := make([]string, 0, len(values))
	for i, item := range values {
		if isJSONNull(item) {
			return nil, invalidField("%s[%d] must not be null", label, i)
		}
		var s string
		if err := json.Unmarshal(item, &s); err != nil {
			return nil, invalidField("%s[%d] must be a string: %v", label, i, err)
		}
		out = append(out, s)
	}
	return out, nil
}

func decodeBool(raw json.RawMessage, label string) (bool, error) {
	if isJSONNull(raw) {
		return false, invalidField("%s must not be null", label)
	}
	var out bool
	if err := json.Unmarshal(raw, &out); err != nil {
		return false, invalidField("%s must be a boolean: %v", label, err)
	}
	return out, nil
}

func decodeNullableInt(raw json.RawMessage, label string) (*int, error) {
	if isJSONNull(raw) {
		return nil, nil
	}
	var number json.Number
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&number); err != nil {
		return nil, invalidField("%s must be an integer or null: %v", label, err)
	}
	parsed, err := number.Int64()
	if err != nil {
		return nil, invalidField("%s must be an integer or null: %v", label, err)
	}
	if int64(int(parsed)) != parsed {
		return nil, invalidField("%s is out of range", label)
	}
	value := int(parsed)
	if value < 0 {
		return nil, invalidField("%s must be non-negative", label)
	}
	return &value, nil
}

func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func rejectTrailingTokens(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	var value any
	if err := dec.Decode(&value); err != nil {
		return invalidJSON("invalid snapshot JSON: %v", err)
	}
	tok, err := dec.Token()
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return invalidJSON("invalid snapshot JSON trailing content: %v", err)
	}
	_ = tok
	return invalidJSON("snapshot must contain exactly one JSON document")
}

func rejectDuplicateKeys(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	return walkJSON(dec, true)
}

func walkJSON(dec *json.Decoder, requireValue bool) error {
	tok, err := dec.Token()
	if err != nil {
		return invalidJSON("invalid snapshot JSON: %v", err)
	}
	switch typed := tok.(type) {
	case json.Delim:
		switch typed {
		case '{':
			seen := map[string]struct{}{}
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return invalidJSON("invalid snapshot JSON: %v", err)
				}
				key, ok := keyTok.(string)
				if !ok {
					return invalidJSON("invalid snapshot JSON object key")
				}
				if _, exists := seen[key]; exists {
					return invalidJSON("duplicate JSON object key %q", key)
				}
				seen[key] = struct{}{}
				if err := walkJSON(dec, true); err != nil {
					return err
				}
			}
			end, err := dec.Token()
			if err != nil {
				return invalidJSON("invalid snapshot JSON: %v", err)
			}
			if end != json.Delim('}') {
				return invalidJSON("invalid snapshot JSON object terminator")
			}
			return nil
		case '[':
			for dec.More() {
				if err := walkJSON(dec, true); err != nil {
					return err
				}
			}
			end, err := dec.Token()
			if err != nil {
				return invalidJSON("invalid snapshot JSON: %v", err)
			}
			if end != json.Delim(']') {
				return invalidJSON("invalid snapshot JSON array terminator")
			}
			return nil
		default:
			return invalidJSON("unexpected JSON delimiter %q", typed)
		}
	default:
		if !requireValue {
			return invalidJSON("unexpected JSON token")
		}
		switch typed.(type) {
		case string, float64, json.Number, bool, nil:
			return nil
		default:
			return invalidJSON("unexpected JSON token type %T", typed)
		}
	}
}

// SortedFeatures returns a sorted copy of features for tests and writers.
func SortedFeatures(features []string) []string {
	out := append([]string(nil), features...)
	sort.Strings(out)
	return out
}
