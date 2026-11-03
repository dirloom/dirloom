package watch

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// roundTrip parses a stream and encodes it again: the bytes must not change,
// and the parsed events must form a valid stream (complete when it ends with
// watch.stopped).
func roundTrip(t *testing.T, data []byte) {
	t.Helper()
	events, rest, err := ParseNDJSON(data)
	if err != nil || len(rest) != 0 {
		t.Fatalf("ParseNDJSON: %v (rest %q)", err, rest)
	}
	var again bytes.Buffer
	if err := NewNDJSONSink(&again).Publish(events); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(again.Bytes(), data) {
		t.Fatalf("round trip\n got %q\nwant %q", again.Bytes(), data)
	}
	complete := len(events) > 0 && events[len(events)-1].Kind == KindStopped
	if err := ValidateStream(events, complete); err != nil {
		t.Fatalf("parsed stream: %v", err)
	}
}

func TestParseNDJSONRejectsWhatTheEncoderNeverWrites(t *testing.T) {
	started := `{"schemaVersion":1,"event":"watch.started","generation":0,"metadata":{"watchVersion":1,"stability":"experimental","comparisonVersion":1,"identityProjectionVersion":1,"moveDetection":"structural-v1","backend":"native"},"capture":{"depth":null,"dirsOnly":false,"hidden":false,"useDefaultIgnores":true,"useGitignore":true,"ignore":[]},"fingerprint":"` + state("a").Fingerprint.String() + `","nodeCount":2}`
	if events, _, err := ParseNDJSON([]byte(started + "\n")); err != nil || len(events) != 1 {
		t.Fatalf("ParseNDJSON(started) = %v, %v", events, err)
	}
	for name, data := range map[string]string{
		"carriage return": started + "\r\n",
		"blank line":      started + "\n\n",
		"not compact":     strings.Replace(started, `"event"`, ` "event"`, 1) + "\n",
		"field order":     `{"event":"rescan.started","schemaVersion":1,"generation":0,"reason":"overflow"}` + "\n",
		"missing field":   `{"schemaVersion":1,"event":"rescan.completed","generation":0,"nodeCount":1}` + "\n",
		"foreign field":   `{"schemaVersion":1,"event":"rescan.started","generation":0,"reason":"overflow","path":"a"}` + "\n",
		"unknown field":   `{"schemaVersion":1,"event":"rescan.started","generation":0,"reason":"overflow","extra":1}` + "\n",
		"unknown event":   `{"schemaVersion":1,"event":"watch.paused","generation":0}` + "\n",
		"schema version":  `{"schemaVersion":2,"event":"rescan.started","generation":0,"reason":"overflow"}` + "\n",
		"two values":      `{"schemaVersion":1,"event":"rescan.started","generation":0,"reason":"overflow"}{}` + "\n",
		"bad fingerprint": `{"schemaVersion":1,"event":"rescan.completed","generation":0,"fingerprint":"x","nodeCount":1}` + "\n",
		"empty nested":    `{"schemaVersion":1,"event":"watch.error","fatal":true,"diagnostic":{}}` + "\n", // found by FuzzParseNDJSON
		"nested order":    `{"schemaVersion":1,"event":"watch.error","fatal":true,"diagnostic":{"message":"m","code":"c"}}` + "\n",
		"node state":      `{"schemaVersion":1,"event":"node.added","generation":1,"path":"a","after":{"kind":"file","size":1}}` + "\n",
	} {
		if _, _, err := ParseNDJSON([]byte(data)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	cut := `{"schemaVersion":1,"event":"node.ad`
	events, rest, err := ParseNDJSON([]byte(started + "\n" + cut))
	if err != nil || len(events) != 1 || string(rest) != cut {
		t.Fatalf("a cut last line: %v, rest %q, %v", events, rest, err)
	}
}

func FuzzParseNDJSON(f *testing.F) {
	files, _ := filepath.Glob(filepath.Join(goldenDir, "*.ndjson"))
	for _, file := range files {
		data, err := os.ReadFile(file) // #nosec G304 -- fixed test golden path.
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	// A JSON value has several spellings (string escapes), so the property is
	// stability: what the parser accepts encodes to a stream the parser
	// accepts again, with the same encoding.
	f.Fuzz(func(t *testing.T, data []byte) {
		events, _, err := ParseNDJSON(data)
		if err != nil {
			return
		}
		_ = ValidateStream(events, false)
		var once, twice bytes.Buffer
		if err := NewNDJSONSink(&once).Publish(events); err != nil {
			t.Fatal(err)
		}
		again, rest, err := ParseNDJSON(once.Bytes())
		if err != nil || len(rest) != 0 {
			t.Fatalf("accepted %q, but its encoding %q is rejected: %v", data, once.Bytes(), err)
		}
		if err := NewNDJSONSink(&twice).Publish(again); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(once.Bytes(), twice.Bytes()) {
			t.Fatalf("encoding is not stable: %q then %q", once.Bytes(), twice.Bytes())
		}
	})
}
