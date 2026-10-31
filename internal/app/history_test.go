package app

import (
	"context"
	"errors"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/dirloom/dirloom/internal/gittest"
	"github.com/dirloom/dirloom/internal/history"
)

func TestHistoryUsageErrorsBeforeGitRuns(t *testing.T) {
	// Not a repository: a usage error must come first, without Git.
	t.Chdir(t.TempDir())
	for _, request := range []HistoryRequest{
		{Revision: "-x", Limit: 1},
		{Revision: ":src/a.go", Limit: 1},
		{Revision: "a\nb", Limit: 1},
		{From: "--output=x", Limit: 1},
		{From: ":0:path", Limit: 1},
		{Limit: 0},
		{Limit: -3},
	} {
		if _, err := History(context.Background(), request); !HistoryUsage(err) {
			t.Fatalf("%+v: %v", request, err)
		}
	}
	if _, err := History(context.Background(), HistoryRequest{Limit: 1, MoveDetection: "fuzzy"}); !artifact.IsInternal(err) {
		t.Fatalf("unknown move detection: %v", err)
	}
}

func TestHistoryWalksTheRepositoryOfTheWorkingDirectory(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("a.txt", "a")
	first := repo.CommitAll("first")
	repo.Write("dir/b.txt", "b")
	second := repo.CommitAll("second")
	repo.Write("dir/b.txt", "rewritten")
	third := repo.CommitAll("third")
	t.Chdir(repo.Dir)

	result, err := History(context.Background(), HistoryRequest{Limit: history.DefaultLimit})
	if err != nil {
		t.Fatal(err)
	}
	if result.To != third || len(result.Entries) != 3 || result.End != history.EndRoot || result.MoveDetection != comparison.MoveDetectionStructuralV1 {
		t.Fatalf("%+v", result)
	}
	if result.Entries[0].Structural || !result.Entries[1].Structural || result.Entries[2].Commit != first {
		t.Fatalf("entries %+v", result.Entries)
	}
	bounded, err := History(context.Background(), HistoryRequest{Revision: "HEAD", From: first, Limit: 5, MoveDetection: comparison.MoveDetectionNone})
	if err != nil || len(bounded.Entries) != 2 || bounded.End != history.EndFrom || bounded.Entries[1].Commit != second {
		t.Fatalf("bounded %+v %v", bounded, err)
	}
}

func TestHistoryDiagnostics(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("a.txt", "a")
	repo.CommitAll("first")
	t.Chdir(repo.Dir)

	_, err := History(context.Background(), HistoryRequest{Revision: "nope", Limit: 1})
	if code, _ := HistoryDiagnostic(err); code != artifact.CodeGitRevisionNotFound || ClassifyHistoryFailure(err) != FailureObservation || HistoryUsage(err) {
		t.Fatalf("unknown revision: %s %v", code, err)
	}
	if code, _ := HistoryDiagnostic(artifact.InternalError("broken")); code != artifact.CodeInternal {
		t.Fatalf("internal: %s", code)
	}
	if code, _ := HistoryDiagnostic(errors.New("plain")); code != DiagnosticHistoryObservationFailure {
		t.Fatalf("fallback: %s", code)
	}
	if ClassifyHistoryFailure(artifact.InternalError("broken")) != FailureInternal || ClassifyHistoryFailure(nil) != "" {
		t.Fatal("classification")
	}
	if code, message := HistoryDiagnostic(nil); code != "" || message != "" {
		t.Fatal("nil diagnostic")
	}
}
