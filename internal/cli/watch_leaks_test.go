package cli

import (
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"
)

var startSignalWatcher sync.Once

// checkReleased fails the test when a watch it starts leaves a goroutine, a
// descriptor or a handle behind once it has ended. Its cleanup runs after
// the watch's own, which ends the engine; the counts may take a moment to
// settle, so it waits for them. The os/signal package starts its watcher
// goroutine, for good, at the first signal.Notify of the process (watch's
// SIGTERM handler): it is started here first, so as not to count it.
func checkReleased(t *testing.T) {
	t.Helper()
	startSignalWatcher.Do(func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGTERM)
		signal.Stop(c)
	})
	goroutines, handles := runtime.NumGoroutine(), openHandles(t)
	t.Cleanup(func() {
		deadline := time.Now().Add(watchWait)
		for {
			g, h := runtime.NumGoroutine(), openHandles(t)
			if g <= goroutines && h <= handles+handleSlack {
				return
			}
			if time.Now().After(deadline) {
				t.Errorf("after the watch: %d goroutines (%d before), %d handles or descriptors (%d before)", g, goroutines, h, handles)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	})
}
