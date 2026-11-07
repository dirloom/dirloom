package backend

import (
	"errors"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/snapshot"
)

// trace records root-handle events under their base name, and control
// events under "control:" and the name the parent reported.
func (r *recorder) trace(rec notifyRecord, control, relevant bool) {
	name := path.Base(rec.path)
	if control {
		name = "control:" + rec.path
	}
	r.record(name, rec.action, relevant)
}

func startRdcw(t *testing.T, root string, capture snapshot.CaptureV1, gate chan struct{}) (*rdcw, *recorder) {
	t.Helper()
	rec := newRecorder()
	b := newNative(Options{Root: root, Capture: capture}).(*rdcw)
	b.trace, b.gate = rec.trace, gate
	if err := b.Start(rec); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b, rec
}

func reconcile(t *testing.T, b *rdcw, root string) bool {
	t.Helper()
	changed, err := b.Reconcile(observe(t, root))
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	return changed
}

func createFiles(t *testing.T, dir string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		f, err := os.Create(filepath.Join(dir, "burst-"+strconv.Itoa(i)))
		if err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
	}
}

func setHidden(t *testing.T, p string, hidden bool) {
	t.Helper()
	ptr, err := windows.UTF16PtrFromString(p)
	if err != nil {
		t.Fatal(err)
	}
	attrs, err := windows.GetFileAttributes(ptr)
	if err != nil {
		t.Fatal(err)
	}
	if hidden {
		attrs |= windows.FILE_ATTRIBUTE_HIDDEN
	} else {
		attrs &^= windows.FILE_ATTRIBUTE_HIDDEN
	}
	if err := windows.SetFileAttributes(ptr, attrs); err != nil {
		t.Fatal(err)
	}
}

func TestRdcwReportsEntriesAndIgnoresContent(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, filepath.Join(root, "a"))
	write(t, filepath.Join(root, "a", "f"), "x")
	b, rec := startRdcw(t, root, snapshot.CaptureV1{UseGitignore: true}, nil)
	if !reconcile(t, b, root) {
		t.Fatal("directories entering the structural set must schedule a verification")
	}
	if reconcile(t, b, root) {
		t.Fatal("an unchanged structure must not schedule a verification")
	}
	appendTo(t, filepath.Join(root, "a", "f"))
	write(t, filepath.Join(root, "a", "f"), "rewritten")
	write(t, filepath.Join(root, "a", "sentinel"), "")
	rec.sawRelevant(t, "sentinel")
	if rec.relevantFor("f") {
		t.Fatal("a content write was relevant")
	}
	write(t, filepath.Join(root, "top"), "")
	rec.sawRelevant(t, "top")
}

func TestRdcwIgnoresExcludedSubtreesAndDepth(t *testing.T) {
	t.Run("excluded subtree", func(t *testing.T) {
		root := t.TempDir()
		mkdirs(t, filepath.Join(root, "skip", "deep"))
		b, rec := startRdcw(t, root, snapshot.CaptureV1{}, nil)
		obs := observe(t, root)
		kept := obs.Records[:0]
		for _, r := range obs.Records {
			if r.Path != "skip" && !strings.HasPrefix(string(r.Path), "skip/") {
				kept = append(kept, r)
			}
		}
		obs.Records = kept
		if _, err := b.Reconcile(obs); err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(root, "skip", "x"), "")
		mkdirs(t, filepath.Join(root, "skip", "deep", "y"))
		write(t, filepath.Join(root, "sentinel"), "")
		rec.sawRelevant(t, "sentinel")
		if rec.relevantFor("x") || rec.relevantFor("y") {
			t.Fatal("a change in an excluded subtree was relevant")
		}
	})
	t.Run("depth 1", func(t *testing.T) {
		root := t.TempDir()
		mkdirs(t, filepath.Join(root, "a"))
		depth := 1
		b, rec := startRdcw(t, root, snapshot.CaptureV1{Depth: &depth}, nil)
		reconcile(t, b, root)
		write(t, filepath.Join(root, "a", "x"), "")
		mkdirs(t, filepath.Join(root, "a", "y"))
		write(t, filepath.Join(root, "top"), "")
		rec.sawRelevant(t, "top")
		if rec.relevantFor("x") || rec.relevantFor("y") {
			t.Fatal("a change beyond the depth limit was relevant")
		}
	})
}

