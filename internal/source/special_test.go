package source

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/filter"
	"github.com/dirloom/dirloom/internal/tree"
)

// TestFromTreeRefusesSpecialEntries checks that a named pipe, socket or
// device observed as a file is refused with unsupported_node_type, naming its
// real type and path, instead of being stored as a file.
func TestFromTreeRefusesSpecialEntries(t *testing.T) {
	for _, special := range []string{"named pipe", "socket", "device", "character device"} {
		root := &tree.Node{Name: "proj", Type: tree.NodeDirectory, Children: []*tree.Node{
			{Name: "a.txt", Path: "a.txt", Type: tree.NodeFile},
			{Name: "run", Path: "run", Type: tree.NodeDirectory, Children: []*tree.Node{
				{Name: "entry", Path: "run/entry", Type: tree.NodeFile, Special: special},
			}},
		}}
		_, err := FromTree(root, FromTreeOptions{})
		var artErr *artifact.Error
		if !errors.As(err, &artErr) || artErr.Code != artifact.CodeUnsupportedKind || artifact.IsInternal(err) {
			t.Fatalf("%s: error = %v, want %s", special, err, artifact.CodeUnsupportedKind)
		}
		if !strings.Contains(artErr.Message, special) || !strings.Contains(artErr.Message, `"run/entry"`) {
			t.Fatalf("%s: message %q does not name the type and the path", special, artErr.Message)
		}
	}
}

func TestFromTreeRejectsSpecialMarkOnNonFile(t *testing.T) {
	root := &tree.Node{Name: "proj", Type: tree.NodeDirectory, Children: []*tree.Node{
		{Name: "d", Path: "d", Type: tree.NodeDirectory, Special: "socket", Children: []*tree.Node{}},
	}}
	if _, err := FromTree(root, FromTreeOptions{}); !artifact.IsInternal(err) {
		t.Fatalf("error = %v, want an internal invariant failure", err)
	}
}

// TestFromTreeInvariantViolationIsInternal checks the call context of Q2:
// an artifact that Dirloom builds from an observation and that breaks an
// invariant is a Dirloom bug, while a collision of distinct names is in the
// observed data.
func TestFromTreeInvariantViolationIsInternal(t *testing.T) {
	for name, children := range map[string][]*tree.Node{
		"orphan":    {{Name: "b", Path: "a/b", Type: tree.NodeFile}},
		"duplicate": {{Name: "a.txt", Path: "a.txt", Type: tree.NodeFile}, {Name: "a.txt", Path: "a.txt", Type: tree.NodeFile}},
	} {
		_, err := FromTree(&tree.Node{Name: "proj", Type: tree.NodeDirectory, Children: children}, FromTreeOptions{})
		var artErr *artifact.Error
		if !errors.As(err, &artErr) || artErr.Code != artifact.CodeInvariant || !artifact.IsInternal(err) {
			t.Fatalf("%s: error = %#v, want an internal %s", name, err, artifact.CodeInvariant)
		}
	}
	_, err := FromTree(&tree.Node{Name: "proj", Type: tree.NodeDirectory, Children: []*tree.Node{
		{Name: "é", Path: "é", Type: tree.NodeFile},
		{Name: "é", Path: "é", Type: tree.NodeFile},
	}}, FromTreeOptions{})
	var artErr *artifact.Error
	if !errors.As(err, &artErr) || artErr.Code != artifact.CodeCollision || artifact.IsInternal(err) {
		t.Fatalf("collision: error = %#v, want a non-internal %s", err, artifact.CodeCollision)
	}
}

// listenUnix holds an AF_UNIX socket open at dir/name until the test ends. It
// skips the test where AF_UNIX is unavailable (Windows before 10 1803).
func listenUnix(t *testing.T, dir, name string) {
	t.Helper()
	listener, err := net.Listen("unix", filepath.Join(dir, name))
	if err != nil {
		t.Skipf("AF_UNIX sockets unavailable: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
}

// shortTempDir returns a directory with a short path: a socket path is limited
// to about 100 bytes, and t.TempDir includes the test name.
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "dl")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// TestFilesystemRefusesSocket observes a real socket on every OS: it is
// refused, unless a filter or --dirs-only removes it first.
func TestFilesystemRefusesSocket(t *testing.T) {
	root := shortTempDir(t)
	mustWrite(t, filepath.Join(root, "a.txt"), "x")
	listenUnix(t, root, "s.sock")
	_, err := observeFSErr(t, root, tree.ScanOptions{RootAbs: root, RootName: "proj"})
	var artErr *artifact.Error
	if !errors.As(err, &artErr) || artErr.Code != artifact.CodeUnsupportedKind || !strings.Contains(artErr.Message, "socket") {
		t.Fatalf("socket observation error = %v, want %s", err, artifact.CodeUnsupportedKind)
	}

	ignore, err := filter.NewIgnoreMatcher([]string{"*.sock"})
	if err != nil {
		t.Fatal(err)
	}
	policy := filter.NewPolicy("", false, ignore, nil, true)
	ignored := observeFS(t, root, tree.ScanOptions{RootAbs: root, RootName: "proj", FilterPolicy: policy})
	if names := childNames(ignored.Root); len(names) != 1 || names[0] != "a.txt" {
		t.Fatalf("ignored socket observed: %v", names)
	}
	dirs := observeFS(t, root, tree.ScanOptions{RootAbs: root, RootName: "proj", Directories: true})
	if len(dirs.Root.Children) != 0 {
		t.Fatalf("--dirs-only observed %v", childNames(dirs.Root))
	}
}
