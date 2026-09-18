// Package clipboard copies rendered UTF-8 text to the operating system clipboard.
package clipboard

import (
	"bytes"
	"errors"
)

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
