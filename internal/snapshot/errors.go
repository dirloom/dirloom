// Package snapshot implements Snapshot Schema v1: durable, self-verifying
// persistence of a Canonical Structural Artifact with Capture Semantics.
package snapshot

import (
	"errors"
	"fmt"
)

// Stable diagnostic categories for Snapshot v1.
const (
	CodeInvalidJSON         = "invalid_snapshot_json"
	CodeUnsupportedSchema   = "unsupported_snapshot_schema"
	CodeUnsupportedArtifact = "unsupported_snapshot_artifact_version"
	CodeMissingField        = "missing_snapshot_field"
	CodeInvalidField        = "invalid_snapshot_field"
	CodeUnsupportedFeature  = "unsupported_snapshot_feature"
	CodeInvalidArtifact     = "invalid_snapshot_artifact"
	CodeFingerprintMismatch = "snapshot_fingerprint_mismatch"
	CodeReadFailure         = "snapshot_read_failure"
	CodeWriteFailure        = "snapshot_write_failure"
	CodeInternalFailure     = "snapshot_internal_failure" // reserved for broken Dirloom invariants
)

// Error is a classifiable snapshot diagnostic.
type Error struct {
	Code     string
	Message  string
	Internal bool
	Cause    error
}

func (e *Error) Error() string {
	if e.Cause == nil {
		return e.Message
	}
	return e.Message + ": " + e.Cause.Error()
}

func (e *Error) Unwrap() error { return e.Cause }

// CodeOf returns the snapshot error category, or empty if err is not typed.
func CodeOf(err error) string {
	var typed *Error
	if errors.As(err, &typed) {
		return typed.Code
	}
	return ""
}

// IsInternal reports whether err is an internal snapshot failure.
func IsInternal(err error) bool {
	var typed *Error
	return errors.As(err, &typed) && typed.Internal
}

func invalidJSON(format string, args ...any) error {
	return &Error{Code: CodeInvalidJSON, Message: fmt.Sprintf(format, args...)}
}

func unsupportedSchema(version int) error {
	return &Error{
		Code:    CodeUnsupportedSchema,
		Message: fmt.Sprintf("unsupported snapshot schemaVersion %d", version),
	}
}

func unsupportedArtifact(version int) error {
	return &Error{
		Code:    CodeUnsupportedArtifact,
		Message: fmt.Sprintf("unsupported snapshot artifactVersion %d", version),
	}
}

func missingField(name string) error {
	return &Error{Code: CodeMissingField, Message: fmt.Sprintf("missing required snapshot field %q", name)}
}

func invalidField(format string, args ...any) error {
	return &Error{Code: CodeInvalidField, Message: fmt.Sprintf(format, args...)}
}

func unsupportedFeature(name string) error {
	return &Error{
		Code:    CodeUnsupportedFeature,
		Message: fmt.Sprintf("unsupported snapshot requiredFeature %q", name),
	}
}

func invalidArtifact(err error) error {
	if err == nil {
		return &Error{Code: CodeInvalidArtifact, Message: "invalid snapshot artifact", Internal: true}
	}
	return &Error{Code: CodeInvalidArtifact, Message: "invalid snapshot artifact", Cause: err}
}

func fingerprintMismatch(embedded, computed string) error {
	return &Error{
		Code:    CodeFingerprintMismatch,
		Message: fmt.Sprintf("snapshot fingerprint mismatch: embedded %s, computed %s", embedded, computed),
	}
}

func readFailure(err error) error {
	if err == nil {
		return &Error{Code: CodeReadFailure, Message: "snapshot read failure", Internal: true}
	}
	return &Error{Code: CodeReadFailure, Message: "snapshot read failure", Cause: err}
}

func writeFailure(err error) error {
	if err == nil {
		return &Error{Code: CodeWriteFailure, Message: "snapshot write failure", Internal: true}
	}
	return &Error{Code: CodeWriteFailure, Message: "snapshot write failure", Cause: err}
}
