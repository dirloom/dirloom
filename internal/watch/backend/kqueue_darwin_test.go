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

func startKqueue(t *testing.T, root string, capture snapshot.CaptureV1, gate chan struct{}) (*kqueue, *recorder) {
	t.Helper()
	rec := newRecorder()
	b := newNative(Options{Root: root, Capture: capture}).(*kqueue)
	b.trace, b.gate = rec.record, gate
	if err := b.Start(rec); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b, rec
}

func reconcile(t *testing.T, b *kqueue, root string) bool {
	t.Helper()
	changed, err := b.Reconcile(observe(t, root))
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	return changed
}

// registered lists the names of the directory and .gitignore registrations.
func registered(b *kqueue) (dirs, ignores string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var d, i []string
	for _, reg := range b.dirs {
		d = append(d, reg.name())
	}
	for _, reg := range b.ignores {
		i = append(i, reg.name())
	}
	sort.Strings(d)
	sort.Strings(i)
	return strings.Join(d, ","), strings.Join(i, ",")
}

// sawNotes waits until an event of the registration named name carried one
// of notes.
func (r *recorder) sawNotes(t *testing.T, name string, notes uint32) {
	t.Helper()
	r.waitFor(t, "an event "+strconv.FormatUint(uint64(notes), 16)+" for "+name, func(r *recorder) bool {
		return r.countNotes(name, notes) > 0
	})
}

// countNotes counts the events of name that carried one of notes; r.mu is
// held.
func (r *recorder) countNotes(name string, notes uint32) int {
	n := 0
	for _, ev := range r.events {
		if ev.name == name && ev.detail&notes != 0 {
			n++
		}
	}
	return n
}

func TestKqueueReportsEntriesAndIgnoresContent(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, filepath.Join(root, "a"))
	write(t, filepath.Join(root, "a", "f"), "x")
	b, rec := startKqueue(t, root, snapshot.CaptureV1{UseGitignore: true}, nil)
	if !reconcile(t, b, root) {
		t.Fatal("registering a new directory must schedule a verification")
	}
	if reconcile(t, b, root) {
		t.Fatal("an unchanged structure must not schedule a verification")
	}
	appendTo(t, filepath.Join(root, "a", "f"))
	write(t, filepath.Join(root, "a", "f"), "rewritten")
	write(t, filepath.Join(root, "sentinel"), "")
	rec.sawRelevant(t, ".")
	if rec.relevantFor("a") {
		t.Fatal("a content write fired its directory's descriptor")
	}
	write(t, filepath.Join(root, "a", "x"), "")
	rec.sawNotes(t, "a", unix.NOTE_WRITE)
	if dirs, ignores := registered(b); dirs != ".,a" || ignores != "" {
		t.Fatalf("registrations %q, %q", dirs, ignores)
	}
}

func TestKqueueNeverRegistersExcludedOrDeeperDirectories(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, filepath.Join(root, "keep", "deeper"), filepath.Join(root, "skip"))
	b, _ := startKqueue(t, root, snapshot.CaptureV1{}, nil)
	obs := observe(t, root)
	kept := obs.Records[:0]
	for _, r := range obs.Records {
		if r.Path != "skip" {
			kept = append(kept, r)
		}
	}
	obs.Records = kept
	if _, err := b.Reconcile(obs); err != nil {
		t.Fatal(err)
	}
	if dirs, _ := registered(b); dirs != ".,keep,keep/deeper" {
		t.Fatalf("registrations %q: an excluded directory was registered", dirs)
	}

	depth := 1
	b, _ = startKqueue(t, root, snapshot.CaptureV1{Depth: &depth}, nil)
	reconcile(t, b, root)
	if dirs, _ := registered(b); dirs != "." {
		t.Fatalf("registrations %q beyond --depth 1", dirs)
	}
}

