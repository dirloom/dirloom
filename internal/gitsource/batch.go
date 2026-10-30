package gitsource

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dirloom/dirloom/internal/artifact"
)

// Batch is the one `git cat-file --batch` process that serves one
// observation. It is used by a single goroutine; only cancellation of its
// context acts concurrently, and only through terminate.
type Batch struct {
	ctx    context.Context
	format ObjectFormat
	cmd    *exec.Cmd
	input  *os.File
	output *os.File
	reader *bufio.Reader

	exited    chan struct{}
	waitErr   error
	stopWatch func() bool
	shutdown  sync.Once

	err      error
	closed   bool
	closeErr error
}

// StartBatch starts the batch process, bound to ctx until Close.
func (r *Runner) StartBatch(ctx context.Context, format ObjectFormat) (*Batch, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	childInput, input, err := os.Pipe()
	if err != nil {
		return nil, unavailable("git cat-file could not be started")
	}
	output, childOutput, err := os.Pipe()
	if err != nil {
		_ = childInput.Close()
		_ = input.Close()
		return nil, unavailable("git cat-file could not be started")
	}

	// #nosec G204 -- git found on PATH with a fixed hardened argument list.
	cmd := exec.Command(r.executable, r.arguments([]string{"cat-file", "--batch"})...)
	r.configure(cmd)
	cmd.Stdin = childInput
	cmd.Stdout = childOutput
	startErr := cmd.Start()
	// The child holds its own copies; keeping ours would hide its exit.
	_ = childInput.Close()
	_ = childOutput.Close()
	if startErr != nil {
		_ = input.Close()
		_ = output.Close()
		return nil, unavailable("git cat-file could not be started")
	}

	b := &Batch{
		ctx:    ctx,
		format: format,
		cmd:    cmd,
		input:  input,
		output: output,
		reader: bufio.NewReaderSize(output, 64<<10),
		exited: make(chan struct{}),
	}
	go func() {
		b.waitErr = cmd.Wait()
		close(b.exited)
	}()
	b.stopWatch = context.AfterFunc(ctx, b.terminate)
	return b, nil
}

// read returns the content of object id, which must have the type of kind
// and fit its limit. After a failure every later read fails the same way:
// the protocol position is lost.
func (b *Batch) read(id string, kind objectKind) ([]byte, error) {
	if b.err != nil {
		return nil, b.err
	}
	if b.closed {
		return nil, artifact.InternalError("git batch read after close")
	}
	if err := b.ctx.Err(); err != nil {
		b.err = err
		return nil, err
	}
	if !b.format.isObjectID(id) {
		b.err = artifact.InternalError("git object id %q is not a full %s id", id, b.format.Name)
		return nil, b.err
	}
	if _, err := io.WriteString(b.input, id+"\n"); err != nil {
		return nil, b.fail(objectUnavailable("git cat-file stopped before object %s was read", id))
	}
	data, err := readResponse(b.reader, id, kind)
	if err != nil {
		return nil, b.fail(err)
	}
	return data, nil
}

func (b *Batch) fail(err error) error {
	if ctxErr := b.ctx.Err(); ctxErr != nil {
		err = ctxErr
	}
	b.err = err
	return err
}

// terminate closes the process input, kills the process if it has not
// exited within waitDelay, and waits for it. Concurrent callers all return
// after it has exited.
func (b *Batch) terminate() {
	b.shutdown.Do(func() {
		_ = b.input.Close()
		timer := time.NewTimer(waitDelay)
		defer timer.Stop()
		select {
		case <-b.exited:
		case <-timer.C:
			_ = b.cmd.Process.Kill()
			<-b.exited
		}
	})
}

// Close ends the process and releases the pipes. It always waits for the
// process; no goroutine or process outlives it.
func (b *Batch) Close() error {
	if b.closed {
		return b.closeErr
	}
	b.closed = true
	b.stopWatch()
	// Nothing is read any more. Closing the output first makes a process
	// still writing a response it was refused (an object above its limit)
	// fail at once, instead of blocking until the grace period ends.
	_ = b.output.Close()
	b.terminate()
	if b.waitErr != nil && b.err == nil {
		b.closeErr = objectUnavailable("git cat-file did not exit cleanly")
	}
	return b.closeErr
}

// maxHeaderLength bounds a batch header line; a real one is under 90 bytes.
const maxHeaderLength = 128

// readResponse reads one batch response for id and checks it in the order
// Git Tree Source v1 lists. It never reads past the announced size and its
// terminating newline.
func readResponse(reader *bufio.Reader, id string, kind objectKind) ([]byte, error) {
	line, err := reader.ReadSlice('\n')
	if errors.Is(err, bufio.ErrBufferFull) || (err == nil && len(line) > maxHeaderLength) {
		return nil, treeInvalid("git cat-file returned a malformed header for object %s", id)
	}
	if err != nil {
		return nil, objectUnavailable("git cat-file response for object %s is truncated", id)
	}
	fields := strings.Split(string(line[:len(line)-1]), " ")

	if len(fields) == 2 && fields[0] == id && fields[1] == "missing" {
		return nil, objectUnavailable("git object %s is not available in this repository", id)
	}
	if len(fields) != 3 || fields[0] != id {
		return nil, treeInvalid("git cat-file returned a malformed header for object %s", id)
	}
	if fields[1] != kind.typ {
		return nil, treeInvalid("git object %s is a %s, expected a %s", id, quoteToken(fields[1]), kind.typ)
	}
	size, ok := parseSize(fields[2])
	if !ok {
		return nil, treeInvalid("git cat-file returned a malformed size for object %s", id)
	}
	if size > kind.limit {
		return nil, treeInvalid("%s %s is %d bytes, above the %s limit", kind.label, id, size, formatLimit(kind.limit))
	}

	data := make([]byte, size)
	if _, err := io.ReadFull(reader, data); err != nil {
		return nil, objectUnavailable("git cat-file response for object %s is truncated", id)
	}
	if terminator, err := reader.ReadByte(); err != nil || terminator != '\n' {
		return nil, objectUnavailable("git cat-file response for object %s is truncated", id)
	}
	return data, nil
}

// parseSize accepts a canonical decimal: digits only, no sign, no leading
// zero.
func parseSize(text string) (int64, bool) {
	if text == "" || len(text) > 18 || (len(text) > 1 && text[0] == '0') {
		return 0, false
	}
	for i := 0; i < len(text); i++ {
		if text[i] < '0' || text[i] > '9' {
			return 0, false
		}
	}
	size, err := strconv.ParseInt(text, 10, 64)
	return size, err == nil
}
