package app

import (
	"bytes"
	"context"
	"errors"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/snapshot"
	"github.com/dirloom/dirloom/internal/source"
)

func TestVerifyMemoryMatchAndMismatch(t *testing.T) {
	expected := directoryArtifact(fileNode("a.txt"))
	validated := mustValidated(t, expected, snapshot.CaptureV1{Ignore: []string{}})
	counter := &countingSource{inner: source.Memory{Artifact: expected}}
	matched, err := VerifyValidatedFromSource(context.Background(), validated, counter)
	if err != nil {
		t.Fatal(err)
	}
	if matched.Status != VerifyMatch || matched.SourceKind != source.KindMemory {
		t.Fatalf("%#v", matched)
	}
	if matched.ExpectedFingerprint != validated.Fingerprint || matched.ActualFingerprint != validated.Fingerprint {
		t.Fatalf("fingerprints = %s %s", matched.ExpectedFingerprint, matched.ActualFingerprint)
	}
	if counter.calls != 1 {
		t.Fatalf("observations = %d", counter.calls)
	}

	other := directoryArtifact(fileNode("b.txt"))
	if expected.CountNodes() != other.CountNodes() {
		t.Fatal("fixture node counts should match so equality cannot be a count check")
	}
	mismatched, err := VerifyValidatedFromSource(context.Background(), validated, source.Memory{Artifact: other})
	if err != nil {
		t.Fatal(err)
	}
	if mismatched.Status != VerifyMismatch {
		t.Fatalf("status = %s", mismatched.Status)
	}
	if mismatched.ExpectedFingerprint != validated.Fingerprint || mismatched.ActualFingerprint == validated.Fingerprint {
		t.Fatal("mismatch fingerprints were not populated")
	}
}

func TestVerifyNilAndBrokenSource(t *testing.T) {
	validated := mustValidated(t, directoryArtifact(), snapshot.CaptureV1{Ignore: []string{}})
	_, err := VerifyValidatedFromSource(context.Background(), validated, nil)
	if !artifact.IsInternal(err) || ClassifyVerifyFailure(err) != FailureInternal {
		t.Fatalf("nil source = %v", err)
	}
	_, err = VerifyValidatedFromSource(context.Background(), validated, stubSource{err: artifact.SourceReadFailure(errors.New("unreadable"))})
	if ClassifyVerifyFailure(err) != FailureObservation || artifact.IsInternal(err) {
		t.Fatalf("broken source = %v", err)
	}
	_, err = Verify(context.Background(), VerifyRequest{})
	if !artifact.IsInternal(err) {
		t.Fatalf("nil reader = %v", err)
	}
}

func TestVerifyContextCancellation(t *testing.T) {
	validated := mustValidated(t, directoryArtifact(fileNode("a.txt")), snapshot.CaptureV1{Ignore: []string{}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := VerifyValidatedFromSource(ctx, validated, source.Memory{Artifact: validated.Artifact})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled = %v", err)
	}
	raw := mustSnapshotBytes(t, directoryArtifact(), snapshot.CaptureV1{Ignore: []string{}})
	_, err = Verify(ctx, VerifyRequest{Snapshot: bytes.NewReader(raw)})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("verify canceled = %v", err)
	}
}

func TestVerifyChildOrderDoesNotAffectResult(t *testing.T) {
	art := nestedArtifact()
	validated := mustValidated(t, art, snapshot.CaptureV1{Ignore: []string{"keep"}})
	rng := rand.New(rand.NewPCG(7, 11))
	for i := 0; i < 16; i++ {
		clone := art.Clone()
		shuffleArtifact(&clone.Root, rng)
		result, err := VerifyValidatedFromSource(context.Background(), validated, source.Memory{Artifact: clone})
		if err != nil || result.Status != VerifyMatch {
			t.Fatalf("permutation %d = %s %v", i, result.Status, err)
		}
	}
}

