package render

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/tree"
)

func TestTextAndMarkdownGolden(t *testing.T) {
	tests := []struct {
		name   string
		format string
		style  string
		golden string
	}{
		{"unicode", FormatText, StyleUnicode, "unicode.golden"},
		{"ascii", FormatText, StyleASCII, "ascii.golden"},
		{"markdown", FormatMarkdown, StyleUnicode, "markdown.golden"},
		{"markdown-tree", FormatMarkdownTree, "ignored", "markdown-tree.golden"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			renderer, err := New(test.format, test.style)
			if err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if err := renderer.Render(&output, sampleTree()); err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join("testdata", test.golden))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(output.Bytes(), want) {
				t.Fatalf("rendered output mismatch\n--- got ---\n%s--- want ---\n%s", output.Bytes(), want)
			}
			assertPortableLineEndings(t, output.Bytes())
		})
	}
}

func TestJSONContractV1(t *testing.T) {
	renderer, err := New(FormatJSON, StyleUnicode)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := renderer.Render(&output, sampleTree()); err != nil {
		t.Fatal(err)
	}
	assertPortableLineEndings(t, output.Bytes())

	var document map[string]any
	if err := json.Unmarshal(output.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if document["schemaVersion"] != float64(1) {
		t.Fatalf("schemaVersion = %#v", document["schemaVersion"])
	}
	root := document["root"].(map[string]any)
	if _, leaked := root["path"]; leaked {
		t.Fatal("internal path leaked into JSON")
	}
	children := root["children"].([]any)
	empty := children[0].(map[string]any)
	if emptyChildren, exists := empty["children"]; !exists || len(emptyChildren.([]any)) != 0 {
		t.Fatalf("empty directory children = %#v", emptyChildren)
	}
	srcChildren := children[1].(map[string]any)["children"].([]any)
	link := srcChildren[0].(map[string]any)
	if link["type"] != "symlink" || link["target"] != "../shared" {
		t.Fatalf("symlink contract = %#v", link)
	}
	if _, exists := link["children"]; exists {
		t.Fatal("symlink must not have children")
	}
	file := children[2].(map[string]any)
	if _, exists := file["children"]; exists {
		t.Fatal("file must not have children")
	}
}

func TestRendererValidation(t *testing.T) {
	if _, err := New("yaml", StyleUnicode); err == nil {
		t.Fatal("unsupported format should fail")
	}
	if _, err := New(FormatText, "auto"); err == nil {
		t.Fatal("unsupported style should fail")
	}
	if _, err := New(FormatMarkdownTree, "ignored"); err != nil {
		t.Fatalf("markdown-tree must not depend on drawing style: %v", err)
	}
}

func assertPortableLineEndings(t *testing.T, data []byte) {
	t.Helper()
	if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		t.Fatal("output contains a UTF-8 BOM")
	}
	if bytes.Contains(data, []byte("\r")) {
		t.Fatal("output contains CR characters")
	}
	if !bytes.HasSuffix(data, []byte("\n")) || strings.HasSuffix(string(data), "\n\n") {
		t.Fatalf("output must end in exactly one LF: %q", data)
	}
}

func sampleTree() *tree.Node {
	return &tree.Node{Name: "project", Type: tree.NodeDirectory, Children: []*tree.Node{
		{Name: "empty", Type: tree.NodeDirectory, Children: []*tree.Node{}},
		{Name: "src", Type: tree.NodeDirectory, Children: []*tree.Node{
			{Name: "link", Type: tree.NodeSymlink, Target: "../shared"},
			{Name: "index.ts", Type: tree.NodeFile},
		}},
		{Name: "README.md", Type: tree.NodeFile},
	}}
}
