//go:build linux || windows || darwin

package backend

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/identity"
	"github.com/dirloom/dirloom/internal/watch"
)

// guard bounds a wait for the kernel to deliver an event; events normally
// arrive in microseconds.
const guard = 10 * time.Second

type traced struct {
	name     string
	detail   uint32
	relevant bool
}

// recorder is the engine's side of a backend under test: it counts the
// signals and keeps every traced event.
type recorder struct {
	mu        sync.Mutex
	notifies  int
	overflows int
	fail      error
	events    []traced
	poke      chan struct{}
}

func newRecorder() *recorder { return &recorder{poke: make(chan struct{}, 1)} }

func (r *recorder) update(f func()) {
	r.mu.Lock()
	f()
	r.mu.Unlock()
	select {
	case r.poke <- struct{}{}:
	default:
	}
}

func (r *recorder) Notify()   { r.update(func() { r.notifies++ }) }
func (r *recorder) Overflow() { r.update(func() { r.overflows++ }) }

// Fail keeps the first failure, as the engine does.
func (r *recorder) Fail(err error) {
	r.update(func() {
		if r.fail == nil {
			r.fail = err
		}
	})
}

func (r *recorder) failure() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fail
}

func (r *recorder) waitFor(t *testing.T, what string, cond func(*recorder) bool) {
	t.Helper()
	deadline := time.After(guard)
	for {
		r.mu.Lock()
		done := cond(r)
		r.mu.Unlock()
		if done {
			return
		}
		select {
		case <-r.poke:
		case <-deadline:
			t.Fatalf("no %s within %s", what, guard)
		}
	}
}

// sawRelevant waits until an event named name was traced as relevant.
func (r *recorder) sawRelevant(t *testing.T, name string) {
	t.Helper()
	r.waitFor(t, "relevant event for "+name, func(r *recorder) bool {
		for _, ev := range r.events {
			if ev.name == name && ev.relevant {
				return true
			}
		}
		return false
	})
}

func (r *recorder) relevantFor(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, ev := range r.events {
		if ev.name == name && ev.relevant {
			return true
		}
	}
	return false
}

// observe builds an observation of root without capture rules: directories,
// files and links, root included.
func observe(t *testing.T, root string) watch.Observation {
	t.Helper()
	records := []identity.Record{{Path: artifact.RootPath, Kind: artifact.KindDirectory}}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == root {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		kind := artifact.KindFile
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			kind = artifact.KindSymlink
		case d.IsDir():
			kind = artifact.KindDirectory
		}
		records = append(records, identity.Record{Path: artifact.Path(filepath.ToSlash(rel)), Kind: kind})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Path < records[j].Path })
	return watch.Observation{Records: records}
}

func mkdirs(t *testing.T, paths ...string) {
	t.Helper()
	for _, p := range paths {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func appendTo(t *testing.T, path string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("more\n")
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// record keeps one traced event: a name, a backend-specific detail (an
// inotify mask, a Windows action) and its relevance.
func (r *recorder) record(name string, detail uint32, relevant bool) {
	r.update(func() { r.events = append(r.events, traced{name, detail, relevant}) })
}

// readerPanics waits for the failure that a panic of the reader reports, a
// broken invariant, then checks that Close still returns: the reader has
// ended and released its done channel.
func readerPanics(t *testing.T, r *recorder, b watch.Backend) {
	t.Helper()
	r.waitFor(t, "failure", func(r *recorder) bool { return r.fail != nil })
	var typed *artifact.Error
	if err := r.failure(); !errors.As(err, &typed) || typed.Code != artifact.CodeInternal || !typed.Internal ||
		typed.Message != "unexpected panic: reader bug" {
		t.Fatalf("failure = %v, want an internal invariant failure", err)
	}
	closed := make(chan struct{})
	go func() {
		_ = b.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(guard):
		t.Fatal("Close blocked after the reader panicked")
	}
}

func rootUnavailable(err error) bool {
	var typed *artifact.Error
	return errors.As(err, &typed) && typed.Code == artifact.CodeWatchRootUnavailable
}
