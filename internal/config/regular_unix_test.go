//go:build linux || darwin

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// TestResolveRefusesDevicesAndPipes runs F2 of the D7 audit on real special
// files: a project configuration linked to /dev/zero, which used to be read
// without end, and one that is a named pipe, which used to block. Both are
// refused at once.
func TestResolveRefusesDevicesAndPipes(t *testing.T) {
	for name, create := range map[string]func(path string) error{
		"link to /dev/zero": func(path string) error { return os.Symlink("/dev/zero", path) },
		"named pipe":        func(path string) error { return unix.Mkfifo(path, 0o644) },
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := create(filepath.Join(root, ".dirloom.yaml")); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				_, err := loaderWithoutUserConfig().Resolve(ResolveOptions{Root: root})
				done <- err
			}()
			select {
			case err := <-done:
				if err == nil || !strings.Contains(err.Error(), "not a regular file") {
					t.Fatalf("Resolve = %v, want a non-regular file refused", err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("Resolve did not return within 10 s")
			}
		})
	}
}
