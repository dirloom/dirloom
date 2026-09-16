//go:build windows

package clipboard

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	cfUnicodeText     = 13
	gmemMoveable      = 0x0002
	clipboardAttempts = 20
	clipboardDelay    = 250 * time.Millisecond
)

var (
	moduser32   = windows.NewLazySystemDLL("user32.dll")
	modkernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procOpenClipboard    = moduser32.NewProc("OpenClipboard")
	procEmptyClipboard   = moduser32.NewProc("EmptyClipboard")
	procCloseClipboard   = moduser32.NewProc("CloseClipboard")
	procSetClipboardData = moduser32.NewProc("SetClipboardData")
	procGlobalAlloc      = modkernel32.NewProc("GlobalAlloc")
	procGlobalLock       = modkernel32.NewProc("GlobalLock")
	procGlobalUnlock     = modkernel32.NewProc("GlobalUnlock")
	procGlobalFree       = modkernel32.NewProc("GlobalFree")
)

type windowsAPI struct {
	open   func() error
	empty  func() error
	close  func() error
	alloc  func(size int) (windows.Handle, error)
	lock   func(windows.Handle) (unsafe.Pointer, error)
	unlock func(windows.Handle) error
	set    func(windows.Handle) error
	free   func(windows.Handle) error
}

type windowsWriter struct {
	api      windowsAPI
	attempts int
	delay    time.Duration
}

func newNativeWriter() Writer {
	return &windowsWriter{api: nativeWindowsAPI(), attempts: clipboardAttempts, delay: clipboardDelay}
}

func lastError(r1 uintptr, err error) error {
	if r1 != 0 {
		return nil
	}
	if err != nil {
		return err
	}
	return syscall.EINVAL
}

func nativeWindowsAPI() windowsAPI {
	return windowsAPI{
		open: func() error {
			r1, _, err := procOpenClipboard.Call(0)
			return lastError(r1, err)
		},
		empty: func() error {
			r1, _, err := procEmptyClipboard.Call()
			return lastError(r1, err)
		},
		close: func() error {
			r1, _, err := procCloseClipboard.Call()
			return lastError(r1, err)
		},
		alloc: func(size int) (windows.Handle, error) {
			r1, _, err := procGlobalAlloc.Call(gmemMoveable, uintptr(size))
			if r1 == 0 {
				if err == nil {
					err = syscall.EINVAL
				}
				return 0, err
			}
			return windows.Handle(r1), nil
		},
		lock: func(handle windows.Handle) (unsafe.Pointer, error) {
			r1, _, err := procGlobalLock.Call(uintptr(handle))
			if r1 == 0 {
				if err == nil {
					err = syscall.EINVAL
				}
				return nil, err
			}
			return unsafe.Add(unsafe.Pointer(nil), r1), nil
		},
		unlock: func(handle windows.Handle) error {
			r1, _, err := procGlobalUnlock.Call(uintptr(handle))
			if r1 == 0 {
				if err == windows.ERROR_SUCCESS {
					return nil
				}
				return err
			}
			return nil
		},
		set: func(handle windows.Handle) error {
			r1, _, err := procSetClipboardData.Call(cfUnicodeText, uintptr(handle))
			return lastError(r1, err)
		},
		free: func(handle windows.Handle) error {
			r1, _, err := procGlobalFree.Call(uintptr(handle))
			if r1 == 0 {
				return err
			}
			return nil
		},
	}
}

func (writer *windowsWriter) Write(data []byte) error {
	encoded := utf16LEWithNUL(data)
	attempts := writer.attempts
	if attempts <= 0 {
		attempts = clipboardAttempts
	}
	delay := writer.delay
	if delay <= 0 {
		delay = clipboardDelay
	}
	var last error
	for attempt := 0; attempt < attempts; attempt++ {
		if err := writer.api.open(); err != nil {
			last = err
			time.Sleep(delay)
			continue
		}
		err := writer.writeLocked(encoded)
		closeErr := writer.api.close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return fmt.Errorf("close clipboard: %w", closeErr)
		}
		return nil
	}
	if last == nil {
		last = fmt.Errorf("clipboard is busy")
	}
	return fmt.Errorf("open clipboard: %w", last)
}

func (writer *windowsWriter) writeLocked(encoded []byte) error {
	if err := writer.api.empty(); err != nil {
		return fmt.Errorf("empty clipboard: %w", err)
	}
	handle, err := writer.api.alloc(len(encoded))
	if err != nil {
		return fmt.Errorf("allocate clipboard memory: %w", err)
	}
	pointer, err := writer.api.lock(handle)
	if err != nil {
		_ = writer.api.free(handle)
		return fmt.Errorf("lock clipboard memory: %w", err)
	}
	destination := unsafe.Slice((*byte)(pointer), len(encoded))
	copy(destination, encoded)
	if err := writer.api.unlock(handle); err != nil {
		_ = writer.api.free(handle)
		return fmt.Errorf("unlock clipboard memory: %w", err)
	}
	if err := writer.api.set(handle); err != nil {
		_ = writer.api.free(handle)
		return fmt.Errorf("set clipboard data: %w", err)
	}
	return nil
}
