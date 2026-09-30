package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/dirloom/dirloom/internal/snapshot"
	"github.com/dirloom/dirloom/internal/source"
)

// DiffSourceKind identifies the prefix of a diff source expression.
type DiffSourceKind string

const (
	// DiffSourceSnapshot loads a Snapshot Schema v1 file.
	DiffSourceSnapshot DiffSourceKind = "snapshot"
	// DiffSourceLive observes a directory with the opposite snapshot's
	// Capture Semantics v1.
	DiffSourceLive DiffSourceKind = "live"
)

// DiffSourceSpec is one parsed source expression: a kind and a path.
// Path is a snapshot file for DiffSourceSnapshot and a directory for
// DiffSourceLive.
type DiffSourceSpec struct {
	Kind DiffSourceKind
	Path string
}

// DiffUsageError reports an invalid diff request. The CLI maps it to exit 2
// with a human diagnostic on stderr, even under --format json.
type DiffUsageError struct {
	Message string
}

func (e *DiffUsageError) Error() string { return e.Message }

// DiffStatus is the successful comparison outcome of diff.
// Failure outcomes are returned as errors and classified separately.
type DiffStatus string

const (
	// DiffNoDifferences means both sources were observed and no structural
	// change exists.
	DiffNoDifferences DiffStatus = "NO_DIFFERENCES"
	// DiffDifferences means both sources were observed and at least one
	// structural change exists.
	DiffDifferences DiffStatus = "DIFFERENCES"
)

// DiffStatusOf derives the public status from a validated diff.
func DiffStatusOf(diff comparison.StructuralDiff) DiffStatus {
	if len(diff.Changes) == 0 {
		return DiffNoDifferences
	}
	return DiffDifferences
}

// ParseDiffSource parses "snapshot:<path>" or "live:<directory>". The
// expression is split on the first colon only; prefixes are lowercase and
// case-sensitive. The path is not validated here.
func ParseDiffSource(expression string) (DiffSourceSpec, error) {
	prefix, path, found := strings.Cut(expression, ":")
	if !found {
		return DiffSourceSpec{}, &DiffUsageError{Message: fmt.Sprintf("source %q has no prefix (expected snapshot:<path> or live:<directory>)", expression)}
	}
	var kind DiffSourceKind
	switch prefix {
	case string(DiffSourceSnapshot):
		kind = DiffSourceSnapshot
	case string(DiffSourceLive):
		kind = DiffSourceLive
	default:
		return DiffSourceSpec{}, &DiffUsageError{Message: fmt.Sprintf("unsupported source prefix %q (expected snapshot:<path> or live:<directory>)", prefix)}
	}
	if path == "" {
		return DiffSourceSpec{}, &DiffUsageError{Message: fmt.Sprintf("source %q requires a non-empty path after %q", expression, prefix+":")}
	}
	return DiffSourceSpec{Kind: kind, Path: path}, nil
}

// DiffRequest names the two sources to compare, in user order.
type DiffRequest struct {
	A DiffSourceSpec
	B DiffSourceSpec
}

// Diff resolves both sources and compares them through the comparison engine.
// Snapshot sides are loaded in user order and the first invalid side is
// reported. A live side is observed with the opposite snapshot's Capture
// Semantics v1. live against live is a usage failure.
func Diff(ctx context.Context, request DiffRequest) (comparison.StructuralDiff, error) {
	if err := ctx.Err(); err != nil {
		return comparison.StructuralDiff{}, err
	}
	if err := validateDiffSpec(request.A, "a"); err != nil {
		return comparison.StructuralDiff{}, err
	}
	if err := validateDiffSpec(request.B, "b"); err != nil {
		return comparison.StructuralDiff{}, err
	}
	if request.A.Kind == DiffSourceLive && request.B.Kind == DiffSourceLive {
		return comparison.StructuralDiff{}, &DiffUsageError{Message: "diff requires at least one snapshot: source; live against live has no snapshot capture semantics to apply"}
	}

	if request.A.Kind == DiffSourceSnapshot {
		validatedA, srcA, err := diffSnapshotSource(request.A, comparison.SideA)
		if err != nil {
			return comparison.StructuralDiff{}, err
		}
		srcB, err := diffSecondSource(request.B, validatedA, request.A.Path)
		if err != nil {
			return comparison.StructuralDiff{}, err
		}
		return comparison.Compare(ctx, srcA, srcB)
	}

	// live against snapshot: load the snapshot first, then build the live
	// source, then compare in user order.
	validatedB, srcB, err := diffSnapshotSource(request.B, comparison.SideB)
	if err != nil {
		return comparison.StructuralDiff{}, err
	}
	srcA, err := diffLiveSource(request.A, validatedB, request.B.Path, comparison.SideA)
	if err != nil {
		return comparison.StructuralDiff{}, err
	}
	return comparison.Compare(ctx, srcA, srcB)
}