func TestKqueueGitignoreDescriptors(t *testing.T) {
	for _, useGitignore := range []bool{true, false} {
		t.Run(strconv.FormatBool(useGitignore), func(t *testing.T) {
			root := t.TempDir()
			mkdirs(t, filepath.Join(root, "sub"))
			write(t, filepath.Join(root, ".gitignore"), "*.log\n")
			write(t, filepath.Join(root, "sub", ".gitignore"), "")
			b, rec := startKqueue(t, root, snapshot.CaptureV1{UseGitignore: useGitignore}, nil)
			reconcile(t, b, root)
			want := ""
			if useGitignore {
				want = ".gitignore,sub/.gitignore"
			}
			if _, ignores := registered(b); ignores != want {
				t.Fatalf(".gitignore registrations %q, want %q", ignores, want)
			}
			appendTo(t, filepath.Join(root, ".gitignore"))
			appendTo(t, filepath.Join(root, "sub", ".gitignore"))
			write(t, filepath.Join(root, "sentinel"), "")
			rec.sawRelevant(t, ".")
			for _, name := range []string{".gitignore", "sub/.gitignore"} {
				if got := rec.relevantFor(name); got != useGitignore {
					t.Fatalf("%s write relevant = %v, want %v", name, got, useGitignore)
				}
			}
			if !useGitignore {
				return
			}
			// An editor's atomic save replaces the file: the old descriptor is
			// dropped and the new file registered at the next reconciliation.
			write(t, filepath.Join(root, ".gitignore.tmp"), "*.tmp\n")
			if err := os.Rename(filepath.Join(root, ".gitignore.tmp"), filepath.Join(root, ".gitignore")); err != nil {
				t.Fatal(err)
			}
			rec.sawNotes(t, ".gitignore", lossNotes)
			if !reconcile(t, b, root) {
				t.Fatal("a replaced .gitignore must be registered again")
			}
			rec.mu.Lock()
			before := rec.countNotes(".gitignore", unix.NOTE_WRITE|unix.NOTE_EXTEND)
			rec.mu.Unlock()
			appendTo(t, filepath.Join(root, ".gitignore"))
			rec.waitFor(t, "a write to the new .gitignore", func(r *recorder) bool {
				return r.countNotes(".gitignore", unix.NOTE_WRITE|unix.NOTE_EXTEND) > before
			})
		})
	}
}

func TestKqueueRootLossFails(t *testing.T) {
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
			b, rec := startKqueue(t, root, snapshot.CaptureV1{Depth: tc.depth}, nil)
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
			rec.mu.Lock()
			writes := rec.countNotes(".", unix.NOTE_WRITE)
			rec.mu.Unlock()
			if tc.depth != nil && writes > 0 {
				t.Fatal("a change below --depth 0 was signalled")
			}
		})
	}
}

func TestKqueueRecreatedDirectoryIsRegisteredAgain(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a")
	mkdirs(t, a)
	b, rec := startKqueue(t, root, snapshot.CaptureV1{}, nil)
	reconcile(t, b, root)
	if err := os.Remove(a); err != nil {
		t.Fatal(err)
	}
	mkdirs(t, a)
	rec.sawNotes(t, "a", unix.NOTE_DELETE)
	if !reconcile(t, b, root) {
		t.Fatal("a recreated directory must be registered again")
	}
	write(t, filepath.Join(a, "x"), "")
	rec.sawNotes(t, "a", unix.NOTE_WRITE)
}

func TestKqueueReconcilesMovedAndReplacedDirectories(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, filepath.Join(root, "a"), filepath.Join(root, "b"))
	b, rec := startKqueue(t, root, snapshot.CaptureV1{}, nil)
	reconcile(t, b, root)
	if err := os.Rename(filepath.Join(root, "a"), filepath.Join(root, "a-old")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "b"), filepath.Join(root, "a")); err != nil {
		t.Fatal(err)
	}
	rec.sawNotes(t, "a", unix.NOTE_RENAME)
	rec.sawNotes(t, "b", unix.NOTE_RENAME)
	if !reconcile(t, b, root) {
		t.Fatal("moved and replaced directories must be registered again")
	}
	if dirs, _ := registered(b); dirs != ".,a,a-old" {
		t.Fatalf("registrations %q", dirs)
	}
	b.mu.Lock()
	for _, p := range []artifact.Path{"a", "a-old"} {
		dev, ino, err := entryID(filepath.Join(root, string(p)), false)
		if reg := b.dirs[p]; err != nil || reg.dev != dev || reg.ino != ino {
			b.mu.Unlock()
			t.Fatalf("registration of %s is keyed to another directory", p)
		}
	}
	b.mu.Unlock()
	write(t, filepath.Join(root, "a", "in-a"), "")
	write(t, filepath.Join(root, "a-old", "in-a-old"), "")
	rec.sawNotes(t, "a", unix.NOTE_WRITE)
	rec.sawNotes(t, "a-old", unix.NOTE_WRITE)
}

