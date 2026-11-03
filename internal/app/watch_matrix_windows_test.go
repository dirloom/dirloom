package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/dirloom/dirloom/internal/watch"
)

func setHidden(t *testing.T, p string, hidden bool) {
	t.Helper()
	ptr, err := windows.UTF16PtrFromString(p)
	must(t, err)
	attrs, err := windows.GetFileAttributes(ptr)
	must(t, err)
	if hidden {
		attrs |= windows.FILE_ATTRIBUTE_HIDDEN
	} else {
		attrs &^= windows.FILE_ATTRIBUTE_HIDDEN
	}
	must(t, windows.SetFileAttributes(ptr, attrs))
}

func TestWatchMatrixWindowsHiddenAttribute(t *testing.T) {
	toggle := func(hidden bool) func(t *testing.T, root string) {
		return func(t *testing.T, root string) { setHidden(t, join(root, "h"), hidden) }
	}
	setup := func(t *testing.T, root string) { writeFiles(t, join(root, "h")) }
	runMatrix(t,
		matrixScenario{name: "hidden excluded", setup: setup, steps: []matrixStep{
			{name: "hidden", do: toggle(true), expect: expectChanges("REMOVED h")},
			{name: "shown", do: toggle(false), expect: expectChanges("ADDED h")},
		}},
		matrixScenario{name: "hidden included", setup: setup, edit: func(r *WatchRequest) { r.Inspect.IncludeHidden = true }, steps: []matrixStep{
			{name: "hidden", do: toggle(true), quiet: true},
			{name: "shown", do: toggle(false), quiet: true},
		}},
	)
}

func TestWatchMatrixWindowsJunction(t *testing.T) {
	outside := t.TempDir()
	runMatrix(t, matrixScenario{name: "junction", steps: []matrixStep{
		{name: "create", do: func(t *testing.T, root string) {
			if out, err := exec.Command("cmd", "/c", "mklink", "/J", join(root, "j"), outside).CombinedOutput(); err != nil {
				t.Fatalf("mklink: %v: %s", err, out)
			}
		}, expect: func(t *testing.T, changes []comparison.Change) {
			t.Helper()
			// The live scanner reports a junction as dirloom snapshot does:
			// one node, never a directory it enters (today kind "file", from
			// Go's irregular file mode).
			if len(changes) != 1 || changes[0].Path != "j" || changes[0].After.Kind == artifact.KindDirectory {
				t.Fatalf("changes %v, want the junction added as one node", describeChanges(changes))
			}
		}},
		// A junction is never entered: neither its target nor a write
		// through it is part of the structure.
		{name: "a change in its target", quiet: true, do: func(t *testing.T, root string) { writeFiles(t, filepath.Join(outside, "x")) }},
		{name: "a change through it", quiet: true, do: func(t *testing.T, root string) { writeFiles(t, join(root, "j", "y")) }},
	}})
}

// TestWatchMatrixWindowsParentRename is A1: a native watch locks the root's
// parent against renaming, a polling watch does not, and then loses its
// root at the next poll.
func TestWatchMatrixWindowsParentRename(t *testing.T) {
	for _, mode := range []string{"native", "poll"} {
		t.Run(mode, func(t *testing.T) {
			parent := join(t.TempDir(), "parent")
			root := join(parent, "root")
			mkdirAll(t, join(root, "a"))
			request := nativeRequest(root)
			if mode == "poll" {
				request = pollRequest(root)
			}
			w := startWatch(t, request)
			w.log.waitState(t, fingerprintOf(t, request.Inspect))
			err := os.Rename(parent, parent+"-moved")
			if mode == "native" {
				if err == nil {
					t.Fatal("the root's parent was renamed during a native watch: A1 no longer holds")
				}
				if err := w.interrupt(t); err != nil {
					t.Fatal(err)
				}
				must(t, os.Rename(parent, parent+"-moved")) // released with the watch
				return
			}
			must(t, err)
			var f *watch.Failure
			if err := w.end(t); !errors.As(err, &f) || f.Diagnostic.Code != artifact.CodeWatchRootUnavailable {
				t.Fatalf("Run = %v, want watch_root_unavailable", err)
			}
		})
	}
}

// TestWatchMatrixWindowsRemoteRoot: native mode refuses a remote drive, and
// the same root works with --poll.
func TestWatchMatrixWindowsRemoteRoot(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, join(dir, "f"))
	vol := filepath.VolumeName(dir)
	unc := `\\localhost\` + strings.TrimSuffix(vol, ":") + `$` + dir[len(vol):]
	if _, err := os.Stat(unc); err != nil {
		t.Skipf("the administrative share is not reachable: %v", err)
	}
	w := startWatch(t, nativeRequest(unc))
	var f *watch.Failure
	if err := w.end(t); !errors.As(err, &f) || f.Diagnostic.Code != artifact.CodeWatchFilesystemUnsupported ||
		!strings.Contains(f.Diagnostic.Message, "--poll") {
		t.Fatalf("native on a remote drive = %v, want watch_filesystem_unsupported", err)
	}
	request := pollRequest(unc)
	w = startWatch(t, request)
	w.log.waitState(t, fingerprintOf(t, request.Inspect))
	writeFiles(t, join(dir, "g"))
	w.log.waitState(t, fingerprintOf(t, request.Inspect))
	if err := w.interrupt(t); err != nil {
		t.Fatal(err)
	}
	if got := describeEvents(w.log.recorded()); got != "started added:g state stopped:interrupted" {
		t.Fatalf("events: %s", got)
	}
}
