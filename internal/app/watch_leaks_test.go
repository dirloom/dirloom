package app

import (
	"runtime"
	"testing"
	"time"
)

// checkReleased fails the test when a watch it starts leaves a goroutine, a
// descriptor or a handle behind once it has ended. Its cleanup runs after
// the watch's own, which ends the engine; the counts may take a moment to
// settle, so it waits for them.
func checkReleased(t *testing.T) {
	t.Helper()
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
