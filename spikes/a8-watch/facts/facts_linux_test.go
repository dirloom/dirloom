package facts

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"dirloom.local/spikes/a8watch/notify"
)

func procInt(t *testing.T, name string) int {
	t.Helper()
	data, err := os.ReadFile("/proc/sys/fs/inotify/" + name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	v, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return v
}

// TestF5Overflow fills the inotify queue without reading.
func TestF5Overflow(t *testing.T) {
	q := procInt(t, "max_queued_events")
	root := t.TempDir()
	w, err := notify.NewInotify(root, notify.StructuralMask)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	files := q + 2000
	if files > 300000 {
		files = 300000
	}
	for i := 0; i < files; i++ {
		f, err := os.Create(filepath.Join(root, "f"+strconv.Itoa(i)))
		if err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
	}
	notes := drain(t, w)
	overflow := notify.Count(notes, notify.OpOverflow)
	evidence(t, "F5", verdict(overflow > 0), "IN_Q_OVERFLOW: max_queued_events=%d, %d files created (create + close-write each) without reading → %d notes, overflow notes=%d",
		q, files, len(notes), overflow)
}

// TestF5IgnoredAfterSelf checks IN_IGNORED after self deletion and self move.
func TestF5IgnoredAfterSelf(t *testing.T) {
	root := t.TempDir()
	d := filepath.Join(root, "d")
	e := filepath.Join(root, "e")
	mkdirs(t, d, e)
	w, err := notify.NewInotify(root, notify.StructuralMask)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if err := w.Watch(d); err != nil {
		t.Fatal(err)
	}
	if err := w.Watch(e); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(d)
	delNotes := drain(t, w)
	_ = os.Rename(e, filepath.Join(root, "e2"))
	moveNotes := drain(t, w)
	opsFor := func(notes []notify.Note, path string) []string {
		var ops []string
		for _, n := range notes {
			if n.Path == path {
				ops = append(ops, string(n.Op))
			}
		}
		return ops
	}
	delOps, moveOps := opsFor(delNotes, d), opsFor(moveNotes, e)
	delOK := strings.Contains(strings.Join(delOps, ","), "self-delete") && strings.Contains(strings.Join(delOps, ","), "ignored")
	moveIgnored := strings.Contains(strings.Join(moveOps, ","), "ignored")
	v := "PARTIAL"
	if delOK && moveIgnored {
		v = "TRUE"
	} else if !delOK {
		v = "FALSE"
	}
	evidence(t, "F5", v, "IN_IGNORED: removed directory's watch → %v; moved directory's watch → %v (a move keeps the watch on the inode, now at the new path)", delOps, moveOps)
}

// TestF5WatchLimit adds watches until the kernel refuses (run with a lowered
// fs.inotify.max_user_watches by the workflow).
func TestF5WatchLimit(t *testing.T) {
	limit := procInt(t, "max_user_watches")
	if os.Getenv("A8_LOW_WATCHES") == "" {
		evidence(t, "F5", "INFO", "max_user_watches=%d: limit not exercised in this step (see the lowered-limit step)", limit)
		return
	}
	root := t.TempDir()
	w, err := notify.NewInotify(root, notify.StructuralMask)
	if err != nil {
		evidence(t, "F5", "INFO", "inotify_init1 failed at max_user_watches=%d: %v", limit, err)
		return
	}
	defer w.Close()
	added := 1
	var refusal error
	for i := 0; i < limit+50; i++ {
		d := filepath.Join(root, "d"+strconv.Itoa(i))
		mkdirs(t, d)
		if err := w.Watch(d); err != nil {
			refusal = err
			break
		}
		added++
	}
	evidence(t, "F5", verdict(errors.Is(refusal, syscall.ENOSPC)), "max_user_watches=%d: %d watches added by this process, then %v", limit, added, refusal)
}

// TestF5InstanceLimit opens inotify instances until the kernel refuses.
func TestF5InstanceLimit(t *testing.T) {
	limit := procInt(t, "max_user_instances")
	var fds []int
	var refusal error
	for i := 0; i < 100000; i++ {
		fd, err := unix.InotifyInit1(unix.IN_CLOEXEC)
		if err != nil {
			refusal = err
			break
		}
		fds = append(fds, fd)
	}
	for _, fd := range fds {
		_ = unix.Close(fd)
	}
	evidence(t, "F5", "INFO", "max_user_instances=%d: %d instances opened, then %v", limit, len(fds), refusal)
}

// TestF6Limits records the runner's inotify settings.
func TestF6Limits(t *testing.T) {
	var uts unix.Utsname
	_ = unix.Uname(&uts)
	release := strings.TrimRight(string(uts.Release[:]), "\x00")
	evidence(t, "F6", "INFO", "kernel %s: max_user_watches=%d, max_queued_events=%d, max_user_instances=%d",
		release, procInt(t, "max_user_watches"), procInt(t, "max_queued_events"), procInt(t, "max_user_instances"))
}

// TestF13Classify classifies local and, when mounted, NFS paths.
func TestF13Classify(t *testing.T) {
	var parts []string
	for _, p := range []string{t.TempDir(), "/dev/shm", "/proc", "/sys"} {
		desc, err := notify.FSType(p)
		_, refused, _ := notify.Classify(p)
		parts = append(parts, fmt.Sprintf("%s → %s refused=%v err=%v", p, desc, refused, err))
	}
	mount, export := os.Getenv("A8_NFS_MOUNT"), os.Getenv("A8_NFS_EXPORT")
	if mount == "" {
		evidence(t, "F13", "INFO", "%s; NFS loopback not mounted in this run", strings.Join(parts, "; "))
		return
	}
	desc, err := notify.FSType(mount)
	_, refused, _ := notify.Classify(mount)
	parts = append(parts, fmt.Sprintf("%s → %s refused=%v err=%v", mount, desc, refused, err))
	dir := filepath.Join(mount, "watch-"+strconv.FormatInt(time.Now().UnixNano(), 10))
	mkdirs(t, dir)
	w, err := notify.NewInotify(dir, notify.StructuralMask)
	if err != nil {
		t.Fatalf("inotify on nfs: %v", err)
	}
	defer w.Close()
	write(t, filepath.Join(dir, "through-mount.txt"), "x")
	viaMount := drain(t, w)
	serverSide := filepath.Join(export, filepath.Base(dir), "server-side.txt")
	serverErr := os.WriteFile(serverSide, []byte("x"), 0o644)
	viaServer := drain(t, w)
	entries, _ := os.ReadDir(dir)
	evidence(t, "F13", "INFO", "%s; inotify on the NFS mount: change through the mount → [%s]; change on the server side (err=%v) → [%s]; a re-observation lists %d entries",
		strings.Join(parts, "; "), summarize(dir, viaMount, 3), serverErr, summarize(dir, viaServer, 3), len(entries))
}

// TestF16DepthZeroControl checks root-loss detection with the control mask.
func TestF16DepthZeroControl(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "anc", "root")
	mkdirs(t, root)
	start := func() notify.Notifier {
		n, err := notify.NewControl(root)
		if err != nil {
			t.Fatalf("control: %v", err)
		}
		return n
	}
	self := func(n notify.Note) bool { return n.Op == notify.OpSelfMove || n.Op == notify.OpSelfDelete }

	c := start()
	_ = os.Rename(root, root+"-renamed")
	renamed, renameLat, _ := waitFor(c, 3*time.Second, self)
	_ = c.Close()
	_ = os.Rename(root+"-renamed", root)

	c = start()
	info, _ := os.Stat(root)
	_ = os.Rename(root, root+"-old")
	mkdirs(t, root)
	replaced, _, _ := waitFor(c, 3*time.Second, self)
	now, _ := os.Stat(root)
	identityChanged := !os.SameFile(info, now)
	_ = c.Close()

	c = start()
	_ = os.RemoveAll(root)
	removed, removeLat, removeNotes := waitFor(c, 3*time.Second, func(n notify.Note) bool { return n.Op == notify.OpSelfDelete })
	_ = c.Close()

	mkdirs(t, root)
	c = start()
	info2, _ := os.Stat(root)
	ancErr := os.Rename(filepath.Join(base, "anc"), filepath.Join(base, "anc2"))
	ancNotes := drain(t, c)
	_, statErr := os.Stat(root)
	_ = c.Close()
	_ = info2
	ok := renamed && replaced && identityChanged && removed
	evidence(t, "F16", verdict(ok), "control mask only: root rename reported=%v in %s; replacement reported=%v, identity changed=%v; removal reported=%v in %s [%s]; ancestor rename (err=%v) → notes [%s], stat of the old root path err=%v (identity check detects it)",
		renamed, ms(renameLat), replaced, identityChanged, removed, ms(removeLat), summarize(base, removeNotes, 3), ancErr, summarize(base, ancNotes, 3), statErr)
}

