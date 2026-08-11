package filter

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

type ignorePattern struct {
	original  string
	segments  []string
	basename  bool
	directory bool
}

// IgnoreMatcher implements the explicit --ignore contract. It is independent
// from .gitignore because command-line rules have no negation semantics and a
// higher, irreversible priority.
type IgnoreMatcher struct {
	patterns []ignorePattern
}

// NewIgnoreMatcher validates and compiles repeated --ignore values.
func NewIgnoreMatcher(values []string) (*IgnoreMatcher, error) {
	matcher := &IgnoreMatcher{}
	for _, value := range values {
		compiled, err := compileIgnorePattern(value)
		if err != nil {
			return nil, err
		}
		matcher.patterns = append(matcher.patterns, compiled)
	}
	return matcher, nil
}

func compileIgnorePattern(value string) (ignorePattern, error) {
	if value == "" {
		return ignorePattern{}, fmt.Errorf("ignore pattern must not be empty")
	}
	if filepath.IsAbs(value) || filepath.VolumeName(value) != "" || hasWindowsDrivePrefix(value) {
		return ignorePattern{}, fmt.Errorf("ignore pattern %q must be relative to the inspected root", value)
	}

	normalized := strings.ReplaceAll(value, `\`, "/")
	if strings.HasPrefix(normalized, "/") {
		return ignorePattern{}, fmt.Errorf("ignore pattern %q must be relative to the inspected root", value)
	}

	directoryOnly := strings.HasSuffix(normalized, "/")
	normalized = strings.TrimSuffix(normalized, "/")
	if normalized == "" {
		return ignorePattern{}, fmt.Errorf("ignore pattern %q is invalid", value)
	}

	segments := strings.Split(normalized, "/")
	for _, segment := range segments {
		if segment == "" || segment == ".." {
			return ignorePattern{}, fmt.Errorf("ignore pattern %q contains an invalid path segment", value)
		}
		if segment == "**" {
			continue
		}
		if _, err := path.Match(segment, "validation"); err != nil {
			return ignorePattern{}, fmt.Errorf("ignore pattern %q is invalid: %w", value, err)
		}
	}

	return ignorePattern{
		original:  value,
		segments:  segments,
		basename:  len(segments) == 1,
		directory: directoryOnly,
	}, nil
}

func hasWindowsDrivePrefix(value string) bool {
	if len(value) < 2 || value[1] != ':' {
		return false
	}
	letter := value[0]
	return letter >= 'A' && letter <= 'Z' || letter >= 'a' && letter <= 'z'
}

// Match reports whether an entry must be excluded by an explicit rule.
func (m *IgnoreMatcher) Match(relativePath string, isDir bool) bool {
	if m == nil {
		return false
	}
	parts := splitRelativePath(relativePath)
	if len(parts) == 0 {
		return false
	}

	for _, pattern := range m.patterns {
		if pattern.directory && !isDir {
			continue
		}
		if pattern.basename {
			matched, _ := path.Match(pattern.segments[0], parts[len(parts)-1])
			if matched {
				return true
			}
			continue
		}
		if matchSegments(pattern.segments, parts) {
			return true
		}
	}
	return false
}

func splitRelativePath(value string) []string {
	value = strings.Trim(strings.ReplaceAll(value, `\`, "/"), "/")
	if value == "" || value == "." {
		return nil
	}
	return strings.Split(value, "/")
}

func matchSegments(pattern, value []string) bool {
	if len(pattern) == 0 {
		return len(value) == 0
	}
	if pattern[0] == "**" {
		if matchSegments(pattern[1:], value) {
			return true
		}
		return len(value) > 0 && matchSegments(pattern, value[1:])
	}
	if len(value) == 0 {
		return false
	}
	matched, err := path.Match(pattern[0], value[0])
	return err == nil && matched && matchSegments(pattern[1:], value[1:])
}