func TestRdcwGitignoreWrites(t *testing.T) {
	for _, useGitignore := range []bool{true, false} {
		t.Run(strconv.FormatBool(useGitignore), func(t *testing.T) {
			root := t.TempDir()
			write(t, filepath.Join(root, ".gitignore"), "*.log\n")
			b, rec := startRdcw(t, root, snapshot.CaptureV1{UseGitignore: useGitignore}, nil)
			reconcile(t, b, root)
			appendTo(t, filepath.Join(root, ".gitignore"))
			write(t, filepath.Join(root, "sentinel"), "")
			rec.sawRelevant(t, "sentinel")
			if got := rec.relevantFor(".gitignore"); got != useGitignore {
				t.Fatalf(".gitignore write relevant = %v, want %v", got, useGitignore)
			}
		})
	}
}

func TestRdcwHiddenAttribute(t *testing.T) {
	for _, tc := range []struct {
		name     string
		capture  snapshot.CaptureV1
		dir      bool
		unhide   bool // the entry starts hidden, out of the state
		relevant bool
	}{
		{"file hidden", snapshot.CaptureV1{}, false, false, true},
		{"directory hidden", snapshot.CaptureV1{}, true, false, true},
		{"file unhidden", snapshot.CaptureV1{}, false, true, true},
		{"hidden entries included", snapshot.CaptureV1{Hidden: true}, false, false, false},
		{"directories only", snapshot.CaptureV1{DirsOnly: true}, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			entry := filepath.Join(root, "e")
			if tc.dir {
				mkdirs(t, entry)
			} else {
				write(t, entry, "")
			}
			if tc.unhide {
				setHidden(t, entry, true)
			}
			b, rec := startRdcw(t, root, tc.capture, nil)
			obs := observe(t, root)
			if tc.unhide {
				obs.Records = obs.Records[:1] // the root only
			}
			if _, err := b.Reconcile(obs); err != nil {
				t.Fatal(err)
			}
			setHidden(t, entry, !tc.unhide)
			write(t, filepath.Join(root, "sentinel"), "")
			rec.sawRelevant(t, "sentinel")
			if got := rec.relevantFor("e"); got != tc.relevant {
				t.Fatalf("attribute change relevant = %v, want %v", got, tc.relevant)
			}
		})
	}
}

func TestRdcwRelevanceOfNamesOutsideTheState(t *testing.T) {
	b := &rdcw{opts: Options{Root: t.TempDir()}, set: map[string]struct{}{".": {}, "a": {}, "é": {}}}
	for _, tc := range []struct {
		rec  notifyRecord
		want bool
	}{
		{notifyRecord{actionAdded, "skip/x"}, false},
		{notifyRecord{actionAdded, "A/new"}, true},         // case-insensitive
		{notifyRecord{actionAdded, "é/new"}, true},        // NFD spelling of a structural directory
		{notifyRecord{actionAdded, "PROGRA~1/x"}, true},    // a short name may hide a structural directory
		{notifyRecord{actionModified, "a/GITIGN~1"}, true}, // or a .gitignore
		{notifyRecord{actionAdded, "skip/�"}, true},        // undecodable
		{notifyRecord{actionModified, "a/.hidden"}, false},
	} {
		if got := b.relevant(tc.rec); got != tc.want {
			t.Errorf("relevant(%+v) = %v, want %v", tc.rec, got, tc.want)
		}
	}
}

// A panic of the reader goroutine ends the watch with a broken invariant
// instead of crashing the process.
func TestRdcwReaderPanicFailsTheWatch(t *testing.T) {
	root := t.TempDir()
	gate := make(chan struct{})
	b, rec := startRdcw(t, root, snapshot.CaptureV1{}, gate)
	b.trace = func(notifyRecord, bool, bool) { panic("reader bug") }
	close(gate)
	reconcile(t, b, root)
	write(t, filepath.Join(root, "f"), "")
	readerPanics(t, rec, b)
}

