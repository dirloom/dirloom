package facts

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"dirloom.local/spikes/a8watch/notify"
)

// TestF1SharedHandle checks that the recursive handle does not lock user
// directories.
func TestF1SharedHandle(t *testing.T) {
	base := t.TempDir()
	parent := filepath.Join(base, "parent")
	root := filepath.Join(parent, "root")
	mkdirs(t, filepath.Join(root, "sub1"), filepath.Join(root, "sub2", "inner"), filepath.Join(root, "sub3"))
	write(t, filepath.Join(root, "sub2", "inner", "f.txt"), "x")
	w, err := notify.OpenRDCW(root, true, notify.StructuralFilter, 64*1024)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer w.Close()
	ok := true
	var steps []string
	try := func(name string, fn func() error) {
		if err := fn(); err != nil {
			ok = false
			steps = append(steps, name+": "+err.Error())
			return
		}
		steps = append(steps, name+": ok")
	}
	try("rename subdirectory", func() error { return os.Rename(filepath.Join(root, "sub1"), filepath.Join(root, "sub1-renamed")) })
	try("remove subtree", func() error { return os.RemoveAll(filepath.Join(root, "sub2")) })
	_ = drain(t, w)
	moved := filepath.Join(parent, "root-renamed")
	try("rename root", func() error { return os.Rename(root, moved) })
	write(t, filepath.Join(moved, "after-rename.txt"), "x")
	afterRename := drain(t, w)
	parent2 := filepath.Join(base, "parent-renamed")
	parentErr := os.Rename(parent, parent2)
	if parentErr == nil {
		parent = parent2
		moved = filepath.Join(parent, "root-renamed")
	}
	try("remove root tree", func() error {
		if err := os.RemoveAll(moved); err != nil {
			return err
		}
		if _, err := os.Stat(moved); err == nil {
			return fmt.Errorf("still present after RemoveAll")
		}
		return nil
	})
	_ = w.Close()

	// The same with the handle closed, and with fsnotify on a fresh root.
	mkdirs(t, filepath.Join(parent, "root2", "sub"))
	parentClosedErr := os.Rename(parent, filepath.Join(base, "parent-closed"))
	if parentClosedErr == nil {
		parent = filepath.Join(base, "parent-closed")
	}
	fsn, err := notify.NewFsnotify(filepath.Join(parent, "root2"), true)
	if err != nil {
		t.Fatalf("fsnotify: %v", err)
	}
	_ = fsn.Watch(filepath.Join(parent, "root2", "sub"))
	parentFsnErr := os.Rename(parent, filepath.Join(base, "parent-fsn"))
	_ = fsn.Close()
	evidence(t, "F1", verdict(ok && parentErr == nil), "%s. Rename of the root's parent: with the recursive root handle open err=%v; after closing it err=%v; with fsnotify watching a root below it err=%v. The handle follows the moved root and names its entries under the old root path: notes after writing into the moved root=[%s]",
		strings.Join(steps, "; "), parentErr, parentClosedErr, parentFsnErr, summarize(parent, afterRename, 3))
}

// TestF2Overflow provokes a buffer overflow with a small buffer.
func TestF2Overflow(t *testing.T) {
	try := func(bufSize, files int) (string, int) {
		root := t.TempDir()
		w, err := notify.OpenRDCW(root, true, notify.StructuralFilter, bufSize)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		defer w.Close()
		write(t, filepath.Join(root, "first.txt"), "x")
		first, _ := w.Wait(2 * time.Second)
		for i := 0; i < files; i++ {
			write(t, filepath.Join(root, "f"+strconv.Itoa(i)), "")
		}
		notes := drain(t, w)
		overflow := notify.Count(notes, notify.OpOverflow)
		return fmt.Sprintf("buffer %d B, %d files created while no read was pending → first read %d notes, then %d notes, overflow notes=%d",
			bufSize, files, len(first), len(notes), overflow), overflow
	}
	small, smallOverflow := try(1024, 500)
	large, _ := try(64*1024, 5000)
	evidence(t, "F2", verdict(smallOverflow > 0), "%s; %s", small, large)
}

