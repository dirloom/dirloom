package identity

import (
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
)

func FuzzFingerprintParse(f *testing.F) {
	fp, err := Compute(emptyArtifact())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(fp.String())
	f.Add("dlm:v1:sha256:" + strings.Repeat("ab", 32))
	f.Add("dlm:v1:sha256:")
	f.Add("DLM:v1:sha256:" + strings.Repeat("ab", 32))
	f.Add("not-a-fp")
	f.Fuzz(func(t *testing.T, value string) {
		parsed, err := Parse(value)
		if err != nil {
			return
		}
		if parsed.String() != value {
			t.Fatalf("accepted %q reformatted as %q", value, parsed.String())
		}
		again, againErr := Parse(parsed.String())
		if againErr != nil || again != parsed {
			t.Fatal(againErr)
		}
	})
}

func FuzzIdentityEncode(f *testing.F) {
	f.Add("a.txt", "directory")
	f.Add("café", "file")
	f.Add("link", "symlink")
	f.Fuzz(func(t *testing.T, name, kind string) {
		path, err := artifact.Canonicalize(name, artifact.POSIXSeparator)
		if err != nil || path == artifact.RootPath {
			return
		}
		nodeKind := artifact.Kind(kind)
		if !nodeKind.Valid() {
			nodeKind = artifact.KindFile
		}
		target := ""
		if nodeKind.HasTarget() {
			target = "target"
		}
		if nodeKind == artifact.KindDirectory {
			target = ""
		}
		art := artifact.Artifact{Root: artifact.Node{
			Path: artifact.RootPath, Name: ".", Kind: artifact.KindDirectory,
			Children: []artifact.Node{{Path: path, Name: path.Base(), Kind: nodeKind, Target: target}},
		}}
		if art.Validate() != nil {
			return
		}
		first, err := Compute(art)
		if err != nil {
			t.Fatal(err)
		}
		second, err := Compute(art)
		if err != nil || first != second {
			t.Fatalf("non-deterministic %v %v", first, second)
		}
	})
}
