package backend

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"golang.org/x/sys/unix"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/watch"
)

const (
	// structuralMask is registered on the root and on every structural
	// directory: entry creation, deletion and moves, the directory's own
	// removal and move, and close-after-write (kept for .gitignore only).
	structuralMask = unix.IN_CREATE | unix.IN_DELETE | unix.IN_MOVED_FROM | unix.IN_MOVED_TO |
		unix.IN_DELETE_SELF | unix.IN_MOVE_SELF | unix.IN_CLOSE_WRITE | unix.IN_ONLYDIR | unix.IN_EXCL_UNLINK
	// belowRootMask never follows a link: the root may be given through a
	// link, as the scanner allows, but nothing below it is.
	belowRootMask = structuralMask | unix.IN_DONT_FOLLOW
	// controlMask is the root's registration at --depth 0: its loss only.
	controlMask   = unix.IN_DELETE_SELF | unix.IN_MOVE_SELF | unix.IN_ONLYDIR
	selfEvents    = unix.IN_DELETE_SELF | unix.IN_MOVE_SELF | unix.IN_IGNORED | unix.IN_UNMOUNT
	entryEvents   = unix.IN_CREATE | unix.IN_DELETE | unix.IN_MOVED_FROM | unix.IN_MOVED_TO
	readBufferLen = 64 << 10
)

func newNative(opts Options) watch.Backend {
	return &inotify{opts: opts, fd: -1, wake: [2]int{-1, -1}}
}

// registration is one inotify watch: the canonical path it serves and the
// device and inode it had when registered.
type registration struct {
	path     artifact.Path
	wd       uint32
	dev, ino uint64
}

// inotify is the Linux backend: one watch per structural directory, read by
// one goroutine that only sets the engine's flags.
type inotify struct {
	opts Options
	fd   int
	wake [2]int // pipe: Close writes to wake[1] to end the reader
	done chan struct{}
	sig  watch.Signals

	mu      sync.Mutex
	closed  bool
	rootWD  uint32
	byWD    map[uint32]*registration
	byPath  map[artifact.Path]*registration
	removed map[uint32]struct{} // watches this backend removed, until their IN_IGNORED

	// Tests only: trace sees every event and its relevance; gate holds the
	// reader before its first read.
	trace func(ev inotifyEvent, relevant bool)
	gate  chan struct{}
}

func (b *inotify) Start(sig watch.Signals) error {
	if name := refusedFilesystem(b.opts.Root); name != "" {
		return artifact.New(artifact.CodeWatchFilesystemUnsupported,
			fmt.Sprintf("the root is on a network or virtual filesystem (%s); use --poll", name), false)
	}
	fd, err := unix.InotifyInit1(unix.IN_CLOEXEC | unix.IN_NONBLOCK)
	if err != nil {
		return initFailure(err)
	}
	var wake [2]int
	if err := unix.Pipe2(wake[:], unix.O_CLOEXEC|unix.O_NONBLOCK); err != nil {
		_ = unix.Close(fd)
		return backendFailure("create the inotify wake-up pipe", err)
	}
	mask := uint32(structuralMask)
	if d := b.opts.Capture.Depth; d != nil && *d == 0 {
		mask = controlMask
	}
	wd, err := unix.InotifyAddWatch(fd, b.opts.Root, mask)
	if err != nil {
		_, _, _ = unix.Close(fd), unix.Close(wake[0]), unix.Close(wake[1])
		return addFailure(err, true)
	}
	root := &registration{path: artifact.RootPath, wd: wdKey(wd)}
	b.mu.Lock()
	b.fd, b.wake, b.sig, b.rootWD = fd, wake, sig, root.wd
	b.byWD = map[uint32]*registration{root.wd: root}
	b.byPath = map[artifact.Path]*registration{artifact.RootPath: root}
	b.removed = map[uint32]struct{}{}
	b.done = make(chan struct{})
	b.mu.Unlock()
	go b.read()
	return nil
}

// Reconcile registers every structural directory of obs once, keyed by path
// and (device, inode), and drops the others. It reports whether a watch was
// added, so that a verification observation covers the window before it.
func (b *inotify) Reconcile(obs watch.Observation) (bool, error) {
	set := watch.StructuralSet(obs.Records, b.opts.Capture.Depth)
	want := make(map[artifact.Path]struct{}, len(set))
	for _, p := range set {
		want[p] = struct{}{}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed || b.fd < 0 {
		return false, nil
	}
	for p, reg := range b.byPath {
		if _, ok := want[p]; !ok && p != artifact.RootPath {
			b.drop(reg)
		}
	}
	changed := false
	for _, p := range set {
		if p == artifact.RootPath {
			continue
		}
		full := filepath.Join(b.opts.Root, filepath.FromSlash(string(p)))
		dev, ino, statErr := directoryID(full)
		if reg, ok := b.byPath[p]; ok {
			if statErr == nil && reg.dev == dev && reg.ino == ino {
				continue
			}
			b.drop(reg) // moved away, replaced, or mounted over
		}
		if errors.Is(statErr, unix.EACCES) {
			continue // not watchable; the reconciliation interval covers it
		}
		if statErr != nil {
			changed = true // gone or replaced since the observation: verify
			continue
		}
		wd, err := unix.InotifyAddWatch(b.fd, full, belowRootMask)
		switch {
		case err == nil:
		case errors.Is(err, unix.ENOENT), errors.Is(err, unix.ENOTDIR), errors.Is(err, unix.ELOOP):
			changed = true
			continue
		case errors.Is(err, unix.EACCES):
			continue // not watchable; the reconciliation interval covers it
		default:
			return changed, addFailure(err, false)
		}
		key := wdKey(wd)
		if old, ok := b.byWD[key]; ok && old.path != p {
			delete(b.byPath, old.path) // the same inode, now under p
		}
		delete(b.removed, key)
		reg := &registration{path: p, wd: key, dev: dev, ino: ino}
		b.byWD[key], b.byPath[p] = reg, reg
		changed = true
	}
	return changed, nil
}

// drop removes a registration and its watch; b.mu is held.
func (b *inotify) drop(reg *registration) {
	if b.byPath[reg.path] == reg {
		delete(b.byPath, reg.path)
	}
	if b.byWD[reg.wd] == reg {
		delete(b.byWD, reg.wd)
		b.removed[reg.wd] = struct{}{}
		_, _ = unix.InotifyRmWatch(b.fd, reg.wd)
	}
}

func directoryID(path string) (dev, ino uint64, err error) {
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		return 0, 0, err
	}
	if st.Mode&unix.S_IFMT != unix.S_IFDIR {
		return 0, 0, unix.ENOTDIR
	}
	return st.Dev, st.Ino, nil
}

