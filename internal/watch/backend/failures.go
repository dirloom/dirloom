//go:build linux || darwin || windows

package backend

import (
	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/watch"
)

// failOnPanic ends the watch with a broken invariant when the reader
// goroutine panics, instead of crashing the process: the CLI's guard covers
// the command's goroutine, not this one. The reader defers it after closing
// its done channel, so that it runs first and Close still returns; no
// reader holds its lock across a call that can panic.
func failOnPanic(sig watch.Signals) {
	if r := recover(); r != nil {
		sig.Fail(artifact.Recovered(r))
	}
}

// limitReached is watch_limit_reached: what names the exhausted resource
// and, where there is one, the system setting that bounds it.
func limitReached(what string) error {
	return artifact.New(artifact.CodeWatchLimitReached, what+"; raise it, narrow the capture, or use --poll", false)
}

func backendFailure(what string, err error) error {
	return artifact.New(artifact.CodeWatchBackendFailure, what+": "+err.Error(), false)
}
