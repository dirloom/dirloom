//go:build windows

package source

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/tree"
)

// TestFilesystemWindowsJunction checks that a junction is kind junction with
// a target, whatever mode Go reports for it (GODEBUG=winsymlink=0 or not).
func TestFilesystemWindowsJunction(t *testing.T) {
	for name, godebug := range map[string]string{"default": "", "winsymlink-0": "winsymlink=0"} {
		// No "=" in the name: it reaches t.TempDir and the mklink command line.
		t.Run(name, func(t *testing.T) {
			t.Setenv("GODEBUG", godebug)
			root := t.TempDir()
			outside := t.TempDir()
			target := filepath.Join(root, "target")
			if err := os.Mkdir(target, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(target, "inside.txt"), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			for link, to := range map[string]string{"junction": target, "outside": outside} {
				cmd := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(root, link), to)
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("mklink /J: %v\n%s", err, output)
				}
			}
			art := observeFS(t, root, tree.ScanOptions{RootAbs: root, RootName: "proj"})
			// A target inside the root is root-relative; one outside it is
			// kept absolute, with "/" separators.
			for name, want := range map[string]string{"junction": "target", "outside": filepath.ToSlash(outside)} {
				var found *artifact.Node
				for i := range art.Root.Children {
					if art.Root.Children[i].Name == name {
						found = &art.Root.Children[i]
					}
				}
				if found == nil {
					t.Fatalf("%s missing: %#v", name, childNames(art.Root))
				}
				if found.Kind != artifact.KindJunction || found.Target != want || len(found.Children) != 0 {
					t.Fatalf("%s = %#v, want a terminal junction to %q", name, found, want)
				}
			}
		})
	}
}

func TestFilesystemWindowsDriveIndependence(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	art := observeFS(t, root, tree.ScanOptions{RootAbs: root, RootName: "proj"})
	if filepath.IsAbs(string(art.Root.Path)) || len(art.Root.Path) >= 2 && art.Root.Path[1] == ':' {
		t.Fatalf("drive leaked into path %q", art.Root.Path)
	}
	for _, child := range art.Root.Children {
		if len(child.Path) >= 2 && child.Path[1] == ':' {
			t.Fatalf("drive leaked into %q", child.Path)
		}
	}
}
