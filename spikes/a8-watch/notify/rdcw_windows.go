package notify

import (
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

// StructuralFilter is the ReadDirectoryChangesW filter planned for the root.
const StructuralFilter = windows.FILE_NOTIFY_CHANGE_FILE_NAME | windows.FILE_NOTIFY_CHANGE_DIR_NAME |
	windows.FILE_NOTIFY_CHANGE_LAST_WRITE | windows.FILE_NOTIFY_CHANGE_ATTRIBUTES

// ControlFilter is the filter planned for the handle on the root's parent.
const ControlFilter = windows.FILE_NOTIFY_CHANGE_FILE_NAME | windows.FILE_NOTIFY_CHANGE_DIR_NAME

// RDCW is a prototype ReadDirectoryChangesW backend on one directory handle.
type RDCW struct {
	dir       string
	recursive bool
	filter    uint32
	handle    windows.Handle
	event     windows.Handle
	ov        windows.Overlapped
	buf       []byte
	only      string // when set, keep only notes about this entry name
}

// NewNative starts the platform's preferred backend on root.
func NewNative(root string) (Notifier, error) {
	return OpenRDCW(root, true, StructuralFilter, 64*1024)
}

// NewControl watches the root's parent, keeping only notes about the root.
func NewControl(root string) (Notifier, error) {
	w, err := OpenRDCW(filepath.Dir(root), false, ControlFilter, 64*1024)
	if err != nil {
		return nil, err
	}
	w.only = filepath.Base(root)
	return w, nil
}

// OpenRDCW opens dir with every share mode and issues the first read.
func OpenRDCW(dir string, recursive bool, filter uint32, bufSize int) (*RDCW, error) {
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, windows.FILE_LIST_DIRECTORY,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		return nil, fmt.Errorf("CreateFile %s: %w", dir, err)
	}
	ev, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		_ = windows.CloseHandle(h)
		return nil, fmt.Errorf("CreateEvent: %w", err)
	}
	w := &RDCW{dir: dir, recursive: recursive, filter: filter, handle: h, event: ev, buf: make([]byte, bufSize)}
	w.ov.HEvent = ev
	if err := w.issue(); err != nil {
		_ = w.Close()
		return nil, err
	}
	return w, nil
}

func (w *RDCW) issue() error {
	if err := windows.ResetEvent(w.event); err != nil {
		return fmt.Errorf("ResetEvent: %w", err)
	}
	err := windows.ReadDirectoryChanges(w.handle, &w.buf[0], uint32(len(w.buf)), w.recursive, w.filter, nil, &w.ov, 0)
	if err != nil && !errors.Is(err, windows.ERROR_IO_PENDING) {
		return fmt.Errorf("ReadDirectoryChangesW: %w", err)
	}
	return nil
}

func (w *RDCW) Name() string           { return "ReadDirectoryChangesW" }
func (w *RDCW) Recursive() bool        { return w.recursive }
func (w *RDCW) Watch(string) error     { return nil }
func (w *RDCW) Unwatch(string) error   { return nil }
func (w *RDCW) Handle() windows.Handle { return w.handle }

func (w *RDCW) Wait(timeout time.Duration) ([]Note, error) {
	r, err := windows.WaitForSingleObject(w.event, uint32(timeout/time.Millisecond))
	if err != nil {
		return nil, fmt.Errorf("WaitForSingleObject: %w", err)
	}
	if r == uint32(windows.WAIT_TIMEOUT) {
		return nil, nil
	}
	var n uint32
	err = windows.GetOverlappedResult(w.handle, &w.ov, &n, false)
	var notes []Note
	switch {
	case errors.Is(err, windows.ERROR_NOTIFY_ENUM_DIR):
		notes = []Note{{Op: OpOverflow}}
	case err != nil:
		return nil, fmt.Errorf("GetOverlappedResult: %w", err)
	case n == 0:
		notes = []Note{{Op: OpOverflow}}
	default:
		notes, err = ParseNotify(w.dir, w.buf[:n])
		if err != nil {
			return notes, err
		}
	}
	if w.only != "" {
		kept := notes[:0]
		for _, note := range notes {
			if note.Op == OpOverflow || strings.EqualFold(filepath.Base(note.Path), w.only) {
				kept = append(kept, note)
			}
		}
		notes = kept
	}
	if err := w.issue(); err != nil {
		return notes, err
	}
	return notes, nil
}

// ParseNotify decodes a FILE_NOTIFY_INFORMATION chain with bounds checks.
func ParseNotify(dir string, data []byte) ([]Note, error) {
	var notes []Note
	for off := 0; ; {
		if len(data)-off < 12 {
			return notes, fmt.Errorf("truncated notify record at %d", off)
		}
		next := int(binary.LittleEndian.Uint32(data[off:]))
		action := binary.LittleEndian.Uint32(data[off+4:])
		nameLen := int(binary.LittleEndian.Uint32(data[off+8:]))
		start := off + 12
		if nameLen%2 != 0 || nameLen > len(data)-start {
			return notes, fmt.Errorf("notify name overruns buffer at %d", off)
		}
		u := make([]uint16, nameLen/2)
		for i := range u {
			u[i] = binary.LittleEndian.Uint16(data[start+2*i:])
		}
		path := filepath.Join(dir, string(utf16.Decode(u)))
		op := OpOther
		switch action {
		case windows.FILE_ACTION_ADDED:
			op = OpCreate
		case windows.FILE_ACTION_REMOVED:
			op = OpDelete
		case windows.FILE_ACTION_MODIFIED:
			op = OpModify
		case windows.FILE_ACTION_RENAMED_OLD_NAME:
			op = OpRenameFrom
		case windows.FILE_ACTION_RENAMED_NEW_NAME:
			op = OpRenameTo
		}
		notes = append(notes, Note{Path: path, Op: op})
		if next == 0 {
			return notes, nil
		}
		if next < 12 || next > len(data)-off {
			return notes, fmt.Errorf("bad NextEntryOffset %d at %d", next, off)
		}
		off += next
	}
}

func (w *RDCW) Close() error {
	_ = windows.CancelIoEx(w.handle, &w.ov)
	var n uint32
	_ = windows.GetOverlappedResult(w.handle, &w.ov, &n, true)
	err := windows.CloseHandle(w.handle)
	_ = windows.CloseHandle(w.event)
	return err
}
