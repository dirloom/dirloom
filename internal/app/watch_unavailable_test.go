//go:build !linux

package app

import (
	"context"
	"errors"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/watch"
)

func TestWatchNativeModeWaitsForTheBackends(t *testing.T) {
	log := newEventLog()
	err := Watch(context.Background(), WatchRequest{Inspect: InspectRequest{Root: t.TempDir()}}, log)
	var f *watch.Failure
	if !errors.As(err, &f) || f.Diagnostic.Code != artifact.CodeWatchBackendFailure {
		t.Fatalf("Watch = %v, want watch_backend_failure until LOTS 5–6", err)
	}
}
