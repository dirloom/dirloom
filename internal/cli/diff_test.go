package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/app"
	"github.com/dirloom/dirloom/internal/artifact"
	configuration "github.com/dirloom/dirloom/internal/config"
)

func diffFixture(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("..", "..", "testdata", "snapshots", "v1", name)
}

func TestDiffSyntax(t *testing.T) {
	stdout, stderr, code := executeForTest(t, "diff", "--help")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "diff <source-a> <source-b>") {
		t.Fatalf("help=(%q, %q, %d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTest(t, "diff")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "Error:") {
		t.Fatalf("missing=(%q, %q, %d)", stdout, stderr, code)
	}
	_, stderr, code = executeForTest(t, "diff", "snapshot:a.dlm.json")
	if code != 2 || !strings.Contains(stderr, "exactly two sources") {
		t.Fatalf("one source=(%q, %d)", stderr, code)
	}
	_, stderr, code = executeForTest(t, "diff", "snapshot:a", "snapshot:b", "snapshot:c")
	if code != 2 {
		t.Fatalf("extra positional=%d %q", code, stderr)
	}
	_, stderr, code = executeForTest(t, "diff", "snapshot:a", "snapshot:b", "--format", "yaml")
	if code != 2 || !strings.Contains(stderr, "format") {
		t.Fatalf("format=(%q, %d)", stderr, code)
	}
	_, stderr, code = executeForTest(t, "diff", "a.dlm.json", "snapshot:b")
	if code != 2 || !strings.Contains(stderr, "no prefix") {
		t.Fatalf("prefix=(%q, %d)", stderr, code)
	}
	_, stderr, code = executeForTest(t, "diff", "SNAPSHOT:a", "snapshot:b")
	if code != 2 || !strings.Contains(stderr, "unsupported source prefix") {
		t.Fatalf("case=(%q, %d)", stderr, code)
	}
}

func TestDiffRejectsStructuralFlags(t *testing.T) {
	flags := [][]string{
		{"--config", "x.yaml"},
		{"--no-user-config"},
		{"--no-config"},
		{"--root", "."},
		{"--depth", "1"},
		{"--dirs-only"},
		{"--hidden"},
		{"--ignore", "x"},
		{"--no-default-ignore"},
		{"--no-gitignore"},
		{"--preset", "docs"},
		{"--output", "out.txt"},
		{"--copy"},
		{"--color", "always"},
		{"--icons", "unicode"},
		{"--theme", "midnight"},
	}
	for _, flag := range flags {
		args := append([]string{"diff", "snapshot:a.dlm.json", "live:."}, flag...)
		stdout, stderr, code := executeForTest(t, args...)
		if code != 2 || stdout != "" || !strings.Contains(stderr, "Error:") {
			t.Fatalf("%v=(%q, %q, %d)", flag, stdout, stderr, code)
		}
	}
	// Inherited persistent flags are rejected with a dedicated diagnostic.
	stdout, stderr, code := executeForTest(t, "--no-config", "diff", "snapshot:a.dlm.json", "live:.")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "--no-config") {
		t.Fatalf("inherited=(%q, %q, %d)", stdout, stderr, code)
	}
}

func TestDiffHumanAndMachineStreams(t *testing.T) {
	empty := diffFixture(t, filepath.Join("valid", "empty.dlm.json"))
	single := diffFixture(t, filepath.Join("valid", "single-file.dlm.json"))

	stdout, stderr, code := executeForTest(t, "diff", "snapshot:"+empty, "snapshot:"+empty)
	if code != 0 || stderr != "" || stdout != "No structural differences.\n" || strings.Contains(stdout, "\x1b") {
		t.Fatalf("no differences=(%q, %q, %d)", stdout, stderr, code)
	}

	stdout, stderr, code = executeForTest(t, "diff", "snapshot:"+empty, "snapshot:"+single)
	if code != 1 || stderr != "" || strings.Contains(stdout, "Error:") || strings.Contains(stdout, "\x1b") {
		t.Fatalf("differences=(%q, %q, %d)", stdout, stderr, code)
	}
	if !strings.Contains(stdout, "Structural Diff\n") || !strings.Contains(stdout, "Summary: 1 added, 0 removed, 0 changed, 1 total\n") {
		t.Fatalf("header=%q", stdout)
	}
	if !strings.Contains(stdout, "ADDED\n  + a.txt\n") {
		t.Fatalf("added section=%q", stdout)
	}
	if strings.Contains(stdout, "REMOVED") || strings.Contains(stdout, "CHANGED") {
		t.Fatalf("empty sections must be omitted=%q", stdout)
	}

	stdout, stderr, code = executeForTest(t, "diff", "snapshot:"+empty, "snapshot:"+empty, "--format", "json")
	assertDiffJSON(t, stdout, stderr, code, 0, "NO_DIFFERENCES")
	assertFrozenDiffJSONContract(t, stdout, true)

	stdout, stderr, code = executeForTest(t, "diff", "snapshot:"+empty, "snapshot:"+single, "--format", "json")
	doc := assertDiffJSON(t, stdout, stderr, code, 1, "DIFFERENCES")
	if doc.Summary == nil || doc.Summary.Added != 1 || doc.Summary.Total != 1 {
		t.Fatalf("summary=%+v", doc.Summary)
	}
	assertFrozenDiffJSONContract(t, stdout, false)
}