func TestKqueueNeverRegistersALink(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, filepath.Join(root, "real"))
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	b, _ := startKqueue(t, root, snapshot.CaptureV1{}, nil)
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
	if dirs, _ := registered(b); dirs != ".,real" {
		t.Fatalf("registrations %q: a link was registered", dirs)
	}
}

// openDescriptors counts this process's descriptors. Listing /dev/fd works
// where stat-ing its entries does not.
func openDescriptors(t *testing.T) int {
	t.Helper()
	f, err := os.Open("/dev/fd")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	names, err := f.Readdirnames(-1)
	if err != nil {
		t.Fatal(err)
	}
	return len(names)
}

func TestKqueueCloseReleasesEverything(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 10; i++ {
		mkdirs(t, filepath.Join(root, "d"+strconv.Itoa(i)))
		write(t, filepath.Join(root, "d"+strconv.Itoa(i), ".gitignore"), "")
	}
	before := openDescriptors(t)
	b := newNative(Options{Root: root, Capture: snapshot.CaptureV1{UseGitignore: true}}).(*kqueue)
	if err := b.Start(newRecorder()); err != nil {
		t.Fatal(err)
	}
	reconcile(t, b, root)
	if during := openDescriptors(t); during < before+21 {
		t.Fatalf("descriptors %d before, %d while watching 11 directories and 10 .gitignore files", before, during)
	}
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
	if err := newNative(Options{Root: root}).Close(); err != nil {
		t.Fatalf("Close before Start: %v", err)
	}
}

func TestKqueueStartsOnLocalFilesystems(t *testing.T) {
	for _, dir := range []string{t.TempDir(), "/dev"} {
		if name := refusedFilesystem(dir); name != "" {
			t.Fatalf("%s refused as %s", dir, name)
		}
	}
	missing := filepath.Join(t.TempDir(), "missing")
	b := newNative(Options{Root: missing}).(*kqueue)
	if err := b.Start(newRecorder()); !rootUnavailable(err) {
		t.Fatalf("Start on a missing root = %v", err)
	}
	_ = b.Close()
}

// TestKqueueDescriptorLimit is A4 through a lowered limit: registering more
// directories than the process may open is watch_limit_reached.
func TestKqueueDescriptorLimit(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 64; i++ {
		mkdirs(t, filepath.Join(root, "d"+strconv.Itoa(i)))
	}
	b, _ := startKqueue(t, root, snapshot.CaptureV1{}, nil)
	obs := observe(t, root)
	var limit unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_NOFILE, &limit); err != nil {
		t.Fatal(err)
	}
	lowered := limit
	lowered.Cur = uint64(openDescriptors(t) + 16) //nolint:gosec // G115: a descriptor count is small and non-negative.
	if err := unix.Setrlimit(unix.RLIMIT_NOFILE, &lowered); err != nil {
		t.Fatal(err)
	}
	_, err := b.Reconcile(obs)
	if restore := unix.Setrlimit(unix.RLIMIT_NOFILE, &limit); restore != nil {
		t.Fatalf("restoring the descriptor limit: %v", restore)
	}
	var typed *artifact.Error
	if !errors.As(err, &typed) || typed.Code != artifact.CodeWatchLimitReached ||
		!strings.Contains(typed.Message, "kern.maxfilesperproc") || !strings.Contains(typed.Message, "--poll") {
		t.Fatalf("Reconcile beyond the descriptor limit = %v, want watch_limit_reached", err)
	}
}
