package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/dirloom/dirloom/internal/snapshot"
	"github.com/dirloom/dirloom/internal/source"
)

func TestParseDiffSource(t *testing.T) {
	valid := map[string]DiffSourceSpec{
		"snapshot:a.dlm.json":          {Kind: DiffSourceSnapshot, Path: "a.dlm.json"},
		"live:dir":                     {Kind: DiffSourceLive, Path: "dir"},
		`snapshot:C:\data\x.dlm.json`:  {Kind: DiffSourceSnapshot, Path: `C:\data\x.dlm.json`},
		"live:relative:with:colons":    {Kind: DiffSourceLive, Path: "relative:with:colons"},
		"snapshot::leading-colon-path": {Kind: DiffSourceSnapshot, Path: ":leading-colon-path"},
		"live:.":                       {Kind: DiffSourceLive, Path: "."},
	}
	for expression, want := range valid {
		got, err := ParseDiffSource(expression)
		if err != nil {
			t.Fatalf("%q: %v", expression, err)
		}
		if got != want {
			t.Fatalf("%q = %#v, want %#v", expression, got, want)
		}
	}
	invalid := []string{
		"a.dlm.json",           // no prefix
		":path",                // empty prefix
		"snapshot:",            // empty path
		"live:",                // empty path
		"SNAPSHOT:x",           // prefixes are case-sensitive
		"Snapshot:x",           // prefixes are case-sensitive
		"LIVE:dir",             // prefixes are case-sensitive
		"git:HEAD",             // unsupported prefix
		" snapshot:a.dlm.json", // leading space is part of the prefix
	}
	for _, expression := range invalid {
		if _, err := ParseDiffSource(expression); err == nil {
			t.Fatalf("%q unexpectedly valid", expression)
		} else if !DiffUsage(err) {
			t.Fatalf("%q error %v is not a usage failure", expression, err)
		}
	}
}

