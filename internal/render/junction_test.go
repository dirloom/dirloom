package render

import (
	"bytes"
	"testing"
)

// TestJunctionRendersAsSymlink checks that the Junction mark never reaches a
// public format: a junction renders byte for byte as a symlink with the same
// target, so no format gains a type value and graph node ids do not change.
func TestJunctionRendersAsSymlink(t *testing.T) {
	junction := sampleTree()
	link := junction.Children[1].Children[0]
	if link.Name != "link" {
		t.Fatalf("sample link = %#v", link)
	}
	link.Junction = true
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
		if err := renderer.Render(&got, junction); err != nil {
			t.Fatalf("%s: %v", test.format, err)
		}
		if err := renderer.Render(&want, sampleTree()); err != nil {
			t.Fatalf("%s: %v", test.format, err)
		}
		if !bytes.Equal(got.Bytes(), want.Bytes()) {
			t.Fatalf("%s/%s: a junction renders unlike a symlink\n--- junction ---\n%s--- symlink ---\n%s", test.format, test.style, got.Bytes(), want.Bytes())
		}
	}
}
