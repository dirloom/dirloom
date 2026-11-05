package tree

import (
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// winsymlinkModes are the two ways Go reports a junction: ModeIrregular by
// default since Go 1.23, ModeSymlink with GODEBUG=winsymlink=0. The reparse
// tag decides in both.
var winsymlinkModes = []struct {
	name, godebug string
	mode          fs.FileMode
}{
	{name: "default", godebug: "", mode: fs.ModeIrregular},
	// No "=" in the name: it reaches t.TempDir and the mklink command line.
	{name: "winsymlink-0", godebug: "winsymlink=0", mode: fs.ModeSymlink},
}

func makeJunction(t testing.TB, link, target string) {
	t.Helper()
	if output, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Fatalf("mklink /J %s: %v\n%s", link, err, output)
	}
}

// requireJunctionMode fails unless Go reports the junction at path with
// mode, so each GODEBUG case really runs the classification it names.
func requireJunctionMode(t *testing.T, path string, mode fs.FileMode) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&mode == 0 {
		t.Fatalf("Go reports junction %s with mode %v, want %v set", path, info.Mode(), mode)
	}
}

func TestScannerWindowsJunctionClassification(t *testing.T) {
	for _, mode := range winsymlinkModes {
		t.Run(mode.name, func(t *testing.T) {
			t.Setenv("GODEBUG", mode.godebug)
			root := t.TempDir()
			outside := t.TempDir()
			target := filepath.Join(root, "target")
			mustMkdirAll(t, target)
			mustWriteFile(t, filepath.Join(target, "inside.txt"), "x")
			mustWriteFile(t, filepath.Join(outside, "outside.txt"), "x")
			gone := filepath.Join(root, "gone")
			mustMkdirAll(t, gone)
			makeJunction(t, filepath.Join(root, "inner"), target)
			makeJunction(t, filepath.Join(root, "outer"), outside)
			makeJunction(t, filepath.Join(root, "broken"), gone)
			if err := os.Remove(gone); err != nil {
				t.Fatal(err)
			}
			requireJunctionMode(t, filepath.Join(root, "inner"), mode.mode)

			model := scanForTest(t, root, nil, false, false, nil)
			for name, want := range map[string]string{"inner": target, "outer": outside, "broken": gone} {
				node := childByName(model, name)
				if node == nil || node.Type != NodeSymlink || !node.Junction || node.Target != filepath.ToSlash(want) || len(node.Children) != 0 {
					t.Fatalf("junction %s = %#v, want a terminal junction to %s", name, node, filepath.ToSlash(want))
				}
			}
			if dir := childByName(model, "target"); dir == nil || dir.Type != NodeDirectory || dir.Junction {
				t.Fatalf("target directory = %#v", dir)
			}
			if n := heldHandles.Load(); n != 0 {
				t.Fatalf("%d handles left open", n)
			}
		})
	}
}

func TestScannerWindowsSymlinksAreNotJunctions(t *testing.T) {
	for _, mode := range winsymlinkModes {
		t.Run(mode.name, func(t *testing.T) {
			t.Setenv("GODEBUG", mode.godebug)
			root := t.TempDir()
			mustMkdirAll(t, filepath.Join(root, "target"))
			mustWriteFile(t, filepath.Join(root, "file.txt"), "x")
			if err := os.Symlink("target", filepath.Join(root, "dirlink")); err != nil {
				t.Skipf("symbolic links unavailable: %v", err)
			}
			if err := os.Symlink("file.txt", filepath.Join(root, "filelink")); err != nil {
				t.Skipf("symbolic links unavailable: %v", err)
			}
			model := scanForTest(t, root, nil, false, false, nil)
			for name, want := range map[string]string{"dirlink": "target", "filelink": "file.txt"} {
				node := childByName(model, name)
				if node == nil || node.Type != NodeSymlink || node.Junction || node.Target != want {
					t.Fatalf("symlink %s = %#v", name, node)
				}
			}
		})
	}
}

// TestScannerWindowsOtherReparsePointIsNotAJunction uses an AF_UNIX socket,
// a reparse point with another tag: it keeps its generic classification.
func TestScannerWindowsOtherReparsePointIsNotAJunction(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "socket")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Skipf("AF_UNIX sockets unavailable: %v", err)
	}
	defer func() { _ = listener.Close() }()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	// The entry must be a reparse point, so the scan reads its tag.
	if data := info.Sys().(*syscall.Win32FileAttributeData); data.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT == 0 {
		t.Skip("this Windows version does not store AF_UNIX sockets as reparse points")
	}
	model := scanForTest(t, root, nil, false, false, nil)
	node := childByName(model, "socket")
	if node == nil || node.Junction || node.Type != classifyNode(info) || node.Special != "socket" {
		t.Fatalf("socket = %#v, want the generic type %q marked as a socket", node, classifyNode(info))
	}
	if n := heldHandles.Load(); n != 0 {
		t.Fatalf("%d handles left open", n)
	}
}

func TestScannerWindowsJunctionBelowALongPath(t *testing.T) {
	parent := t.TempDir()
	short := filepath.Join(parent, "s")
	mustMkdirAll(t, short)
	makeJunction(t, filepath.Join(short, "link"), t.TempDir())
	long := filepath.Join(parent, "long")
	for len(long) < 280 {
		long = filepath.Join(long, "répertoire-long")
	}
	if err := os.MkdirAll(long, 0o755); err != nil {
		t.Skipf("long paths are unavailable in this Windows environment: %v", err)
	}
	// The junction moves with its parent, below a path above MAX_PATH.
	moved := filepath.Join(long, "s")
	if err := os.Rename(short, moved); err != nil {
		t.Skipf("cannot move below a long path: %v", err)
	}
	model := scanForTest(t, moved, nil, false, false, nil)
	if node := childByName(model, "link"); node == nil || node.Type != NodeSymlink || !node.Junction {
		t.Fatalf("junction below a long path = %#v", node)
	}
}

func TestExtendedPath(t *testing.T) {
	long := `C:\` + strings.Repeat(`a\`, 130) + "b"
	unc := `\\server\share\` + strings.Repeat(`a\`, 130) + "b"
	for _, test := range []struct{ in, want string }{
		{`C:\short\path`, `C:\short\path`},
		{long, `\\?\` + long},
		{unc, `\\?\UNC\server\share\` + strings.Repeat(`a\`, 130) + "b"},
		{`\\?\` + long, `\\?\` + long},
		{strings.Repeat(`a\`, 130), strings.Repeat(`a\`, 130)},
	} {
		if got := extendedPath(test.in); got != test.want {
			t.Fatalf("extendedPath(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}
