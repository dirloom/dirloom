package render

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/dirloom/dirloom/internal/diagram"
	"github.com/dirloom/dirloom/internal/tree"
)

func TestDiagramGoldensAndParity(t *testing.T) {
	document, err := diagram.ProjectStructure(sampleTree(), diagram.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}

	outputs := map[string]string{
		"mermaid":  mustRenderDiagram(t, RenderMermaid, document),
		"graphviz": mustRenderDiagram(t, RenderGraphviz, document),
		"d2":       mustRenderDiagram(t, RenderD2, document),
	}
	for name, actual := range outputs {
		assertPortableLineEndings(t, []byte(actual))
		if !strings.Contains(actual, "dirloom-diagram-contract: 1") || !strings.Contains(actual, "view: structure") {
			t.Errorf("%s is missing the stable contract header", name)
		}
		goldenPath := filepath.Join("testdata", name+".golden")
		if os.Getenv("DIRLOOM_UPDATE_GOLDEN") == "1" {
			if err := os.WriteFile(goldenPath, []byte(actual), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		want, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatal(err)
		}
		if actual != string(want) {
			t.Fatalf("%s golden mismatch\n--- got ---\n%s--- want ---\n%s", name, actual, want)
		}
		assertDiagramContainsDocumentOrder(t, name, actual, document)
	}

	left, err := diagram.ProjectStructure(sampleTree(), diagram.Options{View: diagram.ViewStructure, Direction: diagram.DirectionLeftRight})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mustRenderDiagram(t, RenderMermaid, left), "flowchart LR") {
		t.Fatal("left-right Mermaid must emit flowchart LR")
	}
}

func TestDiagramFactoryAcceptsDotAliasAndRejectsTreeLeak(t *testing.T) {
	renderer, err := New("dot", StyleUnicode)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := renderer.Render(&output, sampleTree()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "strict digraph dirloom") {
		t.Fatalf("dot alias output = %q", output.String())
	}

	for _, name := range []string{"mermaid.go", "graphviz.go", "d2.go", "diagram_escape.go"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte("internal/tree")) {
			t.Errorf("%s must not import tree.Node", name)
		}
	}
}

func TestDiagramExplicitLimitDoesNotTruncate(t *testing.T) {
	limit := 2
	renderer, err := NewConfigured(Options{Format: FormatMermaid, Diagram: diagram.Options{MaxNodes: &limit}})
	if err != nil {
		t.Fatal(err)
	}
	err = renderer.Render(io.Discard, sampleTree())
	if err == nil || !strings.Contains(err.Error(), "exceeding the explicit limit") {
		t.Fatalf("limit error = %v", err)
	}
}

func TestDiagramEscapersNeutralizeHostileNames(t *testing.T) {
	for _, name := range hostileDiagramNames() {
		t.Run(fmt.Sprintf("%q", name), func(t *testing.T) {
			document := hostileDocument(name)
			mermaid := mustRenderDiagram(t, RenderMermaid, document)
			graphviz := mustRenderDiagram(t, RenderGraphviz, document)
			d2 := mustRenderDiagram(t, RenderD2, document)
			assertPortableLineEndings(t, []byte(mermaid))
			assertPortableLineEndings(t, []byte(graphviz))
			assertPortableLineEndings(t, []byte(d2))
			if strings.Contains(extractMermaidLabel(t, mermaid), `"`) || strings.Contains(mermaid, `"]; click`) {
				t.Fatalf("Mermaid leaked syntax: %q", mermaid)
			}
			assertQuotedLabelSafe(t, extractDOTLabel(t, graphviz))
			assertQuotedLabelSafe(t, extractD2Label(t, d2))
			if strings.Contains(graphviz, "label=<") || strings.Contains(graphviz, "URL=") {
				t.Fatalf("Graphviz leaked active attributes: %q", graphviz)
			}
			if strings.Contains(d2, "|md") || strings.Contains(d2, "```") {
				t.Fatalf("D2 leaked markdown: %q", d2)
			}
		})
	}
}

func TestDiagramRenderersPropagateWriterFailure(t *testing.T) {
	document, err := diagram.ProjectStructure(sampleTree(), diagram.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	writer := failingDiagramWriter{}
	if err := RenderMermaid(document, writer); err == nil {
		t.Fatal("Mermaid must propagate writer failure")
	}
	if err := RenderGraphviz(document, writer); err == nil {
		t.Fatal("Graphviz must propagate writer failure")
	}
	if err := RenderD2(document, writer); err == nil {
		t.Fatal("D2 must propagate writer failure")
	}
}

func FuzzEscapeMermaidLabel(f *testing.F) {
	for _, seed := range hostileDiagramNames() {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if !utf8.ValidString(value) {
			return
		}
		if strings.Contains(escapeMermaidLabel(value), `"`) {
			t.Fatalf("raw quote leaked from %q", value)
		}
	})
}

func FuzzEscapeDOTLabel(f *testing.F) {
	for _, seed := range hostileDiagramNames() {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if !utf8.ValidString(value) {
			return
		}
		assertQuotedLabelSafe(t, escapeDOTLabel(value))
	})
}

