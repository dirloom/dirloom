package notify

import (
	"errors"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FS adapts fsnotify v1.10.1 to Notifier for the F19 comparison.
//
// With pump, a goroutine drains Events and Errors into memory at once, as a
// real consumer must: on Windows, fsnotify serves Add and event delivery from
// one goroutine, so an Add blocks while an undelivered event is pending
// (found in LOT 0). Without pump, Wait reads the channels directly, which
// lets the kernel queue overflow while nobody reads.
type FS struct {
	w    *fsnotify.Watcher
	pump bool

	mu    sync.Mutex
	notes []Note
	err   error
	wake  chan struct{}
	done  chan struct{}
}

// NewFsnotify watches root (non-recursive: fsnotify v1.10.1 enables
// recursion only in its own tests).
func NewFsnotify(root string, pump bool) (*FS, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	f := &FS{w: w, pump: pump, wake: make(chan struct{}, 1), done: make(chan struct{})}
	if pump {
		go f.run()
	}
	if err := w.Add(root); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

func (f *FS) run() {
	defer close(f.done)
	for {
		select {
		case ev, ok := <-f.w.Events:
			if !ok {
				return
			}
			f.push(fsNote(ev), nil)
		case err, ok := <-f.w.Errors:
			if !ok {
				return
			}
			if errors.Is(err, fsnotify.ErrEventOverflow) {
				f.push(Note{Op: OpOverflow}, nil)
			} else {
				f.push(Note{}, err)
			}
		}
	}
}

func (f *FS) push(n Note, err error) {
	f.mu.Lock()
	if err != nil {
		f.err = err
	} else {
		f.notes = append(f.notes, n)
	}
	f.mu.Unlock()
	select {
	case f.wake <- struct{}{}:
	default:
	}
}

func (f *FS) Name() string             { return "fsnotify" }
func (f *FS) Recursive() bool          { return false }
func (f *FS) Watch(dir string) error   { return f.w.Add(dir) }
func (f *FS) Unwatch(dir string) error { return f.w.Remove(dir) }
func (f *FS) Watches() int             { return len(f.w.WatchList()) }

func (f *FS) Wait(timeout time.Duration) ([]Note, error) {
	if !f.pump {
		return f.waitDirect(timeout)
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		f.mu.Lock()
		notes, err := f.notes, f.err
		f.notes, f.err = nil, nil
		f.mu.Unlock()
		if len(notes) > 0 || err != nil {
			return notes, err
		}
		select {
		case <-f.wake:
		case <-timer.C:
			return nil, nil
		}
	}
}

func (f *FS) waitDirect(timeout time.Duration) ([]Note, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	var notes []Note
	select {
	case ev, ok := <-f.w.Events:
		if !ok {
			return nil, errors.New("fsnotify events closed")
		}
		notes = append(notes, fsNote(ev))
	case err, ok := <-f.w.Errors:
		if !ok {
			return nil, errors.New("fsnotify errors closed")
		}
		if errors.Is(err, fsnotify.ErrEventOverflow) {
			return []Note{{Op: OpOverflow}}, nil
		}
		return nil, err
	case <-timer.C:
		return nil, nil
	}
	for {
		select {
		case ev := <-f.w.Events:
			notes = append(notes, fsNote(ev))
		case err := <-f.w.Errors:
			if errors.Is(err, fsnotify.ErrEventOverflow) {
				notes = append(notes, Note{Op: OpOverflow})
				continue
			}
			return notes, err
		default:
			return notes, nil
		}
	}
}

func fsNote(ev fsnotify.Event) Note {
	switch {
	case ev.Has(fsnotify.Create):
		return Note{Path: ev.Name, Op: OpCreate}
	case ev.Has(fsnotify.Remove):
		return Note{Path: ev.Name, Op: OpDelete}
	case ev.Has(fsnotify.Rename):
		return Note{Path: ev.Name, Op: OpRenameFrom}
	case ev.Has(fsnotify.Write), ev.Has(fsnotify.Chmod):
		return Note{Path: ev.Name, Op: OpModify}
	}
	return Note{Path: ev.Name, Op: OpOther}
}

func (f *FS) Close() error {
	err := f.w.Close()
	if f.pump {
		<-f.done
	}
	return err
}
