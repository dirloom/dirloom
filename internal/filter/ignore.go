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

// matchSegments reports whether value matches pattern, where "**" matches
// any number of segments, none included, and every other segment matches
// exactly one segment through path.Match.
//
// It is iterative, with one backtrack point, as wildcard matching usually
// is: consecutive "**" act as one, and the last "**" seen is the only one
// that may absorb more segments, because every segment pattern matches
// exactly one segment. The cost is at most len(pattern) × len(value)
// segment matches, and the stack does not grow with the input (D7 of the
// v0.4.0-rc1 plan: a pattern of millions of "**" from a snapshot capture
// overflowed the stack of the former recursion, and a few thousand made it
// backtrack exponentially).
func matchSegments(pattern, value []string) bool {
	p, v := 0, 0
	star, starValue := -1, 0 // the pattern index after the last "**", and the value index it was tried at
	for v < len(value) {
		switch {
		case p < len(pattern) && pattern[p] == "**":
			for p < len(pattern) && pattern[p] == "**" {
				p++
			}
			star, starValue = p, v
		case p < len(pattern) && segmentMatches(pattern[p], value[v]):
			p++
			v++
		case star >= 0:
			starValue++
			p, v = star, starValue
		default:
			return false
		}
	}
	for p < len(pattern) && pattern[p] == "**" {
		p++
	}
	return p == len(pattern)
}

func segmentMatches(pattern, value string) bool {
	matched, err := path.Match(pattern, value)
	return err == nil && matched
}
