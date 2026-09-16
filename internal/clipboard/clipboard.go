// Package clipboard copies rendered UTF-8 text to the operating system clipboard.
package clipboard

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

const copyTimeout = 5 * time.Second

// Writer copies canonical UTF-8 render bytes. Identity at this boundary is
// byte-for-byte with the renderer; native OS storage may recode the text.
type Writer interface {
	Write(data []byte) error
}

// Buffer is an in-memory Writer for tests. It never touches a real clipboard.
type Buffer struct {
	Data []byte
}

// Write stores a defensive copy of data.
func (buffer *Buffer) Write(data []byte) error {
	buffer.Data = bytes.Clone(data)
	return nil
}

// Fail is a Writer that always returns Err.
type Fail struct {
	Err error
}

// Write returns the configured error without storing data.
func (fail Fail) Write([]byte) error {
	if fail.Err == nil {
		return errors.New("clipboard write failed")
	}
	return fail.Err
}

type commandRunner func(ctx context.Context, name string, args []string, stdin []byte) error

func runCommand(ctx context.Context, name string, args []string, stdin []byte) error {
	if name == "" {
		return fmt.Errorf("clipboard command is empty")
	}
	command := exec.CommandContext(ctx, name, args...) //nolint:gosec // Clipboard backends invoke fixed OS utilities by absolute path.
	command.Stdin = bytes.NewReader(stdin)
	command.Stdout = nil
	command.Stderr = nil
	if err := command.Run(); err != nil {
		return fmt.Errorf("run %s: %w", name, err)
	}
	return nil
}
