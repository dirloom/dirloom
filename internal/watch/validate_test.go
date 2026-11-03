package watch

import (
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/dirloom/dirloom/internal/identity"
)

// Builders for hand-written streams.

func evStarted(obs Observation) Event {
	return Event{Kind: KindStarted, HasGeneration: true, NodeCount: obs.NodeCount(), Fingerprint: obs.Fingerprint, Metadata: Metadata{
		WatchVersion: Version, Stability: Stability, ComparisonVersion: comparison.Version,
		IdentityProjectionVersion: identity.ProjectionVersion, MoveDetection: comparison.MoveDetectionStructuralV1, Backend: BackendNative,
	}}
}

func evNodes(t *testing.T, gen uint64, a, b Observation) []Event {
	t.Helper()
	var out []Event
	for _, ch := range changesOf(t, a, b, comparison.MoveDetectionStructuralV1) {
		out = append(out, Event{Kind: nodeKind(ch.Op), HasGeneration: true, Generation: gen, Change: ch})
	}
	return out
}

func evState(t *testing.T, gen uint64, a, b Observation) Event {
	t.Helper()
	ref := comparison.SourceRef{Kind: "filesystem"}
	diff, err := comparison.CompareRecords(a.Records, b.Records, ref, ref, comparison.Options{MoveDetection: comparison.MoveDetectionStructuralV1})
	if err != nil {
		t.Fatal(err)
	}
	return Event{Kind: KindStateChanged, HasGeneration: true, Generation: gen, Fingerprint: b.Fingerprint, NodeCount: b.NodeCount(), Summary: diff.Summary}
}

func evGeneration(t *testing.T, gen uint64, a, b Observation) []Event {
	t.Helper()
	return append(evNodes(t, gen, a, b), evState(t, gen, a, b))
}

func evRescan(gen uint64, r RescanReason) Event {
	return Event{Kind: KindRescanStarted, HasGeneration: true, Generation: gen, Rescan: r}
}

func evCompleted(gen uint64, obs Observation) Event {
	return Event{Kind: KindRescanCompleted, HasGeneration: true, Generation: gen, Fingerprint: obs.Fingerprint, NodeCount: obs.NodeCount()}
}

func evError(gen uint64, fatal bool) Event {
	return Event{Kind: KindError, HasGeneration: true, Generation: gen, Fatal: fatal, Diagnostic: Diagnostic{Code: artifact.CodeSourceRead, Message: "busy"}}
}

func evStopped(gen uint64, r StopReason, obs Observation) Event {
	return Event{Kind: KindStopped, HasGeneration: true, Generation: gen, Stop: r, Fingerprint: obs.Fingerprint}
}

func join(parts ...any) []Event {
	var out []Event
	for _, p := range parts {
		switch v := p.(type) {
		case Event:
			out = append(out, v)
		case []Event:
			out = append(out, v...)
		}
	}
	return out
}

func TestValidateStreamAcceptsContractStreams(t *testing.T) {
	a, b, c := tree("a"), tree("a", "b"), tree("a", "b", "c")
	for _, tc := range []struct {
		name     string
		events   []Event
		complete bool
	}{
		{"interrupted", join(evStarted(a), evGeneration(t, 1, a, b), evStopped(1, StopInterrupted, b)), true},
		{"overflow rescan", join(evStarted(a), evRescan(0, RescanOverflow), evGeneration(t, 1, a, b), evCompleted(1, b), evStopped(1, StopInterrupted, b)), true},
		{"reconciliation", join(evStarted(a), evRescan(0, RescanReconciliation), evGeneration(t, 1, a, b), evCompleted(1, b)), false},
		{"error recovery", join(evStarted(a), evError(0, false), evRescan(0, RescanError), evError(0, false), evCompleted(0, a)), false},
		{"fatal", join(evStarted(a), evGeneration(t, 1, a, b), evError(1, true), evStopped(1, StopError, b)), true},
		{"startup failure", join(Event{Kind: KindError, Fatal: true, Diagnostic: Diagnostic{Code: "x"}}, Event{Kind: KindStopped, Stop: StopError}), true},
		{"interrupted before the first state", join(Event{Kind: KindStopped, Stop: StopInterrupted}), true},
		{"two generations", join(evStarted(a), evGeneration(t, 1, a, b), evGeneration(t, 2, b, c)), false},
		{"trailing incomplete generation", join(evStarted(a), evNodes(t, 1, a, b)), false},
		{"empty incomplete", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateStream(tc.events, tc.complete); err != nil {
				t.Fatalf("ValidateStream: %v", err)
			}
		})
	}
}

