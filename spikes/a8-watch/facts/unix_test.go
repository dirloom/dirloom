//go:build linux || darwin

package facts

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"dirloom.local/spikes/a8watch/notify"
)

// TestF18FastRecreate removes and recreates a registered directory as fast as
// possible and checks removal notices, inode reuse and re-registration.
func TestF18FastRecreate(t *testing.T) {
	root := t.TempDir()
	d := filepath.Join(root, "d")
	mkdirs(t, d)
	n, err := notify.NewNative(root)
	if err != nil {
		t.Fatal(err)
	}
	defer n.Close()
	const rounds = 200
	notices, reused, reported := 0, 0, 0
	for i := 0; i < rounds; i++ {
		if err := n.Watch(d); err != nil {
			t.Fatalf("watch: %v", err)
		}
		before, _ := os.Stat(d)
		if err := os.Remove(d); err != nil {
			t.Fatal(err)
		}
		mkdirs(t, d)
		after, _ := os.Stat(d)
		if before.Sys().(*syscall.Stat_t).Ino == after.Sys().(*syscall.Stat_t).Ino {
			reused++
		}
		notes := drain(t, n)
		for _, note := range notes {
			if note.Path == d && (note.Op == notify.OpIgnored || note.Op == notify.OpSelfDelete) {
				notices++
				break
			}
		}
		_ = n.Unwatch(d)
		if err := n.Watch(d); err != nil {
			t.Fatalf("re-watch: %v", err)
		}
		name := "x" + strconv.Itoa(i)
		write(t, filepath.Join(d, name), "")
		found, _, _ := waitFor(n, 2*time.Second, func(note notify.Note) bool {
			return strings.HasSuffix(note.Path, name) || (note.Op == notify.OpDirWrite && note.Path == d)
		})
		if found {
			reported++
		}
		_ = os.Remove(filepath.Join(d, name))
		_ = drain(t, n)
		_ = n.Unwatch(d)
	}
	evidence(t, "F18", verdict(notices == rounds && reported == rounds), "%s, %d rounds: removal notice on the old registration %d/%d, inode number reused %d/%d, change after re-registration reported %d/%d",
		n.Name(), rounds, notices, rounds, reused, rounds, reported, rounds)
}
