package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/snapshot"
)

func TestSnapshotHelp(t *testing.T) {
	stdout, stderr, code := executeForTest(t, "snapshot", "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("help=(%q,%q,%d)", stdout, stderr, code)
	}
	for _, want := range []string{"structural", "--output", "--root", "--ignore", "fingerprint"} {
		if !strings.Contains(strings.ToLower(stdout), strings.ToLower(want)) {
			t.Errorf("help missing %q", want)
		}
	}
}

func TestSnapshotStdoutAndRoot(t *testing.T) {
	root := writeFingerprintFixture(t)
	stdout, stderr, code := executeForTest(t, "snapshot", root, "--no-config")
	if code != 0 || stderr != "" {
		t.Fatalf("stdout=(%q,%q,%d)", stdout, stderr, code)
	}
	if strings.ContainsRune(stdout, '\x1b') {
		t.Fatal("ANSI in snapshot")
	}
	loaded, err := snapshot.LoadBytes([]byte(stdout))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Document.SchemaVersion != 1 || loaded.Document.ArtifactVersion != 1 {
		t.Fatalf("%#v", loaded.Document)
	}
	viaRoot, stderr, code := executeForTest(t, "snapshot", "--root", root, "--no-config")
	if code != 0 || stderr != "" || viaRoot != stdout {
		t.Fatalf("root=(%q,%q,%d)", viaRoot, stderr, code)
	}
}

func TestSnapshotOutputSelfExclusion(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "keep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "architecture.dlm.json")
	stdout, stderr, code := executeForTest(t, "snapshot", root, "--no-config", "--output", out)
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("first=(%q,%q,%d)", stdout, stderr, code)
	}
	first, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := snapshot.LoadBytes(first)
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range loaded.Document.Artifact.Nodes {
		if strings.Contains(node.Path, "architecture.dlm.json") {
			t.Fatalf("output included in artifact: %#v", node)
		}
	}
	stdout, stderr, code = executeForTest(t, "snapshot", root, "--no-config", "--output", out)
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("second=(%q,%q,%d)", stdout, stderr, code)
	}
	second, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("repeat output changed snapshot")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Fatalf("temp leak %s", entry.Name())
		}
	}
}

func TestSnapshotPresentationInvariance(t *testing.T) {
	root := writeFingerprintFixture(t)
	baseline, stderr, code := executeForTest(t, "snapshot", root, "--no-config")
	if code != 0 || stderr != "" {
		t.Fatal(stderr)
	}
	variants := [][]string{
		{root, "--no-config", "--theme", "midnight"},
		{root, "--no-config", "--icons", "nerd"},
		{root, "--no-config", "--color", "always"},
	}
	for _, args := range variants {
		stdout, stderr, code := executeForTest(t, append([]string{"snapshot"}, args...)...)
		if code != 0 || stderr != "" || stdout != baseline {
			t.Fatalf("%v changed snapshot", args)
		}
	}
}

func TestSnapshotUsageAndOutputConflicts(t *testing.T) {
	root := writeFingerprintFixture(t)
	_, stderr, code := executeForTest(t, "snapshot", root, "--root", root)
	if code != 2 || !strings.Contains(stderr, "--root") {
		t.Fatalf("mutual exclusion=(%q,%d)", stderr, code)
	}
	outDir := t.TempDir()
	_, stderr, code = executeForTest(t, "snapshot", root, "--no-config", "--output", outDir)
	if code != 1 {
		t.Fatalf("directory output should fail: %q code=%d", stderr, code)
	}
}

func TestSnapshotFilterConfigParity(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, filepath.Join(root, ".dirloom.yaml"), "schemaVersion: 1\nignore:\n  - generated\n")
	if err := os.Mkdir(filepath.Join(root, "generated"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "generated", "out.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "keep.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	configured, stderr, code := executeForTest(t, "snapshot", root)
	if code != 0 || stderr != "" {
		t.Fatalf("%q %d", stderr, code)
	}
	ignored, _, code := executeForTest(t, "snapshot", root, "--no-config", "--ignore", "generated")
	if code != 0 || ignored != configured {
		t.Fatal("config vs CLI ignore mismatch")
	}
	var document map[string]any
	if err := json.Unmarshal([]byte(configured), &document); err != nil {
		t.Fatal(err)
	}
	capture := document["capture"].(map[string]any)
	ignore := capture["ignore"].([]any)
	if len(ignore) != 1 || ignore[0].(string) != "generated" {
		t.Fatalf("%#v", capture)
	}
}
