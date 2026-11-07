package backend

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/snapshot"
)

func (r *recorder) trace(ev inotifyEvent, relevant bool) {
	r.record(ev.name, ev.mask, relevant)
}

func startInotify(t *testing.T, root string, capture snapshot.CaptureV1, gate chan struct{}) (*inotify, *recorder) {
	t.Helper()
	rec := newRecorder()
	b := newNative(Options{Root: root, Capture: capture}).(*inotify)
	b.trace, b.gate = rec.trace, gate
	if err := b.Start(rec); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b, rec
}

func reconcile(t *testing.T, b *inotify, root string) bool {
	t.Helper()
	changed, err := b.Reconcile(observe(t, root))
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	return changed
}

func registeredPaths(b *inotify) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []string
	for p := range b.byPath {
		out = append(out, string(p))
	}
	sort.Strings(out)
	return out
}

func TestInotifyReportsEntriesAndIgnoresContent(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, filepath.Join(root, "a"))
	write(t, filepath.Join(root, "a", "f"), "x")
	b, rec := startInotify(t, root, snapshot.CaptureV1{UseGitignore: true}, nil)
	if !reconcile(t, b, root) {
		t.Fatal("registering a new directory must schedule a verification")
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
	if got := registeredPaths(b); strings.Join(got, ",") != ".,a" {
		t.Fatalf("registrations %v", got)
	}
}

func TestInotifyGitignoreWrites(t *testing.T) {
	for _, useGitignore := range []bool{true, false} {
		t.Run(strconv.FormatBool(useGitignore), func(t *testing.T) {
			root := t.TempDir()
			write(t, filepath.Join(root, ".gitignore"), "*.log\n")
			_, rec := startInotify(t, root, snapshot.CaptureV1{UseGitignore: useGitignore}, nil)
			appendTo(t, filepath.Join(root, ".gitignore"))
			write(t, filepath.Join(root, "sentinel"), "")
			rec.sawRelevant(t, "sentinel")
			if got := rec.relevantFor(".gitignore"); got != useGitignore {
				t.Fatalf(".gitignore write relevant = %v, want %v", got, useGitignore)
			}
		})
	}
}

// A panic of the reader goroutine ends the watch with a broken invariant
// instead of crashing the process.
func TestInotifyReaderPanicFailsTheWatch(t *testing.T) {
	root := t.TempDir()
	gate := make(chan struct{})
	b, rec := startInotify(t, root, snapshot.CaptureV1{}, gate)
	b.trace = func(inotifyEvent, bool) { panic("reader bug") }
	close(gate)
	reconcile(t, b, root)
	write(t, filepath.Join(root, "f"), "")
	readerPanics(t, rec, b)
}

