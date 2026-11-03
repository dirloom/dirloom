package watch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/dirloom/dirloom/internal/identity"
	"github.com/dirloom/dirloom/internal/snapshot"
)

// lineFields lists, per event, the fields after schemaVersion and event in
// encoder order. generation is optional only on watch.error and
// watch.stopped (a stream that ended before its first state), where
// watch.stopped then has no fingerprint either.
var lineFields = map[Kind][]string{
	KindStarted:         {"generation", "metadata", "capture", "fingerprint", "nodeCount"},
	KindNodeAdded:       {"generation", "path", "after"},
	KindNodeRemoved:     {"generation", "path", "before"},
	KindNodeChanged:     {"generation", "path", "before", "after"},
	KindNodeMoved:       {"generation", "path", "from", "before", "after", "evidence", "nodes"},
	KindStateChanged:    {"generation", "fingerprint", "nodeCount", "summary"},
	KindRescanStarted:   {"generation", "reason"},
	KindRescanCompleted: {"generation", "fingerprint", "nodeCount"},
	KindError:           {"generation", "fatal", "diagnostic"},
	KindStopped:         {"generation", "reason", "fingerprint"},
}

// ParseNDJSON decodes a Watch Event Schema v1 stream strictly: every line
// must be one compact object (no carriage return, no blank line) carrying
// exactly the fields of its event, in encoder order. A last line
// without its line feed is not decoded, as a consumer discards it; it is
// returned as rest. ValidateStream checks the stream's invariants.
func ParseNDJSON(data []byte) (events []Event, rest []byte, err error) {
	for n := 1; len(data) > 0; n++ {
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			return events, data, nil
		}
		ev, err := parseLine(data[:i])
		if err != nil {
			return events, nil, fmt.Errorf("line %d: %w", n, err)
		}
		events = append(events, ev)
		data = data[i+1:]
	}
	return events, nil, nil
}

type lineJSON struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Event         Kind                `json:"event"`
	Generation    *uint64             `json:"generation"`
	Metadata      metadataJSON        `json:"metadata"`
	Capture       snapshot.CaptureV1  `json:"capture"`
	Fingerprint   string              `json:"fingerprint"`
	NodeCount     int                 `json:"nodeCount"`
	Path          string              `json:"path"`
	From          string              `json:"from"`
	Before        *comparisonStateRaw `json:"before"`
	After         *comparisonStateRaw `json:"after"`
	Evidence      comparison.Evidence `json:"evidence"`
	Nodes         int                 `json:"nodes"`
	Summary       summaryJSON         `json:"summary"`
	Reason        string              `json:"reason"`
	Fatal         bool                `json:"fatal"`
	Diagnostic    diagnosticJSON      `json:"diagnostic"`
}

type comparisonStateRaw struct {
	Kind   artifact.Kind `json:"kind"`
	Target *string       `json:"target"`
}

func parseLine(line []byte) (Event, error) {
	var compact bytes.Buffer
	if err := json.Compact(&compact, line); err != nil {
		return Event{}, err
	}
	if !bytes.Equal(compact.Bytes(), line) {
		return Event{}, fmt.Errorf("not a compact JSON object")
	}
	keys, values, err := objectKeys(line)
	if err != nil {
		return Event{}, err
	}
	var l lineJSON
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&l); err != nil {
		return Event{}, err
	}
	if l.SchemaVersion != SchemaVersion {
		return Event{}, fmt.Errorf("schemaVersion %d", l.SchemaVersion)
	}
	want, ok := lineFields[l.Event]
	if !ok {
		return Event{}, fmt.Errorf("unknown event %q", l.Event)
	}
	if l.Generation == nil && (l.Event == KindError || l.Event == KindStopped) {
		want = slices.DeleteFunc(slices.Clone(want), func(k string) bool { return k == "generation" || k == "fingerprint" })
	}
	if got := strings.Join(keys, ","); got != "schemaVersion,event,"+strings.Join(want, ",") {
		return Event{}, fmt.Errorf("%s fields %s", l.Event, got)
	}
	ev := Event{Kind: l.Event, NodeCount: l.NodeCount, Fatal: l.Fatal}
	if l.Generation != nil {
		ev.HasGeneration, ev.Generation = true, *l.Generation
	}
	if _, ok := values["fingerprint"]; ok {
		if ev.Fingerprint, err = identity.Parse(l.Fingerprint); err != nil {
			return Event{}, err
		}
	}
	switch {
	case l.Event == KindStarted:
		ev.Metadata, ev.Capture = Metadata(l.Metadata), l.Capture
	case l.Event.IsNode():
		ev.Change = comparison.Change{
			Path: artifact.Path(l.Path), Op: nodeOperation(l.Event), From: artifact.Path(l.From),
			Before: stateOf(l.Before), After: stateOf(l.After), Evidence: l.Evidence, Nodes: l.Nodes,
		}
	case l.Event == KindStateChanged:
		s := l.Summary
		ev.Summary = comparison.Summary{Added: s.Added, Removed: s.Removed, Changed: s.Changed, Moved: s.Moved, Total: s.Total}
	case l.Event == KindRescanStarted:
		ev.Rescan = RescanReason(l.Reason)
	case l.Event == KindError:
		ev.Diagnostic = Diagnostic{Code: l.Diagnostic.Code, Message: l.Diagnostic.Message}
	case l.Event == KindStopped:
		ev.Stop = StopReason(l.Reason)
	}
	if err := sameShape(line, ev); err != nil {
		return Event{}, fmt.Errorf("%s: %w", l.Event, err)
	}
	return ev, nil
}

// sameShape checks that the encoder writes ev with the fields of line, at
// every level: a value the encoder omits (an empty from, a zero node count,
// an absent target) is not accepted as present, nor the reverse.
func sameShape(line []byte, ev Event) error {
	encoded, err := json.Marshal(eventJSON(ev))
	if err != nil {
		return err
	}
	return compareShape(line, encoded)
}

func compareShape(got, want []byte) error {
	gotKeys, gotValues, errGot := objectKeys(got)
	wantKeys, wantValues, errWant := objectKeys(want)
	if (errGot == nil) != (errWant == nil) {
		return fmt.Errorf("an object where the encoder writes another value, or the reverse")
	}
	if errGot != nil {
		return nil // both are scalars or arrays
	}
	if !slices.Equal(gotKeys, wantKeys) {
		return fmt.Errorf("fields %s, the encoder writes %s", strings.Join(gotKeys, ","), strings.Join(wantKeys, ","))
	}
	for _, key := range gotKeys {
		if err := compareShape(gotValues[key], wantValues[key]); err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
	}
	return nil
}

// objectKeys returns the keys of the JSON object in data, in order, with
// their values.
func objectKeys(data []byte) ([]string, map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if tok, err := decoder.Token(); err != nil || tok != json.Delim('{') {
		return nil, nil, fmt.Errorf("not a JSON object")
	}
	var keys []string
	values := map[string]json.RawMessage{}
	for decoder.More() {
		tok, err := decoder.Token()
		if err != nil {
			return nil, nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, nil, fmt.Errorf("not a JSON object")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, nil, err
		}
		keys, values[key] = append(keys, key), value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, nil, err
	}
	if decoder.More() {
		return nil, nil, fmt.Errorf("more than one value on a line")
	}
	return keys, values, nil
}

func stateOf(raw *comparisonStateRaw) *comparison.NodeState {
	if raw == nil {
		return nil
	}
	return &comparison.NodeState{Kind: raw.Kind, Target: raw.Target}
}
