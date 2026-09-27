package snapshot_test

import (
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/snapshot"
)

func FuzzSnapshotDecode(f *testing.F) {
	seeds := []string{
		`{}`,
		`[]`,
		`null`,
		`{"schemaVersion":1}`,
		mustSeedEmpty(f),
		mustSeedSymlink(f),
		`{"schemaVersion":1,"schemaVersion":1}`,
		`{"schemaVersion":1,"artifactVersion":1,"fingerprint":"bad","requiredFeatures":[],"capture":{"depth":null,"dirsOnly":false,"hidden":false,"useDefaultIgnores":true,"useGitignore":true,"ignore":[]},"artifact":{"nodes":[{"path":".","kind":"directory"}]}}`,
		`{"schemaVersion":1,"artifactVersion":1,"fingerprint":"dlm:v1:sha256:0000000000000000000000000000000000000000000000000000000000000000","requiredFeatures":["x"],"capture":{"depth":null,"dirsOnly":false,"hidden":false,"useDefaultIgnores":true,"useGitignore":true,"ignore":[]},"artifact":{"nodes":[{"path":".","kind":"directory"}]}}`,
		`{"schemaVersion":1,"artifactVersion":1,"fingerprint":"dlm:v1:sha256:0000000000000000000000000000000000000000000000000000000000000000","requiredFeatures":[],"capture":{"depth":null,"dirsOnly":false,"hidden":false,"useDefaultIgnores":true,"useGitignore":true,"ignore":[]},"artifact":{"nodes":[{"path":"src/../x","kind":"file"}]}}`,
	}
	for _, seed := range seeds {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = snapshot.DecodeBytes(data)
	})
}

func FuzzSnapshotValidation(f *testing.F) {
	f.Add([]byte(mustSeedEmpty(f)))
	f.Add([]byte(mustSeedSymlink(f)))
	f.Add([]byte(`{"schemaVersion":2,"artifactVersion":1,"fingerprint":"dlm:v1:sha256:0000000000000000000000000000000000000000000000000000000000000000","requiredFeatures":[],"capture":{"depth":null,"dirsOnly":false,"hidden":false,"useDefaultIgnores":true,"useGitignore":true,"ignore":[]},"artifact":{"nodes":[{"path":".","kind":"directory"}]}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		doc, err := snapshot.DecodeBytes(data)
		if err != nil {
			return
		}
		_, _ = snapshot.Validate(doc)
	})
}

func mustSeedEmpty(t testing.TB) string {
	t.Helper()
	doc, err := snapshot.Build(artifact.Artifact{Root: artifact.Node{Path: ".", Name: ".", Kind: artifact.KindDirectory}}, snapshot.CaptureV1{Ignore: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	b, err := snapshot.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func mustSeedSymlink(t testing.TB) string {
	t.Helper()
	art := artifact.Artifact{Root: artifact.Node{
		Path: ".", Name: ".", Kind: artifact.KindDirectory,
		Children: []artifact.Node{{Path: "link", Name: "link", Kind: artifact.KindSymlink, Target: "missing"}},
	}}
	doc, err := snapshot.Build(art, snapshot.CaptureV1{Ignore: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	b, err := snapshot.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
