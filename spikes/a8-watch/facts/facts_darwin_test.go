package facts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"dirloom.local/spikes/a8watch/notify"
)

// TestF7DirectoryWrite checks which operations fire NOTE_WRITE on a directory.
func TestF7DirectoryWrite(t *testing.T) {
	root := t.TempDir()
	k, err := notify.NewKqueue(root, notify.StructuralFflags)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	f := filepath.Join(root, "a.txt")
	steps := []struct {
		name       string
		structural bool
		do         func() error
	}{
		{"create file", true, func() error { return os.WriteFile(f, []byte("x"), 0o644) }},
		{"append content", false, func() error {
			h, err := os.OpenFile(f, os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				return err
			}
			_, _ = h.WriteString("more")
			return h.Close()
		}},
		{"rewrite in place", false, func() error { return os.WriteFile(f, []byte("rewritten"), 0o644) }},
		{"chmod", false, func() error { return os.Chmod(f, 0o600) }},
		{"touch", false, func() error { return os.Chtimes(f, time.Now(), time.Now()) }},
		{"rename file", true, func() error { return os.Rename(f, filepath.Join(root, "b.txt")) }},
		{"mkdir", true, func() error { return os.Mkdir(filepath.Join(root, "sub"), 0o755) }},
		{"remove file", true, func() error { return os.Remove(filepath.Join(root, "b.txt")) }},
	}
	ok := true
	var parts []string
	for _, s := range steps {
		if err := s.do(); err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		notes := drain(t, k)
		fired := notify.Count(notes, notify.OpDirWrite) > 0
		if fired != s.structural {
			ok = false
		}
		parts = append(parts, fmt.Sprintf("%s → NOTE_WRITE=%v", s.name, fired))
	}
	gi := filepath.Join(root, ".gitignore")
	write(t, gi, "*.log\n")
	_ = drain(t, k)
	giErr := k.WatchFile(gi)
	h, _ := os.OpenFile(gi, os.O_APPEND|os.O_WRONLY, 0)
	_, _ = h.WriteString("tmp/\n")
	_ = h.Close()
	giNotes := drain(t, k)
	evidence(t, "F7", verdict(ok), "%s; .gitignore descriptor (err=%v), append → [%s]", strings.Join(parts, "; "), giErr, summarize(root, giNotes, 3))
}

// TestF8DescriptorLimit opens O_EVTONLY descriptors until the system refuses.
func TestF8DescriptorLimit(t *testing.T) {
	var lim unix.Rlimit
	_ = unix.Getrlimit(unix.RLIMIT_NOFILE, &lim)
	perProc, _ := unix.SysctlUint32("kern.maxfilesperproc")
	total, _ := unix.SysctlUint32("kern.maxfiles")
	dir := t.TempDir()
	var fds []int
	var refusal error
	maxOpen := int(lim.Cur) + 64
	if maxOpen > 300000 {
		maxOpen = 300000
	}
	for i := 0; i < maxOpen; i++ {
		fd, err := unix.Open(dir, unix.O_EVTONLY|unix.O_CLOEXEC, 0)
		if err != nil {
			refusal = err
			break
		}
		fds = append(fds, fd)
	}
	for _, fd := range fds {
		_ = unix.Close(fd)
	}
	evidence(t, "F8", "INFO", "RLIMIT_NOFILE after Go startup: cur=%d max=%d; kern.maxfilesperproc=%d kern.maxfiles=%d; %d O_EVTONLY descriptors opened, then %v",
		lim.Cur, lim.Max, perProc, total, len(fds), refusal)
}

// TestF13Classify classifies local paths.
func TestF13Classify(t *testing.T) {
	var parts []string
	for _, p := range []string{t.TempDir(), "/dev", "/System/Volumes/Data"} {
		name, refused, err := notify.Classify(p)
		parts = append(parts, fmt.Sprintf("%s → %s refused=%v err=%v", p, name, refused, err))
	}
	evidence(t, "F13", "INFO", "%s; no network share available on the runner", strings.Join(parts, "; "))
}

// TestF16DepthZeroControl checks root-loss detection with the control filter.
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
	removed, removeLat, _ := waitFor(c, 3*time.Second, func(n notify.Note) bool { return n.Op == notify.OpSelfDelete })
	_ = c.Close()

	mkdirs(t, root)
	c = start()
	ancErr := os.Rename(filepath.Join(base, "anc"), filepath.Join(base, "anc2"))
	ancNotes := drain(t, c)
	_, statErr := os.Stat(root)
	_ = c.Close()
	ok := renamed && replaced && identityChanged && removed
	evidence(t, "F16", verdict(ok), "control filter only: root rename reported=%v in %s; replacement reported=%v, identity changed=%v; removal reported=%v in %s; ancestor rename (err=%v) → notes [%s], stat of the old root path err=%v (identity check detects it)",
		renamed, ms(renameLat), replaced, identityChanged, removed, ms(removeLat), ancErr, summarize(base, ancNotes, 3), statErr)
}

// TestF17SubMounts attaches a disk image below the root.
func TestF17SubMounts(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	vol := filepath.Join(root, "vol")
	early := filepath.Join(root, "early")
	mkdirs(t, vol, early)
	img := filepath.Join(base, "a8.dmg")
	run := func(args ...string) error {
		out, err := exec.Command("hdiutil", args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	if err := run("create", "-size", "20m", "-fs", "APFS", "-volname", "a8", img); err != nil {
		evidence(t, "F17", "NOT-TESTED", "hdiutil create: %v", err)
		return
	}
	k, err := notify.NewKqueue(root, notify.StructuralFflags)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	if err := k.Watch(early); err != nil {
		t.Fatal(err)
	}
	if err := run("attach", "-nobrowse", "-mountpoint", vol, img); err != nil {
		evidence(t, "F17", "NOT-TESTED", "hdiutil attach: %v", err)
		return
	}
	defer run("detach", vol, "-force")
	_ = drain(t, k)
	if err := k.Watch(vol); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(vol, "in-image.txt"), "x")
	inVol := drain(t, k)
	name, _, _ := notify.Classify(vol)
	img2 := filepath.Join(base, "a8b.dmg")
	earlyNotes := []notify.Note(nil)
	if err := run("create", "-size", "20m", "-fs", "APFS", "-volname", "a8b", img2); err == nil {
		if err := run("attach", "-nobrowse", "-mountpoint", early, img2); err == nil {
			defer run("detach", early, "-force")
			_ = drain(t, k)
			write(t, filepath.Join(early, "under-early-watch.txt"), "x")
			earlyNotes = drain(t, k)
		}
	}
	evidence(t, "F17", "INFO", "disk image (%s) attached below the root, registered after attaching: change → [%s]; image attached over an already registered directory: change → [%s]",
		name, summarize(root, inVol, 3), summarize(root, earlyNotes, 3))
}
