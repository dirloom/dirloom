package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/dirloom/dirloom/internal/filter"
	"github.com/dirloom/dirloom/internal/gitsource"
	"github.com/dirloom/dirloom/internal/snapshot"
	"github.com/dirloom/dirloom/internal/source"
	"github.com/dirloom/dirloom/internal/tree"
)

// DiffSourceKind identifies the prefix of a diff source expression.
type DiffSourceKind string

const (
	// DiffSourceSnapshot loads a Snapshot Schema v1 file.
	DiffSourceSnapshot DiffSourceKind = "snapshot"
	// DiffSourceLive observes a directory with the opposite snapshot's
	// Capture Semantics v1.
	DiffSourceLive DiffSourceKind = "live"
	// DiffSourceGit observes the committed tree a revision names in the
	// repository of the process working directory (Git Tree Source v1).
	DiffSourceGit DiffSourceKind = "git"
)

// diffSourceForms lists the accepted source expressions for diagnostics.
const diffSourceForms = "snapshot:<path>, live:<directory>, or git:<revision>"

// gitRootName labels the root of a Git tree. Revision text never reaches a
// result.
const gitRootName = "git"

// DiffSourceSpec is one parsed source expression: a kind and a path.
// Path is a snapshot file for DiffSourceSnapshot, a directory for
// DiffSourceLive, and a revision for DiffSourceGit.
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

// ParseDiffSource parses "snapshot:<path>", "live:<directory>" or
// "git:<revision>". The expression is split on the first colon only, so a
// revision may contain colons; prefixes are lowercase and case-sensitive.
// Paths are not validated here; a revision that must never reach Git is a
// usage error.
func ParseDiffSource(expression string) (DiffSourceSpec, error) {
	prefix, path, found := strings.Cut(expression, ":")
	if !found {
		return DiffSourceSpec{}, &DiffUsageError{Message: fmt.Sprintf("source %q has no prefix (expected %s)", expression, diffSourceForms)}
	}
	var kind DiffSourceKind
	switch prefix {
	case string(DiffSourceSnapshot):
		kind = DiffSourceSnapshot
	case string(DiffSourceLive):
		kind = DiffSourceLive
	case string(DiffSourceGit):
		kind = DiffSourceGit
	default:
		return DiffSourceSpec{}, &DiffUsageError{Message: fmt.Sprintf("unsupported source prefix %q (expected %s)", prefix, diffSourceForms)}
	}
	if kind == DiffSourceGit {
		if err := gitsource.ValidateRevision(path); err != nil {
			return DiffSourceSpec{}, &DiffUsageError{Message: fmt.Sprintf("source %q: %v", expression, err)}
		}
	}
	if path == "" {
		return DiffSourceSpec{}, &DiffUsageError{Message: fmt.Sprintf("source %q requires a non-empty path after %q", expression, prefix+":")}
	}
	return DiffSourceSpec{Kind: kind, Path: path}, nil
}

// DiffRequest names the two sources to compare, in user order, and the move
// detection strategy. An empty MoveDetection selects structural-v1.
type DiffRequest struct {
	A             DiffSourceSpec
	B             DiffSourceSpec
	MoveDetection comparison.MoveDetection
}

func (r DiffRequest) options() (comparison.Options, error) {
	strategy := r.MoveDetection
	if strategy == "" {
		strategy = comparison.MoveDetectionStructuralV1
	}
	if !strategy.Valid() {
		return comparison.Options{}, artifact.InternalError("unsupported move detection %q", strategy)
	}
	return comparison.Options{MoveDetection: strategy}, nil
}

