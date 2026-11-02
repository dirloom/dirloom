package watch

import (
	"fmt"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/dirloom/dirloom/internal/identity"
	"github.com/dirloom/dirloom/internal/source"
)

// ValidateStream checks the Watch Event Schema v1 invariants on a stream.
// With complete, the stream must end with watch.stopped. Without it, the
// stream may stop anywhere, and a trailing generation without its
// state.changed is ignored, as a consumer discards it. A violation is an
// internal error.
func ValidateStream(events []Event, complete bool) error {
	var c streamCheck
	for i, ev := range events {
		if err := c.next(i, ev); err != nil {
			return err
		}
	}
	if complete && !c.stopped {
		return artifact.InternalError("stream of %d events does not end with %s", len(events), KindStopped)
	}
	return nil
}

type streamCheck struct {
	startup    bool // a fatal watch.error before any state
	moves      comparison.MoveDetection
	gen        uint64
	fp         identity.Fingerprint
	nodes      int
	rescan     bool
	pending    []comparison.Change
	wantNode   bool // after rescan.started (reconciliation)
	afterError bool // previous event is a non-fatal watch.error
	afterFatal bool // previous event is a fatal watch.error
	fatal      bool
	stopped    bool
}

func fail(i int, ev Event, format string, args ...any) error {
	return artifact.InternalError("watch event %d (%s): %s", i, ev.Kind, fmt.Sprintf(format, args...))
}

func (c *streamCheck) next(i int, ev Event) error {
	if c.stopped {
		return fail(i, ev, "event after %s", KindStopped)
	}
	if !ev.Kind.Valid() {
		return fail(i, ev, "unknown event")
	}
	if i == 0 {
		return c.first(i, ev)
	}
	if c.startup {
		if ev.Kind != KindStopped || ev.HasGeneration || ev.Stop != StopError || ev.Fingerprint != (identity.Fingerprint{}) {
			return fail(i, ev, "a startup failure must end with %s (%s) without generation or fingerprint", KindStopped, StopError)
		}
		c.stopped = true
		return nil
	}
	if c.afterFatal && ev.Kind != KindStopped {
		return fail(i, ev, "a fatal %s must be followed by %s", KindError, KindStopped)
	}
	if c.wantNode && !ev.Kind.IsNode() {
		return fail(i, ev, "%s (%s) must be followed by a generation", KindRescanStarted, RescanReconciliation)
	}
	if len(c.pending) > 0 && !ev.Kind.IsNode() && ev.Kind != KindStateChanged {
		return fail(i, ev, "generation %d interrupted", c.gen+1)
	}
	if !ev.HasGeneration {
		return fail(i, ev, "missing generation")
	}
	afterError := c.afterError
	c.afterError, c.afterFatal, c.wantNode = false, false, false
	switch {
	case ev.Kind.IsNode():
		return c.node(i, ev)
	case ev.Kind == KindStateChanged:
		return c.stateChanged(i, ev)
	}
	if ev.Generation != c.gen {
		return fail(i, ev, "generation %d, want the last committed %d", ev.Generation, c.gen)
	}
	switch ev.Kind {
	case KindRescanStarted:
		if !ev.Rescan.Valid() {
			return fail(i, ev, "unknown reason %q", ev.Rescan)
		}
		if c.rescan {
			return fail(i, ev, "rescans nest")
		}
		if ev.Rescan == RescanError && !afterError {
			return fail(i, ev, "reason %s must directly follow a non-fatal %s", RescanError, KindError)
		}
		c.rescan = true
		c.wantNode = ev.Rescan == RescanReconciliation
	case KindRescanCompleted:
		if !c.rescan {
			return fail(i, ev, "no rescan is open")
		}
		if ev.Fingerprint != c.fp || ev.NodeCount != c.nodes {
			return fail(i, ev, "state %s/%d, want %s/%d", ev.Fingerprint, ev.NodeCount, c.fp, c.nodes)
		}
		c.rescan = false
	case KindError:
		if ev.Diagnostic.Code == "" {
			return fail(i, ev, "empty diagnostic code")
		}
		if ev.Fatal {
			if c.fatal {
				return fail(i, ev, "second fatal error")
			}
			c.fatal, c.afterFatal = true, true
		} else {
			c.afterError = true
		}
	case KindStopped:
		return c.stop(i, ev)
	case KindStarted:
		return fail(i, ev, "second %s", KindStarted)
	}
	return nil
}

