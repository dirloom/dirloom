//go:build linux || windows || darwin

package app

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/watch"
)

// TestWatchConvergesDuringConcurrentChanges renames, creates and removes
// directories, and rewrites a .gitignore, while the watch observes the tree,
// then requires it to converge to the canonical state once the changes stop
// (Q5 of the v0.4.0-rc1 plan, anomaly A10).
//
// An observation that a change overtakes fails with an entry that no longer
// exists, and the watch observes again: that is Structural Watch v1, and the
// only error allowed here. A sharing violation or a denied permission would
// be A10 again: a scan blocked by the user, or a directory being deleted
// reported as denied.
func TestWatchConvergesDuringConcurrentChanges(t *testing.T) {
	for _, mode := range []string{"native", "poll"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			mkdirAll(t, join(root, "a", "x", "deep"), join(root, "a", "y"), join(root, "keep"))
			writeFiles(t, join(root, "a", "x", "deep", "f.txt"), join(root, "a", "y", "g.txt"))
			if err := os.WriteFile(join(root, "a", ".gitignore"), []byte("*.log\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			request := nativeRequest(root)
			if mode == "poll" {
				request = pollRequest(root)
			}
			w := startWatch(t, request)
			initial := fingerprintOf(t, request.Inspect)
			w.log.waitState(t, initial)

			// Each round leaves keep/tN and removes the previous one; the last
			// rename moves a away: the final structure differs from the
			// initial one, so converging to it means observing it.
			deadline := time.Now().Add(2 * time.Second)
			rounds := 0
			for ; time.Now().Before(deadline); rounds++ {
				userOp(t, func() error { return os.Rename(join(root, "a"), join(root, "b")) })
				userOp(t, func() error { return os.Rename(join(root, "b"), join(root, "a")) })
				tmp := join(root, "keep", "t"+strconv.Itoa(rounds))
				userOp(t, func() error { return os.MkdirAll(join(tmp, "deep"), 0o755) })
				userOp(t, func() error { return os.WriteFile(join(tmp, "deep", "f.txt"), nil, 0o644) })
				if rounds > 0 {
					previous := join(root, "keep", "t"+strconv.Itoa(rounds-1))
					userOp(t, func() error { return os.RemoveAll(previous) })
				}
				userOp(t, func() error { return os.WriteFile(join(root, "a", ".gitignore"), []byte("*.tmp\n"), 0o644) })
			}
			userOp(t, func() error { return os.Rename(join(root, "a"), join(root, "moved")) })

			final := fingerprintOf(t, request.Inspect)
			if final == initial {
				t.Fatal("the changes left the structure as it was: convergence would prove nothing")
			}
			w.log.waitState(t, final)
			if err := w.interrupt(t); err != nil {
				t.Fatalf("Run = %v", err)
			}
			events := w.log.recorded()
			if err := watch.ValidateStream(events, true); err != nil {
				t.Fatal(err)
			}
			overtaken := 0
			for _, ev := range events {
				if ev.Kind != watch.KindError {
					continue
				}
				if ev.Fatal || ev.Diagnostic.Code != artifact.CodeSourceRead || !strings.Contains(ev.Diagnostic.Message, "entry no longer exists") {
					t.Fatalf("unexpected error: %+v", ev.Diagnostic)
				}
				overtaken++
			}
			t.Logf("%d rounds, %d observations overtaken by a change, %d events", rounds, overtaken, len(events))
		})
	}
}
