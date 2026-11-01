package facts

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"dirloom.local/spikes/a8watch/notify"
)

// TestF11Calls exercises the native calls of this platform end to end.
func TestF11Calls(t *testing.T) {
	root := t.TempDir()
	n, err := notify.NewNative(root)
	if err != nil {
		evidence(t, "F11", "FALSE", "native backend failed to start: %v", err)
		return
	}
	write(t, filepath.Join(root, "probe.txt"), "x")
	notes := drain(t, n)
	closeErr := n.Close()
	kind, remote, classErr := notify.Classify(root)
	handles, handlesErr := notify.OpenHandles()
	ok := len(notes) > 0 && closeErr == nil && classErr == nil && handlesErr == nil
	evidence(t, "F11", verdict(ok), "golang.org/x/sys v0.47.0, %s: start+register ok, notes after one file=%d (%s), close err=%v, classify=%q remote=%v err=%v, open handles=%d err=%v",
		n.Name(), len(notes), summarize(root, notes, 4), closeErr, kind, remote, classErr, handles, handlesErr)
}

// TestF12EditorSaves replays editor save sequences and checks that the
// structure is unchanged after stabilization.
func TestF12EditorSaves(t *testing.T) {
	type pattern struct {
		name string
		save func(dir string) error
	}
	patterns := []pattern{
		{"in place (Notepad++, VS Code default)", func(dir string) error {
			return os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main // edited\n"), 0o644)
		}},
		{"vim backupcopy=no (rename to backup, write, delete backup)", func(dir string) error {
			f := filepath.Join(dir, "main.go")
			if err := os.Rename(f, f+"~"); err != nil {
				return err
			}
			if err := os.WriteFile(f, []byte("package main // vim\n"), 0o644); err != nil {
				return err
			}
			return os.Remove(f + "~")
		}},
		{"JetBrains safe write (tmp, old, swap, delete old)", func(dir string) error {
			f := filepath.Join(dir, "main.go")
			if err := os.WriteFile(f+"___jb_tmp___", []byte("package main // jb\n"), 0o644); err != nil {
				return err
			}
			if err := os.Rename(f, f+"___jb_old___"); err != nil {
				return err
			}
			if err := os.Rename(f+"___jb_tmp___", f); err != nil {
				return err
			}
			return os.Remove(f + "___jb_old___")
		}},
		{"atomic replace (write temp, rename over target)", func(dir string) error {
			f := filepath.Join(dir, "main.go")
			tmp := filepath.Join(dir, ".main.go.tmp")
			if err := os.WriteFile(tmp, []byte("package main // atomic\n"), 0o644); err != nil {
				return err
			}
			return os.Rename(tmp, f)
		}},
	}
	allOK := true
	var details []string
	for _, p := range patterns {
		root := t.TempDir()
		mkdirs(t, filepath.Join(root, "src"))
		write(t, filepath.Join(root, "src", "main.go"), "package main\n")
		n := newRegistered(t, root)
		before := listing(t, root)
		if err := p.save(filepath.Join(root, "src")); err != nil {
			t.Fatalf("%s: %v", p.name, err)
		}
		notes := drain(t, n)
		after := listing(t, root)
		_ = n.Close()
		same := strings.Join(before, ",") == strings.Join(after, ",")
		allOK = allOK && same
		details = append(details, fmt.Sprintf("%s: structure unchanged=%v, notes=%d [%s]", p.name, same, len(notes), summarize(root, notes, 6)))
	}
	evidence(t, "F12", verdict(allOK), "simulated sequences (no real editor run): %s", strings.Join(details, "; "))
}