func validateDiffSpec(spec DiffSourceSpec, side string) error {
	switch spec.Kind {
	case DiffSourceSnapshot, DiffSourceLive:
	default:
		return &DiffUsageError{Message: fmt.Sprintf("source %s has unsupported kind %q (expected snapshot or live)", side, spec.Kind)}
	}
	if spec.Path == "" {
		return &DiffUsageError{Message: fmt.Sprintf("source %s requires a non-empty path", side)}
	}
	return nil
}

// diffSecondSource resolves source B when source A is an already-loaded
// snapshot: either a second snapshot or a live view driven by A's capture
// semantics with A's file self-excluded.
func diffSecondSource(spec DiffSourceSpec, opposite snapshot.Validated, oppositePath string) (source.Source, error) {
	if spec.Kind == DiffSourceSnapshot {
		_, src, err := diffSnapshotSource(spec, comparison.SideB)
		return src, err
	}
	return diffLiveSource(spec, opposite, oppositePath, comparison.SideB)
}

// diffSnapshotSource opens the snapshot file, loads it through the shared
// self-verifying validator, and wraps the validated artifact as a source. The
// adapter never parses Snapshot Schema itself.
func diffSnapshotSource(spec DiffSourceSpec, side comparison.Side) (snapshot.Validated, source.Source, error) {
	// #nosec G304 -- the snapshot path is the explicit diff reference selected by the caller.
	file, err := os.Open(spec.Path)
	if err != nil {
		return snapshot.Validated{}, nil, &comparison.SideError{Side: side, Err: &snapshot.Error{
			Code:    snapshot.CodeReadFailure,
			Message: "snapshot read failure",
			Cause:   err,
		}}
	}
	defer func() { _ = file.Close() }()
	validated, err := snapshot.Load(file)
	if err != nil {
		return snapshot.Validated{}, nil, &comparison.SideError{Side: side, Err: err}
	}
	return validated, source.Snapshot{Artifact: validated.Artifact}, nil
}

// diffLiveSource builds the live source from the opposite snapshot's Capture
// Semantics v1 through the shared capture helper, with conditional
// reference-file self-exclusion. Current configuration is never consulted.
func diffLiveSource(spec DiffSourceSpec, opposite snapshot.Validated, oppositePath string, side comparison.Side) (source.Source, error) {
	outputPath, err := referenceSelfExclusion(spec.Path, oppositePath, opposite.Artifact)
	if err != nil {
		return nil, &comparison.SideError{Side: side, Err: err}
	}
	request := inspectRequestFromCapture(opposite.Document.Capture, spec.Path)
	request.OutputPath = outputPath
	return NewFilesystemSource(request), nil
}

// ClassifyDiffFailure maps a diff error onto the shared failure taxonomy.
// The classification logic is identical to verify; only the fallback
// diagnostic codes differ.
func ClassifyDiffFailure(err error) VerifyFailureKind {
	return ClassifyVerifyFailure(err)
}

// DiffUsage reports whether err is an invalid diff request (exit 2).
func DiffUsage(err error) bool {
	var usage *DiffUsageError
	return errors.As(err, &usage)
}

// Fallback diagnostic codes used only when no typed snapshot or artifact code exists.
const (
	DiagnosticDiffObservationFailure = "diff_observation_failure"
	DiagnosticDiffInternalFailure    = "diff_internal_failure"
)

// DiffDiagnostic returns a stable machine code and message for err.
func DiffDiagnostic(err error) (string, string) {
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
	if ClassifyDiffFailure(err) == FailureInternal {
		return DiagnosticDiffInternalFailure, err.Error()
	}
	return DiagnosticDiffObservationFailure, err.Error()
}
