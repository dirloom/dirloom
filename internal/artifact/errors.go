// Package artifact defines Dirloom's Canonical Structural Artifact v1.
package artifact

import (
	"errors"
	"fmt"
)

// Error codes are stable diagnostic categories for a1.
const (
	CodeInvalidPath        = "invalid_canonical_path"
	CodeCollision          = "canonical_path_collision"
	CodeUnsupportedKind    = "unsupported_node_type"
	CodeInvariant          = "artifact_invariant_violation"
	CodeInternal           = "internal_invariant_failure"
	CodeSourceRead         = "source_read_failure"
	CodeEncoding           = "canonical_encoding_failure"
	CodeInvalidFingerprint = "invalid_fingerprint_representation"
)

// Error is a structured diagnostic. Internal marks a broken Dirloom invariant
// rather than bad caller input or an unreadable source.
type Error struct {
	Code     string
	Message  string
	Paths    []string
	Internal bool
}

func (e *Error) Error() string { return e.Message }

// IsInternal reports whether err is an internal invariant failure.
func IsInternal(err error) bool {
	var typed *Error
	return errors.As(err, &typed) && typed.Internal
}

func invalidPath(format string, args ...any) error {
	return &Error{Code: CodeInvalidPath, Message: fmt.Sprintf(format, args...)}
}

func collision(first, second, canonical string) error {
	return &Error{
		Code:    CodeCollision,
		Message: fmt.Sprintf("canonical path collision between %q and %q -> %q", first, second, canonical),
		Paths:   []string{first, second, canonical},
	}
}

func unsupportedKind(kind string, path string) error {
	return &Error{
		Code:    CodeUnsupportedKind,
		Message: fmt.Sprintf("unsupported node type %q at %q", kind, path),
		Paths:   []string{path},
	}
}

func invariant(format string, args ...any) error {
	return &Error{Code: CodeInvariant, Message: fmt.Sprintf(format, args...)}
}

func internal(format string, args ...any) error {
	return &Error{Code: CodeInternal, Message: fmt.Sprintf(format, args...), Internal: true}
}

// InvalidFingerprint reports a malformed fingerprint string.
func InvalidFingerprint(format string, args ...any) error {
	return &Error{Code: CodeInvalidFingerprint, Message: fmt.Sprintf(format, args...)}
}

// EncodingFailure reports a writer or encoder failure.
func EncodingFailure(err error) error {
	if err == nil {
		return internal("canonical encoding failed without an error")
	}
	return &Error{Code: CodeEncoding, Message: "canonical encoding failure: " + err.Error(), Internal: false}
}

// SourceReadFailure wraps a structural source I/O error.
func SourceReadFailure(err error) error {
	if err == nil {
		return internal("structural source failed without an error")
	}
	return &Error{Code: CodeSourceRead, Message: err.Error()}
}

// InternalError reports a broken Dirloom invariant.
func InternalError(format string, args ...any) error {
	return internal(format, args...)
}

// New reports a diagnostic with an explicit code.
func New(code, message string, internalFailure bool, paths ...string) error {
	return &Error{Code: code, Message: message, Paths: paths, Internal: internalFailure}
}
