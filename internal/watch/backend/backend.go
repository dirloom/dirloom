// Package backend holds the native notification backends of Structural
// Watch v1 (inotify, kqueue, ReadDirectoryChangesW), written on
// golang.org/x/sys. A backend only signals the engine; observations stay
// the truth.
package backend

import (
	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/snapshot"
	"github.com/dirloom/dirloom/internal/watch"
)

// Options describe what a native backend watches.
type Options struct {
	// Root is the absolute, cleaned root.
	Root string
	// Capture selects the structural watch set and the relevance rules.
	Capture snapshot.CaptureV1
}

// New returns the native backend of this platform. Its Start applies the
// filesystem policy and registers the control set; every failure surfaces
// there, as a startup failure of the watch.
func New(opts Options) watch.Backend { return newNative(opts) }

// unavailable is the backend of a platform without native notifications.
type unavailable struct{}

func (unavailable) Start(watch.Signals) error {
	return artifact.New(artifact.CodeWatchBackendFailure,
		"native filesystem notifications are not available on this platform; use --poll", false)
}

func (unavailable) Reconcile(watch.Observation) (bool, error) { return false, nil }

func (unavailable) Close() error { return nil }
