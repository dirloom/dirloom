//go:build windows

package clipboard

import (
	"bytes"
	"errors"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestWindowsWriterCopiesUTF16AndRetries(t *testing.T) {
	opens := 0
	buffer := make([]byte, 64)
	pointer := unsafe.Pointer(unsafe.SliceData(buffer))
	api := windowsAPI{
		open: func() error {
			opens++
			if opens < 3 {
				return errors.New("clipboard busy")
			}
			return nil
		},
		empty: func() error { return nil },
		close: func() error { return nil },
		alloc: func(size int) (windows.Handle, error) {
			if size > len(buffer) {
				t.Fatalf("size = %d", size)
			}
			return 1, nil
		},
		lock: func(windows.Handle) (unsafe.Pointer, error) {
			return pointer, nil
		},
		unlock: func(windows.Handle) error { return nil },
		set:    func(windows.Handle) error { return nil },
		free:   func(windows.Handle) error { return nil },
	}
	writer := &windowsWriter{api: api, attempts: 5, delay: time.Millisecond}
	payload := []byte("ok\n")
	if err := writer.Write(payload); err != nil {
		t.Fatal(err)
	}
	if opens != 3 {
		t.Fatalf("opens = %d, want 3", opens)
	}
	want := utf16LEWithNUL(payload)
	if !bytes.Equal(buffer[:len(want)], want) {
		t.Fatalf("clipboard memory = %v, want %v", buffer[:len(want)], want)
	}
}

func TestWindowsFreesMemoryWhenSetFails(t *testing.T) {
	freed := false
	buffer := make([]byte, 32)
	pointer := unsafe.Pointer(unsafe.SliceData(buffer))
	writer := &windowsWriter{api: windowsAPI{
		open:  func() error { return nil },
		empty: func() error { return nil },
		close: func() error { return nil },
		alloc: func(int) (windows.Handle, error) { return 1, nil },
		lock: func(windows.Handle) (unsafe.Pointer, error) {
			return pointer, nil
		},
		unlock: func(windows.Handle) error { return nil },
		set:    func(windows.Handle) error { return errors.New("set failed") },
		free: func(windows.Handle) error {
			freed = true
			return nil
		},
	}, attempts: 1, delay: time.Millisecond}
	if err := writer.Write([]byte("x")); err == nil {
		t.Fatal("expected set failure")
	}
	if !freed {
		t.Fatal("clipboard memory must be freed when SetClipboardData fails")
	}
}

func TestWindowsWriterGivesUpWhenBusy(t *testing.T) {
	writer := &windowsWriter{api: windowsAPI{
		open: func() error { return errors.New("busy") },
	}, attempts: 3, delay: time.Millisecond}
	if err := writer.Write([]byte("x")); err == nil {
		t.Fatal("expected contention error")
	}
}
