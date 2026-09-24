package snapshot_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/identity"
	"github.com/dirloom/dirloom/internal/snapshot"
)

func sampleArtifact() artifact.Artifact {
	return artifact.Artifact{Root: artifact.Node{
		Path: artifact.RootPath, Name: ".", Kind: artifact.KindDirectory,
		Children: []artifact.Node{
			{Path: "src", Name: "src", Kind: artifact.KindDirectory, Children: []artifact.Node{
				{Path: "src/main.go", Name: "main.go", Kind: artifact.KindFile},
			}},
			{Path: "README.md", Name: "README.md", Kind: artifact.KindFile},
			func() artifact.Node {
				target := "src/main.go"
				return artifact.Node{Path: "link", Name: "link", Kind: artifact.KindSymlink, Target: target}
			}(),
		},
	}}
}

func emptyArtifact() artifact.Artifact {
	return artifact.Artifact{Root: artifact.Node{Path: artifact.RootPath, Name: ".", Kind: artifact.KindDirectory}}
}

func TestBuildEncodeRoundTrip(t *testing.T) {
	art := sampleArtifact()
	doc, err := snapshot.Build(art, snapshot.CaptureV1{
		UseDefaultIgnores: true,
		UseGitignore:      true,
		Ignore:            []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if doc.SchemaVersion != 1 || doc.ArtifactVersion != 1 || len(doc.RequiredFeatures) != 0 {
		t.Fatalf("%#v", doc)
	}
	encoded, err := snapshot.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(encoded, []byte("\n")) || bytes.ContainsRune(encoded, '\x1b') {
		t.Fatalf("encoding shape %q", encoded)
	}
	loaded, err := snapshot.LoadBytes(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Fingerprint.String() != doc.Fingerprint {
		t.Fatalf("fingerprint %s vs %s", loaded.Fingerprint, doc.Fingerprint)
	}
	again, err := snapshot.Marshal(loaded.Document)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, again) {
		t.Fatalf("round-trip bytes differ\n%s\n%s", encoded, again)
	}
}

func TestDeterministicEncodingAndChildPermutation(t *testing.T) {
	base := sampleArtifact()
	permuted := base.Clone()
	permuted.Root.Children[0], permuted.Root.Children[2] = permuted.Root.Children[2], permuted.Root.Children[0]
	capture := snapshot.CaptureV1{UseDefaultIgnores: true, UseGitignore: true, Ignore: []string{"a", "b"}}
	left, err := snapshot.Build(base, capture)
	if err != nil {
		t.Fatal(err)
	}
	right, err := snapshot.Build(permuted, capture)
	if err != nil {
		t.Fatal(err)
	}
	leftBytes, err := snapshot.Marshal(left)
	if err != nil {
		t.Fatal(err)
	}
	rightBytes, err := snapshot.Marshal(right)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(leftBytes, rightBytes) {
		t.Fatal("child permutation changed snapshot bytes")
	}
	if left.Fingerprint != right.Fingerprint {
		t.Fatal("child permutation changed fingerprint")
	}
}

func TestCaptureOutsideFingerprint(t *testing.T) {
	art := emptyArtifact()
	a, err := snapshot.Build(art, snapshot.CaptureV1{Ignore: []string{"a"}, UseDefaultIgnores: true, UseGitignore: true})
	if err != nil {
		t.Fatal(err)
	}
	b, err := snapshot.Build(art, snapshot.CaptureV1{Ignore: []string{"b"}, UseDefaultIgnores: true, UseGitignore: true})
	if err != nil {
		t.Fatal(err)
	}
	if a.Fingerprint != b.Fingerprint {
		t.Fatal("capture-only change altered fingerprint")
	}
	ab, _ := snapshot.Marshal(a)
	bb, _ := snapshot.Marshal(b)
	if bytes.Equal(ab, bb) {
		t.Fatal("capture-only change did not alter snapshot bytes")
	}
}

func TestIgnoreOrderPreserved(t *testing.T) {
	doc, err := snapshot.Build(emptyArtifact(), snapshot.CaptureV1{
		Ignore:            []string{"z", "a", "m"},
		UseDefaultIgnores: true,
		UseGitignore:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := snapshot.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte(`"z"`)) || !bytes.Contains(encoded, []byte(`"a"`)) || !bytes.Contains(encoded, []byte(`"m"`)) {
		t.Fatalf("ignore patterns missing:\n%s", encoded)
	}
	z := bytes.Index(encoded, []byte(`"z"`))
	a := bytes.Index(encoded, []byte(`"a"`))
	m := bytes.Index(encoded, []byte(`"m"`))
	if z >= a || a >= m {
		t.Fatalf("ignore order lost:\n%s", encoded)
	}
}

func TestDepthNullAndFinite(t *testing.T) {
	unlimited, err := snapshot.Build(emptyArtifact(), snapshot.CaptureV1{Ignore: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	enc, _ := snapshot.Marshal(unlimited)
	if !bytes.Contains(enc, []byte(`"depth": null`)) {
		t.Fatalf("want null depth:\n%s", enc)
	}
	zero := 0
	finite, err := snapshot.Build(emptyArtifact(), snapshot.CaptureV1{Depth: &zero, Ignore: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	enc, _ = snapshot.Marshal(finite)
	if !bytes.Contains(enc, []byte(`"depth": 0`)) {
		t.Fatalf("want depth 0:\n%s", enc)
	}
}

func TestDecodeRejects(t *testing.T) {
	cases := []struct {
		name string
		body string
		code string
	}{
		{"empty", "", snapshot.CodeInvalidJSON},
		{"invalid-json", "{", snapshot.CodeInvalidJSON},
		{"trailing", "{}\n{}", snapshot.CodeInvalidJSON},
		{"duplicate-key", `{"schemaVersion":1,"schemaVersion":1}`, snapshot.CodeInvalidJSON},
		{"missing-fingerprint", `{
  "schemaVersion": 1,
  "artifactVersion": 1,
  "requiredFeatures": [],
  "capture": {"depth":null,"dirsOnly":false,"hidden":false,"useDefaultIgnores":true,"useGitignore":true,"ignore":[]},
  "artifact": {"nodes":[{"path":".","kind":"directory"}]}
}`, snapshot.CodeMissingField},
		{"unknown-schema", mustBuildUnknownSchema(t), snapshot.CodeUnsupportedSchema},
		{"unknown-artifact", mustBuildUnknownArtifact(t), snapshot.CodeUnsupportedArtifact},
		{"unknown-feature", mustBuildUnknownFeature(t), snapshot.CodeUnsupportedFeature},
		{"duplicate-feature", mustBuildDuplicateFeature(t), snapshot.CodeInvalidField},
		{"malformed-path", mustBuildMalformedPath(t), snapshot.CodeInvalidArtifact},
		{"fingerprint-mismatch", mustBuildFingerprintMismatch(t), snapshot.CodeFingerprintMismatch},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := snapshot.LoadBytes([]byte(tc.body))
			if err == nil {
				t.Fatal("expected error")
			}
			if snapshot.CodeOf(err) != tc.code {
				t.Fatalf("code=%q want %q (%v)", snapshot.CodeOf(err), tc.code, err)
			}
		})
	}
}

func TestUnknownAdditiveFieldAccepted(t *testing.T) {
	doc, err := snapshot.Build(emptyArtifact(), snapshot.CaptureV1{Ignore: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := snapshot.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	patched := bytes.Replace(encoded, []byte(`"schemaVersion": 1,`), []byte(`"schemaVersion": 1,
  "futureOptional": true,`), 1)
	if _, err := snapshot.LoadBytes(patched); err != nil {
		t.Fatal(err)
	}
}

func TestWriterErrorsPropagate(t *testing.T) {
	doc, err := snapshot.Build(emptyArtifact(), snapshot.CaptureV1{Ignore: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	err = snapshot.Encode(&shortWriter{n: 3}, doc)
	if err == nil {
		t.Fatal("expected write error")
	}
	if snapshot.CodeOf(err) != snapshot.CodeWriteFailure {
		t.Fatalf("code=%q", snapshot.CodeOf(err))
	}
	err = snapshot.Encode(&errWriter{err: errors.New("boom")}, doc)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("%v", err)
	}
}

func TestSymlinkTargetRequiredAndEmptyAllowed(t *testing.T) {
	emptyTarget := ""
	art := artifact.Artifact{Root: artifact.Node{
		Path: artifact.RootPath, Name: ".", Kind: artifact.KindDirectory,
		Children: []artifact.Node{{Path: "broken", Name: "broken", Kind: artifact.KindSymlink, Target: emptyTarget}},
	}}
	doc, err := snapshot.Build(art, snapshot.CaptureV1{Ignore: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := snapshot.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte(`"target": ""`)) {
		t.Fatalf("%s", encoded)
	}
	if _, err := snapshot.LoadBytes(encoded); err != nil {
		t.Fatal(err)
	}
}

func TestCanonicalWriterRejectsInvalidDocument(t *testing.T) {
	doc, err := snapshot.Build(emptyArtifact(), snapshot.CaptureV1{Ignore: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	doc.SchemaVersion = 99
	if _, err := snapshot.Marshal(doc); err == nil || snapshot.CodeOf(err) != snapshot.CodeUnsupportedSchema {
		t.Fatalf("schema: %v", err)
	}
	doc.SchemaVersion = 1
	doc.Fingerprint = "dlm:v1:sha256:" + strings.Repeat("0", 64)
	if _, err := snapshot.Marshal(doc); err == nil || snapshot.CodeOf(err) != snapshot.CodeFingerprintMismatch {
		t.Fatalf("fingerprint: %v", err)
	}
}

func TestSnapshotPOSIXBackslashFilenameRoundTrip(t *testing.T) {
	art := artifact.Artifact{Root: artifact.Node{
		Path: artifact.RootPath, Name: ".", Kind: artifact.KindDirectory,
		Children: []artifact.Node{
			{Path: `a\b.txt`, Name: `a\b.txt`, Kind: artifact.KindFile},
			{Path: `dir`, Name: "dir", Kind: artifact.KindDirectory, Children: []artifact.Node{
				{Path: `dir/a\b.txt`, Name: `a\b.txt`, Kind: artifact.KindFile},
			}},
		},
	}}
	doc, err := snapshot.Build(art, snapshot.CaptureV1{Ignore: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := snapshot.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := snapshot.LoadBytes(encoded)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, node := range loaded.Document.Artifact.Nodes {
		got[node.Path] = node.Kind
	}
	if got[`a\b.txt`] != "file" || got[`dir/a\b.txt`] != "file" {
		t.Fatalf("%#v", got)
	}
}

func TestDecodeRejectsNonJSONUnicodeWhitespace(t *testing.T) {
	valid := mustValidEmpty(t)
	nbsp := []byte{0xC2, 0xA0}
	leading := append(append([]byte{}, nbsp...), valid...)
	trailing := append(append([]byte{}, valid...), nbsp...)
	for _, body := range [][]byte{leading, trailing} {
		_, err := snapshot.DecodeBytes(body)
		if err == nil || snapshot.CodeOf(err) != snapshot.CodeInvalidJSON {
			t.Fatalf("%q: %v", body[:min(8, len(body))], err)
		}
	}
	framed := append(append([]byte(" \t\r\n"), valid...), []byte(" \t\r\n")...)
	if _, err := snapshot.DecodeBytes(framed); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeRejectsInvalidUTF8(t *testing.T) {
	body := mustValidEmpty(t)
	body = append(body, 0xff)
	_, err := snapshot.DecodeBytes(body)
	if err == nil || snapshot.CodeOf(err) != snapshot.CodeInvalidJSON || !strings.Contains(err.Error(), "UTF-8") {
		t.Fatalf("%v", err)
	}
}

func TestLoadRejectsProvableCaptureContradictions(t *testing.T) {
	body := mustValidEmpty(t)
	fileNode := patchSnapshot(t, body, `"kind": "directory"
      }
    ]`, `"kind": "directory"
      },
      {
        "path": "README.md",
        "kind": "file"
      }
    ]`)
	dirsOnly := patchSnapshot(t, []byte(fileNode), `"dirsOnly": false`, `"dirsOnly": true`)
	if _, err := snapshot.LoadBytes([]byte(dirsOnly)); err == nil || snapshot.CodeOf(err) != snapshot.CodeInvalidField {
		t.Fatalf("dirsOnly: %v", err)
	}
	deep := patchSnapshot(t, body, `"depth": null`, `"depth": 0`)
	deep = patchSnapshot(t, []byte(deep), `"kind": "directory"
      }
    ]`, `"kind": "directory"
      },
      {
        "path": "src",
        "kind": "directory"
      }
    ]`)
	if _, err := snapshot.LoadBytes([]byte(deep)); err == nil || snapshot.CodeOf(err) != snapshot.CodeInvalidField {
		t.Fatalf("depth: %v", err)
	}
}

func TestLoadRejectsInvalidArtifactShapes(t *testing.T) {
	body := mustValidEmpty(t)
	cases := []struct {
		name string
		body string
	}{
		{"null-nodes", patchSnapshot(t, body, `"nodes": [
      {
        "path": ".",
        "kind": "directory"
      }
    ]`, `"nodes": null`)},
		{"target-on-file", patchSnapshot(t, body, `"kind": "directory"
      }
    ]`, `"kind": "directory"
      },
      {
        "path": "a.txt",
        "kind": "file",
        "target": "nope"
      }
    ]`)},
		{"missing-symlink-target", patchSnapshot(t, body, `"kind": "directory"
      }
    ]`, `"kind": "directory"
      },
      {
        "path": "link",
        "kind": "symlink"
      }
    ]`)},
		{"parent-not-directory", patchSnapshot(t, body, `"kind": "directory"
      }
    ]`, `"kind": "directory"
      },
      {
        "path": "a.txt",
        "kind": "file"
      },
      {
        "path": "a.txt/child",
        "kind": "file"
      }
    ]`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := snapshot.LoadBytes([]byte(tc.body)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestGoldenValidFixtures(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "snapshots", "v1", "valid")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".dlm.json") {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			loaded, err := snapshot.LoadBytes(data)
			if err != nil {
				t.Fatal(err)
			}
			again, err := snapshot.Marshal(loaded.Document)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(normalizeNewlines(data), again) {
				t.Fatalf("golden re-encode mismatch for %s", entry.Name())
			}
			if _, err := identity.Parse(loaded.Fingerprint.String()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGoldenInvalidFixtures(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "snapshots", "v1", "invalid")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".dlm.json") {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			_, err = snapshot.LoadBytes(data)
			if err == nil {
				t.Fatal("expected invalid fixture to fail")
			}
			if snapshot.CodeOf(err) == "" {
				t.Fatalf("unclassified error: %v", err)
			}
		})
	}
}

func normalizeNewlines(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
}

type shortWriter struct{ n int }

func (w *shortWriter) Write(p []byte) (int, error) {
	if w.n <= 0 {
		return 0, io.ErrShortWrite
	}
	if len(p) > w.n {
		n := w.n
		w.n = 0
		return n, io.ErrShortWrite
	}
	w.n -= len(p)
	return len(p), nil
}

type errWriter struct{ err error }

func (w *errWriter) Write([]byte) (int, error) { return 0, w.err }

func mustValidEmpty(t *testing.T) []byte {
	t.Helper()
	doc, err := snapshot.Build(emptyArtifact(), snapshot.CaptureV1{Ignore: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	b, err := snapshot.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func patchSnapshot(t *testing.T, body []byte, old, new string) string {
	t.Helper()
	out := bytes.Replace(body, []byte(old), []byte(new), 1)
	if bytes.Equal(out, body) {
		t.Fatalf("pattern %q not found", old)
	}
	return string(out)
}

func mustBuildUnknownSchema(t *testing.T) string {
	t.Helper()
	return patchSnapshot(t, mustValidEmpty(t), `"schemaVersion": 1`, `"schemaVersion": 99`)
}

func mustBuildUnknownArtifact(t *testing.T) string {
	t.Helper()
	return patchSnapshot(t, mustValidEmpty(t), `"artifactVersion": 1`, `"artifactVersion": 99`)
}

func mustBuildUnknownFeature(t *testing.T) string {
	t.Helper()
	return patchSnapshot(t, mustValidEmpty(t), `"requiredFeatures": []`, `"requiredFeatures": ["future-feature"]`)
}

func mustBuildDuplicateFeature(t *testing.T) string {
	t.Helper()
	body := mustBuildUnknownFeature(t)
	return strings.Replace(body, `"requiredFeatures": ["future-feature"]`, `"requiredFeatures": ["x","x"]`, 1)
}

func mustBuildMalformedPath(t *testing.T) string {
	t.Helper()
	return patchSnapshot(t, mustValidEmpty(t), `"kind": "directory"
      }
    ]`, `"kind": "directory"
      },
      {
        "path": "src/../other.go",
        "kind": "file"
      }
    ]`)
}

func mustBuildFingerprintMismatch(t *testing.T) string {
	t.Helper()
	body := mustValidEmpty(t)
	start := bytes.Index(body, []byte(`"fingerprint": "`))
	if start < 0 {
		t.Fatal("fingerprint missing")
	}
	end := bytes.IndexByte(body[start:], ',')
	if end < 0 {
		t.Fatal("fingerprint field unterminated")
	}
	return patchSnapshot(t, body, string(body[start:start+end]), `"fingerprint": "dlm:v1:sha256:`+strings.Repeat("0", 64)+`"`)
}
