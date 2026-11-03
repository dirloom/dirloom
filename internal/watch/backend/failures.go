//go:build linux || darwin || windows

package backend

import "github.com/dirloom/dirloom/internal/artifact"

// limitReached is watch_limit_reached: what names the exhausted resource
// and, where there is one, the system setting that bounds it.
func limitReached(what string) error {
	return artifact.New(artifact.CodeWatchLimitReached, what+"; raise it, narrow the capture, or use --poll", false)
}

func backendFailure(what string, err error) error {
	return artifact.New(artifact.CodeWatchBackendFailure, what+": "+err.Error(), false)
}
