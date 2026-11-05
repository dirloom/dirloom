package app

import (
	"errors"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/snapshot"
)

// TestFailureClassificationByContext pins decision Q2 of the v0.4.0-rc1
// plan for verify and diff: a broken Dirloom invariant is INTERNAL_ERROR with
// its own code, while invalid input keeps its status.
func TestFailureClassificationByContext(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		kind VerifyFailureKind
		code string
	}{
		{"invariant in an observed artifact", artifact.AsInternal(artifact.New(artifact.CodeInvariant, "node has no parent", false)), FailureInternal, artifact.CodeInvariant},
		{"invariant read from a snapshot", &snapshot.Error{Code: snapshot.CodeInvalidArtifact, Message: "invalid snapshot artifact", Cause: artifact.New(artifact.CodeInvariant, "node has no parent", false)}, FailureInvalidSnapshot, snapshot.CodeInvalidArtifact},
		{"snapshot internal failure", &snapshot.Error{Code: snapshot.CodeInternalFailure, Message: "read failure without an error", Internal: true}, FailureInternal, snapshot.CodeInternalFailure},
		{"canonical encoding failure", artifact.EncodingFailure(errors.New("hash refused a write")), FailureInternal, artifact.CodeEncoding},
		{"collision in observed data", artifact.New(artifact.CodeCollision, "collision", false), FailureObservation, artifact.CodeCollision},
	} {
		if kind := ClassifyVerifyFailure(test.err); kind != test.kind {
			t.Fatalf("%s: verify status %s, want %s", test.name, kind, test.kind)
		}
		if code, _ := VerifyDiagnostic(test.err); code != test.code {
			t.Fatalf("%s: verify code %s, want %s", test.name, code, test.code)
		}
		if kind := ClassifyDiffFailure(test.err); kind != test.kind {
			t.Fatalf("%s: diff status %s, want %s", test.name, kind, test.kind)
		}
		if code, _ := DiffDiagnostic(test.err); code != test.code {
			t.Fatalf("%s: diff code %s, want %s", test.name, code, test.code)
		}
	}
}