func TestValidateStreamRejectsViolations(t *testing.T) {
	a, b, c := tree("a"), tree("a", "b"), tree("a", "b", "c")
	unsorted := evNodes(t, 1, a, c)
	unsorted[0], unsorted[1] = unsorted[1], unsorted[0]
	wrongOp := evNodes(t, 1, a, b)
	wrongOp[0].Kind = KindNodeRemoved
	badSummary := evState(t, 1, a, b)
	badSummary.Summary.Added = 2
	badMeta := evStarted(a)
	badMeta.Metadata.Stability = "stable"
	for _, tc := range []struct {
		name     string
		events   []Event
		complete bool
	}{
		{"not started", join(evGeneration(t, 1, a, b)), false},
		{"started twice", join(evStarted(a), evStarted(a)), false},
		{"metadata", join(badMeta), false},
		{"generation gap", join(evStarted(a), evGeneration(t, 2, a, b)), false},
		{"unsorted node events", join(evStarted(a), unsorted, evState(t, 1, a, c)), false},
		{"operation mismatch", join(evStarted(a), wrongOp, evState(t, 1, a, b)), false},
		{"summary mismatch", join(evStarted(a), evNodes(t, 1, a, b), badSummary), false},
		{"state without nodes", join(evStarted(a), evState(t, 1, a, b)), false},
		{"fingerprint unchanged", join(evStarted(b), evNodes(t, 1, a, b), evState(t, 1, a, b)), false},
		{"generation interrupted", join(evStarted(a), evNodes(t, 1, a, b), evRescan(0, RescanOverflow)), false},
		{"stale generation", join(evStarted(a), evGeneration(t, 1, a, b), evRescan(0, RescanOverflow)), false},
		{"nested rescans", join(evStarted(a), evRescan(0, RescanOverflow), evRescan(0, RescanOverflow)), false},
		{"error rescan without error", join(evStarted(a), evRescan(0, RescanError)), false},
		{"reconciliation without generation", join(evStarted(a), evRescan(0, RescanReconciliation), evCompleted(0, a)), false},
		{"completed without rescan", join(evStarted(a), evCompleted(0, a)), false},
		{"completed with another state", join(evStarted(a), evRescan(0, RescanOverflow), evCompleted(0, b)), false},
		{"fatal not followed by stopped", join(evStarted(a), evError(0, true), evRescan(0, RescanOverflow)), false},
		{"stopped error without fatal", join(evStarted(a), evStopped(0, StopError, a)), true},
		{"interrupted after fatal", join(evStarted(a), evError(0, true), evStopped(0, StopInterrupted, a)), true},
		{"stopped with another fingerprint", join(evStarted(a), evStopped(0, StopInterrupted, b)), true},
		{"event after stopped", join(evStarted(a), evStopped(0, StopInterrupted, a), evRescan(0, RescanOverflow)), false},
		{"incomplete stream", join(evStarted(a)), true},
		{"missing generation", join(evStarted(a), Event{Kind: KindRescanStarted, Rescan: RescanOverflow}), false},
		{"startup failure not stopped", join(Event{Kind: KindError, Fatal: true, Diagnostic: Diagnostic{Code: "x"}}, evStarted(a)), false},
		{"unknown event", join(evStarted(a), Event{Kind: "node.renamed", HasGeneration: true}), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateStream(tc.events, tc.complete)
			if !artifact.IsInternal(err) {
				t.Fatalf("ValidateStream = %v, want an internal error", err)
			}
		})
	}
}
