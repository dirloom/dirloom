package watch

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/identity"
)

func TestStructuralSetFollowsTheDepthLimit(t *testing.T) {
	obs := tree("a/", "a/b/", "a/b/c/", "a/f", "top")
	link := identity.Record{Path: "l", Kind: artifact.KindSymlink, Target: "a"}
	junction := identity.Record{Path: "j", Kind: artifact.KindJunction, Target: "a"}
	records := append(append([]identity.Record{}, obs.Records...), link, junction)
	depth := func(n int) *int { return &n }
	for _, tc := range []struct {
		name  string
		depth *int
		want  []artifact.Path
	}{
		{"unlimited", nil, []artifact.Path{".", "a", "a/b", "a/b/c"}},
		{"depth 0", depth(0), nil},
		{"depth 1", depth(1), []artifact.Path{"."}},
		{"depth 2", depth(2), []artifact.Path{".", "a"}},
		{"depth 3", depth(3), []artifact.Path{".", "a", "a/b"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := StructuralSet(records, tc.depth); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("StructuralSet = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIdentityOfADirectory(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "d")
	other := filepath.Join(base, "e")
	file := filepath.Join(base, "f")
	for _, d := range []string{dir, other} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := IdentityOf(dir)
	if err != nil {
		t.Fatalf("IdentityOf: %v", err)
	}
	if again, _ := IdentityOf(dir); again != first {
		t.Fatalf("identity changed between two reads: %+v, %+v", first, again)
	}
	if o, _ := IdentityOf(other); o == first {
		t.Fatalf("two directories share the identity %+v", first)
	}
	moved := filepath.Join(base, "moved")
	if err := os.Rename(dir, moved); err != nil {
		t.Fatal(err)
	}
	if m, _ := IdentityOf(moved); m != first {
		t.Fatalf("a renamed directory changed identity: %+v, %+v", first, m)
	}
	if _, err := IdentityOf(file); !errors.Is(err, ErrNotDirectory) {
		t.Fatalf("IdentityOf(file) = %v, want ErrNotDirectory", err)
	}
	if _, err := IdentityOf(filepath.Join(base, "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("IdentityOf(missing) = %v, want os.ErrNotExist", err)
	}
}

func TestIdentityTellsARecreatedDirectoryApart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "d")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	reused := 0
	for i := 0; i < 50; i++ {
		before, err := IdentityOf(dir)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(dir); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		after, err := IdentityOf(dir)
		if err != nil {
			t.Fatal(err)
		}
		if after.Volume == before.Volume && after.File == before.File {
			reused++
		}
		if after == before {
			if before.Birth == 0 && before.Handle == "" && runtime.GOOS != "windows" {
				t.Skipf("this filesystem provides neither a birth time nor a file handle, and reused the inode (round %d)", i)
			}
			t.Fatalf("round %d: a recreated directory kept the identity (volume %d, file %d, birth %d, handle %x)",
				i, before.Volume, before.File, before.Birth, before.Handle)
		}
	}
	t.Logf("file id reused in %d of 50 recreations", reused)
}
