package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/comparison"
)

// Diff Result Schema v2 goldens are the current output. The v1 goldens are
// frozen A4 references: no code emits v1 any more, and --no-moves must
// reproduce their change lists.
const (
	diffGoldenV1 = "v1"
	diffGoldenV2 = "v2"
)

func readDiffGolden(t *testing.T, version, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "diff", version, "golden", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func assertDiffGolden(t *testing.T, name, stdout string) {
	t.Helper()
	if want := readDiffGolden(t, diffGoldenV2, name); stdout != want {
		t.Fatalf("%s\n got %q\nwant %q", name, stdout, want)
	}
}

func goldenNodeState(kind artifact.Kind, target *string) *comparison.NodeState {
	return &comparison.NodeState{Kind: kind, Target: target}
}

// withGoldenMetadata fills the versions and source refs Validate requires; the
// human renderer never prints them.
func withGoldenMetadata(models map[string]comparison.StructuralDiff, moveDetection comparison.MoveDetection) map[string]comparison.StructuralDiff {
	ref := comparison.SourceRef{Kind: "memory", NodeCount: 1}
	for name, model := range models {
		model.Metadata = comparison.Metadata{
			ComparisonVersion:         comparison.Version,
			IdentityProjectionVersion: comparison.IdentityProjectionVersion,
			MoveDetection:             moveDetection,
		}
		model.SourceA = ref
		model.SourceB = ref
		models[name] = model
	}
	return models
}

func goldenTarget(value string) *string { return &value }

// a4HumanGoldenModels are the renderer inputs of the frozen v1 human goldens.
func a4HumanGoldenModels() map[string]comparison.StructuralDiff {
	file := artifact.KindFile
	directory := artifact.KindDirectory
	symlink := artifact.KindSymlink
	return map[string]comparison.StructuralDiff{
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
}

// moveHumanGoldenModels mirror the structural-v1 reference outcomes.
func moveHumanGoldenModels() map[string]comparison.StructuralDiff {
	file := goldenNodeState(artifact.KindFile, nil)
	directory := goldenNodeState(artifact.KindDirectory, nil)
	moved := func(from, to string, state *comparison.NodeState, evidence comparison.Evidence, nodes int) comparison.Change {
		return comparison.Change{
			Path: artifact.Path(to), Op: comparison.OpMoved, From: artifact.Path(from),
			Before: state, After: state, Evidence: evidence, Nodes: nodes,
		}
	}
	return map[string]comparison.StructuralDiff{
		"move-subtree.txt": {
			Changes: []comparison.Change{
				{Path: "src/features", Op: comparison.OpAdded, After: directory},
				moved("src/old/payment", "src/features/payment", directory, comparison.EvidenceSubtree, 4),
			},
			Summary: comparison.Summary{Added: 1, Moved: 1, Total: 2},
		},
		"move-file.txt": {
			Changes: []comparison.Change{
				moved("docs/old/guide.md", "docs/guide.md", file, comparison.EvidenceName, 1),
			},
			Summary: comparison.Summary{Moved: 1, Total: 1},
		},
		"rename-in-place.txt": {
			Changes: []comparison.Change{
				moved("src/payment", "src/billing", directory, comparison.EvidenceSubtree, 3),
			},
			Summary: comparison.Summary{Moved: 1, Total: 1},
		},
		"partial-rewrite.txt": {
			Changes: []comparison.Change{
				{Path: "src/features", Op: comparison.OpAdded, After: directory},
				{Path: "src/features/payment", Op: comparison.OpAdded, After: directory},
				moved("src/old/payment/api", "src/features/payment/api", directory, comparison.EvidenceSubtree, 2),
				{Path: "src/features/payment/new.go", Op: comparison.OpAdded, After: file},
				moved("src/old/payment/service.go", "src/features/payment/service.go", file, comparison.EvidenceName, 1),
				{Path: "src/old/payment", Op: comparison.OpRemoved, Before: directory},
			},
			Summary: comparison.Summary{Added: 3, Removed: 1, Moved: 2, Total: 6},
		},
	}
}

func renderDiffText(t *testing.T, name string, model comparison.StructuralDiff) string {
	t.Helper()
	if err := model.Validate(); err != nil {
		t.Fatalf("%s model: %v", name, err)
	}
	var buf bytes.Buffer
	if err := writeDiffText(&buf, model); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestDiffHumanGoldens(t *testing.T) {
	models := a4HumanGoldenModels()
	for name, model := range moveHumanGoldenModels() {
		models[name] = model
	}
	for name, model := range withGoldenMetadata(models, comparison.MoveDetectionStructuralV1) {
		assertDiffGolden(t, name, renderDiffText(t, name, model))
	}
}

// TestDiffHumanNoMovesMatchesFrozenV1 proves that without move detection the
// human output is A4's, except for the "0 moved" count of the summary line.
func TestDiffHumanNoMovesMatchesFrozenV1(t *testing.T) {
	for name, model := range withGoldenMetadata(a4HumanGoldenModels(), comparison.MoveDetectionNone) {
		want := strings.Replace(readDiffGolden(t, diffGoldenV1, name), " changed, ", " changed, 0 moved, ", 1)
		if got := renderDiffText(t, name, model); got != want {
			t.Fatalf("%s\n got %q\nwant %q", name, got, want)
		}
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

	base, root := movedPaymentTree(t)
	stdout, stderr, code = executeForTest(t, "diff", "snapshot:"+base, "live:"+root, "--format", "json")
	if code != 1 || stderr != "" {
		t.Fatalf("moved=(%q, %q, %d)", stdout, stderr, code)
	}
	assertDiffGolden(t, "moved.json", stdout)

	stdout, stderr, code = executeForTest(t, "diff", "snapshot:"+base, "live:"+root, "--format", "json", "--no-moves")
	if code != 1 || stderr != "" {
		t.Fatalf("moved without detection=(%q, %q, %d)", stdout, stderr, code)
	}
	assertDiffGolden(t, "moved-no-moves.json", stdout)

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
	assertDiffJSONContract(t, readDiffGolden(t, diffGoldenV2, "no-differences.json"), true, comparison.MoveDetectionStructuralV1)
	assertDiffJSONContract(t, readDiffGolden(t, diffGoldenV2, "differences.json"), false, comparison.MoveDetectionStructuralV1)
	assertDiffJSONContract(t, readDiffGolden(t, diffGoldenV2, "moved.json"), false, comparison.MoveDetectionStructuralV1)
	assertDiffJSONContract(t, readDiffGolden(t, diffGoldenV2, "moved-no-moves.json"), false, comparison.MoveDetectionNone)
}

// TestDiffNoMovesMatchesFrozenV1JSON replays the input of every frozen v1
// JSON golden with --no-moves. Apart from the v2 envelope (schemaVersion,
// metadata.moveDetection, summary.moved), the document must be A4's.
func TestDiffNoMovesMatchesFrozenV1JSON(t *testing.T) {
	valid := func(name string) string { return diffFixture(t, filepath.Join("valid", name)) }
	empty := "snapshot:" + valid("empty.dlm.json")
	cases := []struct {
		golden string
		a, b   string
		code   int
	}{
		{"no-differences.json", empty, empty, 0},
		{"differences.json", "snapshot:" + valid("single-file.dlm.json"), "snapshot:" + valid("nested.dlm.json"), 1},
		{"invalid-snapshot.json", "snapshot:" + diffFixture(t, filepath.Join("invalid", "fingerprint-mismatch.dlm.json")), empty, 3},
		{"unsupported-snapshot.json", "snapshot:" + diffFixture(t, filepath.Join("invalid", "unknown-schema.dlm.json")), empty, 4},
		{"observation-error.json", empty, "live:definitely-missing-dir", 5},
		{"snapshot-read-error.json", "snapshot:missing.dlm.json", empty, 5},
	}
	for _, tc := range cases {
		stdout, stderr, code := executeForTest(t, "diff", tc.a, tc.b, "--format", "json", "--no-moves")
		if code != tc.code || stderr != "" {
			t.Fatalf("%s=(%q, %q, %d)", tc.golden, stdout, stderr, code)
		}
		live := decodeDiffDocument(t, stdout)
		frozen := decodeDiffDocument(t, readDiffGolden(t, diffGoldenV1, tc.golden))
		if live["schemaVersion"] != float64(2) || frozen["schemaVersion"] != float64(1) {
			t.Fatalf("%s schema versions = %v, %v", tc.golden, live["schemaVersion"], frozen["schemaVersion"])
		}
		delete(live, "schemaVersion")
		delete(frozen, "schemaVersion")
		if metadata, ok := live["metadata"].(map[string]any); ok {
			if metadata["moveDetection"] != string(comparison.MoveDetectionNone) {
				t.Fatalf("%s metadata = %#v", tc.golden, metadata)
			}
			delete(metadata, "moveDetection")
		}
		if summary, ok := live["summary"].(map[string]any); ok {
			if summary["moved"] != float64(0) {
				t.Fatalf("%s summary = %#v", tc.golden, summary)
			}
			delete(summary, "moved")
		}
		normalizeSnapshotReadMessage(live, frozen)
		if !reflect.DeepEqual(live, frozen) {
			t.Fatalf("%s\n live   %#v\n frozen %#v", tc.golden, live, frozen)
		}
	}
}

func decodeDiffDocument(t *testing.T, data string) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(data), &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

// snapshotReadPrefix precedes the host OS cause in a snapshot read failure;
// that cause is not part of the frozen schema.
const snapshotReadPrefix = "snapshot read failure: open missing.dlm.json: "

func normalizeSnapshotReadMessage(live, frozen map[string]any) {
	liveDiagnostic, ok := live["diagnostic"].(map[string]any)
	if !ok {
		return
	}
	frozenDiagnostic, ok := frozen["diagnostic"].(map[string]any)
	if !ok {
		return
	}
	liveMessage, _ := liveDiagnostic["message"].(string)
	frozenMessage, _ := frozenDiagnostic["message"].(string)
	if strings.HasPrefix(liveMessage, snapshotReadPrefix) && strings.HasPrefix(frozenMessage, snapshotReadPrefix) {
		liveDiagnostic["message"] = frozenMessage
	}
}

// movedPaymentTree snapshots src/old/payment (api/handler.go, service.go),
// then moves it to src/features/payment. The snapshot lives outside the root.
func movedPaymentTree(t *testing.T) (snapshotPath, root string) {
	t.Helper()
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src", "old", "payment", "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{filepath.Join("api", "handler.go"), "service.go"} {
		if err := os.WriteFile(filepath.Join(root, "src", "old", "payment", name), []byte("package payment\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	snapshotPath = filepath.Join(t.TempDir(), "base.dlm.json")
	if stdout, stderr, code := executeForTest(t, "snapshot", root, "--no-config", "--output", snapshotPath); code != 0 {
		t.Fatalf("snapshot=(%q, %q, %d)", stdout, stderr, code)
	}
	if err := os.MkdirAll(filepath.Join(root, "src", "features"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "src", "old", "payment"), filepath.Join(root, "src", "features", "payment")); err != nil {
		t.Fatal(err)
	}
	return snapshotPath, root
}

// assertSnapshotReadErrorGolden locks SNAPSHOT_READ_ERROR in the committed
// golden and in the live JSON document. The host OS supplies the cause after
// snapshotReadPrefix; that suffix is not part of the frozen schema, so it is
// rewritten to the golden's message before the byte comparison.
func assertSnapshotReadErrorGolden(t *testing.T, stdout string) {
	t.Helper()
	golden := readDiffGolden(t, diffGoldenV2, "snapshot-read-error.json")
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
	if !strings.HasPrefix(live.Diagnostic.Message, snapshotReadPrefix) {
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
	if !strings.HasPrefix(want.Diagnostic.Message, snapshotReadPrefix) {
		t.Fatalf("golden message = %q", want.Diagnostic.Message)
	}
	normalized := strings.Replace(stdout, live.Diagnostic.Message, want.Diagnostic.Message, 1)
	if normalized != golden {
		t.Fatalf("snapshot-read-error.json\n got %q\nwant %q", normalized, golden)
	}
}
