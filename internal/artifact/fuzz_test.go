package artifact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNoPresentationOrCLIImports(t *testing.T) {
	forbidden := []string{
		"github.com/dirloom/dirloom/internal/presentation",
		"github.com/dirloom/dirloom/internal/render",
		"github.com/dirloom/dirloom/internal/cli",
		"github.com/spf13/cobra",
	}
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		text := string(data)
		for _, pkg := range forbidden {
			if strings.Contains(text, `"`+pkg+`"`) {
				t.Errorf("%s imports %s", path, pkg)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func FuzzCanonicalPath(f *testing.F) {
	for _, seed := range []string{
		".", "src/main.go", `src\core`, "..", "../x", "a/../../b",
		"café.txt", "cafe\u0301.txt", "Auth", "auth",
		strings.Repeat("n", 200), ".env", "my file.txt", "你好",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, native string) {
		for _, sep := range []byte{POSIXSeparator, WindowsSeparator} {
			path, err := Canonicalize(native, sep)
			if err != nil {
				continue
			}
			if path == "" {
				t.Fatal("empty canonical path")
			}
			if !utf8.ValidString(string(path)) {
				t.Fatal("canonical path is not UTF-8")
			}
			if isAbsoluteNative(string(path), POSIXSeparator) && path != RootPath {
				t.Fatalf("absolute canonical path %q", path)
			}
			if strings.HasPrefix(string(path), "../") || path == ".." {
				t.Fatalf("escaped root: %q", path)
			}
			again, againErr := Canonicalize(string(path), POSIXSeparator)
			if againErr != nil {
				t.Fatalf("accepted path %q is not idempotent: %v", path, againErr)
			}
			if again != path {
				t.Fatalf("idempotence %q vs %q", path, again)
			}
		}
	})
}

func FuzzArtifactValidation(f *testing.F) {
	f.Add(".", "file.txt", uint8(1), "src")
	f.Add("..", "/abs", uint8(9), "a")
	f.Add("café", "cafe\u0301", uint8(2), "")
	f.Fuzz(func(t *testing.T, rootName, childPath string, kind byte, childName string) {
		var nodeKind Kind
		if decoded, err := KindFromCode(kind); err == nil {
			nodeKind = decoded
		} else {
			nodeKind = Kind(string(rune(kind)))
		}
		art := Artifact{Root: Node{
			Path: Path(rootName), Name: rootName, Kind: KindDirectory,
			Children: []Node{{Path: Path(childPath), Name: childName, Kind: nodeKind}},
		}}
		_ = art.Validate()
	})
}
