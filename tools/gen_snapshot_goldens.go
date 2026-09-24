//go:build ignore

package main

import (
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
	filtered := mustBuild(nested, snapshot.CaptureV1{
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
	badSchema := mustBuild(empty, defaultCap)
	badSchema.SchemaVersion = 9
	write("invalid/unknown-schema.dlm.json", badSchema)
	badArt := mustBuild(empty, defaultCap)
	badArt.ArtifactVersion = 9
	write("invalid/unknown-artifact-version.dlm.json", badArt)

	_ = os.WriteFile(filepath.Join(root, "invalid/missing-fingerprint.dlm.json"), []byte(`{
  "schemaVersion": 1,
  "artifactVersion": 1,
  "requiredFeatures": [],
  "capture": {"depth": null, "dirsOnly": false, "hidden": false, "useDefaultIgnores": true, "useGitignore": true, "ignore": []},
  "artifact": {"nodes": [{"path": ".", "kind": "directory"}]}
}
`), 0o644)

	malformed := mustBuild(empty, defaultCap)
	malformed.Artifact.Nodes = append(malformed.Artifact.Nodes, snapshot.NodeV1{Path: "src/../x.go", Kind: "file"})
	b, _ := snapshot.Marshal(malformed)
	_ = os.WriteFile(filepath.Join(root, "invalid/malformed-path.dlm.json"), b, 0o644)

	dup := mustBuild(single, defaultCap)
	dup.Artifact.Nodes = append(dup.Artifact.Nodes, snapshot.NodeV1{Path: "a.txt", Kind: "file"})
	b, _ = snapshot.Marshal(dup)
	_ = os.WriteFile(filepath.Join(root, "invalid/duplicate-node.dlm.json"), b, 0o644)

	hier := mustBuild(empty, defaultCap)
	hier.Artifact.Nodes = append(hier.Artifact.Nodes, snapshot.NodeV1{Path: "src/main.go", Kind: "file"})
	b, _ = snapshot.Marshal(hier)
	_ = os.WriteFile(filepath.Join(root, "invalid/invalid-hierarchy.dlm.json"), b, 0o644)

	kind := mustBuild(empty, defaultCap)
	kind.Artifact.Nodes = append(kind.Artifact.Nodes, snapshot.NodeV1{Path: "x", Kind: "socket"})
	b, _ = snapshot.Marshal(kind)
	_ = os.WriteFile(filepath.Join(root, "invalid/invalid-kind.dlm.json"), b, 0o644)

	mismatch := mustBuild(empty, defaultCap)
	mismatch.Fingerprint = "dlm:v1:sha256:0000000000000000000000000000000000000000000000000000000000000000"
	write("invalid/fingerprint-mismatch.dlm.json", mismatch)

	feat := mustBuild(empty, defaultCap)
	feat.RequiredFeatures = []string{"unknown-feature"}
	write("invalid/unknown-required-feature.dlm.json", feat)

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
