package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/comparison"
)

func assertDiffGolden(t *testing.T, name, stdout string) {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "diff", "v1", "golden", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if stdout != string(data) {
		t.Fatalf("%s\n got %q\nwant %q", name, stdout, string(data))
	}
}

func goldenNodeState(kind artifact.Kind, target *string) *comparison.NodeState {
	return &comparison.NodeState{Kind: kind, Target: target}
}

// withGoldenMetadata fills the versions and source refs Validate requires; the
// human renderer never prints them.
func withGoldenMetadata(models map[string]comparison.StructuralDiff) map[string]comparison.StructuralDiff {
	ref := comparison.SourceRef{Kind: "memory", NodeCount: 1}
	for name, model := range models {
		model.Metadata = comparison.Metadata{
			ComparisonVersion:         comparison.Version,
			IdentityProjectionVersion: comparison.IdentityProjectionVersion,
		}
		model.SourceA = ref
		model.SourceB = ref
		models[name] = model
	}
	return models
}

func goldenTarget(value string) *string { return &value }

func TestDiffHumanGoldens(t *testing.T) {
	file := artifact.KindFile
	directory := artifact.KindDirectory
	symlink := artifact.KindSymlink

	cases := map[string]comparison.StructuralDiff{
		"empty.txt": {},
		"add.txt": {
			Changes: []comparison.Change{
				{Path: "docs/guide.md", Op: comparison.OpAdded, After: goldenNodeState(file, nil)},
			},
			Summary: comparison.Summary{Added: 1, Total: 1},
		},
		"remove.txt": {
			Changes: []comparison.Change{
				{Path: "src/legacy.go", Op: comparison.OpRemoved, Before: goldenNodeState(file, nil)},
			},
			Summary: comparison.Summary{Removed: 1, Total: 1},
		},
		"changed.txt": {
			Changes: []comparison.Change{
				{Path: "node", Op: comparison.OpChanged, Before: goldenNodeState(file, nil), After: goldenNodeState(directory, nil)},
			},
			Summary: comparison.Summary{Changed: 1, Total: 1},
		},
		"mixed.txt": {
			Changes: []comparison.Change{
				{Path: "a.txt", Op: comparison.OpRemoved, Before: goldenNodeState(file, nil)},
				{Path: "b.txt", Op: comparison.OpAdded, After: goldenNodeState(file, nil)},
				{Path: "link", Op: comparison.OpChanged, Before: goldenNodeState(symlink, goldenTarget("a")), After: goldenNodeState(symlink, goldenTarget("b"))},
			},
			Summary: comparison.Summary{Added: 1, Removed: 1, Changed: 1, Total: 3},
		},
		"symlink-target-change.txt": {
			Changes: []comparison.Change{
				{Path: "link", Op: comparison.OpChanged, Before: goldenNodeState(symlink, goldenTarget("target-a")), After: goldenNodeState(symlink, goldenTarget("target-b"))},
				{Path: "ptr", Op: comparison.OpChanged, Before: goldenNodeState(symlink, goldenTarget("target-a")), After: goldenNodeState(file, nil)},
			},
			Summary: comparison.Summary{Changed: 2, Total: 2},
		},
	}
	for name, model := range withGoldenMetadata(cases) {
		if err := model.Validate(); err != nil {
			t.Fatalf("%s model: %v", name, err)
		}
		var buf bytes.Buffer
		if err := writeDiffText(&buf, model); err != nil {
			t.Fatal(err)
		}
		assertDiffGolden(t, name, buf.String())
	}
}

