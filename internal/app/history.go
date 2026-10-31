package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/dirloom/dirloom/internal/gitsource"
	"github.com/dirloom/dirloom/internal/history"
)

// HistoryRequest names one structural history. An empty Revision means
// HEAD; From is optional; Limit is the number of commits examined, at least
// 1; an empty MoveDetection selects structural-v1. The repository is the one
// Git discovers from the process working directory.
type HistoryRequest struct {
	Revision      string
	From          string
	Limit         int
	MoveDetection comparison.MoveDetection
}

// HistoryUsageError reports an invalid history request. The CLI maps it to
// exit 2 with a human diagnostic on stderr, even under --format json.
type HistoryUsageError struct {
	Message string
}

func (e *HistoryUsageError) Error() string { return e.Message }

// History validates the request, then walks the first-parent history
// (Structural History v1). Invalid revisions and limits are usage errors,
// reported before Git runs.
func History(ctx context.Context, request HistoryRequest) (history.Result, error) {
	if err := ctx.Err(); err != nil {
		return history.Result{}, err
	}
	if request.Revision != "" {
		if err := gitsource.ValidateRevision(request.Revision); err != nil {
			return history.Result{}, &HistoryUsageError{Message: fmt.Sprintf("revision %q: %v", request.Revision, err)}
		}
	}
	if request.From != "" {
		if err := gitsource.ValidateRevision(request.From); err != nil {
			return history.Result{}, &HistoryUsageError{Message: fmt.Sprintf("--from %q: %v", request.From, err)}
		}
	}
	if request.Limit < 1 {
		return history.Result{}, &HistoryUsageError{Message: fmt.Sprintf("--limit must be a positive integer, received %d", request.Limit)}
	}
	strategy := request.MoveDetection
	if strategy == "" {
		strategy = comparison.MoveDetectionStructuralV1
	}
	if !strategy.Valid() {
		return history.Result{}, artifact.InternalError("unsupported move detection %q", strategy)
	}
	return history.Walk(ctx, history.Request{
		Revision:      request.Revision,
		From:          request.From,
		Limit:         request.Limit,
		MoveDetection: strategy,
	})
}

// HistoryUsage reports whether err is an invalid history request (exit 2).
func HistoryUsage(err error) bool {
	var usage *HistoryUsageError
	return errors.As(err, &usage)
}

// ClassifyHistoryFailure maps a history error onto the shared failure
// taxonomy: a broken invariant is internal, anything else is an observation
// failure. History reads no snapshot.
func ClassifyHistoryFailure(err error) VerifyFailureKind {
	if err == nil {
		return ""
	}
	if artifact.IsInternal(err) {
		return FailureInternal
	}
	return FailureObservation
}

// Fallback diagnostic codes used only when no typed artifact code exists.
const (
	DiagnosticHistoryObservationFailure = "history_observation_failure"
	DiagnosticHistoryInternalFailure    = "history_internal_failure"
)

// HistoryDiagnostic returns a stable machine code and message for err.
func HistoryDiagnostic(err error) (string, string) {
	if err == nil {
		return "", ""
	}
	var artErr *artifact.Error
	if errors.As(err, &artErr) && artErr.Code != "" {
		return artErr.Code, err.Error()
	}
	if ClassifyHistoryFailure(err) == FailureInternal {
		return DiagnosticHistoryInternalFailure, err.Error()
	}
	return DiagnosticHistoryObservationFailure, err.Error()
}
