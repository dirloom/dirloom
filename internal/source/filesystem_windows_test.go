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

// TestFilesystemWindowsIrregularFileStaysFile observes the app execution
// aliases of WindowsApps, reparse points that Go reports as ModeIrregular:
// they are files, never refused as special entries.
func TestFilesystemWindowsIrregularFileStaysFile(t *testing.T) {
	apps := filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "WindowsApps")
	entries, err := os.ReadDir(apps)
	if err != nil {
		t.Skipf("no WindowsApps folder: %v", err)
	}
	alias := ""
	for _, entry := range entries {
		if info, err := entry.Info(); err == nil && info.Mode()&os.ModeIrregular != 0 && !info.IsDir() {
			alias = entry.Name()
			break
		}
	}
	if alias == "" {
		t.Skip("no app execution alias on this machine")
	}
	depth := 1
	art := observeFS(t, apps, tree.ScanOptions{RootAbs: apps, RootName: "WindowsApps", MaxDepth: &depth})
	for _, child := range art.Root.Children {
		if child.Name == alias {
			if child.Kind != artifact.KindFile {
				t.Fatalf("alias %s = %#v, want a file", alias, child)
			}
			return
		}
	}
	t.Fatalf("alias %s missing: %v", alias, childNames(art.Root))
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