func TestRdcwRootLossFails(t *testing.T) {
	rename := func(root string) error { return os.Rename(root, root+"-moved") }
	for _, tc := range []struct {
		name  string
		depth *int
		lose  func(root string) error
	}{
		{"removed", nil, os.RemoveAll},
		{"renamed", nil, rename},
		{"depth 0 removed", new(int), os.RemoveAll},
		{"depth 0 renamed", new(int), rename},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "root")
			mkdirs(t, filepath.Join(root, "a"))
			b, rec := startRdcw(t, root, snapshot.CaptureV1{Depth: tc.depth}, nil)
			reconcile(t, b, root)
			if tc.depth != nil {
				write(t, filepath.Join(root, "child"), "") // below --depth 0: never signalled
			}
			if err := tc.lose(root); err != nil {
				t.Fatal(err)
			}
			rec.waitFor(t, "a root failure", func(r *recorder) bool { return r.fail != nil })
			if !rootUnavailable(rec.failure()) {
				t.Fatalf("Fail(%v), want watch_root_unavailable", rec.failure())
			}
			if tc.depth != nil && rec.relevantFor("child") {
				t.Fatal("a change below --depth 0 was signalled")
			}
		})
	}
}

// TestRdcwLocksTheRootsAncestorsOnly is the evidence for A1: the root and
// its entries stay renamable while watched, its parent does not, and Close
// releases it.
func TestRdcwLocksTheRootsAncestorsOnly(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "parent")
	root := filepath.Join(parent, "root")
	mkdirs(t, filepath.Join(root, "a"))
	b, _ := startRdcw(t, root, snapshot.CaptureV1{}, nil)
	if err := os.Rename(filepath.Join(root, "a"), filepath.Join(root, "b")); err != nil {
		t.Fatalf("renaming inside the watched root: %v", err)
	}
	if err := os.Rename(parent, parent+"-moved"); err == nil {
		t.Fatal("the root's parent was renamed during a native watch: A1 no longer holds, update the documented limit")
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(parent, parent+"-moved"); err != nil {
		t.Fatalf("the root's parent is still locked after Close: %v", err)
	}
}

func TestRdcwOverflowIsSignalled(t *testing.T) {
	root := t.TempDir()
	gate := make(chan struct{})
	_, rec := startRdcw(t, root, snapshot.CaptureV1{}, gate)
	released := false
	t.Cleanup(func() { // runs before Close, which waits for the reader
		if !released {
			close(gate)
		}
	})
	createFiles(t, root, 5000) // about 150 KiB of records for a 64 KiB buffer
	close(gate)
	released = true
	rec.waitFor(t, "an overflow", func(r *recorder) bool { return r.overflows > 0 })
}

// TestRdcwBurstOverflowFrequency is the evidence for A7: how often bursts of
// creations overflow the 64 KiB buffer while the reader runs. It asserts
// only that the watch stays usable afterwards.
func TestRdcwBurstOverflowFrequency(t *testing.T) {
	for _, n := range []int{1_000, 5_000, 20_000} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			root := t.TempDir()
			b, rec := startRdcw(t, root, snapshot.CaptureV1{}, nil)
			reconcile(t, b, root)
			start := time.Now()
			createFiles(t, root, n)
			elapsed := time.Since(start)
			for i := 0; !rec.sees(t, "sentinel-"+strconv.Itoa(i), root); i++ {
				if i == 10 {
					t.Fatal("no sentinel seen after the burst")
				}
			}
			rec.mu.Lock()
			notifies, overflows := rec.notifies, rec.overflows
			rec.mu.Unlock()
			t.Logf("burst of %d creations in %s: %d relevant records, %d overflows", n, elapsed.Round(time.Millisecond), notifies, overflows)
		})
	}
}

// sees writes name in dir and reports whether it is traced as relevant
// within a second; a sentinel lost in an overflow is not.
func (r *recorder) sees(t *testing.T, name, dir string) bool {
	t.Helper()
	write(t, filepath.Join(dir, name), "")
	deadline := time.After(time.Second)
	for !r.relevantFor(name) {
		select {
		case <-r.poke:
		case <-deadline:
			return false
		}
	}
	return true
}

var procGetProcessHandleCount = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetProcessHandleCount")

func handleCount(t *testing.T) int {
	t.Helper()
	var n uint32
	if r, _, err := procGetProcessHandleCount.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&n))); r == 0 {
		t.Fatal(err)
	}
	return int(n)
}

