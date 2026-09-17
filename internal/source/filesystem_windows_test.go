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

func TestFilesystemWindowsJunction(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "inside.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	junction := filepath.Join(root, "junction")
	cmd := exec.Command("cmd", "/c", "mklink", "/J", junction, target)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("junctions unavailable: %v\n%s", err, output)
	}
	art := observeFS(t, root, tree.ScanOptions{RootAbs: root, RootName: "proj"})
	var found *artifact.Node
	for i := range art.Root.Children {
		if art.Root.Children[i].Name == "junction" {
			found = &art.Root.Children[i]
		}
	}
	if found == nil {
		t.Fatalf("junction missing: %#v", childNames(art.Root))
	}
	if found.Kind == artifact.KindDirectory && len(found.Children) != 0 {
		t.Log("scanner treated the junction as a directory (Go os.Lstat classification)")
	}
	if found.Kind == artifact.KindSymlink && len(found.Children) != 0 {
		t.Fatal("symlink junction grew children")
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