func TestVerifyCaptureMappingDeepCopy(t *testing.T) {
	depth := 3
	ignore := []string{"generated", "*.tmp"}
	capture := snapshot.CaptureV1{
		Depth:             &depth,
		DirsOnly:          true,
		Hidden:            true,
		UseDefaultIgnores: false,
		UseGitignore:      true,
		Ignore:            ignore,
	}
	request := inspectRequestFromCapture(capture, "project")
	depth = 9
	ignore[0] = "mutated"
	if request.Root != "project" || request.MaxDepth == nil || *request.MaxDepth != 3 {
		t.Fatalf("depth aliased: %#v", request.MaxDepth)
	}
	if !request.DirectoriesOnly || !request.IncludeHidden || request.UseDefaultIgnores || !request.UseGitIgnore {
		t.Fatalf("%#v", request)
	}
	if len(request.IgnorePatterns) != 2 || request.IgnorePatterns[0] != "generated" || request.IgnorePatterns[1] != "*.tmp" {
		t.Fatalf("ignore = %#v", request.IgnorePatterns)
	}
	request.IgnorePatterns[0] = "local"
	if capture.Ignore[0] != "mutated" {
		t.Fatalf("request slice aliases capture: %#v", capture.Ignore)
	}
	unlimited := inspectRequestFromCapture(snapshot.CaptureV1{Ignore: []string{}}, "")
	if unlimited.MaxDepth != nil || unlimited.IgnorePatterns == nil {
		t.Fatalf("unlimited = %#v", unlimited)
	}
}

