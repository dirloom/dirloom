package cli

import (
	"bytes"
	"os"
	"path/filepath"
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

// withGoldenMetadata fills the metadata Validate requires; the human renderer
// never prints it.
func withGoldenMetadata(models map[string]comparison.StructuralDiff) map[string]comparison.StructuralDiff {
	ref := comparison.SourceRef{Kind: "memory", NodeCount: 1}
	for name, model := range models {
		model.Metadata = comparison.Metadata{
			ComparisonVersion:         comparison.Version,
			IdentityProjectionVersion: comparison.IdentityProjectionVersion,
			A:                         ref,
			B:                         ref,
		}
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
}
