package notify

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

// StructuralMask is the inotify mask planned for a structural directory.
const StructuralMask = unix.IN_CREATE | unix.IN_DELETE | unix.IN_MOVED_FROM | unix.IN_MOVED_TO |
	unix.IN_DELETE_SELF | unix.IN_MOVE_SELF | unix.IN_CLOSE_WRITE |
	unix.IN_ONLYDIR | unix.IN_DONT_FOLLOW | unix.IN_EXCL_UNLINK

// ControlMask is the inotify mask planned for the root's control registration.
const ControlMask = unix.IN_DELETE_SELF | unix.IN_MOVE_SELF | unix.IN_ONLYDIR | unix.IN_DONT_FOLLOW

// Inotify is a prototype inotify backend: one watch per directory.
type Inotify struct {
	fd   int
	mask uint32
	dirs map[int32]string
	wds  map[string]int32
	buf  []byte
}

// NewNative starts the platform's preferred backend on root.
func NewNative(root string) (Notifier, error) { return NewInotify(root, StructuralMask) }

// NewControl registers only the root's control events.
func NewControl(root string) (Notifier, error) { return NewInotify(root, ControlMask) }

// NewInotify creates an instance and registers root with mask.
func NewInotify(root string, mask uint32) (*Inotify, error) {
	fd, err := unix.InotifyInit1(unix.IN_CLOEXEC | unix.IN_NONBLOCK)
	if err != nil {
		return nil, fmt.Errorf("inotify_init1: %w", err)
	}
	w := &Inotify{fd: fd, mask: mask, dirs: map[int32]string{}, wds: map[string]int32{}, buf: make([]byte, 64*1024)}
	if err := w.Watch(root); err != nil {
		_ = unix.Close(fd)
		return nil, err
	}
	return w, nil
}

func (w *Inotify) Name() string    { return "inotify" }
func (w *Inotify) Recursive() bool { return false }

func (w *Inotify) Watch(dir string) error {
	wd, err := unix.InotifyAddWatch(w.fd, dir, w.mask)
	if err != nil {
		return &os.PathError{Op: "inotify_add_watch", Path: dir, Err: err}
	}
	w.dirs[int32(wd)] = dir
	w.wds[dir] = int32(wd)
	return nil
}

func (w *Inotify) Unwatch(dir string) error {
	wd, ok := w.wds[dir]
	if !ok {
		return nil
	}
	delete(w.wds, dir)
	delete(w.dirs, wd)
	_, err := unix.InotifyRmWatch(w.fd, uint32(wd))
	return err
}

// Watches returns the number of registrations the prototype tracks.
func (w *Inotify) Watches() int { return len(w.wds) }

func (w *Inotify) Wait(timeout time.Duration) ([]Note, error) {
	fds := []unix.PollFd{{Fd: int32(w.fd), Events: unix.POLLIN}}
	for {
		n, err := unix.Poll(fds, int(timeout/time.Millisecond))
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("poll: %w", err)
		}
		if n == 0 {
			return nil, nil
		}
		break
	}
	n, err := unix.Read(w.fd, w.buf)
	if errors.Is(err, unix.EAGAIN) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	return w.parse(w.buf[:n])
}

func (w *Inotify) parse(data []byte) ([]Note, error) {
	var notes []Note
	for off := 0; off < len(data); {
		if len(data)-off < unix.SizeofInotifyEvent {
			return notes, fmt.Errorf("truncated inotify record at %d", off)
		}
		wd := int32(binary.NativeEndian.Uint32(data[off:]))
		mask := binary.NativeEndian.Uint32(data[off+4:])
		nameLen := int(binary.NativeEndian.Uint32(data[off+12:]))
		start := off + unix.SizeofInotifyEvent
		if nameLen < 0 || nameLen > len(data)-start {
			return notes, fmt.Errorf("inotify name overruns buffer at %d", off)
		}
		name := data[start : start+nameLen]
		for len(name) > 0 && name[len(name)-1] == 0 {
			name = name[:len(name)-1]
		}
		off = start + nameLen
		if mask&unix.IN_Q_OVERFLOW != 0 {
			notes = append(notes, Note{Op: OpOverflow})
			continue
		}
		dir := w.dirs[wd]
		path := dir
		if len(name) > 0 {
			path = filepath.Join(dir, string(name))
		}
		switch {
		case mask&unix.IN_IGNORED != 0:
			notes = append(notes, Note{Path: dir, Op: OpIgnored})
			if d, ok := w.dirs[wd]; ok {
				delete(w.dirs, wd)
				if w.wds[d] == wd {
					delete(w.wds, d)
				}
			}
		case mask&unix.IN_DELETE_SELF != 0:
			notes = append(notes, Note{Path: dir, Op: OpSelfDelete})
		case mask&unix.IN_MOVE_SELF != 0:
			notes = append(notes, Note{Path: dir, Op: OpSelfMove})
		case mask&unix.IN_CREATE != 0:
			notes = append(notes, Note{Path: path, Op: OpCreate})
		case mask&unix.IN_DELETE != 0:
			notes = append(notes, Note{Path: path, Op: OpDelete})
		case mask&unix.IN_MOVED_FROM != 0:
			notes = append(notes, Note{Path: path, Op: OpRenameFrom})
		case mask&unix.IN_MOVED_TO != 0:
			notes = append(notes, Note{Path: path, Op: OpRenameTo})
		case mask&(unix.IN_CLOSE_WRITE|unix.IN_MODIFY|unix.IN_ATTRIB) != 0:
			notes = append(notes, Note{Path: path, Op: OpModify})
		default:
			notes = append(notes, Note{Path: path, Op: OpOther})
		}
	}
	return notes, nil
}

func (w *Inotify) Close() error { return unix.Close(w.fd) }
