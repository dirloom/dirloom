package watch

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/dirloom/dirloom/internal/snapshot"
)

// NDJSONSink writes Watch Event Schema v1: one compact JSON object per
// event, HTML escaping disabled, each line ended by one line feed. A batch
// (a whole generation, or one control event) is encoded first and handed to
// the writer in one Write call; the writer itself gives no atomicity.
type NDJSONSink struct {
	w io.Writer
}

// NewNDJSONSink returns a sink writing to w.
func NewNDJSONSink(w io.Writer) *NDJSONSink { return &NDJSONSink{w: w} }

// Publish encodes events and writes them.
func (s *NDJSONSink) Publish(events []Event) error {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	for _, ev := range events {
		if err := encoder.Encode(eventJSON(ev)); err != nil {
			return err
		}
	}
	_, err := s.w.Write(buf.Bytes())
	return err
}

// lineHead starts every line. Generation is absent only on the lines of a
// stream that ended before its first state.
type lineHead struct {
	SchemaVersion int     `json:"schemaVersion"`
	Event         Kind    `json:"event"`
	Generation    *uint64 `json:"generation,omitempty"`
}

type startedJSON struct {
	lineHead
	Metadata    metadataJSON       `json:"metadata"`
	Capture     snapshot.CaptureV1 `json:"capture"`
	Fingerprint string             `json:"fingerprint"`
	NodeCount   int                `json:"nodeCount"`
}

type metadataJSON struct {
	WatchVersion              int                      `json:"watchVersion"`
	Stability                 string                   `json:"stability"`
	ComparisonVersion         int                      `json:"comparisonVersion"`
	IdentityProjectionVersion int                      `json:"identityProjectionVersion"`
	MoveDetection             comparison.MoveDetection `json:"moveDetection"`
	Backend                   BackendName              `json:"backend"`
}

// nodeJSON follows the Diff Result Schema v3 change order and presence
// rules: From, Evidence and Nodes on node.moved only.
type nodeJSON struct {
	lineHead
	Path     string         `json:"path"`
	From     string         `json:"from,omitempty"`
	Before   *nodeStateJSON `json:"before,omitempty"`
	After    *nodeStateJSON `json:"after,omitempty"`
	Evidence string         `json:"evidence,omitempty"`
	Nodes    int            `json:"nodes,omitempty"`
}

// nodeStateJSON keeps target presence explicit: a non-nil pointer to an
// empty string still encodes as "target": "".
type nodeStateJSON struct {
	Kind   string  `json:"kind"`
	Target *string `json:"target,omitempty"`
}

type stateJSON struct {
	lineHead
	Fingerprint string      `json:"fingerprint"`
	NodeCount   int         `json:"nodeCount"`
	Summary     summaryJSON `json:"summary"`
}

type summaryJSON struct {
	Added   int `json:"added"`
	Removed int `json:"removed"`
	Changed int `json:"changed"`
	Moved   int `json:"moved"`
	Total   int `json:"total"`
}

type rescanStartedJSON struct {
	lineHead
	Reason RescanReason `json:"reason"`
}

type rescanCompletedJSON struct {
	lineHead
	Fingerprint string `json:"fingerprint"`
	NodeCount   int    `json:"nodeCount"`
}

type errorJSON struct {
	lineHead
	Fatal      bool           `json:"fatal"`
	Diagnostic diagnosticJSON `json:"diagnostic"`
}

type diagnosticJSON struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type stoppedJSON struct {
	lineHead
	Reason      StopReason `json:"reason"`
	Fingerprint string     `json:"fingerprint,omitempty"`
}

// eventJSON returns the line object of ev, its fields in encoder order.
func eventJSON(ev Event) any {
	head := lineHead{SchemaVersion: SchemaVersion, Event: ev.Kind}
	if ev.HasGeneration {
		generation := ev.Generation
		head.Generation = &generation
	}
	switch {
	case ev.Kind == KindStarted:
		capture := ev.Capture.Clone()
		if capture.Ignore == nil {
			capture.Ignore = []string{}
		}
		return startedJSON{
			lineHead:    head,
			Metadata:    metadataJSON(ev.Metadata),
			Capture:     capture,
			Fingerprint: ev.Fingerprint.String(),
			NodeCount:   ev.NodeCount,
		}
	case ev.Kind.IsNode():
		c := ev.Change
		return nodeJSON{
			lineHead: head,
			Path:     c.Path.String(),
			From:     c.From.String(),
			Before:   nodeStateOf(c.Before),
			After:    nodeStateOf(c.After),
			Evidence: string(c.Evidence),
			Nodes:    c.Nodes,
		}
	case ev.Kind == KindStateChanged:
		s := ev.Summary
		return stateJSON{
			lineHead:    head,
			Fingerprint: ev.Fingerprint.String(),
			NodeCount:   ev.NodeCount,
			Summary:     summaryJSON{Added: s.Added, Removed: s.Removed, Changed: s.Changed, Moved: s.Moved, Total: s.Total},
		}
	case ev.Kind == KindRescanStarted:
		return rescanStartedJSON{lineHead: head, Reason: ev.Rescan}
	case ev.Kind == KindRescanCompleted:
		return rescanCompletedJSON{lineHead: head, Fingerprint: ev.Fingerprint.String(), NodeCount: ev.NodeCount}
	case ev.Kind == KindError:
		return errorJSON{lineHead: head, Fatal: ev.Fatal, Diagnostic: diagnosticJSON{Code: ev.Diagnostic.Code, Message: ev.Diagnostic.Message}}
	default:
		stopped := stoppedJSON{lineHead: head, Reason: ev.Stop}
		if ev.HasGeneration {
			stopped.Fingerprint = ev.Fingerprint.String()
		}
		return stopped
	}
}

func nodeStateOf(state *comparison.NodeState) *nodeStateJSON {
	if state == nil {
		return nil
	}
	out := &nodeStateJSON{Kind: string(state.Kind)}
	if state.Target != nil {
		target := *state.Target
		out.Target = &target
	}
	return out
}
