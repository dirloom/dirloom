package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// deviceFileSystem reports path as a character device of size 0, as Stat
// reports /dev/zero through a link, and fails the test if path is read.
type deviceFileSystem struct {
	fileSystem
	t    *testing.T
	path string
}

func (f deviceFileSystem) Stat(path string) (os.FileInfo, error) {
	if path == f.path {
		return deviceInfo{}, nil
	}
	return f.fileSystem.Stat(path)
}

func (f deviceFileSystem) ReadFile(path string) ([]byte, error) {
	if path == f.path {
		f.t.Errorf("the device %s was read", path)
		return nil, errors.New("device read")
	}
	return f.fileSystem.ReadFile(path)
}

type deviceInfo struct{}

func (deviceInfo) Name() string       { return ".dirloom.yaml" }
func (deviceInfo) Size() int64        { return 0 }
func (deviceInfo) Mode() fs.FileMode  { return fs.ModeDevice | fs.ModeCharDevice | 0o666 }
func (deviceInfo) ModTime() time.Time { return time.Time{} }
func (deviceInfo) IsDir() bool        { return false }
func (deviceInfo) Sys() any           { return nil }

// TestResolveRefusesAConfigThatIsNotARegularFile: a project configuration
// that is a device (a cloned link to /dev/zero) is refused before it is
// read; its size of 0 used to pass the 1 MiB check, and reading it never
// ended (F2 of the D7 audit of the v0.4.0-rc1 plan).
func TestResolveRefusesAConfigThatIsNotARegularFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".dirloom.yaml")
	loader := loaderWithoutUserConfig()
	loader.fs = deviceFileSystem{fileSystem: osFileSystem{}, t: t, path: path}
	_, err := loader.Resolve(ResolveOptions{Root: root})
	if err == nil || IsInvalid(err) || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("device config = %v, want a runtime error naming a non-regular file", err)
	}
}

// TestOSReadFileStopsAfterTheLimit: the reader never reads more than one
// byte beyond 1 MiB, whatever Stat said.
func TestOSReadFileStopsAfterTheLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "big.yaml")
	if err := os.WriteFile(path, make([]byte, 3*maxConfigSize), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := osFileSystem{}.ReadFile(path)
	if err != nil || len(data) != maxConfigSize+1 {
		t.Fatalf("read %d bytes, %v; want %d", len(data), err, maxConfigSize+1)
	}
}
