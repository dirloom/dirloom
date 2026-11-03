package app

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/watch"
)

// TestWatchMatrixDescriptorLimit is A4 through a lowered limit: a native
// watch of more directories than the process may open ends with
// watch_limit_reached, naming the setting and --poll.
func TestWatchMatrixDescriptorLimit(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 64; i++ {
		mkdirAll(t, join(root, "d"+strconv.Itoa(i)))
	}
	var limit unix.Rlimit
	must(t, unix.Getrlimit(unix.RLIMIT_NOFILE, &limit))
	lowered := limit
	lowered.Cur = uint64(openHandles(t) + 24) //nolint:gosec // G115: a descriptor count is small and non-negative.
	must(t, unix.Setrlimit(unix.RLIMIT_NOFILE, &lowered))
	restored := false
	restore := func() {
		if !restored {
			restored = true
			must(t, unix.Setrlimit(unix.RLIMIT_NOFILE, &limit))
		}
	}
	t.Cleanup(restore)
	w := startWatch(t, nativeRequest(root))
	err := w.end(t)
	restore()
	var f *watch.Failure
	if !errors.As(err, &f) || f.Diagnostic.Code != artifact.CodeWatchLimitReached ||
		!strings.Contains(f.Diagnostic.Message, "kern.maxfilesperproc") || !strings.Contains(f.Diagnostic.Message, "--poll") {
		t.Fatalf("Run = %v, want watch_limit_reached", err)
	}
	if err := watch.ValidateStream(w.log.recorded(), true); err != nil {
		t.Fatal(err)
	}
}
