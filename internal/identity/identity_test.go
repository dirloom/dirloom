package identity

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
)

func emptyArtifact() artifact.Artifact {
	return artifact.Artifact{Root: artifact.Node{Path: artifact.RootPath, Name: ".", Kind: artifact.KindDirectory}}
}

func fileArtifact(path string) artifact.Artifact {
	return artifact.Artifact{Root: artifact.Node{
		Path: artifact.RootPath, Name: ".", Kind: artifact.KindDirectory,
		Children: []artifact.Node{{Path: artifact.Path(path), Name: path, Kind: artifact.KindFile}},
	}}
}

func TestVectorEmptyRoot(t *testing.T) {
	wantHex := "444c4d49010100000001000000012e0100"
	assertVector(t, emptyArtifact(), wantHex, "dlm:v1:sha256:"+sha256Hex(wantHex))
}

func TestVectorSingleFile(t *testing.T) {
	wantHex := "444c4d49010100000002000000012e010000000009524541444d452e6d640200"
	assertVector(t, fileArtifact("README.md"), wantHex, "dlm:v1:sha256:"+sha256Hex(wantHex))
}

func TestVectorNestedUnicodeSymlink(t *testing.T) {
	art := artifact.Artifact{Root: artifact.Node{
		Path: artifact.RootPath, Name: ".", Kind: artifact.KindDirectory,
		Children: []artifact.Node{
			{Path: "src", Name: "src", Kind: artifact.KindDirectory, Children: []artifact.Node{
				{Path: "src/café.go", Name: "café.go", Kind: artifact.KindFile},
			}},
			{Path: "link", Name: "link", Kind: artifact.KindSymlink, Target: "src/café.go"},
		},
	}}
	fp, err := Compute(art)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(fp.String()); err != nil {
		t.Fatal(err)
	}
	records, err := ProjectV1(art)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := EncodeV1(&buf, records); err != nil {
		t.Fatal(err)
	}
	gotHex := hex.EncodeToString(buf.Bytes())
	golden := filepath.Join("testdata", "vector-nested.hex")
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Split(strings.TrimSpace(string(want)), "\n")
	if len(fields) != 2 {
		t.Fatalf("golden %s", want)
	}
	if gotHex != fields[0] || fp.String() != fields[1] {
		t.Fatalf("nested vector\nhex %s\nfp  %s\nwant\n%s", gotHex, fp.String(), want)
	}
	if len(records) != 4 {
		t.Fatalf("records = %d", len(records))
	}
	if records[0].Path != "." || records[1].Path != "link" || records[2].Path != "src" || records[3].Path != "src/café.go" {
		t.Fatalf("order %#v", records)
	}
}

func TestProjectionIgnoresChildPermutationAndLabels(t *testing.T) {
	base := artifact.Artifact{
		DisplayRootName: "alpha",
		Root: artifact.Node{
			Path: artifact.RootPath, Name: ".", Kind: artifact.KindDirectory,
			Children: []artifact.Node{
				{Path: "b.txt", Name: "b.txt", Kind: artifact.KindFile},
				{Path: "a.txt", Name: "a.txt", Kind: artifact.KindFile},
				{Path: "dir", Name: "dir", Kind: artifact.KindDirectory, Children: []artifact.Node{
					{Path: "dir/z.txt", Name: "z.txt", Kind: artifact.KindFile},
					{Path: "dir/y.txt", Name: "y.txt", Kind: artifact.KindFile},
				}},
			},
		},
	}
	left, err := Compute(base)
	if err != nil {
		t.Fatal(err)
	}
	permuted := base.Clone()
	permuted.Root.Children[0], permuted.Root.Children[1] = permuted.Root.Children[1], permuted.Root.Children[0]
	permuted.Root.Children[2].Children[0], permuted.Root.Children[2].Children[1] = permuted.Root.Children[2].Children[1], permuted.Root.Children[2].Children[0]
	permuted.DisplayRootName = "completely-different-name"
	right, err := Compute(permuted)
	if err != nil {
		t.Fatal(err)
	}
	if left != right {
		t.Fatalf("permutation or label changed identity\n%s\n%s", left, right)
	}
}

