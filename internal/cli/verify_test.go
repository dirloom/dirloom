package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/app"
	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/clipboard"
	configuration "github.com/dirloom/dirloom/internal/config"
	"github.com/dirloom/dirloom/internal/snapshot"
)

func TestVerifySyntax(t *testing.T) {
	root := writeVerifyTree(t, [][2]string{{"a.txt", "a"}})
	snap := writeVerifySnapshot(t, root)
	stdout, stderr, code := executeForTest(t, "verify", "--help")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "verify <snapshot>") {
		t.Fatalf("help=(%q, %q, %d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTest(t, "verify")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "Error:") {
		t.Fatalf("missing snapshot=(%q, %q, %d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTest(t, "verify", snap, root)
	if code != 0 || stderr != "" || stdout != "Structure matches snapshot.\n" {
		t.Fatalf("positional=(%q, %q, %d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTest(t, "verify", snap, "--root", root)
	if code != 0 || stderr != "" || strings.Contains(stdout, "\x1b") {
		t.Fatalf("root=(%q, %q, %d)", stdout, stderr, code)
	}
	_, stderr, code = executeForTest(t, "verify", snap, root, "--root", root)
	if code != 2 || !strings.Contains(stderr, "mutually") && !strings.Contains(stderr, "cannot be used together") {
		t.Fatalf("exclusive=(%q, %d)", stderr, code)
	}
	_, stderr, code = executeForTest(t, "verify", snap, root, "extra")
	if code != 2 {
		t.Fatalf("extra positional=%d %q", code, stderr)
	}
	_, stderr, code = executeForTest(t, "verify", snap, "--format", "yaml")
	if code != 2 || !strings.Contains(stderr, "format") {
		t.Fatalf("format=(%q, %d)", stderr, code)
	}
}

func TestVerifyRejectsScopeOverrides(t *testing.T) {
	root := writeVerifyTree(t, [][2]string{{"a.txt", "a"}})
	snap := writeVerifySnapshot(t, root)
	flags := [][]string{
		{"--config", "x.yaml"},
		{"--no-user-config"},
		{"--no-config"},
		{"--preset", "docs"},
		{"--depth", "1"},
		{"--dirs-only"},
		{"--hidden"},
		{"--ignore", "x"},
		{"--no-default-ignore"},
		{"--no-gitignore"},
		{"--color", "always"},
		{"--icons", "unicode"},
		{"--theme", "midnight"},
		{"--output", "out.txt"},
		{"--copy"},
	}
	for _, flag := range flags {
		args := append([]string{"verify", snap, "--root", root}, flag...)
		stdout, stderr, code := executeForTest(t, args...)
		if code != 2 || stdout != "" || !strings.Contains(stderr, "Error:") {
			t.Fatalf("%v=(%q, %q, %d)", flag, stdout, stderr, code)
		}
	}
	stdout, stderr, code := executeForTest(t, "--no-config", "verify", snap, "--root", root)
	if code != 2 || stdout != "" || !strings.Contains(stderr, "--no-config") {
		t.Fatalf("inherited=(%q, %q, %d)", stdout, stderr, code)
	}
}

func TestVerifyHumanAndMachineStreams(t *testing.T) {
	root := writeVerifyTree(t, [][2]string{{"a.txt", "a"}})
	snap := writeVerifySnapshot(t, root)
	stdout, stderr, code := executeForTest(t, "verify", snap, "--root", root)
	if code != 0 || stderr != "" || stdout != "Structure matches snapshot.\n" || strings.Contains(stdout, "\x1b") {
		t.Fatalf("match=(%q, %q, %d)", stdout, stderr, code)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = executeForTest(t, "verify", snap, "--root", root)
	if code != 1 || stderr != "" || strings.Contains(stdout, "Error:") || strings.Contains(stdout, "\x1b") {
		t.Fatalf("mismatch=(%q, %q, %d)", stdout, stderr, code)
	}
	lines := strings.Split(stdout, "\n")
	if len(lines) != 4 || lines[0] != "Structure differs from snapshot." || lines[3] != "" {
		t.Fatalf("mismatch lines=%q", stdout)
	}
	if !strings.HasPrefix(lines[1], "Expected: dlm:v1:sha256:") || !strings.HasPrefix(lines[2], "Actual:   dlm:v1:sha256:") {
		t.Fatalf("fingerprint lines=%q", stdout)
	}
	if strings.Contains(stdout, "dirloom diff") || strings.Contains(strings.ToLower(stdout), "renamed") || strings.Contains(strings.ToLower(stdout), "moved") {
		t.Fatalf("mismatch explained a change: %q", stdout)
	}

	stdout, stderr, code = executeForTest(t, "verify", snap, "--root", root, "--format", "json")
	assertVerifyJSON(t, stdout, stderr, code, 1, "MISMATCH", true)
	if strings.Contains(stdout, "\x1b") {
		t.Fatal("ANSI in JSON")
	}
}

func TestVerifyClassifiedExits(t *testing.T) {
	root := t.TempDir()
	invalidDir := filepath.Join("..", "..", "testdata", "snapshots", "v1", "invalid")
	entries, err := os.ReadDir(invalidDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		path := filepath.Join(invalidDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		_, loadErr := snapshot.LoadBytes(data)
		want := 3
		status := "INVALID_SNAPSHOT"
		switch snapshot.CodeOf(loadErr) {
		case snapshot.CodeUnsupportedSchema, snapshot.CodeUnsupportedArtifact, snapshot.CodeUnsupportedFeature:
			want = 4
			status = "UNSUPPORTED_SNAPSHOT"
		}
		stdout, stderr, code := executeForTest(t, "verify", path, "--root", root)
		if code != want || stdout != "" || !strings.Contains(stderr, "Error:") {
			t.Fatalf("%s text=(%q, %q, %d)", entry.Name(), stdout, stderr, code)
		}
		if want == 3 && !strings.Contains(stderr, "invalid snapshot:") {
			t.Fatalf("%s stderr=%q", entry.Name(), stderr)
		}
		if want == 4 && !strings.Contains(stderr, "unsupported snapshot:") {
			t.Fatalf("%s stderr=%q", entry.Name(), stderr)
		}
		if strings.Contains(stderr, "Structure differs") {
			t.Fatalf("%s reported a live mismatch", entry.Name())
		}
		stdout, stderr, code = executeForTest(t, "verify", path, "--root", root, "--format", "json")
		assertVerifyJSON(t, stdout, stderr, code, want, status, false)
		if strings.Contains(stdout, "expectedFingerprint") {
			t.Fatalf("%s exposed an untrusted fingerprint\n%s", entry.Name(), stdout)
		}
	}

	missing := filepath.Join(t.TempDir(), "missing.dlm.json")
	stdout, stderr, code := executeForTest(t, "verify", missing, "--root", root)
	if code != 5 || stdout != "" || !strings.Contains(stderr, "Error:") || !strings.Contains(stderr, "snapshot read failure") {
		t.Fatalf("read=(%q, %q, %d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTest(t, "verify", missing, "--root", root, "--format", "json")
	assertVerifyJSON(t, stdout, stderr, code, 5, "SNAPSHOT_READ_ERROR", false)

	snap := writeVerifySnapshot(t, writeVerifyTree(t, [][2]string{{"a.txt", "a"}}))
	missingRoot := filepath.Join(t.TempDir(), "absent")
	stdout, stderr, code = executeForTest(t, "verify", snap, "--root", missingRoot)
	if code != 5 || stdout != "" || !strings.Contains(stderr, "Error:") {
		t.Fatalf("observe=(%q, %q, %d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTest(t, "verify", snap, "--root", missingRoot, "--format", "json")
	doc := assertVerifyJSON(t, stdout, stderr, code, 5, "OBSERVATION_ERROR", false)
	if doc.ExpectedFingerprint == "" || doc.ActualFingerprint != "" {
		t.Fatalf("observation fingerprints=%+v", doc)
	}
}

func TestVerifyDoesNotReadConfigOrPresentation(t *testing.T) {
	root := writeVerifyTree(t, [][2]string{{"a.txt", "a"}, {".dirloom.yaml", "schemaVersion: [\n"}})
	snap := writeVerifySnapshot(t, root)
	stdout, stderr, code := executeForTest(t, "verify", snap, "--root", root)
	if code != 0 || stderr != "" || stdout != "Structure matches snapshot.\n" {
		t.Fatalf("malformed config=(%q, %q, %d)", stdout, stderr, code)
	}

	config := "schemaVersion: 1\ndefaults:\n  dirsOnly: true\n"
	configured := writeVerifyTree(t, [][2]string{{"a.txt", "a"}, {".dirloom.yaml", config}})
	configuredSnap := writeVerifySnapshot(t, configured)
	stdout, stderr, code = executeForTest(t, "verify", configuredSnap, "--root", configured)
	if code != 0 || stderr != "" {
		t.Fatalf("dirsOnly config=(%q, %q, %d)", stdout, stderr, code)
	}

	var out, errBuf bytes.Buffer
	loader := configuration.NewLoader(configuration.WithUserConfigDir(func() (string, error) {
		return "", errors.New("user configuration disabled in tests")
	}))
	status := execute(context.Background(), []string{"verify", snap, "--root", root, "--format", "text"}, &out, &errBuf, "v0.1.0-test", commandDependencies{
		loader:    loader,
		evaluator: nil,
		clipboard: &clipboard.Buffer{},
	})
	if status != 0 || errBuf.Len() != 0 || strings.Contains(out.String(), "\x1b") {
		t.Fatalf("nil presentation=(%q, %q, %d)", out.String(), errBuf.String(), status)
	}
}

func TestVerifyStdoutWriteFailure(t *testing.T) {
	root := writeVerifyTree(t, [][2]string{{"a.txt", "a"}})
	snap := writeVerifySnapshot(t, root)
	loader := configuration.NewLoader(configuration.WithUserConfigDir(func() (string, error) {
		return "", errors.New("user configuration disabled in tests")
	}))
	var stderr bytes.Buffer
	code := executeWithLoader(context.Background(), []string{"verify", snap, "--root", root, "--format", "json"}, failingWriter{}, &stderr, "v0.1.0-test", loader)
	if code != 5 || !strings.Contains(stderr.String(), "Error: write verify result:") {
		t.Fatalf("write=(%q, %d)", stderr.String(), code)
	}
}

func TestVerifyInternalRendering(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := renderVerifyFailure(&stdout, "text", app.FailureInternal, artifact.InternalError("broken invariant"), app.VerifyResult{})
	code := processStatus(err, &stderr)
	if code != 6 || stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "Error: internal error:") {
		t.Fatalf("text internal=(%q, %q, %d)", stdout.String(), stderr.String(), code)
	}
	stdout.Reset()
	stderr.Reset()
	err = renderVerifyFailure(&stdout, "json", app.FailureInternal, artifact.InternalError("broken invariant"), app.VerifyResult{})
	code = processStatus(err, &stderr)
	assertVerifyJSON(t, stdout.String(), stderr.String(), code, 6, "INTERNAL_ERROR", false)
}

func TestVerifyJSONEscapeAndGoldens(t *testing.T) {
	var buf bytes.Buffer
	if err := writeVerifyJSON(&buf, verifyJSONDocument{
		SchemaVersion: 1,
		Status:        "OBSERVATION_ERROR",
		Diagnostic:    &verifyDiagnostic{Code: "source_read_failure", Message: "a < b & c"},
	}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), `\u003c`) || !strings.Contains(buf.String(), "a < b & c") || !strings.HasSuffix(buf.String(), "\n") {
		t.Fatalf("encoded=%q", buf.String())
	}

	empty := t.TempDir()
	outside := filepath.Join(t.TempDir(), "empty.dlm.json")
	fixture := filepath.Join("..", "..", "testdata", "snapshots", "v1", "valid", "empty.dlm.json")
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, data, 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := executeForTest(t, "verify", outside, "--root", empty, "--format", "json")
	if code != 0 || stderr != "" {
		t.Fatalf("golden match=(%q, %q, %d)", stdout, stderr, code)
	}
	assertGolden(t, "match.json", stdout)

	withFile := writeVerifyTree(t, [][2]string{{"a.txt", "a"}})
	stdout, stderr, code = executeForTest(t, "verify", outside, "--root", withFile, "--format", "json")
	if code != 1 || stderr != "" {
		t.Fatalf("golden mismatch=(%q, %q, %d)", stdout, stderr, code)
	}
	assertGolden(t, "mismatch.json", stdout)

	stdout, stderr, code = executeForTest(t, "verify", filepath.Join("..", "..", "testdata", "snapshots", "v1", "invalid", "fingerprint-mismatch.dlm.json"), "--root", empty, "--format", "json")
	if code != 3 || stderr != "" {
		t.Fatalf("golden invalid=(%q, %q, %d)", stdout, stderr, code)
	}
	assertGolden(t, "invalid-snapshot.json", stdout)

	stdout, stderr, code = executeForTest(t, "verify", filepath.Join("..", "..", "testdata", "snapshots", "v1", "invalid", "unknown-schema.dlm.json"), "--root", empty, "--format", "json")
	if code != 4 || stderr != "" {
		t.Fatalf("golden unsupported=(%q, %q, %d)", stdout, stderr, code)
	}
	assertGolden(t, "unsupported-snapshot.json", stdout)
}

func TestVerifyCLIOutputSelfExclusion(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "architecture.dlm.json")
	stdout, stderr, code := executeForTest(t, "snapshot", root, "--no-config", "--output", output)
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("snapshot=(%q, %q, %d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTest(t, "verify", output, "--root", root)
	if code != 0 || stderr != "" || stdout != "Structure matches snapshot.\n" {
		t.Fatalf("verify=(%q, %q, %d)", stdout, stderr, code)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(wd) }()
	stdout, stderr, code = executeForTest(t, "snapshot", "--no-config", "--output", "architecture.dlm.json")
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("cwd snapshot=(%q, %q, %d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTest(t, "verify", "architecture.dlm.json")
	if code != 0 || stderr != "" || stdout != "Structure matches snapshot.\n" {
		t.Fatalf("relative verify=(%q, %q, %d)", stdout, stderr, code)
	}
}

func TestVerifyCompletions(t *testing.T) {
	stdout, _, code := executeForTest(t, "__complete", "verify", "--format", "")
	if code != 0 || !strings.Contains(stdout, "text") || !strings.Contains(stdout, "json") {
		t.Fatalf("format completion=%q code=%d", stdout, code)
	}
	stdout, _, code = executeForTest(t, "__complete", "verify", "--root", "")
	if code != 0 || !strings.Contains(stdout, ":") {
		t.Fatalf("root completion=%q code=%d", stdout, code)
	}
	stdout, _, code = executeForTest(t, "__complete", "verify", "")
	if code != 0 || !strings.Contains(stdout, ":") {
		t.Fatalf("snapshot completion=%q code=%d", stdout, code)
	}
}

type verifyJSONBody struct {
	SchemaVersion       int    `json:"schemaVersion"`
	Status              string `json:"status"`
	ExpectedFingerprint string `json:"expectedFingerprint"`
	ActualFingerprint   string `json:"actualFingerprint"`
	Diagnostic          *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"diagnostic"`
}

func assertVerifyJSON(t *testing.T, stdout, stderr string, code, wantCode int, status string, bothFingerprints bool) verifyJSONBody {
	t.Helper()
	if code != wantCode || stderr != "" || !strings.HasSuffix(stdout, "\n") || strings.Contains(stdout, "\x1b") {
		t.Fatalf("json code=%d stderr=%q stdout=%q", code, stderr, stdout)
	}
	var doc verifyJSONBody
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.SchemaVersion != 1 || doc.Status != status {
		t.Fatalf("%+v", doc)
	}
	if bothFingerprints && (doc.ExpectedFingerprint == "" || doc.ActualFingerprint == "" || doc.Diagnostic != nil) {
		t.Fatalf("%+v", doc)
	}
	if !bothFingerprints && doc.Diagnostic == nil {
		t.Fatalf("%+v", doc)
	}
	if doc.Diagnostic != nil && (doc.Diagnostic.Code == "" || doc.Diagnostic.Message == "") {
		t.Fatalf("%+v", doc)
	}
	return doc
}

func assertGolden(t *testing.T, name, stdout string) {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "verify", "v1", "golden", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if stdout != string(data) {
		t.Fatalf("%s\n got %q\nwant %q", name, stdout, string(data))
	}
}

func writeVerifyTree(t *testing.T, files [][2]string) string {
	t.Helper()
	root := t.TempDir()
	for _, file := range files {
		path := filepath.Join(root, filepath.FromSlash(file[0]))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(file[1]), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func writeVerifySnapshot(t *testing.T, root string) string {
	t.Helper()
	result, err := app.Snapshot(context.Background(), app.InspectRequest{
		Root: root, UseDefaultIgnores: false, UseGitIgnore: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "architecture.dlm.json")
	if err := os.WriteFile(path, result.Bytes, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
