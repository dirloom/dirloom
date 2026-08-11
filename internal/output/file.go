// Package output provides transactional filesystem output.
package output

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// WriteFile atomically replaces path with data. It never deletes an existing
// destination as a fallback and never follows a symlink destination.
func WriteFile(path string, data []byte) (err error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve output path %q: %w", path, err)
	}
	absPath = filepath.Clean(absPath)
	parent := filepath.Dir(absPath)

	parentInfo, err := os.Stat(parent)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("output directory %q does not exist", parent)
		}
		return fmt.Errorf("open output directory %q: %w", parent, err)
	}
	if !parentInfo.IsDir() {
		return fmt.Errorf("output parent %q is not a directory", parent)
	}

	mode := os.FileMode(0o644)
	if info, lstatErr := os.Lstat(absPath); lstatErr == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("output path %q is a symlink; refusing to follow it", path)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("output path %q is not a regular file", path)
		}
		mode = info.Mode().Perm()
	} else if !errors.Is(lstatErr, os.ErrNotExist) {
		return fmt.Errorf("inspect output path %q: %w", path, lstatErr)
	}

	temporary, err := os.CreateTemp(parent, "."+filepath.Base(absPath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary output in %q: %w", parent, err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}()

	if err := temporary.Chmod(mode); err != nil {
		return fmt.Errorf("set temporary output permissions: %w", err)
	}
	if err := writeAll(temporary, data); err != nil {
		return fmt.Errorf("write temporary output: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync temporary output: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary output: %w", err)
	}
	if err := replaceAtomic(temporaryPath, absPath); err != nil {
		return fmt.Errorf("atomically replace output %q: %w", path, err)
	}
	return nil
}

func writeAll(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		written, err := writer.Write(data)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		data = data[written:]
	}
	return nil
}