func mustDiffSnapshotFile(t *testing.T, dir, name string, raw []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustDiffSide(t *testing.T, err error) comparison.Side {
	t.Helper()
	side, ok := comparison.SideOf(err)
	if !ok {
		t.Fatalf("%v has no side", err)
	}
	return side
}

func mustDiff(t *testing.T, request DiffRequest) comparison.StructuralDiff {
	t.Helper()
	diff, err := Diff(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	return diff
}

func TestDiffSourceCombinations(t *testing.T) {
	store := t.TempDir()
	root := writeTree(t, t.TempDir(), [][2]string{{"src/main.go", "package main\n"}, {"README.md", "hi\n"}})
	snapA := mustDiffSnapshotFile(t, store, "a.dlm.json", mustFilesystemSnapshot(t, root))
	snapB := mustDiffSnapshotFile(t, store, "b.dlm.json", mustFilesystemSnapshot(t, root))

	t.Run("snapshot against snapshot", func(t *testing.T) {
		diff := mustDiff(t, DiffRequest{
			A: DiffSourceSpec{Kind: DiffSourceSnapshot, Path: snapA},
			B: DiffSourceSpec{Kind: DiffSourceSnapshot, Path: snapB},
		})
		if DiffStatusOf(diff) != DiffNoDifferences {
			t.Fatalf("status = %s", DiffStatusOf(diff))
		}
		if diff.Metadata.A.Kind != source.KindSnapshot || diff.Metadata.B.Kind != source.KindSnapshot {
			t.Fatalf("kinds = %s %s", diff.Metadata.A.Kind, diff.Metadata.B.Kind)
		}
	})
	t.Run("snapshot against live", func(t *testing.T) {
		diff := mustDiff(t, DiffRequest{
			A: DiffSourceSpec{Kind: DiffSourceSnapshot, Path: snapA},
			B: DiffSourceSpec{Kind: DiffSourceLive, Path: root},
		})
		if DiffStatusOf(diff) != DiffNoDifferences {
			t.Fatalf("status = %s", DiffStatusOf(diff))
		}
		if diff.Metadata.A.Kind != source.KindSnapshot || diff.Metadata.B.Kind != source.KindFilesystem {
			t.Fatalf("kinds = %s %s", diff.Metadata.A.Kind, diff.Metadata.B.Kind)
		}
	})
	t.Run("live against snapshot", func(t *testing.T) {
		diff := mustDiff(t, DiffRequest{
			A: DiffSourceSpec{Kind: DiffSourceLive, Path: root},
			B: DiffSourceSpec{Kind: DiffSourceSnapshot, Path: snapB},
		})
		if DiffStatusOf(diff) != DiffNoDifferences {
			t.Fatalf("status = %s", DiffStatusOf(diff))
		}
		if diff.Metadata.A.Kind != source.KindFilesystem || diff.Metadata.B.Kind != source.KindSnapshot {
			t.Fatalf("kinds = %s %s", diff.Metadata.A.Kind, diff.Metadata.B.Kind)
		}
	})
	t.Run("live against live is a usage failure", func(t *testing.T) {
		_, err := Diff(context.Background(), DiffRequest{
			A: DiffSourceSpec{Kind: DiffSourceLive, Path: root},
			B: DiffSourceSpec{Kind: DiffSourceLive, Path: root},
		})
		if err == nil || !DiffUsage(err) {
			t.Fatalf("live against live = %v", err)
		}
	})
	t.Run("empty spec is a usage failure", func(t *testing.T) {
		_, err := Diff(context.Background(), DiffRequest{
			A: DiffSourceSpec{Kind: DiffSourceSnapshot, Path: snapA},
			B: DiffSourceSpec{},
		})
		if !DiffUsage(err) {
			t.Fatalf("empty spec = %v", err)
		}
	})
}

func TestDiffContentOnlyChange(t *testing.T) {
	store := t.TempDir()
	root := writeTree(t, t.TempDir(), [][2]string{{"src/main.go", "package main\n"}})
	snap := mustDiffSnapshotFile(t, store, "ref.dlm.json", mustFilesystemSnapshot(t, root))
	if err := os.WriteFile(filepath.Join(root, "src", "main.go"), []byte("package main\n// entirely rewritten\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	diff := mustDiff(t, DiffRequest{
		A: DiffSourceSpec{Kind: DiffSourceSnapshot, Path: snap},
		B: DiffSourceSpec{Kind: DiffSourceLive, Path: root},
	})
	if DiffStatusOf(diff) != DiffNoDifferences {
		t.Fatalf("content-only change = %s (%+v)", DiffStatusOf(diff), diff.Changes)
	}
}

func TestDiffMoveIsRemoveAndAdd(t *testing.T) {
	store := t.TempDir()
	root := writeTree(t, t.TempDir(), [][2]string{{"a/one.txt", "a"}, {"b/.keep", ""}})
	snap := mustDiffSnapshotFile(t, store, "ref.dlm.json", mustFilesystemSnapshot(t, root))
	if err := os.Rename(filepath.Join(root, "a", "one.txt"), filepath.Join(root, "b", "one.txt")); err != nil {
		t.Fatal(err)
	}
	diff := mustDiff(t, DiffRequest{
		A: DiffSourceSpec{Kind: DiffSourceSnapshot, Path: snap},
		B: DiffSourceSpec{Kind: DiffSourceLive, Path: root},
	})
	if DiffStatusOf(diff) != DiffDifferences {
		t.Fatalf("status = %s", DiffStatusOf(diff))
	}
	var removed, added []artifact.Path
	for _, change := range diff.Changes {
		switch change.Op {
		case comparison.OpRemoved:
			removed = append(removed, change.Path)
		case comparison.OpAdded:
			added = append(added, change.Path)
		case comparison.OpChanged:
			// A move never produces CHANGED entries.
		default:
			t.Fatalf("unexpected operation %q: no move detection vocabulary may exist", change.Op)
		}
	}
	if len(removed) != 1 || removed[0] != "a/one.txt" {
		t.Fatalf("removed = %v", removed)
	}
	if len(added) != 1 || added[0] != "b/one.txt" {
		t.Fatalf("added = %v", added)
	}
}

func TestDiffCaptureSemanticsDriveObservation(t *testing.T) {
	store := t.TempDir()
	snapshotSpec := func(name string, raw []byte) DiffSourceSpec {
		return DiffSourceSpec{Kind: DiffSourceSnapshot, Path: mustDiffSnapshotFile(t, store, name, raw)}
	}
	liveSpec := func(root string) DiffSourceSpec {
		return DiffSourceSpec{Kind: DiffSourceLive, Path: root}
	}
	assertStatus := func(t *testing.T, snap DiffSourceSpec, root string, want DiffStatus) {
		t.Helper()
		diff := mustDiff(t, DiffRequest{A: snap, B: liveSpec(root)})
		if DiffStatusOf(diff) != want {
			t.Fatalf("status = %s, want %s (%+v)", DiffStatusOf(diff), want, diff.Changes)
		}
	}

	t.Run("depth", func(t *testing.T) {
		root := writeTree(t, t.TempDir(), [][2]string{{"top.txt", "t"}, {"dir/nested.txt", "n"}})
		depth := 1
		snap := snapshotSpec("depth.dlm.json", mustFilesystemSnapshotWith(t, root, InspectRequest{Root: root, MaxDepth: &depth}))
		assertStatus(t, snap, root, DiffNoDifferences)
		if err := os.WriteFile(filepath.Join(root, "dir", "extra.txt"), []byte("e"), 0o644); err != nil {
			t.Fatal(err)
		}
		assertStatus(t, snap, root, DiffNoDifferences)
		if err := os.WriteFile(filepath.Join(root, "other.txt"), []byte("o"), 0o644); err != nil {
			t.Fatal(err)
		}
		assertStatus(t, snap, root, DiffDifferences)
	})
	t.Run("dirs only", func(t *testing.T) {
		root := writeTree(t, t.TempDir(), [][2]string{{"src/main.go", "package main\n"}})
		snap := snapshotSpec("dirs.dlm.json", mustFilesystemSnapshotWith(t, root, InspectRequest{Root: root, DirectoriesOnly: true}))
		if err := os.WriteFile(filepath.Join(root, "extra.go"), []byte("package extra\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		assertStatus(t, snap, root, DiffNoDifferences)
		if err := os.Mkdir(filepath.Join(root, "docs"), 0o755); err != nil {
			t.Fatal(err)
		}
		assertStatus(t, snap, root, DiffDifferences)
	})
	t.Run("hidden", func(t *testing.T) {
		root := writeTree(t, t.TempDir(), [][2]string{{"visible.txt", "v"}})
		snap := snapshotSpec("hidden.dlm.json", mustFilesystemSnapshotWith(t, root, InspectRequest{Root: root, IncludeHidden: false}))
		if err := os.WriteFile(filepath.Join(root, ".secret"), []byte("s"), 0o644); err != nil {
			t.Fatal(err)
		}
		assertStatus(t, snap, root, DiffNoDifferences)
	})
	t.Run("default ignores", func(t *testing.T) {
		root := writeTree(t, t.TempDir(), [][2]string{{"main.go", "package main\n"}})
		snap := snapshotSpec("default.dlm.json", mustFilesystemSnapshotWith(t, root, InspectRequest{Root: root, UseDefaultIgnores: true}))
		if err := os.MkdirAll(filepath.Join(root, "node_modules", "pkg"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "node_modules", "pkg", "index.js"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		assertStatus(t, snap, root, DiffNoDifferences)
	})
	t.Run("gitignore", func(t *testing.T) {
		root := writeTree(t, t.TempDir(), [][2]string{{".gitignore", "ignored.txt\n"}, {"kept.txt", "k"}, {"ignored.txt", "i"}})
		snap := snapshotSpec("gitignore.dlm.json", mustFilesystemSnapshotWith(t, root, InspectRequest{Root: root, UseGitIgnore: true}))
		assertStatus(t, snap, root, DiffNoDifferences)
		if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		diff := mustDiff(t, DiffRequest{A: snap, B: liveSpec(root)})
		if DiffStatusOf(diff) != DiffDifferences || len(diff.Changes) != 1 || diff.Changes[0].Op != comparison.OpAdded || diff.Changes[0].Path != "ignored.txt" {
			t.Fatalf("gitignore change = %+v", diff.Changes)
		}
	})
	t.Run("custom ignore", func(t *testing.T) {
		root := writeTree(t, t.TempDir(), [][2]string{{"note.txt", "n"}})
		snap := snapshotSpec("ignore.dlm.json", mustFilesystemSnapshotWith(t, root, InspectRequest{Root: root, IgnorePatterns: []string{"*.log"}}))
		if err := os.WriteFile(filepath.Join(root, "another.log"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		assertStatus(t, snap, root, DiffNoDifferences)
		if err := os.WriteFile(filepath.Join(root, "kept.txt"), []byte("k"), 0o644); err != nil {
			t.Fatal(err)
		}
		assertStatus(t, snap, root, DiffDifferences)
	})
	t.Run("config never consulted", func(t *testing.T) {
		// A valid config that would exclude a.txt if it were consulted:
		// the diff must still observe a.txt and report no change.
		root := writeTree(t, t.TempDir(), [][2]string{{"a.txt", "a"}})
		snap := snapshotSpec("config.dlm.json", mustFilesystemSnapshot(t, root))
		if err := os.WriteFile(filepath.Join(root, ".dirloom.yaml"), []byte("schemaVersion: 1\nignore:\n  - a.txt\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		assertStatus(t, snap, root, DiffNoDifferences)
		// A malformed config would fail the run if it were parsed.
		if err := os.WriteFile(filepath.Join(root, ".dirloom.yaml"), []byte("schemaVersion: [\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		assertStatus(t, snap, root, DiffNoDifferences)
	})
}

func TestDiffSelfExclusion(t *testing.T) {
	t.Run("snapshot against live", func(t *testing.T) {
		root := writeTree(t, t.TempDir(), [][2]string{{"a.txt", "a"}})
		output := filepath.Join(root, "architecture.dlm.json")
		created, err := Snapshot(context.Background(), InspectRequest{
			Root: root, OutputPath: output, UseDefaultIgnores: false, UseGitIgnore: false,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(output, created.Bytes, 0o644); err != nil {
			t.Fatal(err)
		}
		diff := mustDiff(t, DiffRequest{
			A: DiffSourceSpec{Kind: DiffSourceSnapshot, Path: output},
			B: DiffSourceSpec{Kind: DiffSourceLive, Path: root},
		})
		if DiffStatusOf(diff) != DiffNoDifferences {
			t.Fatalf("self exclusion = %s (%+v)", DiffStatusOf(diff), diff.Changes)
		}
	})
	t.Run("live against snapshot", func(t *testing.T) {
		root := writeTree(t, t.TempDir(), [][2]string{{"a.txt", "a"}})
		output := filepath.Join(root, "architecture.dlm.json")
		created, err := Snapshot(context.Background(), InspectRequest{
			Root: root, OutputPath: output, UseDefaultIgnores: false, UseGitIgnore: false,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(output, created.Bytes, 0o644); err != nil {
			t.Fatal(err)
		}
		diff := mustDiff(t, DiffRequest{
			A: DiffSourceSpec{Kind: DiffSourceLive, Path: root},
			B: DiffSourceSpec{Kind: DiffSourceSnapshot, Path: output},
		})
		if DiffStatusOf(diff) != DiffNoDifferences {
			t.Fatalf("self exclusion = %s (%+v)", DiffStatusOf(diff), diff.Changes)
		}
	})
	t.Run("foreign snapshot file is not excluded", func(t *testing.T) {
		root := writeTree(t, t.TempDir(), [][2]string{{"a.txt", "a"}})
		reference := mustDiffSnapshotFile(t, t.TempDir(), "ref.dlm.json", mustFilesystemSnapshot(t, root))
		foreign := filepath.Join(root, "foreign.dlm.json")
		if err := os.WriteFile(foreign, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		diff := mustDiff(t, DiffRequest{
			A: DiffSourceSpec{Kind: DiffSourceSnapshot, Path: reference},
			B: DiffSourceSpec{Kind: DiffSourceLive, Path: root},
		})
		if len(diff.Changes) != 1 || diff.Changes[0].Op != comparison.OpAdded || diff.Changes[0].Path != "foreign.dlm.json" {
			t.Fatalf("foreign snapshot = %+v", diff.Changes)
		}
	})
}

func TestDiffSnapshotValidationClassification(t *testing.T) {
	store := t.TempDir()
	valid := mustDiffSnapshotFile(t, store, "valid.dlm.json", mustFilesystemSnapshot(t, writeTree(t, t.TempDir(), [][2]string{{"a.txt", "a"}})))
	invalid := mustDiffSnapshotFile(t, store, "invalid.dlm.json", []byte(`{"schemaVersion": 1, "broken`))
	unsupportedRaw := mustSnapshotBytes(t, directoryArtifact(), snapshot.CaptureV1{Ignore: []string{}})
	unsupportedRaw = []byte(strings.Replace(string(unsupportedRaw), `"schemaVersion": 1`, `"schemaVersion": 2`, 1))
	unsupported := mustDiffSnapshotFile(t, store, "unsupported.dlm.json", unsupportedRaw)
	missing := filepath.Join(store, "missing.dlm.json")

	spec := func(path string) DiffSourceSpec { return DiffSourceSpec{Kind: DiffSourceSnapshot, Path: path} }

	_, err := Diff(context.Background(), DiffRequest{A: spec(invalid), B: spec(valid)})
	if ClassifyDiffFailure(err) != FailureInvalidSnapshot || mustDiffSide(t, err) != comparison.SideA {
		t.Fatalf("invalid A = %v side %s", err, mustDiffSide(t, err))
	}
	_, err = Diff(context.Background(), DiffRequest{A: spec(valid), B: spec(invalid)})
	if ClassifyDiffFailure(err) != FailureInvalidSnapshot || mustDiffSide(t, err) != comparison.SideB {
		t.Fatalf("invalid B = %v side %s", err, mustDiffSide(t, err))
	}
	_, err = Diff(context.Background(), DiffRequest{A: spec(invalid), B: spec(unsupported)})
	if mustDiffSide(t, err) != comparison.SideA {
		t.Fatalf("first invalid side = %s", mustDiffSide(t, err))
	}
	_, err = Diff(context.Background(), DiffRequest{A: spec(unsupported), B: spec(valid)})
	if ClassifyDiffFailure(err) != FailureUnsupportedSnapshot {
		t.Fatalf("unsupported = %v", err)
	}
	_, err = Diff(context.Background(), DiffRequest{A: spec(missing), B: spec(valid)})
	if ClassifyDiffFailure(err) != FailureSnapshotRead || mustDiffSide(t, err) != comparison.SideA {
		t.Fatalf("missing = %v side %s", err, mustDiffSide(t, err))
	}
	// A live side against an invalid snapshot reports the snapshot side.
	_, err = Diff(context.Background(), DiffRequest{
		A: DiffSourceSpec{Kind: DiffSourceLive, Path: t.TempDir()},
		B: spec(invalid),
	})
	if ClassifyDiffFailure(err) != FailureInvalidSnapshot || mustDiffSide(t, err) != comparison.SideB {
		t.Fatalf("live against invalid = %v side %s", err, mustDiffSide(t, err))
	}
}

func TestDiffLiveObservationErrors(t *testing.T) {
	store := t.TempDir()
	snap := mustDiffSnapshotFile(t, store, "ref.dlm.json", mustFilesystemSnapshot(t, writeTree(t, t.TempDir(), [][2]string{{"a.txt", "a"}})))
	missing := filepath.Join(t.TempDir(), "missing")

	_, err := Diff(context.Background(), DiffRequest{
		A: DiffSourceSpec{Kind: DiffSourceSnapshot, Path: snap},
		B: DiffSourceSpec{Kind: DiffSourceLive, Path: missing},
	})
	if ClassifyDiffFailure(err) != FailureObservation || mustDiffSide(t, err) != comparison.SideB {
		t.Fatalf("missing live B = %v side %s", err, mustDiffSide(t, err))
	}
	_, err = Diff(context.Background(), DiffRequest{
		A: DiffSourceSpec{Kind: DiffSourceLive, Path: missing},
		B: DiffSourceSpec{Kind: DiffSourceSnapshot, Path: snap},
	})
	if ClassifyDiffFailure(err) != FailureObservation || mustDiffSide(t, err) != comparison.SideA {
		t.Fatalf("missing live A = %v side %s", err, mustDiffSide(t, err))
	}
	code, message := DiffDiagnostic(err)
	if code == "" || message == "" {
		t.Fatalf("diagnostic = %q %q", code, message)
	}
	if code, _ := DiffDiagnostic(errors.New("plain")); code != DiagnosticDiffObservationFailure {
		t.Fatalf("fallback = %s", code)
	}
	// A typed internal artifact error surfaces its own stable code.
	if code, _ := DiffDiagnostic(artifact.InternalError("broken")); code != artifact.CodeInternal {
		t.Fatalf("internal = %s", code)
	}
	// An internal failure without a typed code uses the diff fallback.
	if code, _ := DiffDiagnostic(artifact.New("", "codeless internal", true)); code != DiagnosticDiffInternalFailure {
		t.Fatalf("codeless internal = %s", code)
	}
	if code, _ := DiffDiagnostic(nil); code != "" {
		t.Fatalf("nil = %s", code)
	}
}

func TestDiffContextCancellation(t *testing.T) {
	store := t.TempDir()
	snap := mustDiffSnapshotFile(t, store, "ref.dlm.json", mustFilesystemSnapshot(t, writeTree(t, t.TempDir(), [][2]string{{"a.txt", "a"}})))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Diff(ctx, DiffRequest{
		A: DiffSourceSpec{Kind: DiffSourceSnapshot, Path: snap},
		B: DiffSourceSpec{Kind: DiffSourceSnapshot, Path: snap},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled = %v", err)
	}
}

func TestDiffUnicodeAndCaseSensitivity(t *testing.T) {
	store := t.TempDir()
	snapshotOf := func(name string, art artifact.Artifact) DiffSourceSpec {
		return DiffSourceSpec{Kind: DiffSourceSnapshot, Path: mustDiffSnapshotFile(t, store, name, mustSnapshotBytes(t, art, snapshot.CaptureV1{Ignore: []string{}}))}
	}

	t.Run("NFC normalization", func(t *testing.T) {
		root := t.TempDir()
		nfd := "é.txt"
		if err := os.WriteFile(filepath.Join(root, nfd), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		raw := mustFilesystemSnapshot(t, root)
		validated, err := snapshot.LoadBytes(raw)
		if err != nil {
			t.Fatal(err)
		}
		if !artifactContains(validated.Artifact, artifact.Path("é.txt")) {
			t.Fatalf("NFD name was not normalized: %#v", validated.Artifact.Root.Children)
		}
		diff := mustDiff(t, DiffRequest{
			A: DiffSourceSpec{Kind: DiffSourceSnapshot, Path: mustDiffSnapshotFile(t, store, "nfc.dlm.json", raw)},
			B: DiffSourceSpec{Kind: DiffSourceLive, Path: root},
		})
		if DiffStatusOf(diff) != DiffNoDifferences {
			t.Fatalf("NFC round trip = %+v", diff.Changes)
		}
	})
	t.Run("case sensitive paths", func(t *testing.T) {
		upper := snapshotOf("upper.dlm.json", directoryArtifact(fileNode("Auth.txt")))
		lower := snapshotOf("lower.dlm.json", directoryArtifact(fileNode("auth.txt")))
		diff := mustDiff(t, DiffRequest{A: upper, B: lower})
		if len(diff.Changes) != 2 {
			t.Fatalf("changes = %+v", diff.Changes)
		}
		// Canonical ordering is byte-wise: "Auth.txt" sorts before "auth.txt".
		if diff.Changes[0].Op != comparison.OpRemoved || diff.Changes[0].Path != "Auth.txt" {
			t.Fatalf("first = %+v", diff.Changes[0])
		}
		if diff.Changes[1].Op != comparison.OpAdded || diff.Changes[1].Path != "auth.txt" {
			t.Fatalf("second = %+v", diff.Changes[1])
		}
	})
}

func TestDiffDeepPathsAndFanOut(t *testing.T) {
	store := t.TempDir()
	var wide []artifact.Node
	for i := 0; i < 1000; i++ {
		wide = append(wide, fileNode("f"+itoa(i)+".txt"))
	}
	base := directoryArtifact(wide...)
	trimmed := make([]artifact.Node, 0, len(wide))
	trimmed = append(trimmed, wide[:len(wide)-1]...)
	mutated := directoryArtifact(append(trimmed, directoryNode("sub", fileNodeAt("sub/x.txt", "x.txt")))...)

	snapA := DiffSourceSpec{Kind: DiffSourceSnapshot, Path: mustDiffSnapshotFile(t, store, "wide-a.dlm.json", mustSnapshotBytes(t, base, snapshot.CaptureV1{Ignore: []string{}}))}
	snapB := DiffSourceSpec{Kind: DiffSourceSnapshot, Path: mustDiffSnapshotFile(t, store, "wide-b.dlm.json", mustSnapshotBytes(t, mutated, snapshot.CaptureV1{Ignore: []string{}}))}
	diff := mustDiff(t, DiffRequest{A: snapA, B: snapB})
	if diff.Summary.Removed != 1 || diff.Summary.Added != 2 || diff.Summary.Total != 3 {
		t.Fatalf("summary = %+v", diff.Summary)
	}
	if diff.Metadata.A.NodeCount != 1001 || diff.Metadata.B.NodeCount != 1002 {
		t.Fatalf("node counts = %+v", diff.Metadata)
	}
}

func TestDiffLargeSubtreeAddAndRemove(t *testing.T) {
	store := t.TempDir()
	var subtree []artifact.Node
	for i := 0; i < 500; i++ {
		subtree = append(subtree, fileNodeAt("big/f"+itoa(i)+".txt", "f"+itoa(i)+".txt"))
	}
	withSubtree := directoryArtifact(fileNode("root.txt"), directoryNode("big", subtree...))
	withoutSubtree := directoryArtifact(fileNode("root.txt"))

	snapA := DiffSourceSpec{Kind: DiffSourceSnapshot, Path: mustDiffSnapshotFile(t, store, "sub-a.dlm.json", mustSnapshotBytes(t, withSubtree, snapshot.CaptureV1{Ignore: []string{}}))}
	snapB := DiffSourceSpec{Kind: DiffSourceSnapshot, Path: mustDiffSnapshotFile(t, store, "sub-b.dlm.json", mustSnapshotBytes(t, withoutSubtree, snapshot.CaptureV1{Ignore: []string{}}))}

	removed := mustDiff(t, DiffRequest{A: snapA, B: snapB})
	if removed.Summary.Removed != 501 || removed.Summary.Added != 0 || removed.Summary.Changed != 0 {
		t.Fatalf("removed summary = %+v", removed.Summary)
	}
	added := mustDiff(t, DiffRequest{A: snapB, B: snapA})
	if added.Summary.Added != 501 || added.Summary.Removed != 0 || added.Summary.Changed != 0 {
		t.Fatalf("added summary = %+v", added.Summary)
	}
}