// TestF17SubMounts mounts a tmpfs and a bind mount below the root (sudo).
func TestF17SubMounts(t *testing.T) {
	if os.Getenv("A8_SUDO") == "" {
		evidence(t, "F17", "NOT-TESTED", "Linux sub-mounts need sudo (set A8_SUDO=1)")
		return
	}
	sudo := func(args ...string) error {
		out, err := exec.Command("sudo", args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	base := t.TempDir()
	root := filepath.Join(base, "root")
	src := filepath.Join(base, "src")
	mnt := filepath.Join(root, "mnt")
	early := filepath.Join(root, "early")
	bind := filepath.Join(root, "bind")
	mkdirs(t, mnt, early, bind, src)
	w, err := notify.NewInotify(root, notify.StructuralMask)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if err := w.Watch(early); err != nil {
		t.Fatal(err)
	}
	if err := sudo("mount", "-t", "tmpfs", "tmpfs", mnt); err != nil {
		evidence(t, "F17", "NOT-TESTED", "mount tmpfs: %v", err)
		return
	}
	defer sudo("umount", mnt)
	_ = sudo("chmod", "777", mnt)
	_ = sudo("mount", "-t", "tmpfs", "tmpfs", early)
	defer sudo("umount", early)
	_ = sudo("chmod", "777", early)
	_ = sudo("mount", "--bind", src, bind)
	defer sudo("umount", bind)
	_ = drain(t, w)
	if err := w.Watch(mnt); err != nil {
		t.Fatal(err)
	}
	if err := w.Watch(bind); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(mnt, "in-tmpfs.txt"), "x")
	inMount := drain(t, w)
	write(t, filepath.Join(early, "under-early-watch.txt"), "x")
	earlyNotes := drain(t, w)
	write(t, filepath.Join(src, "via-source-path.txt"), "x")
	bindNotes := drain(t, w)
	desc, _ := notify.FSType(mnt)
	evidence(t, "F17", "INFO", "tmpfs mounted below the root (%s), registered after mounting: change → [%s]; tmpfs mounted over an already registered directory: change → [%s]; bind mount registered, change made through the source path → [%s]",
		desc, summarize(root, inMount, 3), summarize(root, earlyNotes, 3), summarize(root, bindNotes, 3))
}
