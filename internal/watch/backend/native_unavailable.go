//go:build !linux

package backend

import (
	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/watch"
)

// newNative is the unavailable backend until LOTS 5–6 add the
// ReadDirectoryChangesW and kqueue backends; Linux has inotify.
func newNative(Options) watch.Backend { return unavailable{} }

// unavailable is the backend of a platform without native notifications.
type unavailable struct{}

func (unavailable) Start(watch.Signals) error {
	return artifact.New(artifact.CodeWatchBackendFailure,
		"native filesystem notifications are not available on this platform; use --poll", false)
}

func (unavailable) Reconcile(watch.Observation) (bool, error) { return false, nil }

func (unavailable) Close() error { return nil }
