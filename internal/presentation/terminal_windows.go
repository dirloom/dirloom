//go:build windows

package presentation

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func prepareANSI(file *os.File) (func() error, error) {
	handle := windows.Handle(file.Fd())
	var original uint32
	if err := windows.GetConsoleMode(handle, &original); err != nil {
		return nil, err
	}
	if original&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0 {
		return func() error { return nil }, nil
	}
	if err := windows.SetConsoleMode(handle, original|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
		return nil, err
	}
	return func() error {
		if err := windows.SetConsoleMode(handle, original); err != nil {
			return fmt.Errorf("restore Windows console mode: %w", err)
		}
		return nil
	}, nil
}