func TestDiffJSONGoldens(t *testing.T) {
	valid := func(name string) string { return diffFixture(t, filepath.Join("valid", name)) }
	empty := valid("empty.dlm.json")

	stdout, stderr, code := executeForTest(t, "diff", "snapshot:"+empty, "snapshot:"+empty, "--format", "json")
	if code != 0 || stderr != "" {
		t.Fatalf("no-differences=(%q, %q, %d)", stdout, stderr, code)
	}
	assertDiffGolden(t, "no-differences.json", stdout)

	stdout, stderr, code = executeForTest(t, "diff", "snapshot:"+valid("single-file.dlm.json"), "snapshot:"+valid("nested.dlm.json"), "--format", "json")
	if code != 1 || stderr != "" {
		t.Fatalf("differences=(%q, %q, %d)", stdout, stderr, code)
	}
	assertDiffGolden(t, "differences.json", stdout)

	stdout, stderr, code = executeForTest(t, "diff", "snapshot:"+diffFixture(t, filepath.Join("invalid", "fingerprint-mismatch.dlm.json")), "snapshot:"+empty, "--format", "json")
	if code != 3 || stderr != "" {
		t.Fatalf("invalid=(%q, %q, %d)", stdout, stderr, code)
	}
	assertDiffGolden(t, "invalid-snapshot.json", stdout)

	stdout, stderr, code = executeForTest(t, "diff", "snapshot:"+diffFixture(t, filepath.Join("invalid", "unknown-schema.dlm.json")), "snapshot:"+empty, "--format", "json")
	if code != 4 || stderr != "" {
		t.Fatalf("unsupported=(%q, %q, %d)", stdout, stderr, code)
	}
	assertDiffGolden(t, "unsupported-snapshot.json", stdout)

	stdout, stderr, code = executeForTest(t, "diff", "snapshot:"+empty, "live:definitely-missing-dir", "--format", "json")
	if code != 5 || stderr != "" {
		t.Fatalf("observation=(%q, %q, %d)", stdout, stderr, code)
	}
	assertDiffGolden(t, "observation-error.json", stdout)

	stdout, stderr, code = executeForTest(t, "diff", "snapshot:missing.dlm.json", "snapshot:"+empty, "--format", "json")
	if code != 5 || stderr != "" {
		t.Fatalf("snapshot-read=(%q, %q, %d)", stdout, stderr, code)
	}
	assertSnapshotReadErrorGolden(t, stdout)
	assertFrozenDiffJSONContract(t, readGolden(t, "no-differences.json"), true)
	assertFrozenDiffJSONContract(t, readGolden(t, "differences.json"), false)
}

func readGolden(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "diff", "v1", "golden", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// assertSnapshotReadErrorGolden locks SNAPSHOT_READ_ERROR in the committed
// golden and in the live JSON document. The host OS supplies the cause after
// "snapshot read failure: open missing.dlm.json: "; that suffix is not part of
// the frozen schema, so it is rewritten to the golden's message before the
// byte comparison.
func assertSnapshotReadErrorGolden(t *testing.T, stdout string) {
	t.Helper()
	const prefix = "snapshot read failure: open missing.dlm.json: "
	golden := readGolden(t, "snapshot-read-error.json")
	if !strings.Contains(golden, `"status": "SNAPSHOT_READ_ERROR"`) {
		t.Fatal("golden does not encode SNAPSHOT_READ_ERROR")
	}
	var live struct {
		Status     string `json:"status"`
		Diagnostic struct {
			Source  string `json:"source"`
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"diagnostic"`
	}
	if err := json.Unmarshal([]byte(stdout), &live); err != nil {
		t.Fatal(err)
	}
	if live.Status != "SNAPSHOT_READ_ERROR" || live.Diagnostic.Source != "a" || live.Diagnostic.Code != "snapshot_read_failure" {
		t.Fatalf("live = %+v", live)
	}
	if !strings.HasPrefix(live.Diagnostic.Message, prefix) {
		t.Fatalf("message = %q", live.Diagnostic.Message)
	}
	var want struct {
		Diagnostic struct {
			Message string `json:"message"`
		} `json:"diagnostic"`
	}
	if err := json.Unmarshal([]byte(golden), &want); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(want.Diagnostic.Message, prefix) {
		t.Fatalf("golden message = %q", want.Diagnostic.Message)
	}
	normalized := strings.Replace(stdout, live.Diagnostic.Message, want.Diagnostic.Message, 1)
	if normalized != golden {
		t.Fatalf("snapshot-read-error.json\n got %q\nwant %q", normalized, golden)
	}
}