// TestF3ParentControl checks the non-recursive handle on the root's parent.
func TestF3ParentControl(t *testing.T) {
	base := t.TempDir()
	parent := filepath.Join(base, "p")
	root := filepath.Join(parent, "root")
	mkdirs(t, filepath.Join(root, "sub"))
	control, err := notify.NewControl(root)
	if err != nil {
		t.Fatalf("control: %v", err)
	}
	defer control.Close()
	structural, err := notify.NewNative(root)
	if err != nil {
		t.Fatalf("structural: %v", err)
	}
	isRoot := func(op notify.Op) func(notify.Note) bool {
		return func(n notify.Note) bool { return n.Op == op && strings.EqualFold(filepath.Base(n.Path), "root") }
	}
	renameErr := os.Rename(root, filepath.Join(parent, "root-x"))
	renamed, renameLat, _ := waitFor(control, 3*time.Second, isRoot(notify.OpRenameFrom))
	backErr := os.Rename(filepath.Join(parent, "root-x"), root)
	_ = drain(t, control)
	parentRenameErr := os.Rename(parent, filepath.Join(base, "p2"))
	if parentRenameErr == nil {
		parent = filepath.Join(base, "p2")
		root = filepath.Join(parent, "root")
	}
	_ = drain(t, control)
	removeErr := os.RemoveAll(root)
	removedOpen, removeLat, _ := waitFor(control, 3*time.Second, isRoot(notify.OpDelete))
	_ = structural.Close()
	removedClosed := removedOpen
	closedLat := time.Duration(0)
	if !removedOpen {
		removedClosed, closedLat, _ = waitFor(control, 3*time.Second, isRoot(notify.OpDelete))
	}
	ok := renameErr == nil && renamed && backErr == nil && removeErr == nil && removedClosed
	evidence(t, "F3", verdict(ok), "root rename (err=%v) reported=%v in %s; rename parent while both handles open: err=%v; root removal (err=%v) reported while structural handle open=%v in %s, after closing it=%v in %s",
		renameErr, renamed, ms(renameLat), parentRenameErr, removeErr, removedOpen, ms(removeLat), removedClosed, ms(closedLat))
}