// TestF14Burst creates a nested tree in one burst, then registers the new
// directories as the engine would, and checks later changes are reported.
func TestF14Burst(t *testing.T) {
	root := t.TempDir()
	n, err := notify.NewNative(root)
	if err != nil {
		t.Fatalf("native: %v", err)
	}
	defer n.Close()
	deep := filepath.Join(root, "a", "b", "c", "d")
	mkdirs(t, deep)
	for _, d := range []string{"a", "a/b", "a/b/c", "a/b/c/d"} {
		write(t, filepath.Join(root, filepath.FromSlash(d), "f.txt"), "x")
	}
	burst := drain(t, n)
	deepSeen := 0
	for _, note := range burst {
		if strings.Contains(filepath.ToSlash(note.Path), "/a/b/") {
			deepSeen++
		}
	}
	registered := 0
	if !n.Recursive() {
		for _, d := range dirsBelow(t, root) {
			if err := n.Watch(d); err != nil {
				t.Fatalf("watch %s: %v", d, err)
			}
			registered++
		}
	}
	verification := listing(t, root)
	write(t, filepath.Join(deep, "late.txt"), "late")
	found, elapsed, late := waitFor(n, 3*time.Second, func(note notify.Note) bool {
		p := filepath.ToSlash(note.Path)
		return strings.HasSuffix(p, "/a/b/c/d/late.txt") || (note.Op == notify.OpDirWrite && strings.HasSuffix(p, "/a/b/c/d"))
	})
	evidence(t, "F14", verdict(found && len(verification) == 8), "%s: burst notes=%d (below a/b before registration: %d); registered after burst=%d; verification listing=%d entries; later change in a/b/c/d reported=%v in %s [%s]",
		n.Name(), len(burst), deepSeen, registered, len(verification), found, ms(elapsed), summarize(root, late, 4))
}

// TestF15BlockedStdout runs a child whose stdout pipe is never read, then
// interrupts it and measures how it ends.
func TestF15BlockedStdout(t *testing.T) {
	helper := buildHelper(t)
	type outcome struct {
		code    int
		elapsed time.Duration
		stderr  []string
	}
	run := func(read bool, selfInterrupt time.Duration, signal bool) (outcome, error) {
		args := []string{"blocked-writer", "-grace", "2s"}
		if selfInterrupt > 0 {
			args = append(args, "-self-interrupt", selfInterrupt.String())
		}
		cmd := exec.Command(helper, args...)
		prepareGroup(cmd)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return outcome{}, err
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return outcome{}, err
		}
		if err := cmd.Start(); err != nil {
			return outcome{}, err
		}
		if read {
			go func() { _, _ = io.Copy(io.Discard, stdout) }()
		}
		lines := make(chan string, 16)
		var mu sync.Mutex
		var got []string
		go func() {
			sc := bufio.NewScanner(stderr)
			for sc.Scan() {
				mu.Lock()
				got = append(got, sc.Text())
				mu.Unlock()
				lines <- sc.Text()
			}
			close(lines)
		}()
		var start time.Time
		deadline := time.After(20 * time.Second)
	loop:
		for {
			select {
			case l, ok := <-lines:
				if !ok {
					break loop
				}
				if l == "BLOCKED" {
					start = time.Now()
					if signal {
						if err := interruptChild(cmd); err != nil {
							mu.Lock()
							got = append(got, "interrupt error: "+err.Error())
							mu.Unlock()
						}
					}
				}
				if strings.HasPrefix(l, "WRITE-DONE") && start.IsZero() {
					start = time.Now()
				}
			case <-deadline:
				_ = cmd.Process.Kill()
				break loop
			}
		}
		err = cmd.Wait()
		o := outcome{elapsed: time.Since(start)}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			o.code = exitErr.ExitCode()
		} else if err != nil {
			o.code = -1
		}
		mu.Lock()
		o.stderr = append([]string(nil), got...)
		mu.Unlock()
		return o, nil
	}

	control, err := run(true, 0, false)
	if err != nil {
		t.Fatalf("control: %v", err)
	}
	blockedSignal, err := run(false, 0, true)
	if err != nil {
		t.Fatalf("signal: %v", err)
	}
	blockedSelf, err := run(false, 300*time.Millisecond, false)
	if err != nil {
		t.Fatalf("self: %v", err)
	}
	expired := func(o outcome) bool {
		return o.code == 5 && contains(o.stderr, "GRACE-EXPIRED") && o.elapsed < 4*time.Second
	}
	ok := control.code == 0 && expired(blockedSelf)
	signalNote := fmt.Sprintf("signal path: exit %d after %s, stderr %v", blockedSignal.code, ms(blockedSignal.elapsed), blockedSignal.stderr)
	evidence(t, "F15", verdict(ok), "4 MiB generation, grace 2s. Consumer reading: exit %d, stderr %v. Consumer not reading, internal interrupt: exit %d %s after BLOCKED, stderr %v. %s",
		control.code, control.stderr, blockedSelf.code, ms(blockedSelf.elapsed), blockedSelf.stderr, signalNote)
}

func contains(lines []string, want string) bool {
	for _, l := range lines {
		if strings.Contains(l, want) {
			return true
		}
	}
	return false
}

