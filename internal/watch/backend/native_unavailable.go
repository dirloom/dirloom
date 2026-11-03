//go:build !linux && !windows && !darwin

package backend

import (
	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/watch"
)

// newNative is the unavailable backend: Linux has inotify, Windows
// ReadDirectoryChangesW and macOS kqueue; other platforms use --poll.
func newNative(Options) watch.Backend { return unavailable{} }

// unavailable is the backend of a platform without native notifications.
type unavailable struct{}

func (unavailable) Start(watch.Signals) error {
	return artifact.New(artifact.CodeWatchBackendFailure,
		"native filesystem notifications are not available on this platform; use --poll", false)
}

func (unavailable) Reconcile(watch.Observation) (bool, error) { return false, nil }

func (unavailable) Close() error { return nil }
