package app

import (
	"errors"
	"os"
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
	outside, other := t.TempDir(), t.TempDir()
	runMatrix(t, matrixScenario{name: "junction", steps: []matrixStep{
		// The live scanner reports a junction as dirloom snapshot does: one
		// junction node with its target, never a directory it enters.
		{name: "create", do: func(t *testing.T, root string) { makeJunction(t, join(root, "j"), outside) },
			expect: func(t *testing.T, changes []comparison.Change) {
				t.Helper()
				expectChanges("ADDED j")(t, changes)
				if after := changes[0].After; after.Kind != artifact.KindJunction || *after.Target != filepath.ToSlash(outside) {
					t.Fatalf("added %+v, want a junction to %s", after, outside)
				}
			}},
		// A junction is never entered: neither its target nor a write
		// through it is part of the structure.
		{name: "a change in its target", quiet: true, do: func(t *testing.T, root string) { writeFiles(t, filepath.Join(outside, "x")) }},
		{name: "a change through it", quiet: true, do: func(t *testing.T, root string) { writeFiles(t, join(root, "j", "y")) }},
		{name: "retarget", do: func(t *testing.T, root string) {
			must(t, os.Remove(join(root, "j")))
			makeJunction(t, join(root, "j"), other)
		}, expect: expectChanges("CHANGED j (junction:" + filepath.ToSlash(outside) + "->junction:" + filepath.ToSlash(other) + ")")},
	}})
}

// TestWatchWindowsJunctionRetargetEvents retargets a junction in place, in one
// atomic step, during an active watch: the next generation is exactly
// node.changed for the junction, then state.changed.
func TestWatchWindowsJunctionRetargetEvents(t *testing.T) {
	for _, mode := range []string{"native", "poll"} {
		t.Run(mode, func(t *testing.T) {
			root, outside, other := t.TempDir(), t.TempDir(), t.TempDir()
			makeJunction(t, join(root, "j"), outside)
			request := nativeRequest(root)
			if mode == "poll" {
				request = pollRequest(root)
			}
			w := startWatch(t, request)
			w.log.waitState(t, fingerprintOf(t, request.Inspect))
			mark := len(w.log.recorded())
			setJunctionTarget(t, join(root, "j"), other)
			w.log.waitState(t, fingerprintOf(t, request.Inspect))
			if err := w.interrupt(t); err != nil {
				t.Fatalf("Run = %v", err)
			}
			events := w.log.recorded()
			if got := describeEvents(events[mark:]); got != "changed:j state stopped:interrupted" {
				t.Fatalf("events after the retarget: %s", got)
			}
			change := events[mark].Change
			if change.Before.Kind != artifact.KindJunction || *change.Before.Target != filepath.ToSlash(outside) ||
				change.After.Kind != artifact.KindJunction || *change.After.Target != filepath.ToSlash(other) {
				t.Fatalf("node.changed = %+v -> %+v", change.Before, change.After)
			}
			if err := watch.ValidateStream(events, true); err != nil {
				t.Fatal(err)
			}
		})
	}
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
