package render

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/dirloom/dirloom/internal/tree"
)

// wtf8Surrogate is how Go spells, on Windows, a file name unit that is an
// unpaired UTF-16 surrogate (U+D800, WTF-8): three bytes that are not UTF-8.
const wtf8Surrogate = "\xed\xa0\x80"

// noWriteWriter fails the test on any write: a refused tree must not reach
// its destination at all.
type noWriteWriter struct{ t *testing.T }

func (w noWriteWriter) Write(p []byte) (int, error) {
	w.t.Helper()
	w.t.Fatalf("refused tree wrote %d bytes: %q", len(p), p)
	return 0, errors.New("unexpected write")
}

// TestJSONRefusesInvalidUTF8 pins the A11 correction: tree JSON refuses a
// name or a link target that is not valid UTF-8 instead of replacing it with
// U+FFFD, and it writes nothing.
func TestJSONRefusesInvalidUTF8(t *testing.T) {
	for _, test := range []struct {
		name  string
		root  *tree.Node
		path  string
		field string
		value string
	}{
		{
			name: "nested name",
			root: &tree.Node{Name: "project", Type: tree.NodeDirectory, Children: []*tree.Node{
				{Name: "src", Type: tree.NodeDirectory, Children: []*tree.Node{
					{Name: "ok.txt", Type: tree.NodeFile},
					{Name: "bad" + wtf8Surrogate + ".txt", Type: tree.NodeFile},
				}},
			}},
			path: "src/bad" + wtf8Surrogate + ".txt", field: "name", value: "bad" + wtf8Surrogate + ".txt",
		},
		{
			name: "invalid byte",
			root: &tree.Node{Name: "project", Type: tree.NodeDirectory, Children: []*tree.Node{
				{Name: "latin1-\xe9.txt", Type: tree.NodeFile},
			}},
			path: "latin1-\xe9.txt", field: "name", value: "latin1-\xe9.txt",
		},
		{
			name: "directory name",
			root: &tree.Node{Name: "project", Type: tree.NodeDirectory, Children: []*tree.Node{
				{Name: "dir" + wtf8Surrogate, Type: tree.NodeDirectory, Children: []*tree.Node{}},
			}},
			path: "dir" + wtf8Surrogate, field: "name", value: "dir" + wtf8Surrogate,
		},
		{
			name:  "root name",
			root:  &tree.Node{Name: "top" + wtf8Surrogate, Type: tree.NodeDirectory, Children: []*tree.Node{}},
			path:  "",
			field: "name", value: "top" + wtf8Surrogate,
		},
		{
			name: "symlink target",
			root: &tree.Node{Name: "project", Type: tree.NodeDirectory, Children: []*tree.Node{
				{Name: "link", Type: tree.NodeSymlink, Target: "../bad" + wtf8Surrogate},
			}},
			path: "link", field: "target", value: "../bad" + wtf8Surrogate,
		},
		{
			name: "junction target",
			root: &tree.Node{Name: "project", Type: tree.NodeDirectory, Children: []*tree.Node{
				{Name: "junction", Type: tree.NodeSymlink, Target: "C:/work/dir" + wtf8Surrogate, Junction: true},
			}},
			path: "junction", field: "target", value: "C:/work/dir" + wtf8Surrogate,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			renderer, err := New(FormatJSON, StyleUnicode)
			if err != nil {
				t.Fatal(err)
			}
			err = renderer.Render(noWriteWriter{t}, test.root)
			var invalid *InvalidUTF8Error
			if !errors.As(err, &invalid) {
				t.Fatalf("Render error = %v, want *InvalidUTF8Error", err)
			}
			if invalid.Path != test.path || invalid.Field != test.field || invalid.Value != test.value {
				t.Fatalf("error = %#v, want path %q, field %q, value %q", invalid, test.path, test.field, test.value)
			}
			message := err.Error()
			if !utf8.ValidString(message) || !strings.Contains(message, `\x`) || !strings.Contains(message, "not valid UTF-8") {
				t.Fatalf("message %q must stay UTF-8, escape the invalid bytes and name the defect", message)
			}
		})
	}
}

// TestJSONRefusalReportsFirstInDocumentOrder checks that the first invalid
// string in document order is the one reported.
func TestJSONRefusalReportsFirstInDocumentOrder(t *testing.T) {
	root := &tree.Node{Name: "project", Type: tree.NodeDirectory, Children: []*tree.Node{
		{Name: "a", Type: tree.NodeDirectory, Children: []*tree.Node{
			{Name: "link", Type: tree.NodeSymlink, Target: "x" + wtf8Surrogate},
		}},
		{Name: "b" + wtf8Surrogate, Type: tree.NodeFile},
	}}
	var invalid *InvalidUTF8Error
	if err := (jsonRenderer{}).Render(noWriteWriter{t}, root); !errors.As(err, &invalid) || invalid.Path != "a/link" || invalid.Field != "target" {
		t.Fatalf("Render error = %v, want the target of a/link", err)
	}
}

// TestTreeJSONGoldensNeverHoldReplacement guards Q3: no tree JSON golden,
// renderer or CLI, may consecrate a U+FFFD replacement character, encoded or
// literal.
func TestTreeJSONGoldensNeverHoldReplacement(t *testing.T) {
	var paths []string
	for _, pattern := range []string{
		filepath.Join("testdata", "json*.golden"),
		filepath.Join("..", "..", "testdata", "identity", "golden", "*.tree.json"),
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, matches...)
	}
	if len(paths) < 4 {
		t.Fatalf("found %d tree JSON goldens, want at least 4: %v", len(paths), paths)
	}
	for _, path := range paths {
		data, err := os.ReadFile(path) // #nosec G304 -- fixed test golden paths.
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(bytes.ToLower(data), []byte(`\ufffd`)) || bytes.ContainsRune(data, utf8.RuneError) {
			t.Errorf("%s holds a U+FFFD replacement character", path)
		}
	}
}