func TestInotifyRootLossFails(t *testing.T) {
	for _, tc := range []struct {
		name  string
		depth *int
		lose  func(root string) error
	}{
		{"removed", nil, os.RemoveAll},
		{"renamed", nil, func(root string) error { return os.Rename(root, root+"-moved") }},
		{"depth 0 removed", new(int), os.RemoveAll},
		{"depth 0 renamed", new(int), func(root string) error { return os.Rename(root, root+"-moved") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "root")
			mkdirs(t, filepath.Join(root, "a"))
			b, rec := startInotify(t, root, snapshot.CaptureV1{Depth: tc.depth}, nil)
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

func TestInotifyRecreatedDirectoryIsRegisteredAgain(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a")
	mkdirs(t, a)
	b, rec := startInotify(t, root, snapshot.CaptureV1{}, nil)
	reconcile(t, b, root)
	if err := os.Remove(a); err != nil {
		t.Fatal(err)
	}
	mkdirs(t, a)
	rec.sawRelevant(t, "a")
	rec.waitFor(t, "the dropped registration", func(r *recorder) bool {
		for _, ev := range r.events {
			if ev.detail&unix.IN_IGNORED != 0 {
				return true
			}
		}
		return false
	})
	if !reconcile(t, b, root) {
		t.Fatal("a recreated directory must be registered again")
	}
	write(t, filepath.Join(a, "x"), "")
	rec.sawRelevant(t, "x")
}

func TestInotifyReconcilesMovedAndReplacedDirectories(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, filepath.Join(root, "a"), filepath.Join(root, "b"))
	b, rec := startInotify(t, root, snapshot.CaptureV1{}, nil)
	reconcile(t, b, root)
	if err := os.Rename(filepath.Join(root, "a"), filepath.Join(root, "a-old")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "b"), filepath.Join(root, "a")); err != nil {
		t.Fatal(err)
	}
	if !reconcile(t, b, root) {
		t.Fatal("moved and replaced directories must be registered again")
	}
	if got := registeredPaths(b); strings.Join(got, ",") != ".,a,a-old" {
		t.Fatalf("registrations %v", got)
	}
	b.mu.Lock()
	for _, p := range []artifact.Path{"a", "a-old"} {
		dev, ino, err := directoryID(filepath.Join(root, string(p)))
		if reg := b.byPath[p]; err != nil || reg.dev != dev || reg.ino != ino {
			b.mu.Unlock()
			t.Fatalf("registration of %s is keyed to another directory", p)
		}
	}
	b.mu.Unlock()
	write(t, filepath.Join(root, "a", "in-a"), "")
	write(t, filepath.Join(root, "a-old", "in-a-old"), "")
	rec.sawRelevant(t, "in-a")
	rec.sawRelevant(t, "in-a-old")
}

func TestInotifyOverflowIsSignalled(t *testing.T) {
	data, err := os.ReadFile("/proc/sys/fs/inotify/max_queued_events")
	if err != nil {
		t.Fatal(err)
	}
	queue, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	root := t.TempDir()
	gate := make(chan struct{})
	_, rec := startInotify(t, root, snapshot.CaptureV1{}, gate)
	released := false
	t.Cleanup(func() { // runs before Close, which waits for the reader
		if !released {
			close(gate)
		}
	})
	for i := 0; i <= queue; i++ {
		f, err := os.Create(filepath.Join(root, "f"+strconv.Itoa(i)))
		if err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
	}
	close(gate)
	released = true
	rec.waitFor(t, "an overflow", func(r *recorder) bool { return r.overflows > 0 })
}

func TestInotifyNeverRegistersALink(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, filepath.Join(root, "real"))
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	b, _ := startInotify(t, root, snapshot.CaptureV1{}, nil)
	forged := observe(t, root)
	for i := range forged.Records {
		if forged.Records[i].Path == "link" {
			forged.Records[i].Kind = artifact.KindDirectory // as if a directory became a link meanwhile
		}
	}
	changed, err := b.Reconcile(forged)
	if err != nil || !changed {
		t.Fatalf("Reconcile = %v, %v; want a scheduled verification", changed, err)
	}
	if got := registeredPaths(b); strings.Join(got, ",") != ".,real" {
		t.Fatalf("registrations %v: a link was registered", got)
	}
}

func openDescriptors(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}

func TestInotifyCloseReleasesEverything(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 10; i++ {
		mkdirs(t, filepath.Join(root, "d"+strconv.Itoa(i)))
	}
	before := openDescriptors(t)
	rec := newRecorder()
	b := newNative(Options{Root: root}).(*inotify)
	if err := b.Start(rec); err != nil {
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
	if after := openDescriptors(t); after != before {
		t.Fatalf("descriptors %d before, %d after Close", before, after)
	}
}

func TestInotifyStartsOnLocalFilesystems(t *testing.T) {
	for _, dir := range []string{t.TempDir(), "/dev/shm"} {
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		if name := refusedFilesystem(dir); name != "" {
			t.Fatalf("%s refused as %s", dir, name)
		}
	}
	missing := filepath.Join(t.TempDir(), "missing")
	b := newNative(Options{Root: missing}).(*inotify)
	if err := b.Start(newRecorder()); !rootUnavailable(err) {
		t.Fatalf("Start on a missing root = %v", err)
	}
	_ = b.Close()
}

func TestLinuxRefusedTypesMatchTheKernelConstants(t *testing.T) {
	for magic, name := range map[int64]string{
		unix.NFS_SUPER_MAGIC:  "nfs",
		unix.SMB_SUPER_MAGIC:  "smb",
		unix.SMB2_SUPER_MAGIC: "smb2",
		unix.CIFS_SUPER_MAGIC: "cifs",
		unix.V9FS_MAGIC:       "9p",
		unix.AFS_SUPER_MAGIC:  "afs",
		unix.CEPH_SUPER_MAGIC: "ceph",
		unix.CODA_SUPER_MAGIC: "coda",
	} {
		if got := classifyLinux(magic); got != name {
			t.Errorf("classifyLinux(%#x) = %q, want %q", magic, got, name)
		}
	}
	if len(linuxRefusedTypes) != 8 {
		t.Errorf("%d refused types, want 8", len(linuxRefusedTypes))
	}
}

func TestInotifyLimitMessages(t *testing.T) {
	for _, tc := range []struct {
		err     error
		setting string
	}{
		{addFailure(unix.ENOSPC, false), "fs.inotify.max_user_watches"},
		{initFailure(unix.EMFILE), "fs.inotify.max_user_instances"},
	} {
		var typed *artifact.Error
		if !errors.As(tc.err, &typed) || typed.Code != artifact.CodeWatchLimitReached ||
			!strings.Contains(typed.Message, tc.setting) || !strings.Contains(typed.Message, "--poll") {
			t.Errorf("limit error %v", tc.err)
		}
	}
}