func TestRdcwCloseReleasesEverything(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	mkdirs(t, filepath.Join(root, "a"))
	cycle := func() {
		b := newNative(Options{Root: root}).(*rdcw)
		if err := b.Start(newRecorder()); err != nil {
			t.Fatal(err)
		}
		reconcile(t, b, root)
		if err := b.Close(); err != nil {
			t.Fatal(err)
		}
		if err := b.Close(); err != nil {
			t.Fatalf("second Close: %v", err)
		}
		select {
		case <-b.done:
		default:
			t.Fatal("the reader goroutine is still running")
		}
	}
	cycle() // handles the runtime opens once
	before := handleCount(t)
	for i := 0; i < 20; i++ {
		cycle()
	}
	// A leak holds five handles per cycle (two watches and the stop event).
	if after := handleCount(t); after-before >= 20 {
		t.Fatalf("handles %d before, %d after 20 Start and Close cycles", before, after)
	}
	if err := newNative(Options{Root: root}).Close(); err != nil {
		t.Fatalf("Close before Start: %v", err)
	}
	if n := HeldHandles(); n != 0 {
		t.Fatalf("%d handles held after every backend closed", n)
	}
}

func TestRdcwVolumePolicy(t *testing.T) {
	dir := t.TempDir()
	if name := refusedVolume(dir); name != "" {
		t.Fatalf("%s refused as %s", dir, name)
	}
	missing := filepath.Join(dir, "missing")
	b := newNative(Options{Root: missing}).(*rdcw)
	if err := b.Start(newRecorder()); !rootUnavailable(err) {
		t.Fatalf("Start on a missing root = %v", err)
	}
	_ = b.Close()

	vol := filepath.VolumeName(dir)
	unc := `\\localhost\` + strings.TrimSuffix(vol, ":") + `$` + dir[len(vol):]
	if _, err := os.Stat(unc); err != nil {
		t.Skipf("the administrative share is not reachable: %v", err)
	}
	b = newNative(Options{Root: unc}).(*rdcw)
	err := b.Start(newRecorder())
	var typed *artifact.Error
	if !errors.As(err, &typed) || typed.Code != artifact.CodeWatchFilesystemUnsupported || !strings.Contains(typed.Message, "--poll") {
		t.Fatalf("Start on %s = %v, want watch_filesystem_unsupported", unc, err)
	}
	_ = b.Close()
}

// TestRdcwAcceptsAFailedProbe watches a subst drive, on which
// GetVolumePathName fails (F13): an unrecognized volume is accepted (A6).
// It changes the session's drive mappings, so it runs on CI only.
func TestRdcwAcceptsAFailedProbe(t *testing.T) {
	if os.Getenv("GITHUB_ACTIONS") != "true" {
		t.Skip("subst changes the session's drive mappings: CI only")
	}
	letter := ""
	for c := 'Z'; c >= 'G'; c-- {
		if _, err := os.Stat(string(c) + `:\`); err != nil {
			letter = string(c) + ":"
			break
		}
	}
	if letter == "" {
		t.Skip("no free drive letter")
	}
	dir := t.TempDir()
	if out, err := exec.Command("subst", letter, dir).CombinedOutput(); err != nil {
		t.Fatalf("subst: %v: %s", err, out)
	}
	t.Cleanup(func() { _ = exec.Command("subst", letter, "/d").Run() })
	root := letter + `\w`
	mkdirs(t, root)
	if name := refusedVolume(root); name != "" {
		t.Fatalf("a subst drive refused as %s", name)
	}
	b, rec := startRdcw(t, root, snapshot.CaptureV1{}, nil)
	reconcile(t, b, root)
	write(t, filepath.Join(root, "sentinel"), "")
	rec.sawRelevant(t, "sentinel")
}

func TestRdcwLimitMessages(t *testing.T) {
	for _, err := range []error{
		openFailure(windows.ERROR_NO_SYSTEM_RESOURCES),
		openFailure(windows.ERROR_TOO_MANY_OPEN_FILES),
		rootReadFailure(windows.ERROR_NOT_ENOUGH_QUOTA),
	} {
		var typed *artifact.Error
		if !errors.As(err, &typed) || typed.Code != artifact.CodeWatchLimitReached || !strings.Contains(typed.Message, "--poll") {
			t.Errorf("limit error %v", err)
		}
	}
}