func (c *streamCheck) first(i int, ev Event) error {
	switch {
	case ev.Kind == KindStarted:
		if !ev.HasGeneration || ev.Generation != 0 {
			return fail(i, ev, "generation must be 0")
		}
		m := ev.Metadata
		if m.WatchVersion != Version || m.Stability != Stability || m.ComparisonVersion != comparison.Version ||
			m.IdentityProjectionVersion != identity.ProjectionVersion || !m.MoveDetection.Valid() ||
			(m.Backend != BackendNative && m.Backend != BackendPoll) {
			return fail(i, ev, "invalid metadata %+v", m)
		}
		if ev.NodeCount < 1 {
			return fail(i, ev, "node count %d without a root", ev.NodeCount)
		}
		c.moves, c.fp, c.nodes = m.MoveDetection, ev.Fingerprint, ev.NodeCount
	case ev.Kind == KindError && ev.Fatal && !ev.HasGeneration && ev.Diagnostic.Code != "":
		c.startup = true
	case ev.Kind == KindStopped && ev.Stop == StopInterrupted && !ev.HasGeneration && ev.Fingerprint == (identity.Fingerprint{}):
		c.stopped = true
	default:
		return fail(i, ev, "a stream must start with %s or end before its first state", KindStarted)
	}
	return nil
}

func (c *streamCheck) node(i int, ev Event) error {
	if ev.Generation != c.gen+1 {
		return fail(i, ev, "generation %d, want %d", ev.Generation, c.gen+1)
	}
	if nodeOperation(ev.Kind) != ev.Change.Op {
		return fail(i, ev, "carries operation %s", ev.Change.Op)
	}
	if n := len(c.pending); n > 0 && c.pending[n-1].Path >= ev.Change.Path {
		return fail(i, ev, "path %q is not after %q", ev.Change.Path, c.pending[n-1].Path)
	}
	c.pending = append(c.pending, ev.Change)
	return nil
}

func (c *streamCheck) stateChanged(i int, ev Event) error {
	if len(c.pending) == 0 {
		return fail(i, ev, "generation without node events")
	}
	if ev.Generation != c.gen+1 {
		return fail(i, ev, "generation %d, want %d", ev.Generation, c.gen+1)
	}
	if ev.Fingerprint == c.fp {
		return fail(i, ev, "fingerprint unchanged")
	}
	diff := comparison.StructuralDiff{
		Metadata: comparison.Metadata{
			ComparisonVersion:         comparison.Version,
			IdentityProjectionVersion: identity.ProjectionVersion,
			MoveDetection:             c.moves,
		},
		SourceA: comparison.SourceRef{Kind: source.KindFilesystem, NodeCount: c.nodes},
		SourceB: comparison.SourceRef{Kind: source.KindFilesystem, NodeCount: ev.NodeCount},
		Summary: ev.Summary,
		Changes: c.pending,
	}
	if err := diff.Validate(); err != nil {
		return fail(i, ev, "%v", err)
	}
	c.gen, c.fp, c.nodes, c.pending = ev.Generation, ev.Fingerprint, ev.NodeCount, nil
	return nil
}

func (c *streamCheck) stop(i int, ev Event) error {
	if !ev.Stop.Valid() {
		return fail(i, ev, "unknown reason %q", ev.Stop)
	}
	if ev.Stop == StopInterrupted && c.fatal {
		return fail(i, ev, "interrupted after a fatal error")
	}
	if ev.Stop == StopError && !c.fatal {
		return fail(i, ev, "reason %s without a fatal error", StopError)
	}
	if ev.Fingerprint != c.fp {
		return fail(i, ev, "fingerprint %s, want the state's %s", ev.Fingerprint, c.fp)
	}
	c.stopped = true
	return nil
}
