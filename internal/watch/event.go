// Package watch is the engine of Structural Watch v1: it turns triggers
// (backend notifications, the reconciliation timer, the poll timer) into
// full observations, compares each with the last reported state, and
// publishes Watch Event Schema v1 events. It has no structural model of its
// own: observations come from an Observer, comparisons from the comparison
// engine, and notifications from a Backend. NDJSONSink encodes the events
// (the contract) and TextSink renders them for a person; the caller picks
// one.
package watch

import (
	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/dirloom/dirloom/internal/identity"
	"github.com/dirloom/dirloom/internal/snapshot"
)

const (
	// Version is the Structural Watch contract version.
	Version = 1
	// SchemaVersion is the Watch Event Schema version.
	SchemaVersion = 1
	// Stability is the stability of both contracts.
	Stability = "experimental"
)

// Kind is the closed Watch Event Schema v1 event vocabulary.
type Kind string

const (
	KindStarted         Kind = "watch.started"
	KindNodeAdded       Kind = "node.added"
	KindNodeRemoved     Kind = "node.removed"
	KindNodeChanged     Kind = "node.changed"
	KindNodeMoved       Kind = "node.moved"
	KindStateChanged    Kind = "state.changed"
	KindRescanStarted   Kind = "rescan.started"
	KindRescanCompleted Kind = "rescan.completed"
	KindError           Kind = "watch.error"
	KindStopped         Kind = "watch.stopped"
)

// Valid reports whether k is one of the events.
func (k Kind) Valid() bool {
	switch k {
	case KindStarted, KindNodeAdded, KindNodeRemoved, KindNodeChanged, KindNodeMoved,
		KindStateChanged, KindRescanStarted, KindRescanCompleted, KindError, KindStopped:
		return true
	default:
		return false
	}
}

// IsNode reports whether k is a node event.
func (k Kind) IsNode() bool {
	return k == KindNodeAdded || k == KindNodeRemoved || k == KindNodeChanged || k == KindNodeMoved
}

// nodeKind maps a comparison operation to its node event.
func nodeKind(op comparison.Operation) Kind {
	switch op {
	case comparison.OpAdded:
		return KindNodeAdded
	case comparison.OpRemoved:
		return KindNodeRemoved
	case comparison.OpChanged:
		return KindNodeChanged
	default:
		return KindNodeMoved
	}
}

// nodeOperation maps a node event back to its comparison operation.
func nodeOperation(k Kind) comparison.Operation {
	switch k {
	case KindNodeAdded:
		return comparison.OpAdded
	case KindNodeRemoved:
		return comparison.OpRemoved
	case KindNodeChanged:
		return comparison.OpChanged
	default:
		return comparison.OpMoved
	}
}

// RescanReason says why a rescan opened.
type RescanReason string

const (
	RescanOverflow       RescanReason = "overflow"
	RescanReconciliation RescanReason = "reconciliation"
	RescanError          RescanReason = "error"
)

// Valid reports whether r is one of the reasons.
func (r RescanReason) Valid() bool {
	return r == RescanOverflow || r == RescanReconciliation || r == RescanError
}

// StopReason says why a stream ended.
type StopReason string

const (
	StopInterrupted StopReason = "interrupted"
	StopError       StopReason = "error"
)

// Valid reports whether r is one of the reasons.
func (r StopReason) Valid() bool { return r == StopInterrupted || r == StopError }

// BackendName names the trigger source in watch.started; never the platform
// mechanism.
type BackendName string

const (
	BackendNative BackendName = "native"
	BackendPoll   BackendName = "poll"
)

// Metadata is the watch.started metadata object.
type Metadata struct {
	WatchVersion              int
	Stability                 string
	ComparisonVersion         int
	IdentityProjectionVersion int
	MoveDetection             comparison.MoveDetection
	Backend                   BackendName
}

// Diagnostic is the watch.error diagnostic object.
type Diagnostic struct {
	Code    string
	Message string
}

// Event is one Watch Event Schema v1 event. Kind selects the fields that
// matter; the others stay zero:
//
//   - watch.started: Generation 0, Metadata, Capture, Fingerprint, NodeCount
//   - node events: Generation, Change (a validated comparison change)
//   - state.changed: Generation, Fingerprint, NodeCount, Summary
//   - rescan.started: Generation, Rescan
//   - rescan.completed: Generation, Fingerprint, NodeCount
//   - watch.error: Generation, Fatal, Diagnostic
//   - watch.stopped: Generation, Stop, Fingerprint
//
// HasGeneration is false only on the events of a stream that ended before
// its first state; watch.stopped then has no fingerprint either.
type Event struct {
	Kind          Kind
	HasGeneration bool
	Generation    uint64
	Metadata      Metadata
	Capture       snapshot.CaptureV1
	Fingerprint   identity.Fingerprint
	NodeCount     int
	Change        comparison.Change
	Summary       comparison.Summary
	Rescan        RescanReason
	Stop          StopReason
	Fatal         bool
	Diagnostic    Diagnostic
}