// TestF19Comparison compares the x/sys prototype with fsnotify v1.10.1.
func TestF19Comparison(t *testing.T) {
	root, dirs, entries, created := bigTree(t)
	type result struct {
		name       string
		registered int
		regTime    time.Duration
		regErr     error
		handles    int
		writeNotes int
	}
	measure := func(name string, start func(string) (notify.Notifier, error)) result {
		r := result{name: name}
		h0, _ := notify.OpenHandles()
		t0 := time.Now()
		n, err := start(root)
		if err != nil {
			r.regErr = err
			return r
		}
		r.registered = 1
		if !n.Recursive() {
			for _, d := range dirs {
				if err := n.Watch(d); err != nil {
					r.regErr = err
					break
				}
				r.registered++
			}
		}
		r.regTime = time.Since(t0)
		h1, _ := notify.OpenHandles()
		r.handles = h1 - h0
		if r.regErr == nil {
			for i := 0; i < 200; i++ {
				f := filepath.Join(dirs[i*len(dirs)/200], "f0.txt")
				fh, err := os.OpenFile(f, os.O_APPEND|os.O_WRONLY, 0)
				if err == nil {
					_, _ = fh.WriteString("more\n")
					_ = fh.Close()
				}
			}
			notes, _ := notify.Drain(n, 700*time.Millisecond, 15*time.Second)
			r.writeNotes = len(notes)
		}
		_ = n.Close()
		return r
	}
	native := measure("x/sys", notify.NewNative)
	fsn := measure("fsnotify v1.10.1", func(root string) (notify.Notifier, error) { return notify.NewFsnotify(root, true) })
	format := func(r result) string {
		return fmt.Sprintf("%s: registrations=%d in %s, handles/descriptors held=%d, error=%v, notes for 200 content appends=%d",
			r.name, r.registered, ms(r.regTime), r.handles, r.regErr, r.writeNotes)
	}
	evidence(t, "F19", "INFO", "tree %d directories, %d entries (created in %s). %s. %s. Backend code: %s",
		len(dirs), entries, ms(created), format(native), format(fsn), backendLines(t))
	fsnotifyBehavior(t)
}

// fsnotifyBehavior records fsnotify's Add-while-pending, overflow and
// root-loss behavior.
func fsnotifyBehavior(t *testing.T) {
	probe := t.TempDir()
	mkdirs(t, filepath.Join(probe, "sub"))
	pw, err := notify.NewFsnotify(probe, false)
	if err != nil {
		t.Fatalf("fsnotify: %v", err)
	}
	for i := 0; i < 50; i++ {
		write(t, filepath.Join(probe, "pending"+strconv.Itoa(i)), "x")
	}
	time.Sleep(300 * time.Millisecond)
	added := make(chan error, 1)
	go func() { added <- pw.Watch(filepath.Join(probe, "sub")) }()
	blocked := false
	var addErr error
	select {
	case addErr = <-added:
	case <-time.After(3 * time.Second):
		blocked = true
		_ = drain(t, pw)
		select {
		case addErr = <-added:
		case <-time.After(5 * time.Second):
		}
	}
	_ = pw.Close()

	root := t.TempDir()
	w, err := notify.NewFsnotify(root, false)
	if err != nil {
		t.Fatalf("fsnotify: %v", err)
	}
	files := 20000
	if runtime.GOOS == "darwin" {
		files = 2000
	}
	for i := 0; i < files; i++ {
		write(t, filepath.Join(root, "f"+strconv.Itoa(i)), "")
	}
	notes, _ := notify.Drain(w, 700*time.Millisecond, 60*time.Second)
	overflow := notify.Count(notes, notify.OpOverflow)
	_ = w.Close()

	base := t.TempDir()
	small := filepath.Join(base, "root")
	mkdirs(t, small)
	w, err = notify.NewFsnotify(small, true)
	if err != nil {
		t.Fatalf("fsnotify: %v", err)
	}
	renameErr := os.Rename(small, filepath.Join(base, "root-moved"))
	renamed := drain(t, w)
	removeErr := os.RemoveAll(filepath.Join(base, "root-moved"))
	removed := drain(t, w)
	_ = w.Close()
	evidence(t, "F19", "INFO", "fsnotify: Add blocked for >3 s while 50 file events were undelivered=%v (Add err=%v); %d files created without reading → %d notes, overflow reported=%d; root renamed (err=%v) → [%s]; root removed (err=%v) → [%s]",
		blocked, addErr, files, len(notes), overflow, renameErr, summarize(base, renamed, 4), removeErr, summarize(base, removed, 4))
}

