// Package notify holds throwaway prototypes of the v0.4.0-a8 notification
// backends. It exists only for LOT 0 (platform facts) and is deleted when
// LOT 0 closes; nothing here is production code.
package notify

import (
	"errors"
	"time"
)

// Op is a backend-neutral name for a raw notification.
type Op string

const (
	OpCreate     Op = "create"
	OpDelete     Op = "delete"
	OpRenameFrom Op = "rename-from"
	OpRenameTo   Op = "rename-to"
	OpModify     Op = "modify"    // content or metadata of an entry
	OpDirWrite   Op = "dir-write" // kqueue: the directory changed, no entry name
	OpSelfDelete Op = "self-delete"
	OpSelfMove   Op = "self-move"
	OpIgnored    Op = "ignored" // registration dropped by the system
	OpOverflow   Op = "overflow"
	OpOther      Op = "other"
)

// Note is one raw notification. Path is the entry path when the backend
// names it, otherwise the registered path; it is empty for an overflow.
type Note struct {
	Path string
	Op   Op
}

// Notifier is the smallest backend surface the LOT 0 facts need.
type Notifier interface {
	Name() string
	// Recursive reports whether one registration covers the whole root.
	Recursive() bool
	// Watch registers a directory below the root (no-op when recursive).
	Watch(dir string) error
	// Unwatch drops a registration (no-op when recursive).
	Unwatch(dir string) error
	// Wait returns the next notifications, or nil when timeout expires.
	Wait(timeout time.Duration) ([]Note, error)
	Close() error
}

// Drain collects notifications until none arrives for quiet, or until max.
func Drain(n Notifier, quiet, max time.Duration) ([]Note, error) {
	var all []Note
	deadline := time.Now().Add(max)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return all, nil
		}
		wait := quiet
		if wait > remaining {
			wait = remaining
		}
		notes, err := n.Wait(wait)
		if err != nil {
			return all, err
		}
		if len(notes) == 0 {
			return all, nil
		}
		all = append(all, notes...)
	}
}

// Count returns how many notes have the given op.
func Count(notes []Note, op Op) int {
	n := 0
	for _, note := range notes {
		if note.Op == op {
			n++
		}
	}
	return n
}

// ErrUnsupported is returned by probes that do not exist on a platform.
var ErrUnsupported = errors.New("unsupported on this platform")