func TestDiffClassifiedExits(t *testing.T) {
	empty := diffFixture(t, filepath.Join("valid", "empty.dlm.json"))
	invalid := diffFixture(t, filepath.Join("invalid", "fingerprint-mismatch.dlm.json"))
	unsupported := diffFixture(t, filepath.Join("invalid", "unknown-schema.dlm.json"))

	stdout, stderr, code := executeForTest(t, "diff", "snapshot:"+invalid, "snapshot:"+empty)
	if code != 3 || stdout != "" || !strings.Contains(stderr, "Error:") || !strings.Contains(stderr, "invalid snapshot:") {
		t.Fatalf("invalid text=(%q, %q, %d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTest(t, "diff", "snapshot:"+invalid, "snapshot:"+empty, "--format", "json")
	doc := assertDiffJSON(t, stdout, stderr, code, 3, "INVALID_SNAPSHOT")
	if doc.Diagnostic == nil || doc.Diagnostic.Source != "a" {
		t.Fatalf("diagnostic=%+v", doc.Diagnostic)
	}

	stdout, stderr, code = executeForTest(t, "diff", "snapshot:"+empty, "snapshot:"+unsupported)
	if code != 4 || stdout != "" || !strings.Contains(stderr, "unsupported snapshot:") {
		t.Fatalf("unsupported text=(%q, %q, %d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTest(t, "diff", "snapshot:"+empty, "snapshot:"+unsupported, "--format", "json")
	doc = assertDiffJSON(t, stdout, stderr, code, 4, "UNSUPPORTED_SNAPSHOT")
	if doc.Diagnostic == nil || doc.Diagnostic.Source != "b" {
		t.Fatalf("diagnostic=%+v", doc.Diagnostic)
	}

	missing := filepath.Join(t.TempDir(), "missing.dlm.json")
	stdout, stderr, code = executeForTest(t, "diff", "snapshot:"+missing, "snapshot:"+empty)
	if code != 5 || stdout != "" || !strings.Contains(stderr, "snapshot read failure") {
		t.Fatalf("read=(%q, %q, %d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTest(t, "diff", "snapshot:"+missing, "snapshot:"+empty, "--format", "json")
	assertDiffJSON(t, stdout, stderr, code, 5, "SNAPSHOT_READ_ERROR")

	missingDir := filepath.Join(t.TempDir(), "absent")
	stdout, stderr, code = executeForTest(t, "diff", "snapshot:"+empty, "live:"+missingDir)
	if code != 5 || stdout != "" || !strings.Contains(stderr, "Error:") {
		t.Fatalf("observe=(%q, %q, %d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTest(t, "diff", "snapshot:"+empty, "live:"+missingDir, "--format", "json")
	doc = assertDiffJSON(t, stdout, stderr, code, 5, "OBSERVATION_ERROR")
	if doc.Diagnostic == nil || doc.Diagnostic.Source != "b" || doc.Diagnostic.Code != "source_read_failure" {
		t.Fatalf("diagnostic=%+v", doc.Diagnostic)
	}
}

func TestDiffUsageFailuresStayHumanUnderJSON(t *testing.T) {
	stdout, stderr, code := executeForTest(t, "diff", "live:.", "live:.", "--format", "json")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "Error:") {
		t.Fatalf("live live=(%q, %q, %d)", stdout, stderr, code)
	}
	if strings.Contains(stderr, "{") {
		t.Fatalf("usage failure leaked JSON: %q", stderr)
	}
}

func TestDiffStdoutWriteFailure(t *testing.T) {
	empty := diffFixture(t, filepath.Join("valid", "empty.dlm.json"))
	loader := configuration.NewLoader(configuration.WithUserConfigDir(func() (string, error) {
		return "", errors.New("user configuration disabled in tests")
	}))
	var stderr bytes.Buffer
	code := executeWithLoader(context.Background(), []string{"diff", "snapshot:" + empty, "snapshot:" + empty, "--format", "json"}, failingWriter{}, &stderr, "v0.1.0-test", loader)
	if code != 5 || !strings.Contains(stderr.String(), "Error: write diff result:") {
		t.Fatalf("write=(%q, %d)", stderr.String(), code)
	}
}

func TestDiffInternalRendering(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := renderDiffFailure(&stdout, "text", app.FailureInternal, artifact.InternalError("broken invariant"))
	code := processStatus(err, &stderr)
	if code != 6 || stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "Error: internal error:") {
		t.Fatalf("text internal=(%q, %q, %d)", stdout.String(), stderr.String(), code)
	}
	stdout.Reset()
	stderr.Reset()
	err = renderDiffFailure(&stdout, "json", app.FailureInternal, artifact.InternalError("broken invariant"))
	code = processStatus(err, &stderr)
	assertDiffJSON(t, stdout.String(), stderr.String(), code, 6, "INTERNAL_ERROR")
}

func TestDiffJSONEscape(t *testing.T) {
	var buf bytes.Buffer
	if err := writeDiffJSON(&buf, diffJSONDocument{
		SchemaVersion: 1,
		Status:        "OBSERVATION_ERROR",
		Diagnostic:    &diffDiagnostic{Code: "source_read_failure", Message: "a < b & c"},
	}); err != nil {
		t.Fatal(err)
	}
	// EscapeHTML is disabled: raw characters stay readable, the escaped
	// form must not appear.
	if strings.Contains(buf.String(), "\\u003c") || !strings.Contains(buf.String(), "a < b & c") || !strings.HasSuffix(buf.String(), "\n") {
		t.Fatalf("encoded=%q", buf.String())
	}
	if strings.Contains(buf.String(), "&amp;") {
		t.Fatalf("HTML escaping leaked: %q", buf.String())
	}
}

func TestDiffCompletions(t *testing.T) {
	stdout, _, code := executeForTest(t, "__complete", "diff", "--format", "")
	if code != 0 || !strings.Contains(stdout, "text") || !strings.Contains(stdout, "json") {
		t.Fatalf("format completion=%q code=%d", stdout, code)
	}
	stdout, _, code = executeForTest(t, "__complete", "diff", "")
	if code != 0 || !strings.Contains(stdout, "live:") || !strings.Contains(stdout, "snapshot:") {
		t.Fatalf("source completion=%q code=%d", stdout, code)
	}
	stdout, _, code = executeForTest(t, "__complete", "diff", "live:")
	if code != 0 {
		t.Fatalf("live completion=%q code=%d", stdout, code)
	}
}

type diffJSONBody struct {
	SchemaVersion int    `json:"schemaVersion"`
	Status        string `json:"status"`
	Summary       *struct {
		Added   int `json:"added"`
		Removed int `json:"removed"`
		Changed int `json:"changed"`
		Total   int `json:"total"`
	} `json:"summary"`
	Diagnostic *struct {
		Source  string `json:"source"`
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"diagnostic"`
}

// assertFrozenDiffJSONContract locks Diff Result Schema v1: top-level sources,
// metadata without a/b, operation instead of op, and changes as [] when empty.
func assertFrozenDiffJSONContract(t *testing.T, stdout string, empty bool) {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	sources, ok := doc["sources"].(map[string]any)
	if !ok || sources["a"] == nil || sources["b"] == nil {
		t.Fatalf("sources = %#v", doc["sources"])
	}
	metadata, ok := doc["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata = %#v", doc["metadata"])
	}
	if _, buried := metadata["a"]; buried {
		t.Fatal("metadata.a must be absent")
	}
	if _, buried := metadata["b"]; buried {
		t.Fatal("metadata.b must be absent")
	}
	if metadata["comparisonVersion"] != float64(1) || metadata["identityProjectionVersion"] != float64(1) {
		t.Fatalf("metadata = %#v", metadata)
	}
	changes, ok := doc["changes"].([]any)
	if !ok {
		t.Fatalf("changes must be an array, got %#v", doc["changes"])
	}
	if empty && len(changes) != 0 {
		t.Fatalf("empty diff changes = %#v", changes)
	}
	if strings.Contains(stdout, `"op"`) {
		t.Fatalf("public field op must be absent: %s", stdout)
	}
	if !empty && !strings.Contains(stdout, `"operation"`) {
		t.Fatalf("public field operation missing: %s", stdout)
	}
	if empty && !strings.Contains(stdout, `"changes": []`) {
		t.Fatalf("changes must be [] not null: %q", stdout)
	}
}

func assertDiffJSON(t *testing.T, stdout, stderr string, code, wantCode int, status string) diffJSONBody {
	t.Helper()
	if code != wantCode || stderr != "" || !strings.HasSuffix(stdout, "\n") || strings.Contains(stdout, "\x1b") {
		t.Fatalf("json code=%d stderr=%q stdout=%q", code, stderr, stdout)
	}
	var doc diffJSONBody
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.SchemaVersion != 1 || doc.Status != status {
		t.Fatalf("%+v", doc)
	}
	return doc
}
