package render

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/diagram"
)

func TestSemanticMarkdownDocumentationUsesRendererContract(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "markdown-tree.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	pattern := regexp.MustCompile(`(?s)<!-- dirloom-markdown-tree-output:basic -->\r?\n` + "```markdown" + `\r?\n(.*?)\r?\n` + "```")
	match := pattern.FindSubmatch(data)
	if match == nil {
		t.Fatal("documented semantic Markdown output was not found")
	}

	renderer, err := New(FormatMarkdownTree, StyleUnicode)
	if err != nil {
		t.Fatal(err)
	}
	var actual bytes.Buffer
	if err := renderer.Render(&actual, sampleTree()); err != nil {
		t.Fatal(err)
	}
	want := append(append([]byte(nil), match[1]...), '\n')
	if !bytes.Equal(actual.Bytes(), want) {
		t.Fatalf("documented output differs from renderer\n--- actual ---\n%s--- documented ---\n%s", actual.Bytes(), want)
	}

	text := string(data)
	for _, contract := range []string{
		"dirloom --format markdown-tree",
		"The existing `markdown` format remains unchanged.",
		"no ANSI sequences",
		"[symlink]",
	} {
		if !strings.Contains(text, contract) {
			t.Errorf("semantic Markdown guide is missing %q", contract)
		}
	}

	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "docs/markdown-tree.md") {
		t.Fatal("README does not link to the semantic Markdown guide")
	}
}

func TestGraphExportDocumentationUsesRendererContract(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "graph-exports.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	document, err := diagram.ProjectStructure(sampleTree(), diagram.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	examples := []struct {
		marker string
		render func(diagram.Document, io.Writer) error
		lang   string
	}{
		{"dirloom-graph-output:mermaid", RenderMermaid, "mermaid"},
		{"dirloom-graph-output:graphviz", RenderGraphviz, "dot"},
		{"dirloom-graph-output:d2", RenderD2, "d2"},
	}
	text := string(data)
	for _, example := range examples {
		pattern := regexp.MustCompile(`(?s)<!-- ` + example.marker + ` -->\r?\n` + "```" + example.lang + `\r?\n(.*?)\r?\n` + "```")
		match := pattern.FindSubmatch(data)
		if match == nil {
			t.Fatalf("documented %s output was not found", example.marker)
		}
		var actual bytes.Buffer
		if err := example.render(document, &actual); err != nil {
			t.Fatal(err)
		}
		want := append(append([]byte(nil), match[1]...), '\n')
		if !bytes.Equal(actual.Bytes(), want) {
			t.Fatalf("%s documented output differs\n--- actual ---\n%s--- documented ---\n%s", example.marker, actual.Bytes(), want)
		}
	}
	for _, contract := range []string{
		"dirloom --format mermaid",
		"dirloom --format graphviz",
		"dirloom --format d2",
		"--format dot",
		"ContractVersion",
		"maxNodes",
		"structure",
		"no image rendering",
	} {
		if !strings.Contains(text, contract) {
			t.Errorf("graphical export guide is missing %q", contract)
		}
	}
}
