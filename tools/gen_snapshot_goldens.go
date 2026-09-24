//go:build ignore

package main

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/snapshot"
)

func main() {
	root := filepath.Join("testdata", "snapshots", "v1")
	_ = os.MkdirAll(filepath.Join(root, "valid"), 0o755)
	_ = os.MkdirAll(filepath.Join(root, "invalid"), 0o755)

	write := func(rel string, doc snapshot.Document) {
		b, err := snapshot.Marshal(doc)
		if err != nil {
			panic(err)
		}
		if err := os.WriteFile(filepath.Join(root, rel), b, 0o644); err != nil {
			panic(err)
		}
	}
	mustBuild := func(art artifact.Artifact, cap snapshot.CaptureV1) snapshot.Document {
		doc, err := snapshot.Build(art, cap)
		if err != nil {
			panic(err)
		}
		return doc
	}

	empty := artifact.Artifact{Root: artifact.Node{Path: ".", Name: ".", Kind: artifact.KindDirectory}}
	single := artifact.Artifact{Root: artifact.Node{Path: ".", Name: ".", Kind: artifact.KindDirectory, Children: []artifact.Node{
		{Path: "a.txt", Name: "a.txt", Kind: artifact.KindFile},
	}}}
	nested := artifact.Artifact{Root: artifact.Node{Path: ".", Name: ".", Kind: artifact.KindDirectory, Children: []artifact.Node{
		{Path: "src", Name: "src", Kind: artifact.KindDirectory, Children: []artifact.Node{
			{Path: "src/main.go", Name: "main.go", Kind: artifact.KindFile},
		}},
		{Path: "README.md", Name: "README.md", Kind: artifact.KindFile},
	}}}
	unicode := artifact.Artifact{Root: artifact.Node{Path: ".", Name: ".", Kind: artifact.KindDirectory, Children: []artifact.Node{
		{Path: "café.txt", Name: "café.txt", Kind: artifact.KindFile},
	}}}
	symlink := artifact.Artifact{Root: artifact.Node{Path: ".", Name: ".", Kind: artifact.KindDirectory, Children: []artifact.Node{
		{Path: "src", Name: "src", Kind: artifact.KindDirectory, Children: []artifact.Node{
			{Path: "src/main.go", Name: "main.go", Kind: artifact.KindFile},
		}},
		{Path: "link", Name: "link", Kind: artifact.KindSymlink, Target: "src/main.go"},
	}}}
	depth := 1
	// The persisted artifact must be one that depth=1 and dirsOnly=true can observe:
	// the root and its immediate directories, with no files and no deeper nodes.
	filteredArt := artifact.Artifact{Root: artifact.Node{Path: ".", Name: ".", Kind: artifact.KindDirectory, Children: []artifact.Node{
		{Path: "src", Name: "src", Kind: artifact.KindDirectory},
	}}}
	filtered := mustBuild(filteredArt, snapshot.CaptureV1{
		Depth: &depth, DirsOnly: true, Hidden: true,
		UseDefaultIgnores: false, UseGitignore: false,
		Ignore: []string{"generated", "tmp"},
	})

	defaultCap := snapshot.CaptureV1{UseDefaultIgnores: true, UseGitignore: true, Ignore: []string{}}
	write("valid/empty.dlm.json", mustBuild(empty, defaultCap))
	write("valid/single-file.dlm.json", mustBuild(single, defaultCap))
	write("valid/nested.dlm.json", mustBuild(nested, defaultCap))
	write("valid/unicode.dlm.json", mustBuild(unicode, defaultCap))
	write("valid/symlink.dlm.json", mustBuild(symlink, defaultCap))
	write("valid/filtered-view.dlm.json", filtered)

	_ = os.WriteFile(filepath.Join(root, "invalid/invalid-json.dlm.json"), []byte("{"), 0o644)
	validEmpty, err := snapshot.Marshal(mustBuild(empty, defaultCap))
	if err != nil {
		panic(err)
	}
	writeBytes := func(rel string, body []byte) {
		if err := os.WriteFile(filepath.Join(root, rel), body, 0o644); err != nil {
			panic(err)
		}
	}
	// Corrupt fixtures are patched bytes. The production writer refuses invalid documents.
	writeBytes("invalid/unknown-schema.dlm.json", bytesReplace(validEmpty, `"schemaVersion": 1`, `"schemaVersion": 9`, 1))
	writeBytes("invalid/unknown-artifact-version.dlm.json", bytesReplace(validEmpty, `"artifactVersion": 1`, `"artifactVersion": 9`, 1))

	_ = os.WriteFile(filepath.Join(root, "invalid/missing-fingerprint.dlm.json"), []byte(`{
  "schemaVersion": 1,
  "artifactVersion": 1,
  "requiredFeatures": [],
  "capture": {"depth": null, "dirsOnly": false, "hidden": false, "useDefaultIgnores": true, "useGitignore": true, "ignore": []},
  "artifact": {"nodes": [{"path": ".", "kind": "directory"}]}
}
`), 0o644)

	writeBytes("invalid/malformed-path.dlm.json", insertNode(validEmpty, `{"path": "src/../x.go", "kind": "file"}`))
	validSingle, err := snapshot.Marshal(mustBuild(single, defaultCap))
	if err != nil {
		panic(err)
	}
	writeBytes("invalid/duplicate-node.dlm.json", insertNode(validSingle, `{"path": "a.txt", "kind": "file"}`))
	writeBytes("invalid/invalid-hierarchy.dlm.json", insertNode(validEmpty, `{"path": "src/main.go", "kind": "file"}`))
	writeBytes("invalid/invalid-kind.dlm.json", insertNode(validEmpty, `{"path": "x", "kind": "socket"}`))
	fingerprint := fingerprintField(validEmpty)
	writeBytes("invalid/fingerprint-mismatch.dlm.json", bytesReplace(
		validEmpty,
		fingerprint,
		`"fingerprint": "dlm:v1:sha256:0000000000000000000000000000000000000000000000000000000000000000"`,
		1,
	))
	writeBytes("invalid/unknown-required-feature.dlm.json", bytesReplace(validEmpty, `"requiredFeatures": []`, `"requiredFeatures": ["unknown-feature"]`, 1))

	_ = os.WriteFile(filepath.Join(root, "invalid/duplicate-required-feature.dlm.json"), []byte(`{
  "schemaVersion": 1,
  "artifactVersion": 1,
  "fingerprint": "dlm:v1:sha256:0000000000000000000000000000000000000000000000000000000000000000",
  "requiredFeatures": ["x", "x"],
  "capture": {"depth": null, "dirsOnly": false, "hidden": false, "useDefaultIgnores": true, "useGitignore": true, "ignore": []},
  "artifact": {"nodes": [{"path": ".", "kind": "directory"}]}
}
`), 0o644)

	_ = os.WriteFile(filepath.Join(root, "invalid/duplicate-json-key.dlm.json"), []byte(`{
  "schemaVersion": 1,
  "schemaVersion": 1,
  "artifactVersion": 1,
  "fingerprint": "dlm:v1:sha256:0000000000000000000000000000000000000000000000000000000000000000",
  "requiredFeatures": [],
  "capture": {"depth": null, "dirsOnly": false, "hidden": false, "useDefaultIgnores": true, "useGitignore": true, "ignore": []},
  "artifact": {"nodes": [{"path": ".", "kind": "directory"}]}
}
`), 0o644)
}

func bytesReplace(data []byte, old, new string, n int) []byte {
	out := bytes.Replace(data, []byte(old), []byte(new), n)
	if bytes.Equal(out, data) {
		panic("snapshot golden pattern not found: " + old)
	}
	return out
}

func fingerprintField(data []byte) string {
	const key = `"fingerprint": "`
	start := bytes.Index(data, []byte(key))
	if start < 0 {
		panic("fingerprint field missing")
	}
	end := bytes.IndexByte(data[start+len(key):], '"')
	if end < 0 {
		panic("fingerprint field unterminated")
	}
	return string(data[start : start+len(key)+end+1])
}

func insertNode(data []byte, nodeJSON string) []byte {
	const root = `{
        "path": ".",
        "kind": "directory"
      }`
	if !bytes.Contains(data, []byte(root)) {
		panic("root node not found")
	}
	return bytesReplace(data, root, root+",\n      "+nodeJSON, 1)
}
