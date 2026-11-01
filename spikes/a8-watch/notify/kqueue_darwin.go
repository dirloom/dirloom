package notify

import (
	"errors"
	"fmt"
	"time"

	"golang.org/x/sys/unix"
)

// StructuralFflags is the kqueue filter planned for a structural directory.
const StructuralFflags = unix.NOTE_WRITE | unix.NOTE_DELETE | unix.NOTE_RENAME | unix.NOTE_REVOKE

// ControlFflags is the kqueue filter planned for the root's control registration.
const ControlFflags = unix.NOTE_DELETE | unix.NOTE_RENAME | unix.NOTE_REVOKE

// FileFflags is the kqueue filter planned for a .gitignore file.
const FileFflags = unix.NOTE_WRITE | unix.NOTE_EXTEND | unix.NOTE_DELETE | unix.NOTE_RENAME | unix.NOTE_REVOKE

// Kqueue is a prototype kqueue backend: one O_EVTONLY descriptor per path.
type Kqueue struct {
	kq     int
	fflags uint32
	paths  map[int]string
	files  map[int]bool
	fds    map[string]int
	events []unix.Kevent_t
}

// NewNative starts the platform's preferred backend on root.
func NewNative(root string) (Notifier, error) { return NewKqueue(root, StructuralFflags) }

// NewControl registers only the root's control events.
func NewControl(root string) (Notifier, error) { return NewKqueue(root, ControlFflags) }

// NewKqueue creates a kqueue and registers root with fflags.
func NewKqueue(root string, fflags uint32) (*Kqueue, error) {
	kq, err := unix.Kqueue()
	if err != nil {
		return nil, fmt.Errorf("kqueue: %w", err)
	}
	unix.CloseOnExec(kq)
	w := &Kqueue{kq: kq, fflags: fflags, paths: map[int]string{}, files: map[int]bool{}, fds: map[string]int{}, events: make([]unix.Kevent_t, 256)}
	if err := w.Watch(root); err != nil {
		_ = unix.Close(kq)
		return nil, err
	}
	return w, nil
}

func (w *Kqueue) Name() string    { return "kqueue" }
func (w *Kqueue) Recursive() bool { return false }

func (w *Kqueue) Watch(dir string) error { return w.add(dir, w.fflags, unix.O_DIRECTORY, false) }

// WatchFile registers one file (a .gitignore) with FileFflags.
func (w *Kqueue) WatchFile(path string) error { return w.add(path, FileFflags, 0, true) }

func (w *Kqueue) add(path string, fflags uint32, extra int, file bool) error {
	fd, err := unix.Open(path, unix.O_EVTONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|extra, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	var ev unix.Kevent_t
	unix.SetKevent(&ev, fd, unix.EVFILT_VNODE, unix.EV_ADD|unix.EV_CLEAR|unix.EV_ENABLE)
	ev.Fflags = fflags
	if _, err := unix.Kevent(w.kq, []unix.Kevent_t{ev}, nil, nil); err != nil {
		_ = unix.Close(fd)
		return fmt.Errorf("kevent add %s: %w", path, err)
	}
	w.paths[fd] = path
	w.files[fd] = file
	w.fds[path] = fd
	return nil
}

func (w *Kqueue) Unwatch(dir string) error {
	fd, ok := w.fds[dir]
	if !ok {
		return nil
	}
	delete(w.fds, dir)
	delete(w.paths, fd)
	delete(w.files, fd)
	return unix.Close(fd)
}

// Watches returns the number of registrations the prototype tracks.
func (w *Kqueue) Watches() int { return len(w.fds) }

func (w *Kqueue) Wait(timeout time.Duration) ([]Note, error) {
	ts := unix.NsecToTimespec(int64(timeout))
	n, err := unix.Kevent(w.kq, nil, w.events, &ts)
	if errors.Is(err, unix.EINTR) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("kevent wait: %w", err)
	}
	var notes []Note
	for _, ev := range w.events[:n] {
		fd := int(ev.Ident)
		path := w.paths[fd]
		switch {
		case ev.Fflags&unix.NOTE_DELETE != 0:
			notes = append(notes, Note{Path: path, Op: OpSelfDelete})
		case ev.Fflags&unix.NOTE_RENAME != 0:
			notes = append(notes, Note{Path: path, Op: OpSelfMove})
		case ev.Fflags&unix.NOTE_REVOKE != 0:
			notes = append(notes, Note{Path: path, Op: OpIgnored})
		case ev.Fflags&(unix.NOTE_WRITE|unix.NOTE_EXTEND) != 0 && w.files[fd]:
			notes = append(notes, Note{Path: path, Op: OpModify})
		case ev.Fflags&unix.NOTE_WRITE != 0:
			notes = append(notes, Note{Path: path, Op: OpDirWrite})
		default:
			notes = append(notes, Note{Path: path, Op: OpOther})
		}
	}
	return notes, nil
}

func (w *Kqueue) Close() error {
	for fd := range w.paths {
		_ = unix.Close(fd)
	}
	return unix.Close(w.kq)
}
