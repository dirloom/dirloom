package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configuration "github.com/dirloom/dirloom/internal/config"
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

func TestSnapshotStdoutShortWrite(t *testing.T) {
	root := writeFingerprintFixture(t)
	loader := configuration.NewLoader(configuration.WithUserConfigDir(func() (string, error) {
		return "", os.ErrNotExist
	}))
	var want, stderr bytes.Buffer
	args := []string{"snapshot", root, "--no-config"}
	if code := executeWithLoader(context.Background(), args, &want, &stderr, "v0.1.0-test", loader); code != 0 || stderr.Len() != 0 {
		t.Fatalf("baseline=(%s,%d)", stderr.String(), code)
	}
	writer := &limitedWriter{max: 3}
	stderr.Reset()
	if code := executeWithLoader(context.Background(), args, writer, &stderr, "v0.1.0-test", loader); code != 0 || stderr.Len() != 0 {
		t.Fatalf("short write=(%s,%d)", stderr.String(), code)
	}
	if writer.String() != want.String() {
		t.Fatalf("short write dropped bytes: got %d want %d", writer.Len(), want.Len())
	}
}

func TestSnapshotStructuralFlagsConfigAndMutations(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, [][2]string{
		{"src/main.go", "x"},
		{".hidden", "x"},
		{"skip.log", "x"},
		{".gitignore", "*.log\n"},
		{"node_modules/pkg/index.js", "x"},
		{"keep.go", "x"},
		{"generated/out.go", "x"},
	})
	base := snapshotStdout(t, nil, root, "--no-config")
	if snapshotStdout(t, nil, root, "--no-config", "--hidden") == base {
		t.Fatal("--hidden did not change the snapshot")
	}
	depth := snapshotStdout(t, nil, root, "--no-config", "--depth", "1")
	if depth == base || strings.Contains(depth, "main.go") {
		t.Fatal("--depth 1 still contains src/main.go or matched the full snapshot")
	}
	dirs := snapshotStdout(t, nil, root, "--no-config", "--dirs-only")
	if dirs == base || strings.Contains(dirs, `"kind": "file"`) {
		t.Fatal("--dirs-only still contains a file")
	}
	if snapshotStdout(t, nil, root, "--no-config", "--no-gitignore") == base {
		t.Fatal("--no-gitignore did not change the snapshot")
	}
	if snapshotStdout(t, nil, root, "--no-config", "--no-default-ignore") == base {
		t.Fatal("--no-default-ignore did not change the snapshot")
	}

	userDir := t.TempDir()
	writeCLIConfig(t, filepath.Join(userDir, "dirloom", "config.yaml"), "schemaVersion: 1\nignore:\n  - generated\n")
	loader := configuration.NewLoader(configuration.WithUserConfigDir(func() (string, error) { return userDir, nil }))
	fromUser := snapshotStdout(t, loader, root)
	if strings.Contains(fromUser, `"path": "generated"`) {
		t.Fatalf("user config did not ignore generated:\n%s", fromUser)
	}
	withoutUser := snapshotStdout(t, loader, root, "--no-user-config")
	if !strings.Contains(withoutUser, `"path": "generated"`) || withoutUser == fromUser {
		t.Fatal("--no-user-config did not restore generated")
	}

	if err := os.WriteFile(filepath.Join(root, "added.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	added := snapshotStdout(t, nil, root, "--no-config")
	if added == base {
		t.Fatal("addition did not change the snapshot")
	}
	if err := os.Rename(filepath.Join(root, "added.go"), filepath.Join(root, "renamed.go")); err != nil {
		t.Fatal(err)
	}
	renamed := snapshotStdout(t, nil, root, "--no-config")
	if renamed == added {
		t.Fatal("rename did not change the snapshot")
	}
	if err := os.Remove(filepath.Join(root, "renamed.go")); err != nil {
		t.Fatal(err)
	}
	if snapshotStdout(t, nil, root, "--no-config") == renamed {
		t.Fatal("removal did not change the snapshot")
	}

	file := filepath.Join(root, "keep.go")
	stdout, stderr, code := executeForTest(t, "snapshot", file, "--no-config")
	if code == 0 || stdout != "" || !strings.Contains(stderr, "not a directory") {
		t.Fatalf("regular file target=(%q,%q,%d)", stdout, stderr, code)
	}
}

func snapshotStdout(t *testing.T, loader *configuration.Loader, args ...string) string {
	t.Helper()
	var stdout, stderr string
	var code int
	if loader == nil {
		stdout, stderr, code = executeForTest(t, append([]string{"snapshot"}, args...)...)
	} else {
		stdout, stderr, code = executeForTestWithLoader(t, loader, append([]string{"snapshot"}, args...)...)
	}
	if code != 0 || stderr != "" {
		t.Fatalf("%v=(%q,%q,%d)", args, stdout, stderr, code)
	}
	if _, err := snapshot.LoadBytes([]byte(stdout)); err != nil {
		t.Fatal(err)
	}
	return stdout
}

type limitedWriter struct {
	buf bytes.Buffer
	max int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if len(p) > w.max {
		p = p[:w.max]
	}
	return w.buf.Write(p)
}

func (w *limitedWriter) String() string { return w.buf.String() }
func (w *limitedWriter) Len() int       { return w.buf.Len() }