func FuzzEscapeD2Label(f *testing.F) {
	for _, seed := range hostileDiagramNames() {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if !utf8.ValidString(value) {
			return
		}
		assertQuotedLabelSafe(t, escapeD2Label(value))
	})
}

func mustRenderDiagram(t *testing.T, renderFn func(diagram.Document, io.Writer) error, document diagram.Document) string {
	t.Helper()
	var output bytes.Buffer
	if err := renderFn(document, &output); err != nil {
		t.Fatal(err)
	}
	return output.String()
}

func assertDiagramContainsDocumentOrder(t *testing.T, name, encoded string, document diagram.Document) {
	t.Helper()
	last := -1
	for _, node := range document.Nodes {
		index := strings.Index(encoded, node.ID)
		if index < 0 {
			t.Fatalf("%s missing node %s", name, node.ID)
		}
		if index < last {
			t.Fatalf("%s reordered node %s", name, node.ID)
		}
		last = index
	}
	last = -1
	connector := " --> "
	if name != "mermaid" {
		connector = " -> "
	}
	for _, edge := range document.Edges {
		needle := edge.From + connector + edge.To
		index := strings.Index(encoded, needle)
		if index < 0 {
			t.Fatalf("%s missing edge %s", name, needle)
		}
		if index < last {
			t.Fatalf("%s reordered edge %s", name, needle)
		}
		last = index
	}
}

func hostileDocument(label string) diagram.Document {
	return diagram.Document{
		ContractVersion: diagram.ContractVersion,
		View:            diagram.ViewStructure,
		Direction:       diagram.DirectionTopDown,
		Nodes: []diagram.Node{
			{ID: "n_root", Label: "root/", Kind: diagram.NodeDirectory},
			{ID: "n_deadbeefdeadbeefdeadbeefdeadbeef", Label: label, Kind: diagram.NodeFile},
		},
		Edges: []diagram.Edge{{From: "n_root", To: "n_deadbeefdeadbeefdeadbeefdeadbeef", Relation: diagram.RelationContains}},
	}
}

func hostileDiagramNames() []string {
	return []string{
		`foo"]; click n_root "javascript:alert(1)`,
		"end",
		"graph",
		"style",
		`quote"here`,
		`\N\G\E\T\H\L`,
		`back\slash`,
		"with`ticks`",
		"[]{}<>&",
		"line\nfeed",
		"return\rchar",
		"tab\tchar",
		"nul\x00byte",
		"esc\x1bseq",
		"bidi\u202ehidden",
		"café",
		"e\u0301.txt",
		"日本語",
		"проект",
	}
}

func extractMermaidLabel(t *testing.T, encoded string) string {
	t.Helper()
	match := regexp.MustCompile(`n_deadbeefdeadbeefdeadbeefdeadbeef\["(.*)"\]`).FindStringSubmatch(encoded)
	if match == nil {
		t.Fatalf("Mermaid label not found in %q", encoded)
	}
	return match[1]
}

func extractDOTLabel(t *testing.T, encoded string) string {
	t.Helper()
	match := regexp.MustCompile(`n_deadbeefdeadbeefdeadbeefdeadbeef \[label="(.*)"\];`).FindStringSubmatch(encoded)
	if match == nil {
		t.Fatalf("DOT label not found in %q", encoded)
	}
	return match[1]
}

func extractD2Label(t *testing.T, encoded string) string {
	t.Helper()
	match := regexp.MustCompile(`n_deadbeefdeadbeefdeadbeefdeadbeef: "(.*)"`).FindStringSubmatch(encoded)
	if match == nil {
		t.Fatalf("D2 label not found in %q", encoded)
	}
	return match[1]
}

func assertQuotedLabelSafe(t *testing.T, escaped string) {
	t.Helper()
	for index := 0; index < len(escaped); index++ {
		if escaped[index] != '"' {
			continue
		}
		if index == 0 || escaped[index-1] != '\\' {
			t.Fatalf("unescaped quote in %q", escaped)
		}
	}
}

type failingDiagramWriter struct{}

func (failingDiagramWriter) Write([]byte) (int, error) {
	return 0, errors.New("synthetic write failure")
}

func BenchmarkMermaidRenderer(b *testing.B) {
	benchmarkDiagramRenderer(b, FormatMermaid)
}

func BenchmarkGraphvizRenderer(b *testing.B) {
	benchmarkDiagramRenderer(b, FormatGraphviz)
}

func BenchmarkD2Renderer(b *testing.B) {
	benchmarkDiagramRenderer(b, FormatD2)
}

func benchmarkDiagramRenderer(b *testing.B, format string) {
	b.Helper()
	renderer, err := New(format, StyleUnicode)
	if err != nil {
		b.Fatal(err)
	}
	model := largeDiagramTree(2000)
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if err := renderer.Render(io.Discard, model); err != nil {
			b.Fatal(err)
		}
	}
}

func largeDiagramTree(count int) *tree.Node {
	children := make([]*tree.Node, 0, count-1)
	for index := 0; index < count-1; index++ {
		name := fmt.Sprintf("file-%04d.txt", index)
		children = append(children, &tree.Node{Name: name, Path: name, Type: tree.NodeFile})
	}
	return &tree.Node{Name: "wide", Type: tree.NodeDirectory, Children: children}
}
