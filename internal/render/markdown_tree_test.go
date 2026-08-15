package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/tree"
)

func TestMarkdownTreeEscapesMarkdownAndUnsafeCharacters(t *testing.T) {
	model := &tree.Node{Name: "pro`ject", Type: tree.NodeDirectory, Children: []*tree.Node{
		{Name: " leading ", Type: tree.NodeDirectory},
		{Name: "`ticks`", Type: tree.NodeFile},
		{Name: "line\n\\\u202e\x1b", Type: tree.NodeFile},
		{Name: "   ", Type: tree.NodeFile},
		{Name: "link", Type: tree.NodeSymlink, Target: "../sha`red"},
		{Name: "unknown", Type: tree.NodeSymlink},
	}}
	renderer, err := New(FormatMarkdownTree, StyleUnicode)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := renderer.Render(&output, model); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"- ``pro`ject/``",
		"  - `  leading / `",
		"  - `` `ticks` ``",
		"  - `line\\n\\\\\\u{202E}\\u{001B}`",
		"  - `\\x20\\x20\\x20`",
		"  - `link` -> ``../sha`red``",
		"  - `unknown` [symlink]",
	} {
		if !strings.Contains(output.String(), want+"\n") {
			t.Errorf("output does not contain %q:\n%s", want, output.String())
		}
	}
	if strings.ContainsRune(output.String(), '\x1b') {
		t.Fatalf("output contains a raw ANSI escape: %q", output.String())
	}
	assertPortableLineEndings(t, output.Bytes())
}

func TestMarkdownTreeWriteFailure(t *testing.T) {
	renderer, err := New(FormatMarkdownTree, StyleUnicode)
	if err != nil {
		t.Fatal(err)
	}
	if err := renderer.Render(failingWriter{}, sampleTree()); err == nil {
		t.Fatal("expected write failure")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, bytes.ErrTooLarge
}
