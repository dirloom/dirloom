package app

import (
	"errors"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/watch"
)

// nativeRequest watches root with native notifications. The reconciliation
// interval (60 s) is far beyond every wait of these tests, so a change they
// see was signalled by the backend.
func nativeRequest(root string) WatchRequest {
	return WatchRequest{
		Inspect:  InspectRequest{Root: root, UseDefaultIgnores: true, UseGitIgnore: true},
		Debounce: 20 * watch.MinDebounce,
	}
}

func TestWatchNativeMatchesDiffAndConverges(t *testing.T) {
	for _, seed := range []uint64{1, 2, 3} {
		t.Run(strconv.FormatUint(seed, 10), func(t *testing.T) {
			root := t.TempDir()
			mkdirAll(t, filepath.Join(root, "a", "b"), filepath.Join(root, "c"))
			writeFiles(t, filepath.Join(root, "a", "x"), filepath.Join(root, "a", "b", "y"), filepath.Join(root, "c", "z"), filepath.Join(root, "top"))
			request := nativeRequest(root)
			w := startWatch(t, request)
			w.log.waitState(t, fingerprintOf(t, request.Inspect))
			rng := rand.New(rand.NewPCG(seed, 0xa8))
			for step := 0; step < 25; step++ {
				mutate(t, rng, root, step)
				w.log.waitState(t, fingerprintOf(t, request.Inspect))
			}
			if err := w.interrupt(t); err != nil {
				t.Fatalf("Run = %v", err)
			}
			events := w.log.recorded()
			if err := watch.ValidateStream(events, true); err != nil {
				t.Fatal(err)
			}
			checkDiffEquivalence(t, events, w.obs)
			if events[0].Metadata.Backend != watch.BackendNative {
				t.Fatalf("metadata = %+v", events[0].Metadata)
			}
			for _, ev := range events {
				if ev.Kind == watch.KindRescanStarted && ev.Rescan == watch.RescanReconciliation {
					t.Fatal("a change was found by reconciliation: a notification was missed")
				}
			}
		})
	}
}

func TestWatchNativeReportsABurstBelowNewDirectories(t *testing.T) {
	root := t.TempDir()
	request := nativeRequest(root)
	w := startWatch(t, request)
	w.log.waitState(t, fingerprintOf(t, request.Inspect))
	deep := filepath.Join(root, "a", "b", "c", "d")
	mkdirAll(t, deep)
	for _, d := range []string{"a", "a/b", "a/b/c", "a/b/c/d"} {
		writeFiles(t, filepath.Join(root, filepath.FromSlash(d), "f"))
	}
	w.log.waitState(t, fingerprintOf(t, request.Inspect))
	writeFiles(t, filepath.Join(deep, "late"))
	w.log.waitState(t, fingerprintOf(t, request.Inspect))
	_ = w.interrupt(t)
	if err := watch.ValidateStream(w.log.recorded(), true); err != nil {
		t.Fatal(err)
	}
}

func TestWatchNativeContentEditsAreNotStructural(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, filepath.Join(root, "f"))
	request := nativeRequest(root)
	w := startWatch(t, request)
	w.log.waitState(t, fingerprintOf(t, request.Inspect))
	for i := 0; i < 20; i++ {
		if err := os.WriteFile(filepath.Join(root, "f"), []byte(strconv.Itoa(i)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFiles(t, filepath.Join(root, "sentinel"))
	w.log.waitState(t, fingerprintOf(t, request.Inspect))
	_ = w.interrupt(t)
	events := w.log.recorded()
	nodes := 0
	for _, ev := range events {
		if ev.Kind.IsNode() {
			nodes++
		}
	}
	if nodes != 1 {
		t.Fatalf("%d node events for 20 content edits and one new file", nodes)
	}
}

func TestWatchNativeRootLossEndsTheWatch(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	mkdirAll(t, filepath.Join(root, "a"))
	request := nativeRequest(root)
	w := startWatch(t, request)
	w.log.waitState(t, fingerprintOf(t, request.Inspect))
	if err := os.Rename(root, root+"-moved"); err != nil {
		t.Fatal(err)
	}
	var f *watch.Failure
	if err := w.end(t); !errors.As(err, &f) || f.Diagnostic.Code != artifact.CodeWatchRootUnavailable {
		t.Fatalf("Run = %v, want watch_root_unavailable", err)
	}
	events := w.log.recorded()
	if err := watch.ValidateStream(events, true); err != nil {
		t.Fatal(err)
	}
	assertNoRoot(t, events, root)
}
