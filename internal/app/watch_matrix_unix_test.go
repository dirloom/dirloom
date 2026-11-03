//go:build linux || darwin

package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dirloom/dirloom/internal/watch"
)

func TestWatchMatrixUnreadableDirectoryRecovers(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permissions do not apply to root")
	}
	for _, mode := range []string{"native", "poll"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			locked := join(root, "locked")
			mkdirAll(t, locked)
			writeFiles(t, join(locked, "inside"))
			request := nativeRequest(root)
			if mode == "poll" {
				request = pollRequest(root)
			}
			w := startWatch(t, request)
			w.log.waitState(t, fingerprintOf(t, request.Inspect))
			must(t, os.Chmod(locked, 0))
			t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
			mkdirAll(t, join(root, "trigger")) // a permission change alone is not structural
			w.log.waitFor(t, "a rescan for the unreadable directory", func(events []watch.Event) bool {
				last := events[len(events)-1]
				return last.Kind == watch.KindRescanStarted && last.Rescan == watch.RescanError
			})
			must(t, os.Chmod(locked, 0o755))
			w.log.waitState(t, fingerprintOf(t, request.Inspect))
			w.log.waitFor(t, "the end of the rescan", func(events []watch.Event) bool {
				return events[len(events)-1].Kind == watch.KindRescanCompleted
			})
			_ = w.interrupt(t)
			events := w.log.recorded()
			if got := describeEvents(events); !strings.HasPrefix(got, "started error:") || !strings.Contains(got, " rescan:error ") ||
				!strings.HasSuffix(got, "added:trigger state completed stopped:interrupted") {
				t.Fatalf("events: %s", got)
			}
			if err := watch.ValidateStream(events, true); err != nil {
				t.Fatal(err)
			}
			assertNoRoot(t, events, root)
		})
	}
}

// TestWatchMatrixAncestorRenamed: no notification reaches the watch when an
// ancestor of the root is renamed; the identity check of the next
// observation (a poll, or a reconciliation) ends it.
func TestWatchMatrixAncestorRenamed(t *testing.T) {
	for _, mode := range []string{"native", "poll"} {
		t.Run(mode, func(t *testing.T) {
			parent := join(t.TempDir(), "parent")
			root := join(parent, "root")
			mkdirAll(t, join(root, "a"))
			request := nativeRequest(root)
			if mode == "poll" {
				request = pollRequest(root)
			}
			w := startWatchWith(t, request, func(c *watch.Config) {
				if c.Backend != nil {
					c.Reconcile = 300 * time.Millisecond
				}
			})
			w.log.waitState(t, fingerprintOf(t, request.Inspect))
			must(t, os.Rename(parent, parent+"-moved"))
			if err := w.end(t); err == nil {
				t.Fatal("the watch survived the rename of its parent")
			}
			if got := describeEvents(w.log.recorded()); got != "started error:watch_root_unavailable! stopped:error" {
				t.Fatalf("events: %s", got)
			}
		})
	}
}

// mountOver mounts a fresh filesystem over dir and returns how to unmount
// it, or skips the test where mounting needs privileges the run lacks.
func mountOver(t *testing.T, dir string) func() {
	t.Helper()
	if os.Getenv("GITHUB_ACTIONS") != "true" {
		t.Skip("mounting changes the machine: CI only")
	}
	run := func(args ...string) {
		t.Helper()
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("%s: %v: %s", strings.Join(args, " "), err, out)
		}
	}
	if runtime.GOOS == "linux" {
		run("sudo", "-n", "mount", "-t", "tmpfs", "-o", "size=1m,mode=0777", "tmpfs", dir)
		return func() { _ = exec.Command("sudo", "-n", "umount", dir).Run() }
	}
	image := filepath.Join(t.TempDir(), "volume.dmg")
	run("hdiutil", "create", "-size", "2m", "-fs", "HFS+", "-volname", "dirloom", image)
	run("hdiutil", "attach", image, "-mountpoint", dir, "-nobrowse")
	return func() { _ = exec.Command("hdiutil", "detach", dir, "-force").Run() }
}

// TestWatchMatrixMountOverARegisteredDirectory: the mount hides the
// registered directory without a notification; the next observation sees
// another (device, inode) at its path and registers the mount (A3, F17).
func TestWatchMatrixMountOverARegisteredDirectory(t *testing.T) {
	root := t.TempDir()
	mnt := join(root, "mnt")
	mkdirAll(t, mnt)
	writeFiles(t, join(mnt, "covered"))
	request := nativeRequest(root)
	w := startWatch(t, request)
	w.log.waitState(t, fingerprintOf(t, request.Inspect))
	unmount := mountOver(t, mnt)
	t.Cleanup(unmount)
	mkdirAll(t, join(root, "trigger"))
	w.log.waitState(t, fingerprintOf(t, request.Inspect))
	writeFiles(t, join(mnt, "inside-the-mount"))
	w.log.waitState(t, fingerprintOf(t, request.Inspect))
	_ = w.interrupt(t)
	unmount()
	events := w.log.recorded()
	if got := describeEvents(events); !strings.Contains(got, "added:mnt/inside-the-mount") || strings.Contains(got, "rescan:") {
		t.Fatalf("events: %s", got)
	}
}