// TestF4Links checks what changes beneath links inside the root report.
func TestF4Links(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	outside := filepath.Join(base, "outside")
	mkdirs(t, root, outside)
	var otherVolume string
	if vol := filepath.VolumeName(base); !strings.EqualFold(vol, "D:") {
		if _, err := os.Stat(`D:\`); err == nil {
			if d, err := os.MkdirTemp(`D:\`, "a8-lot0-"); err == nil {
				otherVolume = d
				defer os.RemoveAll(d)
			}
		}
	}
	junction := func(link, target string) error {
		out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	jErr := junction(filepath.Join(root, "j"), outside)
	sErr := os.Symlink(outside, filepath.Join(root, "s"))
	j2Err := errors.New("no other volume")
	if otherVolume != "" {
		j2Err = junction(filepath.Join(root, "j2"), otherVolume)
	}
	w := newRegistered(t, root)
	defer w.Close()
	_ = drain(t, w)
	write(t, filepath.Join(outside, "via-target.txt"), "x")
	viaTarget := drain(t, w)
	write(t, filepath.Join(root, "j", "via-link.txt"), "x")
	viaLink := drain(t, w)
	var viaOther []notify.Note
	if otherVolume != "" && j2Err == nil {
		write(t, filepath.Join(otherVolume, "other-volume.txt"), "x")
		viaOther = drain(t, w)
	}
	fi, lerr := os.Lstat(filepath.Join(root, "j"))
	mode := "?"
	if lerr == nil {
		mode = fi.Mode().String()
	}
	quiet := len(viaTarget) == 0 && len(viaLink) == 0 && len(viaOther) == 0
	evidence(t, "F4", verdict(quiet), "junction err=%v, symlink err=%v, junction to other volume %q err=%v; change in target dir → [%s]; write through junction path → [%s]; change on other volume → [%s]; Lstat(junction) mode %s",
		jErr, sErr, otherVolume, j2Err, summarize(root, viaTarget, 3), summarize(root, viaLink, 3), summarize(root, viaOther, 3), mode)
	evidence(t, "F17", "INFO", "Windows: a volume mount point is a reparse point like a junction (IO_REPARSE_TAG_MOUNT_POINT); junction to another volume: err=%v, its changes → [%s]; Lstat mode %s (no VHD mounted in this run)",
		j2Err, summarize(root, viaOther, 3), mode)
}

// TestF9CtrlBreak sends CTRL_BREAK_EVENT to a child in a new process group.
func TestF9CtrlBreak(t *testing.T) {
	helper := buildHelper(t)
	cmd := exec.Command(helper, "signal-wait")
	prepareGroup(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(stdout)
	ready := sc.Scan() && sc.Text() == "READY"
	sendErr := interruptChild(cmd)
	start := time.Now()
	got := ""
	if sc.Scan() {
		got = sc.Text()
	}
	waitErr := cmd.Wait()
	delivered := strings.HasPrefix(got, "GOT")
	evidence(t, "F9", verdict(ready && sendErr == nil && delivered), "GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT) err=%v; child said %q after %s; child exit %v. CTRL_CLOSE/LOGOFF/SHUTDOWN not reproduced (best effort by contract)",
		sendErr, got, ms(time.Since(start)), waitErr)
}

// TestF13Classify classifies local, UNC and subst paths.
func TestF13Classify(t *testing.T) {
	dir := t.TempDir()
	var parts []string
	probe := func(label, path string) {
		desc, err := notify.FSType(path)
		_, remote, _ := notify.Classify(path)
		parts = append(parts, fmt.Sprintf("%s %s → %s refused=%v err=%v", label, path, desc, remote, err))
	}
	probe("temp", dir)
	vol := filepath.VolumeName(dir)
	unc := `\\localhost\` + strings.TrimSuffix(vol, ":") + `$` + strings.TrimPrefix(dir, vol)
	probe("UNC admin share", unc)
	var uncNotes []notify.Note
	uncErr := error(nil)
	if w, err := notify.OpenRDCW(unc, true, notify.StructuralFilter, 64*1024); err == nil {
		write(t, filepath.Join(dir, "local-change.txt"), "x")
		uncNotes = drain(t, w)
		_ = w.Close()
	} else {
		uncErr = err
	}
	letter := ""
	for c := 'S'; c <= 'Z'; c++ {
		if _, err := os.Stat(string(c) + `:\`); err != nil {
			letter = string(c) + ":"
			break
		}
	}
	if letter != "" {
		if out, err := exec.Command("subst", letter, dir).CombinedOutput(); err == nil {
			probe("subst", letter+`\`)
			_ = exec.Command("subst", letter, "/d").Run()
		} else {
			parts = append(parts, fmt.Sprintf("subst failed: %v %s", err, strings.TrimSpace(string(out))))
		}
	}
	evidence(t, "F13", "INFO", "%s; watching the UNC path (err=%v) and changing the directory locally → [%s]",
		strings.Join(parts, "; "), uncErr, summarize(dir, uncNotes, 3))
}

// TestF16DepthZeroControl checks root-loss detection through the parent.
func TestF16DepthZeroControl(t *testing.T) {
	base := t.TempDir()
	anc := filepath.Join(base, "anc")
	parent := filepath.Join(anc, "p")
	root := filepath.Join(parent, "root")
	mkdirs(t, root)
	info, _ := os.Stat(root)
	lazy, _ := os.Stat(root)
	_ = os.SameFile(info, info) // forces the file id to load now; lazy keeps Go's default
	control, err := notify.NewControl(root)
	if err != nil {
		t.Fatalf("control: %v", err)
	}
	defer control.Close()
	about := func(n notify.Note) bool { return strings.EqualFold(filepath.Base(n.Path), "root") }
	renameErr := os.Rename(root, filepath.Join(parent, "root-old"))
	renamed, renameLat, _ := waitFor(control, 3*time.Second, about)
	mkdirs(t, root)
	replacedNotes := drain(t, control)
	now, _ := os.Stat(root)
	identityChanged := !os.SameFile(info, now)
	lazyChanged := !os.SameFile(lazy, now)
	removeErr := os.RemoveAll(root)
	removed, removeLat, _ := waitFor(control, 3*time.Second, about)
	mkdirs(t, root)
	info2, _ := os.Stat(root)
	_ = os.SameFile(info2, info2)
	_ = drain(t, control)
	ancErr := os.Rename(anc, filepath.Join(base, "anc2"))
	ancNotes := drain(t, control)
	_, statErr := os.Stat(root)
	detected := statErr != nil || func() bool { s, _ := os.Stat(root); return !os.SameFile(info2, s) }()
	ok := renamed && identityChanged && removed && (ancErr != nil || detected)
	evidence(t, "F16", verdict(ok), "parent handle: root rename (err=%v) reported=%v in %s; replacement → notes [%s], identity changed=%v with the file id loaded at startup, %v with os.SameFile's lazy load (it reads the id by path when compared); removal reported=%v in %s (err=%v); ancestor rename err=%v → notes [%s], identity check detects=%v",
		renameErr, renamed, ms(renameLat), summarize(parent, replacedNotes, 4), identityChanged, lazyChanged, removed, ms(removeLat), removeErr, ancErr, summarize(base, ancNotes, 3), detected)
}

// TestF18FastRecreate removes and recreates a directory under the recursive handle.
func TestF18FastRecreate(t *testing.T) {
	root := t.TempDir()
	d := filepath.Join(root, "d")
	mkdirs(t, d)
	w := newRegistered(t, root)
	defer w.Close()
	reported := 0
	const rounds = 50
	for i := 0; i < rounds; i++ {
		if err := os.Remove(d); err != nil {
			t.Fatalf("remove: %v", err)
		}
		mkdirs(t, d)
		name := "x" + strconv.Itoa(i)
		write(t, filepath.Join(d, name), "")
		found, _, _ := waitFor(w, 2*time.Second, func(n notify.Note) bool { return strings.HasSuffix(n.Path, name) })
		if found {
			reported++
		}
		if err := os.Remove(filepath.Join(d, name)); err != nil {
			t.Fatalf("remove file: %v", err)
		}
	}
	evidence(t, "F18", verdict(reported == rounds), "recursive handle, no registration needed: %d/%d rounds of remove+recreate+write reported the new file", reported, rounds)
}

var _ = windows.DRIVE_REMOTE