func TestVerifyReferenceSelfExclusion(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(root, "architecture.dlm.json")
	if err := os.WriteFile(inside, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(outside, "architecture.dlm.json")
	if err := os.WriteFile(external, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	empty := directoryArtifact()
	got, err := referenceSelfExclusion(root, external, empty)
	if err != nil || got != "" {
		t.Fatalf("outside = %q %v", got, err)
	}
	got, err = referenceSelfExclusion(root, inside, empty)
	if err != nil || got == "" {
		t.Fatalf("inside absent = %q %v", got, err)
	}
	if filepath.Clean(got) != filepath.Clean(inside) {
		t.Fatalf("output path = %q", got)
	}
	present := directoryArtifact(fileNode("architecture.dlm.json"))
	got, err = referenceSelfExclusion(root, inside, present)
	if err != nil || got != "" {
		t.Fatalf("inside present = %q %v", got, err)
	}
	dotted := filepath.Join(root, "sub", "..", "architecture.dlm.json")
	got, err = referenceSelfExclusion(root, dotted, present)
	if err != nil || got != "" {
		t.Fatalf("canonical present = %q %v", got, err)
	}
	missing := filepath.Join(root, "sub", "..", "missing.dlm.json")
	if err := os.WriteFile(filepath.Join(root, "missing.dlm.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = referenceSelfExclusion(root, missing, empty)
	if err != nil || filepath.Clean(got) != filepath.Clean(filepath.Join(root, "missing.dlm.json")) {
		t.Fatalf("canonical absent = %q %v", got, err)
	}
	relocated := filepath.Join(root, "copied.dlm.json")
	if err := os.WriteFile(relocated, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = referenceSelfExclusion(root, relocated, empty)
	if err != nil || got == "" {
		t.Fatalf("relocated = %q %v", got, err)
	}
	got, err = referenceSelfExclusion(root, "", empty)
	if err != nil || got != "" {
		t.Fatalf("empty path = %q %v", got, err)
	}
}

func TestVerifyClassifiesSnapshotFixtures(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "snapshots", "v1", "invalid")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("missing invalid snapshot corpus")
	}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		_, loadErr := snapshot.LoadBytes(data)
		if loadErr == nil {
			t.Fatalf("%s unexpectedly valid", entry.Name())
		}
		kind := ClassifyVerifyFailure(loadErr)
		switch snapshot.CodeOf(loadErr) {
		case snapshot.CodeUnsupportedSchema, snapshot.CodeUnsupportedArtifact, snapshot.CodeUnsupportedFeature:
			if kind != FailureUnsupportedSnapshot {
				t.Fatalf("%s code %s -> %s", entry.Name(), snapshot.CodeOf(loadErr), kind)
			}
		case snapshot.CodeInvalidJSON, snapshot.CodeMissingField, snapshot.CodeInvalidField, snapshot.CodeInvalidArtifact, snapshot.CodeFingerprintMismatch:
			if kind != FailureInvalidSnapshot {
				t.Fatalf("%s code %s -> %s", entry.Name(), snapshot.CodeOf(loadErr), kind)
			}
		default:
			t.Fatalf("%s unclassified code %s", entry.Name(), snapshot.CodeOf(loadErr))
		}
		if snapshot.CodeOf(loadErr) == snapshot.CodeFingerprintMismatch && kind != FailureInvalidSnapshot {
			t.Fatal("embedded fingerprint mismatch classified as live mismatch")
		}
	}
	internalSnap := &snapshot.Error{Code: snapshot.CodeInternalFailure, Message: "broken", Internal: true}
	if !snapshot.IsInternal(internalSnap) || ClassifyVerifyFailure(internalSnap) != FailureInternal {
		t.Fatal("snapshot internal")
	}
	internalArt := artifact.InternalError("broken invariant")
	if !artifact.IsInternal(internalArt) || ClassifyVerifyFailure(internalArt) != FailureInternal {
		t.Fatal("artifact internal")
	}
	observed := artifact.SourceReadFailure(errors.New("permission denied"))
	if ClassifyVerifyFailure(observed) != FailureObservation {
		t.Fatal(ClassifyVerifyFailure(observed))
	}
	code, message := VerifyDiagnostic(observed)
	if code != artifact.CodeSourceRead || message == "" {
		t.Fatalf("diagnostic = %s %s", code, message)
	}
	code, _ = VerifyDiagnostic(errors.New("plain"))
	if code != DiagnosticObservationFailure {
		t.Fatal(code)
	}
	code, _ = VerifyDiagnostic(context.Canceled)
	if code != DiagnosticObservationFailure || ClassifyVerifyFailure(context.Canceled) != FailureObservation {
		t.Fatalf("cancel = %s", code)
	}
}

func TestVerifyFilesystemOutcomes(t *testing.T) {
	t.Run("unchanged", func(t *testing.T) {
		root := writeTree(t, t.TempDir(), [][2]string{{"src/main.go", "package main\n"}, {"README.md", "hi\n"}})
		assertVerifyStatus(t, root, root, VerifyMatch)
	})
	t.Run("add", func(t *testing.T) {
		root := writeTree(t, t.TempDir(), [][2]string{{"a.txt", "a"}})
		snap := mustFilesystemSnapshot(t, root)
		if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("b"), 0o644); err != nil {
			t.Fatal(err)
		}
		assertSnapshotStatus(t, snap, root, VerifyMismatch)
	})
	t.Run("remove", func(t *testing.T) {
		root := writeTree(t, t.TempDir(), [][2]string{{"a.txt", "a"}, {"b.txt", "b"}})
		snap := mustFilesystemSnapshot(t, root)
		if err := os.Remove(filepath.Join(root, "b.txt")); err != nil {
			t.Fatal(err)
		}
		assertSnapshotStatus(t, snap, root, VerifyMismatch)
	})
	t.Run("rename", func(t *testing.T) {
		root := writeTree(t, t.TempDir(), [][2]string{{"a.txt", "a"}})
		snap := mustFilesystemSnapshot(t, root)
		if err := os.Rename(filepath.Join(root, "a.txt"), filepath.Join(root, "c.txt")); err != nil {
			t.Fatal(err)
		}
		assertSnapshotStatus(t, snap, root, VerifyMismatch)
	})
	t.Run("move", func(t *testing.T) {
		root := writeTree(t, t.TempDir(), [][2]string{{"a/one.txt", "a"}, {"b/.keep", ""}})
		snap := mustFilesystemSnapshot(t, root)
		if err := os.Rename(filepath.Join(root, "a", "one.txt"), filepath.Join(root, "b", "one.txt")); err != nil {
			t.Fatal(err)
		}
		result := assertSnapshotStatus(t, snap, root, VerifyMismatch)
		if result.Status != VerifyMismatch {
			t.Fatal(result.Status)
		}
	})
	t.Run("file to directory", func(t *testing.T) {
		root := writeTree(t, t.TempDir(), [][2]string{{"node.txt", "x"}})
		snap := mustFilesystemSnapshot(t, root)
		if err := os.Remove(filepath.Join(root, "node.txt")); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(root, "node.txt"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "node.txt", "child.txt"), []byte("c"), 0o644); err != nil {
			t.Fatal(err)
		}
		assertSnapshotStatus(t, snap, root, VerifyMismatch)
	})
	t.Run("content only", func(t *testing.T) {
		root := writeTree(t, t.TempDir(), [][2]string{{"src/main.go", "package main\n"}})
		snap := mustFilesystemSnapshot(t, root)
		if err := os.WriteFile(filepath.Join(root, "src", "main.go"), []byte("package main\n// edit\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		assertSnapshotStatus(t, snap, root, VerifyMatch)
	})
	t.Run("rename root", func(t *testing.T) {
		parent := t.TempDir()
		root := writeTree(t, filepath.Join(parent, "original"), [][2]string{{"a.txt", "a"}})
		snap := mustFilesystemSnapshot(t, root)
		moved := filepath.Join(parent, "renamed-root")
		if err := os.Rename(root, moved); err != nil {
			t.Fatal(err)
		}
		assertSnapshotStatus(t, snap, moved, VerifyMatch)
	})
	t.Run("copy tree", func(t *testing.T) {
		payload := [][2]string{{"src/main.go", "package main\n"}, {"README.md", "hi\n"}}
		left := writeTree(t, filepath.Join(t.TempDir(), "left-name"), payload)
		right := writeTree(t, filepath.Join(t.TempDir(), "right-name"), payload)
		snap := mustFilesystemSnapshot(t, left)
		assertSnapshotStatus(t, snap, right, VerifyMatch)
	})
	t.Run("snapshot relocated", func(t *testing.T) {
		root := writeTree(t, t.TempDir(), [][2]string{{"a.txt", "a"}})
		snap := mustFilesystemSnapshot(t, root)
		relocated := filepath.Join(t.TempDir(), "elsewhere.dlm.json")
		if err := os.WriteFile(relocated, snap, 0o644); err != nil {
			t.Fatal(err)
		}
		result, err := Verify(context.Background(), VerifyRequest{Snapshot: bytes.NewReader(snap), SnapshotPath: relocated, Root: root})
		if err != nil || result.Status != VerifyMatch {
			t.Fatalf("%s %v", result.Status, err)
		}
	})
	t.Run("self exclusion", func(t *testing.T) {
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
		result, err := Verify(context.Background(), VerifyRequest{
			Snapshot: bytes.NewReader(created.Bytes), SnapshotPath: output, Root: root,
		})
		if err != nil || result.Status != VerifyMatch {
			t.Fatalf("excluded reference = %s %v", result.Status, err)
		}
	})
	t.Run("do not exclude expected reference path", func(t *testing.T) {
		root := writeTree(t, t.TempDir(), [][2]string{{"architecture.dlm.json", "original"}, {"a.txt", "a"}})
		created, err := Snapshot(context.Background(), InspectRequest{Root: root, UseDefaultIgnores: false, UseGitIgnore: false})
		if err != nil {
			t.Fatal(err)
		}
		reference := filepath.Join(root, "architecture.dlm.json")
		if err := os.WriteFile(reference, created.Bytes, 0o644); err != nil {
			t.Fatal(err)
		}
		result, err := Verify(context.Background(), VerifyRequest{
			Snapshot: bytes.NewReader(created.Bytes), SnapshotPath: reference, Root: root,
		})
		if err != nil || result.Status != VerifyMatch {
			t.Fatalf("expected reference path = %s %v", result.Status, err)
		}
	})
}

func TestVerifyCaptureSemanticsDriveObservation(t *testing.T) {
	t.Run("hidden", func(t *testing.T) {
		root := writeTree(t, t.TempDir(), [][2]string{{"visible.txt", "v"}, {".secret", "s"}})
		hidden := mustFilesystemSnapshotWith(t, root, InspectRequest{Root: root, IncludeHidden: true})
		plain := mustFilesystemSnapshotWith(t, root, InspectRequest{Root: root, IncludeHidden: false})
		assertSnapshotStatus(t, hidden, root, VerifyMatch)
		assertSnapshotStatus(t, plain, root, VerifyMatch)
		if bytes.Equal(hidden, plain) {
			t.Fatal("hidden capture did not change the snapshot")
		}
	})
	t.Run("depth", func(t *testing.T) {
		root := writeTree(t, t.TempDir(), [][2]string{{"top.txt", "t"}, {"dir/nested.txt", "n"}})
		depth := 1
		snap := mustFilesystemSnapshotWith(t, root, InspectRequest{Root: root, MaxDepth: &depth})
		assertSnapshotStatus(t, snap, root, VerifyMatch)
		if err := os.WriteFile(filepath.Join(root, "dir", "extra.txt"), []byte("e"), 0o644); err != nil {
			t.Fatal(err)
		}
		assertSnapshotStatus(t, snap, root, VerifyMatch)
		if err := os.WriteFile(filepath.Join(root, "other.txt"), []byte("o"), 0o644); err != nil {
			t.Fatal(err)
		}
		assertSnapshotStatus(t, snap, root, VerifyMismatch)
	})
	t.Run("dirs only", func(t *testing.T) {
		root := writeTree(t, t.TempDir(), [][2]string{{"src/main.go", "package main\n"}})
		snap := mustFilesystemSnapshotWith(t, root, InspectRequest{Root: root, DirectoriesOnly: true})
		assertSnapshotStatus(t, snap, root, VerifyMatch)
		if err := os.WriteFile(filepath.Join(root, "extra.go"), []byte("package extra\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		assertSnapshotStatus(t, snap, root, VerifyMatch)
		if err := os.Mkdir(filepath.Join(root, "docs"), 0o755); err != nil {
			t.Fatal(err)
		}
		assertSnapshotStatus(t, snap, root, VerifyMismatch)
	})
	t.Run("custom ignore", func(t *testing.T) {
		root := writeTree(t, t.TempDir(), [][2]string{{"note.txt", "n"}, {"note.log", "l"}})
		snap := mustFilesystemSnapshotWith(t, root, InspectRequest{Root: root, IgnorePatterns: []string{"*.log", "other"}})
		assertSnapshotStatus(t, snap, root, VerifyMatch)
		if err := os.WriteFile(filepath.Join(root, "another.log"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		assertSnapshotStatus(t, snap, root, VerifyMatch)
		if err := os.WriteFile(filepath.Join(root, "kept.txt"), []byte("k"), 0o644); err != nil {
			t.Fatal(err)
		}
		assertSnapshotStatus(t, snap, root, VerifyMismatch)
	})
	t.Run("gitignore", func(t *testing.T) {
		root := writeTree(t, t.TempDir(), [][2]string{{".gitignore", "ignored.txt\n"}, {"kept.txt", "k"}, {"ignored.txt", "i"}})
		with := mustFilesystemSnapshotWith(t, root, InspectRequest{Root: root, UseGitIgnore: true})
		without := mustFilesystemSnapshotWith(t, root, InspectRequest{Root: root, UseGitIgnore: false})
		assertSnapshotStatus(t, with, root, VerifyMatch)
		assertSnapshotStatus(t, without, root, VerifyMatch)
		if bytes.Equal(with, without) {
			t.Fatal("gitignore capture did not change the snapshot")
		}
		if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		assertSnapshotStatus(t, with, root, VerifyMismatch)
	})
	t.Run("default ignores", func(t *testing.T) {
		root := writeTree(t, t.TempDir(), [][2]string{{"node_modules/pkg/index.js", "x"}, {"main.go", "package main\n"}})
		on := mustFilesystemSnapshotWith(t, root, InspectRequest{Root: root, UseDefaultIgnores: true})
		off := mustFilesystemSnapshotWith(t, root, InspectRequest{Root: root, UseDefaultIgnores: false})
		assertSnapshotStatus(t, on, root, VerifyMatch)
		assertSnapshotStatus(t, off, root, VerifyMatch)
		if bytes.Equal(on, off) {
			t.Fatal("default ignores did not change the snapshot")
		}
	})
	t.Run("malformed config ignored", func(t *testing.T) {
		root := writeTree(t, t.TempDir(), [][2]string{{"a.txt", "a"}})
		snap := mustFilesystemSnapshot(t, root)
		if err := os.WriteFile(filepath.Join(root, ".dirloom.yaml"), []byte("schemaVersion: [\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		assertSnapshotStatus(t, snap, root, VerifyMatch)
	})
}

func TestVerifyLiveFixtures(t *testing.T) {
	for _, name := range []string{"basic", "nested"} {
		t.Run(name, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", "verify", "v1", "live", name)
			assertVerifyStatus(t, root, root, VerifyMatch)
		})
	}
}

func TestVerifyObservationAndReadErrors(t *testing.T) {
	validated := mustValidated(t, directoryArtifact(fileNode("a.txt")), snapshot.CaptureV1{Ignore: []string{}})
	raw := mustSnapshotBytes(t, directoryArtifact(fileNode("a.txt")), snapshot.CaptureV1{Ignore: []string{}})
	missing := filepath.Join(t.TempDir(), "missing")
	_, err := Verify(context.Background(), VerifyRequest{Snapshot: bytes.NewReader(raw), Root: missing})
	if ClassifyVerifyFailure(err) != FailureObservation {
		t.Fatalf("missing root = %v", err)
	}
	fileRoot := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(fileRoot, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = Verify(context.Background(), VerifyRequest{Snapshot: bytes.NewReader(raw), Root: fileRoot})
	if ClassifyVerifyFailure(err) != FailureObservation {
		t.Fatalf("file root = %v", err)
	}
	result, err := VerifyValidatedFromSource(context.Background(), validated, stubSource{err: artifact.New(artifact.CodeCollision, "canonical path collision", false, "a")})
	if err != nil && ClassifyVerifyFailure(err) != FailureObservation {
		t.Fatal(ClassifyVerifyFailure(err))
	}
	if result.ExpectedFingerprint != validated.Fingerprint {
		t.Fatal("observation error dropped the expected fingerprint")
	}
}

func TestPropertyVerifyIdentityPreservation(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 5))
	capture := snapshot.CaptureV1{Hidden: true, Ignore: []string{"*.tmp"}}
	for i := 0; i < 12; i++ {
		art := randomArtifact(rng, 4+i)
		raw := mustSnapshotBytes(t, art, capture)
		validated, err := snapshot.LoadBytes(raw)
		if err != nil {
			t.Fatal(err)
		}
		result, err := VerifyValidatedFromSource(context.Background(), validated, source.Memory{Artifact: art})
		if err != nil || result.Status != VerifyMatch {
			t.Fatalf("artifact %d = %s %v", i, result.Status, err)
		}
		reloaded, err := snapshot.LoadBytes(raw)
		if err != nil {
			t.Fatal(err)
		}
		again, err := VerifyValidatedFromSource(context.Background(), reloaded, source.Memory{Artifact: art})
		if err != nil || again.Status != VerifyMatch || again.ActualFingerprint != result.ActualFingerprint {
			t.Fatalf("round trip %d", i)
		}
	}
}

func TestPropertyVerifyStructuralMutation(t *testing.T) {
	base := nestedArtifact()
	validated := mustValidated(t, base, snapshot.CaptureV1{Ignore: []string{}})
	mutations := []artifact.Artifact{
		directoryArtifact(fileNode("other.txt"), directoryNode("src", fileNodeAt("src/main.go", "main.go"))),
		directoryArtifact(fileNode("README.md"), artifact.Node{Path: "src", Name: "src", Kind: artifact.KindSymlink, Target: "elsewhere"}),
		directoryArtifact(fileNode("README.md"), directoryNode("src", fileNodeAt("src/main.go", "main.go"), fileNodeAt("src/extra.go", "extra.go"))),
		directoryArtifact(fileNode("README.md"), directoryNode("src", artifact.Node{Path: "src/main.go", Name: "main.go", Kind: artifact.KindSymlink, Target: "lib"})),
	}
	for i, mutated := range mutations {
		if err := mutated.Validate(); err != nil {
			t.Fatalf("mutation %d: %v", i, err)
		}
		result, err := VerifyValidatedFromSource(context.Background(), validated, source.Memory{Artifact: mutated})
		if err != nil {
			t.Fatal(err)
		}
		if result.Status != VerifyMismatch {
			t.Fatalf("mutation %d status = %s", i, result.Status)
		}
	}
}

type countingSource struct {
	inner source.Source
	calls int
}

func (c *countingSource) Kind() source.Kind {
	if c.inner == nil {
		return source.KindMemory
	}
	return c.inner.Kind()
}

func (c *countingSource) Observe(ctx context.Context) (*artifact.Artifact, error) {
	c.calls++
	return c.inner.Observe(ctx)
}

type stubSource struct {
	err error
}

func (stubSource) Kind() source.Kind { return source.KindMemory }

func (s stubSource) Observe(context.Context) (*artifact.Artifact, error) {
	return nil, s.err
}

func directoryArtifact(children ...artifact.Node) artifact.Artifact {
	return artifact.Artifact{Root: artifact.Node{
		Path: artifact.RootPath, Name: ".", Kind: artifact.KindDirectory, Children: children,
	}}
}

func fileNode(name string) artifact.Node {
	return artifact.Node{Path: artifact.Path(name), Name: name, Kind: artifact.KindFile}
}

func fileNodeAt(path, name string) artifact.Node {
	return artifact.Node{Path: artifact.Path(path), Name: name, Kind: artifact.KindFile}
}

func directoryNode(name string, children ...artifact.Node) artifact.Node {
	return artifact.Node{Path: artifact.Path(name), Name: name, Kind: artifact.KindDirectory, Children: children}
}

func nestedArtifact() artifact.Artifact {
	return directoryArtifact(
		fileNode("README.md"),
		directoryNode("src", fileNodeAt("src/main.go", "main.go")),
	)
}

func mustValidated(t testing.TB, art artifact.Artifact, capture snapshot.CaptureV1) snapshot.Validated {
	t.Helper()
	validated, err := snapshot.LoadBytes(mustSnapshotBytes(t, art, capture))
	if err != nil {
		t.Fatal(err)
	}
	return validated
}

func mustSnapshotBytes(t testing.TB, art artifact.Artifact, capture snapshot.CaptureV1) []byte {
	t.Helper()
	doc, err := snapshot.Build(art, capture)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := snapshot.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mustFilesystemSnapshot(t testing.TB, root string) []byte {
	t.Helper()
	return mustFilesystemSnapshotWith(t, root, InspectRequest{Root: root, UseDefaultIgnores: false, UseGitIgnore: false})
}

func mustFilesystemSnapshotWith(t testing.TB, root string, request InspectRequest) []byte {
	t.Helper()
	if request.Root == "" {
		request.Root = root
	}
	result, err := Snapshot(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	return result.Bytes
}

func assertVerifyStatus(t *testing.T, snapshotRoot, liveRoot string, want VerifyStatus) appResult {
	t.Helper()
	return assertSnapshotStatus(t, mustFilesystemSnapshot(t, snapshotRoot), liveRoot, want)
}

type appResult = VerifyResult

func assertSnapshotStatus(t *testing.T, raw []byte, liveRoot string, want VerifyStatus) VerifyResult {
	t.Helper()
	result, err := Verify(context.Background(), VerifyRequest{Snapshot: bytes.NewReader(raw), Root: liveRoot})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != want {
		t.Fatalf("status = %s, want %s", result.Status, want)
	}
	return result
}

func shuffleArtifact(node *artifact.Node, rng *rand.Rand) {
	rng.Shuffle(len(node.Children), func(i, j int) {
		node.Children[i], node.Children[j] = node.Children[j], node.Children[i]
	})
	for i := range node.Children {
		shuffleArtifact(&node.Children[i], rng)
	}
}

func randomArtifact(rng *rand.Rand, n int) artifact.Artifact {
	children := make([]artifact.Node, n)
	for i := range children {
		name := "f" + itoa(i) + ".txt"
		children[i] = fileNode(name)
	}
	rng.Shuffle(len(children), func(i, j int) {
		children[i], children[j] = children[j], children[i]
	})
	return directoryArtifact(children...)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits [16]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
}
