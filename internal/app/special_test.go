package app

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/watch"
)

// listenSocket holds an AF_UNIX socket open at dir/name and returns a
// function that closes and removes it. It skips the test where AF_UNIX is
// unavailable.
func listenSocket(t *testing.T, dir, name string) func() {
	t.Helper()
	path := filepath.Join(dir, name)
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Skipf("AF_UNIX sockets unavailable: %v", err)
	}
	remove := func() {
		_ = listener.Close()
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
	t.Cleanup(remove)
	return remove
}

// shortRoot returns a root with a short path: a socket path is limited to
// about 100 bytes, and t.TempDir includes the test name.
func shortRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "dl")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func requireUnsupportedNode(t *testing.T, what string, err error) {
	t.Helper()
	var artErr *artifact.Error
	if !errors.As(err, &artErr) || artErr.Code != artifact.CodeUnsupportedKind || artifact.IsInternal(err) {
		t.Fatalf("%s: error = %v, want %s", what, err, artifact.CodeUnsupportedKind)
	}
	if !strings.Contains(artErr.Message, `"socket" at "s.sock"`) {
		t.Fatalf("%s: message %q does not name the type and the path", what, artErr.Message)
	}
}

// TestStructuralCommandsRefuseSpecialEntry runs each structural command on a
// tree holding a socket: each refuses it with unsupported_node_type, while
// filters and --dirs-only avoid the refusal and the tree view still lists it.
func TestStructuralCommandsRefuseSpecialEntry(t *testing.T) {
	ctx := context.Background()
	root := shortRoot(t)
	writeFiles(t, filepath.Join(root, "a.txt"))
	baseline := mustFilesystemSnapshot(t, root)
	listenSocket(t, root, "s.sock")

	_, err := Fingerprint(ctx, InspectRequest{Root: root})
	requireUnsupportedNode(t, "fingerprint", err)
	_, err = Snapshot(ctx, InspectRequest{Root: root})
	requireUnsupportedNode(t, "snapshot", err)

	_, err = Verify(ctx, VerifyRequest{Snapshot: bytes.NewReader(baseline), Root: root})
	requireUnsupportedNode(t, "verify", err)
	if kind := ClassifyVerifyFailure(err); kind != FailureObservation {
		t.Fatalf("verify failure = %s, want %s", kind, FailureObservation)
	}
	if code, _ := VerifyDiagnostic(err); code != artifact.CodeUnsupportedKind {
		t.Fatalf("verify code = %s", code)
	}

	snap := mustDiffSnapshotFile(t, t.TempDir(), "ref.dlm.json", baseline)
	_, err = Diff(ctx, DiffRequest{
		A: DiffSourceSpec{Kind: DiffSourceSnapshot, Path: snap},
		B: DiffSourceSpec{Kind: DiffSourceLive, Path: root},
	})
	requireUnsupportedNode(t, "diff", err)
	if kind := ClassifyDiffFailure(err); kind != FailureObservation {
		t.Fatalf("diff failure = %s, want %s", kind, FailureObservation)
	}
	if code, _ := DiffDiagnostic(err); code != artifact.CodeUnsupportedKind {
		t.Fatalf("diff code = %s", code)
	}

	for name, request := range map[string]InspectRequest{
		"ignored":   {Root: root, IgnorePatterns: []string{"*.sock"}},
		"dirs-only": {Root: root, DirectoriesOnly: true},
	} {
		if _, err := Fingerprint(ctx, request); err != nil {
			t.Fatalf("%s: fingerprint = %v", name, err)
		}
	}

	model, err := Inspect(ctx, InspectRequest{Root: root})
	if err != nil {
		t.Fatalf("tree view: %v", err)
	}
	found := false
	for _, child := range model.Children {
		if child.Name == "s.sock" {
			found = child.Type == "file" && child.Special == "socket"
		}
	}
	if !found {
		t.Fatal("the tree view no longer lists the socket as a file")
	}
}

// TestWatchSpecialEntryAtStartup: a refused entry in the view makes the
// startup observation fail; once its retries are spent, the watch stops with
// a fatal unsupported_node_type.
func TestWatchSpecialEntryAtStartup(t *testing.T) {
	root := shortRoot(t)
	writeFiles(t, filepath.Join(root, "a.txt"))
	listenSocket(t, root, "s.sock")
	w := startWatchWith(t, pollRequest(root), func(c *watch.Config) { c.StartupRetry = 300 * time.Millisecond })
	err := w.end(t)
	var failure *watch.Failure
	if !errors.As(err, &failure) || failure.Diagnostic.Code != artifact.CodeUnsupportedKind || failure.Internal {
		t.Fatalf("Run = %v, want a fatal %s", err, artifact.CodeUnsupportedKind)
	}
	if got := describeEvents(w.log.recorded()); got != "error:unsupported_node_type! stopped:error" {
		t.Fatalf("events: %s", got)
	}
}

// TestWatchSpecialEntryWhileRunning: a refused entry that appears during a
// watch gives non-fatal errors and no new state; the watch keeps the last
// state and converges again once the entry is gone.
func TestWatchSpecialEntryWhileRunning(t *testing.T) {
	for _, mode := range []string{"native", "poll"} {
		t.Run(mode, func(t *testing.T) {
			root := shortRoot(t)
			writeFiles(t, filepath.Join(root, "a.txt"))
			request := nativeRequest(root)
			if mode == "poll" {
				request = pollRequest(root)
			}
			w := startWatch(t, request)
			before := fingerprintOf(t, request.Inspect)
			w.log.waitState(t, before)
			mark := len(w.log.recorded())

			remove := listenSocket(t, root, "s.sock")
			w.log.waitFor(t, "a non-fatal unsupported_node_type", func(events []watch.Event) bool {
				for _, ev := range events[mark:] {
					if ev.Kind == watch.KindError && !ev.Fatal && ev.Diagnostic.Code == artifact.CodeUnsupportedKind {
						return true
					}
				}
				return false
			})
			for _, ev := range w.log.recorded()[mark:] {
				if ev.Kind == watch.KindStateChanged || ev.Kind.IsNode() {
					t.Fatalf("a state was published with the socket present: %s", describeEvents(w.log.recorded()[mark:]))
				}
			}

			// Without the socket, a new file is reported against the last
			// published state.
			remove()
			writeFiles(t, filepath.Join(root, "b.txt"))
			w.log.waitState(t, fingerprintOf(t, request.Inspect))
			if err := w.interrupt(t); err != nil {
				t.Fatalf("Run = %v", err)
			}
			events := w.log.recorded()
			if err := watch.ValidateStream(events, true); err != nil {
				t.Fatal(err)
			}
			nodes := 0
			for _, ev := range events[mark:] {
				if ev.Kind.IsNode() {
					nodes++
					if string(ev.Change.Path) != "b.txt" {
						t.Fatalf("unexpected change %s after the socket left: %s", ev.Change.Path, describeEvents(events[mark:]))
					}
				}
			}
			if nodes != 1 {
				t.Fatalf("events after the socket left: %s", describeEvents(events[mark:]))
			}
		})
	}
}
