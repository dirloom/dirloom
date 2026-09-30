package app

import (
	"context"
	"errors"
	"io"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/identity"
	"github.com/dirloom/dirloom/internal/snapshot"
	"github.com/dirloom/dirloom/internal/source"
)

// VerifyStatus is the successful comparison outcome of verify.
// Failure outcomes are returned as errors and classified separately.
type VerifyStatus string

const (
	// VerifyMatch means the live Fingerprint v1 equals the validated snapshot.
	VerifyMatch VerifyStatus = "MATCH"
	// VerifyMismatch means both fingerprints were produced and they differ.
	VerifyMismatch VerifyStatus = "MISMATCH"
)

// VerifyFailureKind classifies verify errors without inspecting message text.
type VerifyFailureKind string

const (
	// FailureInvalidSnapshot is corrupt or semantically invalid Snapshot Schema v1.
	FailureInvalidSnapshot VerifyFailureKind = "INVALID_SNAPSHOT"
	// FailureUnsupportedSnapshot is a well-formed snapshot this release cannot accept.
	FailureUnsupportedSnapshot VerifyFailureKind = "UNSUPPORTED_SNAPSHOT"
	// FailureSnapshotRead is an operational failure reading the reference file.
	FailureSnapshotRead VerifyFailureKind = "SNAPSHOT_READ_ERROR"
	// FailureObservation is a live source that did not yield a trustworthy fingerprint.
	FailureObservation VerifyFailureKind = "OBSERVATION_ERROR"
	// FailureInternal is a broken Dirloom invariant.
	FailureInternal VerifyFailureKind = "INTERNAL_ERROR"
)

// Fallback diagnostic codes used only when no typed snapshot or artifact code exists.
const (
	DiagnosticObservationFailure = "verify_observation_failure"
	DiagnosticInternalFailure    = "verify_internal_failure"
)

// VerifyResult is the application-level comparison outcome.
// Node counts are intentionally absent: they must not decide MATCH or MISMATCH.
type VerifyResult struct {
	Status              VerifyStatus
	ExpectedFingerprint identity.Fingerprint
	ActualFingerprint   identity.Fingerprint
	SourceKind          source.Kind
}

// VerifyRequest loads one snapshot and observes one live root.
// SnapshotPath is used only for conditional reference-file self-exclusion.
type VerifyRequest struct {
	Snapshot     io.Reader
	SnapshotPath string
	Root         string
}

// Verify loads the reference through snapshot.Load and compares it to one live observation.
func Verify(ctx context.Context, request VerifyRequest) (VerifyResult, error) {
	if err := ctx.Err(); err != nil {
		return VerifyResult{}, err
	}
	if request.Snapshot == nil {
		return VerifyResult{}, artifact.InternalError("verify snapshot reader is nil")
	}
	validated, err := snapshot.Load(request.Snapshot)
	if err != nil {
		return VerifyResult{}, err
	}
	return verifyValidated(ctx, validated, request.Root, request.SnapshotPath)
}

// VerifyValidatedFromSource compares a validated snapshot to one source observation.
// The only comparison is typed Fingerprint v1 equality.
func VerifyValidatedFromSource(ctx context.Context, validated snapshot.Validated, src source.Source) (VerifyResult, error) {
	partial := VerifyResult{ExpectedFingerprint: validated.Fingerprint}
	if err := ctx.Err(); err != nil {
		return partial, err
	}
	if src == nil {
		return partial, artifact.InternalError("verify source is nil")
	}
	art, err := src.Observe(ctx)
	if err != nil {
		return partial, err
	}
	if art == nil {
		return partial, artifact.InternalError("verify source returned a nil artifact")
	}
	actual, err := identity.Compute(*art)
	if err != nil {
		return partial, err
	}
	status := VerifyMismatch
	if validated.Fingerprint == actual {
		status = VerifyMatch
	}
	return VerifyResult{
		Status:              status,
		ExpectedFingerprint: validated.Fingerprint,
		ActualFingerprint:   actual,
		SourceKind:          src.Kind(),
	}, nil
}

func verifyValidated(ctx context.Context, validated snapshot.Validated, root, snapshotPath string) (VerifyResult, error) {
	root = normalizeVerifyRoot(root)
	outputPath, err := referenceSelfExclusion(root, snapshotPath, validated.Artifact)
	if err != nil {
		return VerifyResult{ExpectedFingerprint: validated.Fingerprint}, err
	}
	request := inspectRequestFromCapture(validated.Document.Capture, root)
	request.OutputPath = outputPath
	return VerifyValidatedFromSource(ctx, validated, NewFilesystemSource(request))
}

func normalizeVerifyRoot(root string) string {
	if root == "" {
		return "."
	}
	return root
}

// ClassifyVerifyFailure maps a verify error onto the public failure taxonomy.
// It uses typed snapshot and artifact diagnostics, never message substrings.
func ClassifyVerifyFailure(err error) VerifyFailureKind {
	if err == nil {
		return ""
	}
	if artifact.IsInternal(err) || snapshot.IsInternal(err) {
		return FailureInternal
	}
	switch snapshot.CodeOf(err) {
	case snapshot.CodeInvalidJSON, snapshot.CodeMissingField, snapshot.CodeInvalidField, snapshot.CodeInvalidArtifact, snapshot.CodeFingerprintMismatch:
		return FailureInvalidSnapshot
	case snapshot.CodeUnsupportedSchema, snapshot.CodeUnsupportedArtifact, snapshot.CodeUnsupportedFeature:
		return FailureUnsupportedSnapshot
	case snapshot.CodeReadFailure:
		return FailureSnapshotRead
	case snapshot.CodeInternalFailure:
		return FailureInternal
	}
	return FailureObservation
}

// VerifyDiagnostic returns a stable machine code and message for err.
func VerifyDiagnostic(err error) (string, string) {
	if err == nil {
		return "", ""
	}
	if code := snapshot.CodeOf(err); code != "" {
		return code, err.Error()
	}
	var artErr *artifact.Error
	if errors.As(err, &artErr) && artErr.Code != "" {
		return artErr.Code, artErr.Error()
	}
	if ClassifyVerifyFailure(err) == FailureInternal {
		return DiagnosticInternalFailure, err.Error()
	}
	return DiagnosticObservationFailure, err.Error()
}