func TestProjectionKindAndPathSensitivity(t *testing.T) {
	file := fileArtifact("a.txt")
	fpFile, err := Compute(file)
	if err != nil {
		t.Fatal(err)
	}
	link := artifact.Artifact{Root: artifact.Node{
		Path: artifact.RootPath, Name: ".", Kind: artifact.KindDirectory,
		Children: []artifact.Node{{Path: "a.txt", Name: "a.txt", Kind: artifact.KindSymlink, Target: "b.txt"}},
	}}
	fpLink, err := Compute(link)
	if err != nil {
		t.Fatal(err)
	}
	if fpFile == fpLink {
		t.Fatal("kind mutation kept fingerprint")
	}
	renamed := fileArtifact("b.txt")
	fpRenamed, err := Compute(renamed)
	if err != nil {
		t.Fatal(err)
	}
	if fpFile == fpRenamed {
		t.Fatal("path mutation kept fingerprint")
	}
}

func TestWriterFailureIsPropagated(t *testing.T) {
	records, err := ProjectV1(emptyArtifact())
	if err != nil {
		t.Fatal(err)
	}
	err = EncodeV1(failingWriter{}, records)
	if err == nil {
		t.Fatal("writer error ignored")
	}
	var typed *artifact.Error
	if !errors.As(err, &typed) || typed.Code != artifact.CodeEncoding {
		t.Fatalf("want encoding failure, got %v", err)
	}
}

func TestParseFingerprint(t *testing.T) {
	fp, err := Compute(emptyArtifact())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(fp.String())
	if err != nil || parsed != fp {
		t.Fatalf("round trip %v %v", parsed, err)
	}

	valid := fp.String()
	upper := strings.ToUpper(valid[len("dlm:v1:sha256:"):])
	cases := []struct {
		in   string
		want string
	}{
		{"other:v1:sha256:" + strings.Repeat("ab", 32), "namespace"},
		{"dlm:v2:sha256:" + strings.Repeat("ab", 32), "version"},
		{"dlm:v1:md5:" + strings.Repeat("ab", 32), "algorithm"},
		{"dlm:v1:sha256:" + strings.Repeat("ab", 16), "length"},
		{"dlm:v1:sha256:" + strings.Repeat("zz", 32), "hex"},
		{"dlm:v1:sha256:" + upper, "lowercase"},
		{"dlm:v1:sha256:" + strings.Repeat("ab", 32) + ":x", "separator"},
		{"dlm:v1:sha256:", "empty"},
		{"not-a-fingerprint", "invalid"},
	}
	for _, test := range cases {
		if _, err := Parse(test.in); err == nil {
			t.Errorf("accepted %q (%s)", test.in, test.want)
		}
	}
}

func TestJSONDocumentSchema(t *testing.T) {
	fp, err := Compute(emptyArtifact())
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := NewJSONDocument(fp, 1).WriteJSON(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{`"schemaVersion": 1`, `"identityVersion": 1`, `"algorithm": "sha256"`, `"nodeCount": 1`, fp.String()} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON missing %s\n%s", want, out)
		}
	}
	if strings.ContainsRune(out, '\x1b') {
		t.Fatal("ANSI in JSON")
	}
}

func assertVector(t *testing.T, art artifact.Artifact, wantHex, wantFP string) {
	t.Helper()
	records, err := ProjectV1(art)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := EncodeV1(&buf, records); err != nil {
		t.Fatal(err)
	}
	got := hex.EncodeToString(buf.Bytes())
	if got != wantHex {
		t.Fatalf("bytes\n got %s\nwant %s", got, wantHex)
	}
	fp, err := Compute(art)
	if err != nil {
		t.Fatal(err)
	}
	if fp.String() != wantFP {
		t.Fatalf("fingerprint %s want %s", fp.String(), wantFP)
	}
}

func sha256Hex(payloadHex string) string {
	raw, err := hex.DecodeString(payloadHex)
	if err != nil {
		panic(err)
	}
	return sha256Of(raw)
}

func sha256Of(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("synthetic write failure")
}

func TestEncodeDoesNotUseJSON(t *testing.T) {
	records, err := ProjectV1(fileArtifact("README.md"))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := EncodeV1(&buf, records); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(buf.Bytes(), []byte("{")) || bytes.Contains(buf.Bytes(), []byte("schemaVersion")) {
		t.Fatal("encoding looks like JSON")
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("DLMI")) {
		t.Fatal("missing magic")
	}
}

var _ io.Writer = failingWriter{}