// read waits for notifications and turns each into flags, until Close.
func (b *inotify) read() {
	defer close(b.done)
	if b.gate != nil {
		<-b.gate
	}
	buf := make([]byte, readBufferLen)
	fds := []unix.PollFd{
		{Fd: fdKey(b.fd), Events: unix.POLLIN},
		{Fd: fdKey(b.wake[0]), Events: unix.POLLIN},
	}
	for {
		if _, err := unix.Poll(fds, -1); err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			b.sig.Fail(backendFailure("wait for inotify events", err))
			return
		}
		if fds[1].Revents != 0 {
			return
		}
		n, err := unix.Read(b.fd, buf)
		if err != nil {
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) {
				continue
			}
			b.sig.Fail(backendFailure("read inotify events", err))
			return
		}
		if err := parseInotify(buf[:n], b.handle); err != nil {
			b.sig.Overflow() // an unreadable batch is lost notifications
		}
	}
}

func (b *inotify) handle(ev inotifyEvent) {
	relevant := b.classify(ev)
	if b.trace != nil {
		b.trace(ev, relevant)
	}
}

// classify signals the engine for one event and reports whether it was
// relevant. Only content writes to files other than .gitignore, and late
// events of watches this backend removed, are dropped.
func (b *inotify) classify(ev inotifyEvent) bool {
	if ev.mask&unix.IN_Q_OVERFLOW != 0 {
		b.sig.Overflow()
		return true
	}
	b.mu.Lock()
	reg, known := b.byWD[ev.wd]
	_, removed := b.removed[ev.wd]
	isRoot := known && ev.wd == b.rootWD
	if ev.mask&unix.IN_IGNORED != 0 {
		delete(b.removed, ev.wd)
		if known {
			delete(b.byWD, ev.wd)
			if b.byPath[reg.path] == reg {
				delete(b.byPath, reg.path)
			}
		}
	}
	b.mu.Unlock()
	switch {
	case isRoot && ev.mask&selfEvents != 0:
		b.sig.Fail(artifact.New(artifact.CodeWatchRootUnavailable, "the watched root was removed or moved", false))
		return true
	case removed && !known:
		return false
	case ev.mask&unix.IN_CLOSE_WRITE != 0 && ev.mask&(entryEvents|selfEvents) == 0:
		if !b.opts.Capture.UseGitignore || ev.name != ".gitignore" {
			return false
		}
	}
	b.sig.Notify()
	return true
}

func (b *inotify) Close() error {
	b.mu.Lock()
	if b.closed || b.fd < 0 {
		b.closed = true
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	b.mu.Unlock()
	_, _ = unix.Write(b.wake[1], []byte{1})
	<-b.done
	_, _, _ = unix.Close(b.fd), unix.Close(b.wake[0]), unix.Close(b.wake[1])
	return nil
}

// refusedFilesystem names the root's filesystem type when the policy
// refuses it. A probe that fails means "unrecognized": accepted.
func refusedFilesystem(root string) string {
	var st unix.Statfs_t
	if err := unix.Statfs(root, &st); err != nil {
		return ""
	}
	return classifyLinux(st.Type)
}

func initFailure(err error) error {
	switch {
	case errors.Is(err, unix.EMFILE):
		return limitReached("too many inotify instances (fs.inotify.max_user_instances)")
	case errors.Is(err, unix.ENFILE), errors.Is(err, unix.ENOMEM):
		return limitReached("the system cannot open another inotify instance")
	}
	return backendFailure("start inotify", err)
}

func addFailure(err error, root bool) error {
	switch {
	case errors.Is(err, unix.ENOSPC):
		return limitReached("the inotify watch limit is reached (fs.inotify.max_user_watches)")
	case errors.Is(err, unix.ENOMEM):
		return limitReached("the kernel cannot allocate another inotify watch")
	case root && (errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ENOTDIR)):
		return artifact.New(artifact.CodeWatchRootUnavailable, "the watched root does not exist", false)
	}
	return backendFailure("register a directory with inotify", err)
}

// wdKey and fdKey convert kernel descriptors, which are non-negative int32
// values, to the types of the inotify and poll interfaces.
func wdKey(wd int) uint32 { return uint32(wd) } //nolint:gosec // G115: inotify watch descriptors are non-negative int32 values.

func fdKey(fd int) int32 { return int32(fd) } //nolint:gosec // G115: file descriptors are non-negative int32 values.
