// Package facts checks the v0.4.0-a8 platform facts (plan §2.1) on a real
// system and records evidence. Tests fail only on harness errors: a fact
// that turns out false is a finding, recorded with verdict FALSE.
package facts

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"dirloom.local/spikes/a8watch/notify"
)

var evidenceMu sync.Mutex

// evidence logs one finding and appends it to $A8_EVIDENCE as JSON.
func evidence(t *testing.T, fact, verdict, format string, args ...any) {
	t.Helper()
	detail := fmt.Sprintf(format, args...)
	t.Logf("EVIDENCE %s %s %s: %s", fact, runtime.GOOS, verdict, detail)
	path := os.Getenv("A8_EVIDENCE")
	if path == "" {
		return
	}
	line, _ := json.Marshal(map[string]string{"fact": fact, "os": runtime.GOOS, "verdict": verdict, "detail": detail})
	evidenceMu.Lock()
	defer evidenceMu.Unlock()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Errorf("open evidence file: %v", err)
		return
	}
	defer f.Close()
	_, _ = f.Write(append(line, '\n'))
}

func verdict(ok bool) string {
	if ok {
		return "TRUE"
	}
	return "FALSE"
}

func mkdirs(t *testing.T, paths ...string) {
	t.Helper()
	for _, p := range paths {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", p, err)
		}
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// listing returns the relative entries below root, directories with "/".
func listing(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			rel += "/"
		}
		out = append(out, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	sort.Strings(out)
	return out
}

// dirsBelow returns every directory below root (root excluded).
func dirsBelow(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	for _, rel := range listing(t, root) {
		if strings.HasSuffix(rel, "/") {
			out = append(out, filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(rel, "/"))))
		}
	}
	return out
}

func drain(t *testing.T, n notify.Notifier) []notify.Note {
	t.Helper()
	notes, err := notify.Drain(n, 400*time.Millisecond, 8*time.Second)
	if err != nil {
		t.Logf("drain error: %v", err)
	}
	return notes
}

// waitFor reads notifications until pred matches one or timeout expires.
func waitFor(n notify.Notifier, timeout time.Duration, pred func(notify.Note) bool) (bool, time.Duration, []notify.Note) {
	start := time.Now()
	var all []notify.Note
	for time.Since(start) < timeout {
		notes, err := n.Wait(100 * time.Millisecond)
		if err != nil {
			return false, time.Since(start), all
		}
		all = append(all, notes...)
		for _, note := range notes {
			if pred(note) {
				return true, time.Since(start), all
			}
		}
	}
	return false, time.Since(start), all
}

// summarize renders notes compactly: op:base, at most limit entries.
func summarize(root string, notes []notify.Note, limit int) string {
	if len(notes) == 0 {
		return "none"
	}
	parts := make([]string, 0, limit)
	for i, n := range notes {
		if i == limit {
			parts = append(parts, fmt.Sprintf("… (%d total)", len(notes)))
			break
		}
		rel := n.Path
		if r, err := filepath.Rel(root, n.Path); err == nil && !strings.HasPrefix(r, "..") {
			rel = filepath.ToSlash(r)
		}
		parts = append(parts, fmt.Sprintf("%s:%s", n.Op, rel))
	}
	return strings.Join(parts, ", ")
}

func ms(d time.Duration) string { return fmt.Sprintf("%d ms", d.Milliseconds()) }

var (
	helperOnce sync.Once
	helperPath string
	helperErr  error
)

// buildHelper builds cmd/helper once per test binary.
func buildHelper(t *testing.T) string {
	t.Helper()
	helperOnce.Do(func() {
		dir, err := os.MkdirTemp("", "a8-helper-")
		if err != nil {
			helperErr = err
			return
		}
		helperPath = filepath.Join(dir, "helper")
		if runtime.GOOS == "windows" {
			helperPath += ".exe"
		}
		cmd := exec.Command("go", "build", "-o", helperPath, "./cmd/helper")
		cmd.Dir = ".."
		out, err := cmd.CombinedOutput()
		if err != nil {
			helperErr = fmt.Errorf("go build helper: %v: %s", err, out)
		}
	})
	if helperErr != nil {
		t.Fatalf("%v", helperErr)
	}
	return helperPath
}

// newRegistered starts the native backend on root and, for per-directory
// backends, registers every existing directory below it.
func newRegistered(t *testing.T, root string) notify.Notifier {
	t.Helper()
	n, err := notify.NewNative(root)
	if err != nil {
		t.Fatalf("native backend: %v", err)
	}
	if !n.Recursive() {
		for _, d := range dirsBelow(t, root) {
			if err := n.Watch(d); err != nil {
				t.Fatalf("watch %s: %v", d, err)
			}
		}
	}
	return n
}
