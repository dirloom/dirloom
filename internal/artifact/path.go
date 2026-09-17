package artifact

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Path is a root-relative canonical Dirloom path: NFC, '/' separators, no
// absolute prefix, no escaping '..'.
type Path string

// RootPath is the synthetic artifact root.
const RootPath Path = "."

// WindowsSeparator is the native separator used by Windows filesystem sources.
const WindowsSeparator byte = '\\'

// POSIXSeparator is the native separator used by POSIX filesystem sources.
const POSIXSeparator byte = '/'

// Canonicalize converts a source-relative native path into a canonical Path.
// nativeSeparator is the separator of the source. On POSIX it must be '/':
// '\' is then a valid filename character, not a separator. On Windows both
// '\' and '/' are separators.
func Canonicalize(native string, nativeSeparator byte) (Path, error) {
	if native == "" {
		return "", invalidPath("canonical path must not be empty")
	}
	if !utf8.ValidString(native) {
		return "", invalidPath("canonical path is not valid UTF-8")
	}
	if strings.IndexByte(native, 0) >= 0 {
		return "", invalidPath("canonical path must not contain NUL")
	}
	if isAbsoluteNative(native, nativeSeparator) {
		return "", invalidPath("canonical path must not be absolute: %q", native)
	}

	segments := splitNative(native, nativeSeparator)
	out := make([]string, 0, len(segments))
	for _, segment := range segments {
		switch segment {
		case "", ".":
			continue
		case "..":
			if len(out) == 0 {
				return "", invalidPath("canonical path %q escapes the root", native)
			}
			out = out[:len(out)-1]
			continue
		}
		normalized, err := CanonicalizeName(segment)
		if err != nil {
			return "", err
		}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return RootPath, nil
	}
	return Path(strings.Join(out, "/")), nil
}

// CanonicalizeName NFC-normalizes a single path segment.
func CanonicalizeName(name string) (string, error) {
	if name == "" {
		return "", invalidPath("node name must not be empty")
	}
	if !utf8.ValidString(name) {
		return "", invalidPath("node name is not valid UTF-8")
	}
	if strings.IndexByte(name, 0) >= 0 {
		return "", invalidPath("node name must not contain NUL")
	}
	if strings.Contains(name, "/") {
		return "", invalidPath("node name %q contains the canonical separator", name)
	}
	normalized := norm.NFC.String(name)
	if normalized == "" {
		return "", invalidPath("node name normalizes to empty")
	}
	if normalized == "." || normalized == ".." {
		return "", invalidPath("node name %q is not a valid path segment", name)
	}
	if strings.Contains(normalized, "/") {
		return "", invalidPath("node name %q contains the canonical separator after NFC", name)
	}
	if !utf8.ValidString(normalized) {
		return "", invalidPath("node name is not valid UTF-8 after NFC")
	}
	return normalized, nil
}

// String returns the canonical path.
func (p Path) String() string { return string(p) }

// Parent returns the parent canonical path.
func (p Path) Parent() (Path, bool) {
	if p == RootPath || p == "" {
		return "", false
	}
	index := strings.LastIndex(string(p), "/")
	if index < 0 {
		return RootPath, true
	}
	return Path(p[:index]), true
}

// Base returns the last segment, or "." for the root.
func (p Path) Base() string {
	if p == RootPath || p == "" {
		return "."
	}
	index := strings.LastIndex(string(p), "/")
	if index < 0 {
		return string(p)
	}
	return string(p[index+1:])
}

// Join appends a canonicalized child name to parent.
func Join(parent Path, name string) (Path, error) {
	normalized, err := CanonicalizeName(name)
	if err != nil {
		return "", err
	}
	if parent == RootPath || parent == "" {
		return Path(normalized), nil
	}
	return Path(string(parent) + "/" + normalized), nil
}

// IsNFC reports whether p is already NFC.
func (p Path) IsNFC() bool {
	return norm.NFC.IsNormalString(string(p))
}

func splitNative(native string, nativeSeparator byte) []string {
	if nativeSeparator == WindowsSeparator {
		native = strings.ReplaceAll(native, `\`, "/")
		return strings.Split(native, "/")
	}
	return strings.Split(native, "/")
}

func isAbsoluteNative(native string, nativeSeparator byte) bool {
	if native == "" {
		return false
	}
	if native[0] == '/' {
		return true
	}
	if nativeSeparator == WindowsSeparator {
		if strings.HasPrefix(native, `\\`) || strings.HasPrefix(native, "//") {
			return true
		}
		if len(native) >= 2 && native[1] == ':' {
			return true
		}
		if native[0] == '\\' {
			return true
		}
	}
	return false
}