// Diff resolves both sources and compares them through the comparison engine.
// Snapshot sides are loaded in user order and the first invalid side is
// reported. A live or Git side is observed with the opposite snapshot's
// Capture Semantics v1; git against git applies none. live against live and
// git against live are usage failures: neither side carries Capture
// Semantics.
func Diff(ctx context.Context, request DiffRequest) (comparison.StructuralDiff, error) {
	if err := ctx.Err(); err != nil {
		return comparison.StructuralDiff{}, err
	}
	opts, err := request.options()
	if err != nil {
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
	if (request.A.Kind == DiffSourceGit && request.B.Kind == DiffSourceLive) || (request.A.Kind == DiffSourceLive && request.B.Kind == DiffSourceGit) {
		return comparison.StructuralDiff{}, &DiffUsageError{Message: "git: against live: is not supported: neither side carries capture semantics; compare git:<revision> with a snapshot: of the working tree, or with a second git: revision"}
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
		return comparison.Compare(ctx, srcA, srcB, opts)
	}

	if request.A.Kind == DiffSourceGit && request.B.Kind == DiffSourceGit {
		srcA, err := diffGitSource(request.A, nil, comparison.SideA)
		if err != nil {
			return comparison.StructuralDiff{}, err
		}
		srcB, err := diffGitSource(request.B, nil, comparison.SideB)
		if err != nil {
			return comparison.StructuralDiff{}, err
		}
		return comparison.Compare(ctx, srcA, srcB, opts)
	}

	// live or git against snapshot: load the snapshot first, then build
	// source A from its capture semantics, then compare in user order.
	validatedB, srcB, err := diffSnapshotSource(request.B, comparison.SideB)
	if err != nil {
		return comparison.StructuralDiff{}, err
	}
	var srcA source.Source
	if request.A.Kind == DiffSourceGit {
		srcA, err = diffGitSource(request.A, &validatedB.Document.Capture, comparison.SideA)
	} else {
		srcA, err = diffLiveSource(request.A, validatedB, request.B.Path, comparison.SideA)
	}
	if err != nil {
		return comparison.StructuralDiff{}, err
	}
	return comparison.Compare(ctx, srcA, srcB, opts)
}

func validateDiffSpec(spec DiffSourceSpec, side string) error {
	switch spec.Kind {
	case DiffSourceSnapshot, DiffSourceLive, DiffSourceGit:
	default:
		return &DiffUsageError{Message: fmt.Sprintf("source %s has unsupported kind %q (expected snapshot, live, or git)", side, spec.Kind)}
	}
	if spec.Kind == DiffSourceGit {
		if err := gitsource.ValidateRevision(spec.Path); err != nil {
			return &DiffUsageError{Message: fmt.Sprintf("source %s: %v", side, err)}
		}
	}
	if spec.Path == "" {
		return &DiffUsageError{Message: fmt.Sprintf("source %s requires a non-empty path", side)}
	}
	return nil
}

// diffSecondSource resolves source B when source A is an already-loaded
// snapshot: a second snapshot, a live view driven by A's capture semantics
// with A's file self-excluded, or a Git tree under A's capture semantics.
func diffSecondSource(spec DiffSourceSpec, opposite snapshot.Validated, oppositePath string) (source.Source, error) {
	switch spec.Kind {
	case DiffSourceSnapshot:
		_, src, err := diffSnapshotSource(spec, comparison.SideB)
		return src, err
	case DiffSourceGit:
		return diffGitSource(spec, &opposite.Document.Capture, comparison.SideB)
	default:
		return diffLiveSource(spec, opposite, oppositePath, comparison.SideB)
	}
}

// diffGitSource builds a Git source. With a snapshot's capture semantics the
// tree is filtered exactly as Inspect filters a directory, through the same
// scanner, with no output path: a committed tree never contains the snapshot
// file being read. Without capture semantics (git against git) every entry
// is observed: unlimited depth, hidden entries, no ignore rules.
func diffGitSource(spec DiffSourceSpec, capture *snapshot.CaptureV1, side comparison.Side) (source.Source, error) {
	scan := tree.ScanOptions{RootName: gitRootName}
	if capture != nil {
		request := inspectRequestFromCapture(*capture, "")
		explicit, err := filter.NewIgnoreMatcher(request.IgnorePatterns)
		if err != nil {
			return nil, &comparison.SideError{Side: side, Err: artifact.SourceReadFailure(err)}
		}
		var git *filter.GitIgnore
		if request.UseGitIgnore {
			git = filter.NewGitIgnore()
		}
		scan.MaxDepth = request.MaxDepth
		scan.Directories = request.DirectoriesOnly
		scan.UseGitIgnore = request.UseGitIgnore
		scan.FilterPolicy = filter.NewPolicy("", request.UseDefaultIgnores, explicit, git, request.IncludeHidden)
		scan.GitIgnore = git
	}
	return &source.Git{Revision: spec.Path, Scan: scan}, nil
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
