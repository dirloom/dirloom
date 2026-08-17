package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/diagram"
)

func TestCLIDiagramFormatsAndAlias(t *testing.T) {
	root := filepath.Join(t.TempDir(), "diagram project é")
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	mermaid, stderr, code := executeForTest(t, root, "--no-config", "--format", "mermaid")
	if code != 0 || stderr != "" || !strings.Contains(mermaid, "flowchart TB") || !strings.Contains(mermaid, "dirloom-diagram-contract: 1") {
		t.Fatalf("mermaid=(%q,%q,%d)", mermaid, stderr, code)
	}
	graphviz, stderr, code := executeForTest(t, root, "--no-config", "--format", "graphviz")
	if code != 0 || stderr != "" || !strings.Contains(graphviz, "strict digraph dirloom") {
		t.Fatalf("graphviz=(%q,%q,%d)", graphviz, stderr, code)
	}
	alias, stderr, code := executeForTest(t, root, "--no-config", "--format", "dot")
	if code != 0 || stderr != "" || alias != graphviz {
		t.Fatalf("dot alias diverged\ngraphviz=%q\nalias=%q stderr=%q code=%d", graphviz, alias, stderr, code)
	}
	d2, stderr, code := executeForTest(t, root, "--no-config", "--format", "d2", "--diagram-direction", "left-right")
	if code != 0 || stderr != "" || !strings.Contains(d2, "direction: right") {
		t.Fatalf("d2=(%q,%q,%d)", d2, stderr, code)
	}
	if strings.ContainsRune(mermaid, '\x1b') || strings.ContainsRune(graphviz, '\x1b') || strings.ContainsRune(d2, '\x1b') {
		t.Fatal("diagram formats must stay canonical")
	}
}

func TestCLIDiagramRejectsIncompatibleFlagsAndPreservesDestination(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "visible.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "structure.mmd")
	if err := os.WriteFile(output, []byte("preserved"), 0o644); err != nil {
		t.Fatal(err)
	}

	invalid := [][]string{
		{root, "--no-config", "--format", "text", "--diagram-view", "structure"},
		{root, "--no-config", "--format", "json", "--diagram-direction", "left-right"},
		{root, "--no-config", "--format", "markdown", "--diagram-max-nodes", "10"},
		{root, "--no-config", "--format", "mermaid", "--style", "ascii"},
		{root, "--no-config", "--format", "graphviz", "--theme", "default"},
		{root, "--no-config", "--format", "d2", "--icons", "unicode"},
		{root, "--no-config", "--format", "mermaid", "--color", "always"},
		{root, "--no-config", "--diagram-view", ""},
	}
	for _, args := range invalid {
		stdout, stderr, code := executeForTest(t, args...)
		if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "Error: ") {
			t.Fatalf("%#v=(%q,%q,%d)", args, stdout, stderr, code)
		}
	}

	stdout, stderr, code := executeForTest(t, root, "--no-config", "--format", "mermaid", "--diagram-max-nodes", "1", "--output", output)
	if code != 1 || stdout != "" || !strings.Contains(stderr, "exceeding the explicit limit") {
		t.Fatalf("explicit limit=(%q,%q,%d)", stdout, stderr, code)
	}
	data, err := os.ReadFile(output)
	if err != nil || string(data) != "preserved" {
		t.Fatalf("destination changed after diagram limit: data=%q err=%v", data, err)
	}

	stdout, stderr, code = executeForTest(t, root, "--no-config", "--format", "mermaid", "--color", "never", "--icons", "never")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "flowchart TB") {
		t.Fatalf("neutral diagram flags=(%q,%q,%d)", stdout, stderr, code)
	}
}

func TestCLIDiagramDoesNotInferFormatFromExtension(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "visible.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "structure.mmd")
	stdout, stderr, code := executeForTest(t, root, "--no-config", "--output", output)
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("inferred?=(%q,%q,%d)", stdout, stderr, code)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "flowchart") || strings.Contains(text, "digraph") || !strings.Contains(text, "visible.txt") {
		t.Fatalf("extension inferred a diagram format: %q", text)
	}
}

func TestCLIDiagramFileMatchesStdoutAndWarnsAtThreshold(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "visible.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := executeForTest(t, root, "--no-config", "--format", "d2")
	if code != 0 || stderr != "" {
		t.Fatalf("stdout render=(%q,%q,%d)", stdout, stderr, code)
	}
	output := filepath.Join(t.TempDir(), "structure.d2")
	fileStdout, fileStderr, fileCode := executeForTest(t, root, "--no-config", "--format", "d2", "--output", output)
	if fileCode != 0 || fileStdout != "" || fileStderr != "" {
		t.Fatalf("file render=(%q,%q,%d)", fileStdout, fileStderr, fileCode)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != stdout {
		t.Fatalf("file bytes differ from stdout\nfile=%q\nout=%q", data, stdout)
	}

	wide := t.TempDir()
	for index := 0; index < diagram.LargeGraphWarningThreshold; index++ {
		name := filepath.Join(wide, "file-"+strconv.Itoa(index)+".txt")
		if err := os.WriteFile(name, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, warnStderr, warnCode := executeForTest(t, wide, "--no-config", "--no-default-ignore", "--format", "mermaid")
	if warnCode != 0 || !strings.Contains(warnStderr, "Warning: diagram contains") {
		t.Fatalf("warning=(%q,%d)", warnStderr, warnCode)
	}
}

func TestCLIConfigExplainMarksInheritedDiagramOptionsInactive(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, filepath.Join(root, ".dirloom.yaml"), `schemaVersion: 1
defaults:
  format: json
diagram:
  direction: left-right
  maxNodes: 25
`)
	stdout, stderr, code := executeForTest(t, "config", "explain", root)
	if code != 0 || stderr != "" {
		t.Fatalf("explain=(%q,%q,%d)", stdout, stderr, code)
	}
	for _, want := range []string{
		"format: json (project:",
		"diagram.direction: left-right (project:",
		"inactive for json",
		"diagram.maxNodes: 25 (project:",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("explanation missing %q\n%s", want, stdout)
		}
	}
}
