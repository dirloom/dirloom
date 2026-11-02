package backend

import "github.com/dirloom/dirloom/internal/watch"

// newNative is the unavailable backend until LOTS 4–6 add the inotify,
// ReadDirectoryChangesW and kqueue backends behind build tags.
func newNative(Options) watch.Backend { return unavailable{} }