// backendLines counts the lines of the prototype and of fsnotify's backend.
func backendLines(t *testing.T) string {
	own := map[string]string{"linux": "inotify_linux.go", "darwin": "kqueue_darwin.go", "windows": "rdcw_windows.go"}[runtime.GOOS]
	theirs := map[string]string{"linux": "backend_inotify.go", "darwin": "backend_kqueue.go", "windows": "backend_windows.go"}[runtime.GOOS]
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/fsnotify/fsnotify").Output()
	if err != nil {
		return fmt.Sprintf("go list failed: %v", err)
	}
	dir := strings.TrimSpace(string(out))
	return fmt.Sprintf("prototype %s %d lines (events only, no reconciliation); fsnotify %s %d lines plus shared %d",
		own, lines(filepath.Join("..", "notify", own)), theirs, lines(filepath.Join(dir, theirs)), lines(filepath.Join(dir, "fsnotify.go")))
}

func lines(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return -1
	}
	return strings.Count(string(data), "\n")
}

var (
	bigOnce    sync.Once
	bigRoot    string
	bigDirs    []string
	bigEntries int
	bigCreated time.Duration
	bigErr     error
)

// bigTree builds once a tree of A8_F19_DIRS directories (default 10000) with
// ten files each, in 100 top-level groups.
func bigTree(t *testing.T) (string, []string, int, time.Duration) {
	t.Helper()
	bigOnce.Do(func() {
		count := 10000
		if v, err := strconv.Atoi(os.Getenv("A8_F19_DIRS")); err == nil && v > 0 {
			count = v
		}
		bigRoot, bigErr = os.MkdirTemp("", "a8-big-")
		if bigErr != nil {
			return
		}
		start := time.Now()
		for i := 0; i < count; i++ {
			d := filepath.Join(bigRoot, fmt.Sprintf("g%02d", i%100), fmt.Sprintf("d%05d", i))
			if bigErr = os.MkdirAll(d, 0o755); bigErr != nil {
				return
			}
			bigDirs = append(bigDirs, d)
			for j := 0; j < 10; j++ {
				if bigErr = os.WriteFile(filepath.Join(d, fmt.Sprintf("f%d.txt", j)), nil, 0o644); bigErr != nil {
					return
				}
			}
		}
		groups, _ := filepath.Glob(filepath.Join(bigRoot, "g*"))
		sort.Strings(groups)
		bigDirs = append(groups, bigDirs...)
		bigEntries = len(bigDirs) + count*10
		bigCreated = time.Since(start)
	})
	if bigErr != nil {
		t.Fatalf("big tree: %v", bigErr)
	}
	return bigRoot, bigDirs, bigEntries, bigCreated
}

// TestF20Reconciliation times `dirloom fingerprint` on 10k and big trees.
func TestF20Reconciliation(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "dirloom")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-o", bin, "./cmd/dirloom")
	build.Dir = filepath.Join("..", "..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build dirloom: %v: %s", err, out)
	}
	small := t.TempDir()
	for i := 0; i < 1000; i++ {
		d := filepath.Join(small, fmt.Sprintf("g%02d", i%10), fmt.Sprintf("d%04d", i))
		mkdirs(t, d)
		for j := 0; j < 9; j++ {
			write(t, filepath.Join(d, fmt.Sprintf("f%d.txt", j)), "")
		}
	}
	big, _, _, _ := bigTree(t)
	var details []string
	for _, root := range []string{small, big} {
		var times []time.Duration
		nodes := "?"
		for i := 0; i < 5; i++ {
			start := time.Now()
			out, err := exec.Command(bin, "fingerprint", root, "--format", "json").Output()
			times = append(times, time.Since(start))
			if err != nil {
				t.Fatalf("fingerprint: %v", err)
			}
			var doc map[string]any
			if json.Unmarshal(out, &doc) == nil {
				nodes = fmt.Sprint(doc["nodeCount"])
			}
		}
		sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
		details = append(details, fmt.Sprintf("%s nodes: median %s (min %s, max %s)", nodes, ms(times[2]), ms(times[0]), ms(times[4])))
	}
	evidence(t, "F20", "INFO", "`dirloom fingerprint --format json` (process start included), 5 runs: %s", strings.Join(details, "; "))
}

// TestRaceProbe is run under -race by the workflow (F10).
func TestRaceProbe(t *testing.T) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	n := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			n++
			mu.Unlock()
		}()
	}
	wg.Wait()
	if n != 8 {
		t.Fatal("unexpected count")
	}
}
