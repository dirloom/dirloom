package render

import (
	"bytes"
	"testing"
)

// TestSpecialEntryRendersAsFile checks that the Special mark never reaches a
// public format: a socket renders byte for byte as the file it always was.
func TestSpecialEntryRendersAsFile(t *testing.T) {
	special := sampleTree()
	readme := special.Children[2]
	if readme.Name != "README.md" {
		t.Fatalf("sample file = %#v", readme)
	}
	readme.Special = "socket"
	for _, test := range []struct{ format, style string }{
		{FormatText, StyleUnicode},
		{FormatText, StyleASCII},
		{FormatMarkdown, StyleUnicode},
		{FormatMarkdownTree, StyleUnicode},
		{FormatJSON, StyleUnicode},
		{FormatMermaid, StyleUnicode},
		{FormatGraphviz, StyleUnicode},
		{FormatD2, StyleUnicode},
	} {
		renderer, err := New(test.format, test.style)
		if err != nil {
			t.Fatal(err)
		}
		var got, want bytes.Buffer
		if err := renderer.Render(&got, special); err != nil {
			t.Fatalf("%s: %v", test.format, err)
		}
		if err := renderer.Render(&want, sampleTree()); err != nil {
			t.Fatalf("%s: %v", test.format, err)
		}
		if !bytes.Equal(got.Bytes(), want.Bytes()) {
			t.Fatalf("%s/%s: a special entry renders unlike a file\n--- special ---\n%s--- file ---\n%s", test.format, test.style, got.Bytes(), want.Bytes())
		}
	}
}
