package snapshot

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
)

// withLimits lowers the reader limits for one test, so that documents stay
// small; the production values are measured, not tested at full size.
func withLimits(t *testing.T, set func()) {
	t.Helper()
	saved := limits
	set()
	t.Cleanup(func() { limits = saved })
}

// limitTestSnapshot returns a valid snapshot of n files below one directory.
func limitTestSnapshot(t *testing.T, n int, ignore []string) []byte {
	t.Helper()
	dir := artifact.Node{Path: "d", Name: "d", Kind: artifact.KindDirectory}
	for i := range n {
		name := fmt.Sprintf("f%03d", i)
		dir.Children = append(dir.Children, artifact.Node{Path: artifact.Path("d/" + name), Name: name, Kind: artifact.KindFile})
	}
	art := artifact.Artifact{Root: artifact.Node{Path: artifact.RootPath, Name: ".", Kind: artifact.KindDirectory, Children: []artifact.Node{dir}}}
	doc, err := Build(art, CaptureV1{Ignore: ignore})
	if err != nil {
		t.Fatal(err)
	}
	data, err := Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func requireLimit(t *testing.T, what string, err error) {
	t.Helper()
	if CodeOf(err) != CodeLimitExceeded || IsInternal(err) {
		t.Fatalf("%s: error = %v, want %s", what, err, CodeLimitExceeded)
	}
}

// countingReader serves an endless stream of spaces and counts what it
// served.
type countingReader struct{ served int }

func (r *countingReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}
	r.served += len(p)
	return len(p), nil
}

func TestLoadRefusesOversizedSnapshotWithoutReadingIt(t *testing.T) {
	data := limitTestSnapshot(t, 3, []string{})
	withLimits(t, func() { limits.bytes = len(data) })
	if _, err := Load(bytes.NewReader(data)); err != nil {
		t.Fatalf("a snapshot at the byte limit is refused: %v", err)
	}
	limits.bytes = len(data) - 1
	_, err := Load(bytes.NewReader(data))
	requireLimit(t, "one byte over the limit", err)
	_, err = LoadBytes(data)
	requireLimit(t, "LoadBytes over the limit", err)

	endless := &countingReader{}
	_, err = Load(endless)
	requireLimit(t, "an endless input", err)
	if endless.served > limits.bytes+1+bytes.MinRead {
		t.Fatalf("the reader consumed %d bytes for a limit of %d", endless.served, limits.bytes)
	}
}

func TestLoadRefusesDeepNesting(t *testing.T) {
	data := limitTestSnapshot(t, 1, []string{})
	// An unknown additive field is accepted and ignored, so only its nesting
	// can refuse the document. The document object is level 1.
	nested := func(levels int) []byte {
		field := `"x":` + strings.Repeat("[", levels) + strings.Repeat("]", levels) + `,`
		return bytes.Replace(data, []byte("{"), []byte("{"+field), 1)
	}
	withLimits(t, func() { limits.depth = 8 })
	if _, err := LoadBytes(nested(7)); err != nil {
		t.Fatalf("nesting at the limit is refused: %v", err)
	}
	_, err := LoadBytes(nested(8))
	requireLimit(t, "nesting one level over the limit", err)
}

func TestLoadRefusesTooManyNodes(t *testing.T) {
	// The root, d, and three files: five nodes.
	data := limitTestSnapshot(t, 3, []string{})
	withLimits(t, func() { limits.nodes = 5 })
	if _, err := LoadBytes(data); err != nil {
		t.Fatalf("a node count at the limit is refused: %v", err)
	}
	limits.nodes = 4
	_, err := LoadBytes(data)
	requireLimit(t, "one node over the limit", err)
}

func TestLoadRefusesTooManyIgnorePatterns(t *testing.T) {
	data := limitTestSnapshot(t, 1, []string{"a", "b", "c"})
	withLimits(t, func() { limits.ignore = 3 })
	if _, err := LoadBytes(data); err != nil {
		t.Fatalf("ignore patterns at the limit are refused: %v", err)
	}
	limits.ignore = 2
	_, err := LoadBytes(data)
	requireLimit(t, "one ignore pattern over the limit", err)
}

// TestWriterHonorsReaderLimits checks that Dirloom never writes a snapshot it
// would refuse to read: Build and Encode refuse it, and nothing is written.
func TestWriterHonorsReaderLimits(t *testing.T) {
	data := limitTestSnapshot(t, 3, []string{"a", "b"})
	doc, err := DecodeBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	withLimits(t, func() { limits.bytes = len(data) - 1 })
	var out bytes.Buffer
	requireLimit(t, "encoding one byte over the limit", Encode(&out, doc))
	if out.Len() != 0 {
		t.Fatalf("Encode wrote %d bytes before refusing", out.Len())
	}
	limits.bytes = MaxBytes

	art := artifact.Artifact{Root: artifact.Node{Path: artifact.RootPath, Name: ".", Kind: artifact.KindDirectory, Children: []artifact.Node{
		{Path: "a", Name: "a", Kind: artifact.KindFile}, {Path: "b", Name: "b", Kind: artifact.KindFile},
	}}}
	limits.nodes = 2
	_, err = Build(art, CaptureV1{Ignore: []string{}})
	requireLimit(t, "building one node over the limit", err)
	limits.nodes, limits.ignore = MaxNodes, 1
	_, err = Build(art, CaptureV1{Ignore: []string{"a", "b"}})
	requireLimit(t, "building one ignore pattern over the limit", err)
}

func TestLoadRefusesTooManyRequiredFeatures(t *testing.T) {
	data := limitTestSnapshot(t, 1, []string{})
	features := bytes.Replace(data, []byte(`"requiredFeatures": []`), []byte(`"requiredFeatures": ["a", "b"]`), 1)
	if bytes.Equal(features, data) {
		t.Fatal("the test snapshot has no empty requiredFeatures")
	}
	withLimits(t, func() { limits.features = 1 })
	_, err := LoadBytes(features)
	requireLimit(t, "one required feature over the limit", err)
}
