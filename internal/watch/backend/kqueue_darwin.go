package backend

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"sync"

	"golang.org/x/sys/unix"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/watch"
)

const (
	// lossNotes are a descriptor's removal, move or revocation: fatal for the
	// root, a dropped registration below it.
	lossNotes = unix.NOTE_DELETE | unix.NOTE_RENAME | unix.NOTE_REVOKE
	// directoryNotes is registered on the root and on every structural
	// directory: NOTE_WRITE fires when its entries change, never for a
	// content write to one of its files (F7).
	directoryNotes = unix.NOTE_WRITE | lossNotes
	// ignoreNotes is registered on each .gitignore of a structural directory.
	ignoreNotes = unix.NOTE_WRITE | unix.NOTE_EXTEND | unix.NOTE_ATTRIB | lossNotes
	eventBatch  = 64
	// wakeIdent is the EVFILT_USER event Close triggers to end the reader.
	wakeIdent = 0
)

func newNative(opts Options) watch.Backend { return &kqueue{opts: opts, kq: -1} }

// registration is one descriptor: a structural directory, or the
// .gitignore of one, with the device and inode it had when opened.
type registration struct {
	path     artifact.Path // the directory, or the directory holding the .gitignore
	ignore   bool
	fd       int
	dev, ino uint64
}

func (r *registration) name() string {
	if r.ignore {
		return path.Join(string(r.path), ".gitignore")
	}
	return string(r.path)
}

// kqueue is the macOS backend: one O_EVTONLY descriptor per structural
// directory and per .gitignore, read by one goroutine that only sets the
// engine's flags.
type kqueue struct {
	opts Options
	kq   int
	done chan struct{}
	sig  watch.Signals

	mu      sync.Mutex
	closed  bool
	rootFD  int
	byFD    map[int]*registration
	dirs    map[artifact.Path]*registration
	ignores map[artifact.Path]*registration // by directory

	// Tests only: trace sees every event, by registration name, and its
	// relevance; gate holds the reader before its first wait.
	trace func(name string, fflags uint32, relevant bool)
	gate  chan struct{}
}

func (b *kqueue) Start(sig watch.Signals) error {
	if name := refusedFilesystem(b.opts.Root); name != "" {
		return artifact.New(artifact.CodeWatchFilesystemUnsupported,
			fmt.Sprintf("the root is on a network or virtual filesystem (%s); use --poll", name), false)
	}
	kq, err := unix.Kqueue()
	if err != nil {
		if exhausted(err) {
			return descriptorLimit()
		}
		return backendFailure("create a kqueue", err)
	}
	unix.CloseOnExec(kq)
	wake := unix.Kevent_t{Ident: wakeIdent, Filter: unix.EVFILT_USER, Flags: unix.EV_ADD | unix.EV_CLEAR}
	if _, err := unix.Kevent(kq, []unix.Kevent_t{wake}, nil, nil); err != nil {
		_ = unix.Close(kq)
		return backendFailure("register the kqueue wake-up event", err)
	}
	notes := uint32(directoryNotes)
	if d := b.opts.Capture.Depth; d != nil && *d == 0 {
		notes = lossNotes
	}
	// The root follows a root link, as the scanner does; nothing below it does.
	fd, dev, ino, err := openEvents(b.opts.Root, true, false)
	if err == nil {
		if err = register(kq, fd, notes); err != nil {
			_ = unix.Close(fd)
		}
	}
	if err != nil {
		_ = unix.Close(kq)
		return rootFailure(err)
	}
	root := &registration{path: artifact.RootPath, fd: fd, dev: dev, ino: ino}
	b.mu.Lock()
	b.kq, b.sig, b.rootFD = kq, sig, fd
	b.byFD = map[int]*registration{fd: root}
	b.dirs = map[artifact.Path]*registration{artifact.RootPath: root}
	b.ignores = map[artifact.Path]*registration{}
	b.done = make(chan struct{})
	b.mu.Unlock()
	go b.read()
	return nil
}

// Reconcile opens a descriptor for every structural directory of obs, and
// for its .gitignore when .gitignore files apply, once, keyed by path and
// (device, inode), and closes the others. It reports whether a descriptor
// was added, so that a verification observation covers the window before it.
func (b *kqueue) Reconcile(obs watch.Observation) (bool, error) {
	set := watch.StructuralSet(obs.Records, b.opts.Capture.Depth)
	want := make(map[artifact.Path]struct{}, len(set))
	for _, p := range set {
		want[p] = struct{}{}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed || b.kq < 0 {
		return false, nil
	}
	for p, reg := range b.dirs {
		if _, ok := want[p]; !ok && p != artifact.RootPath {
			b.drop(reg)
		}
	}
	for p, reg := range b.ignores {
		if _, ok := want[p]; !ok {
			b.drop(reg)
		}
	}
	changed := false
	for _, p := range set {
		dir := filepath.Join(b.opts.Root, filepath.FromSlash(string(p)))
		if p != artifact.RootPath {
			added, err := b.add(p, dir, false)
			if err != nil {
				return changed, err
			}
			changed = changed || added
		}
		if b.opts.Capture.UseGitignore {
			added, err := b.add(p, filepath.Join(dir, ".gitignore"), true)
			if err != nil {
				return changed, err
			}
			changed = changed || added
		}
	}
	return changed, nil
}

// add keeps, replaces or opens the registration of a directory, or of its
// .gitignore, and reports whether a verification is needed; b.mu is held.
func (b *kqueue) add(p artifact.Path, full string, ignore bool) (bool, error) {
	regs := b.dirs
	if ignore {
		regs = b.ignores
	}
	dev, ino, statErr := entryID(full, ignore)
	if reg, ok := regs[p]; ok {
		if statErr == nil && reg.dev == dev && reg.ino == ino {
			return false, nil
		}
		b.drop(reg) // moved away, replaced, or mounted over
	}
	switch {
	case statErr == nil:
	case errors.Is(statErr, unix.EACCES):
		return false, nil // not watchable; the reconciliation interval covers it
	default:
		// A directory gone or replaced since the observation: verify. A
		// missing .gitignore is the common case and needs nothing.
		return !ignore, nil
	}
	fd, dev, ino, err := openEvents(full, false, ignore)
	switch {
	case err == nil:
	case errors.Is(err, unix.ENOENT), errors.Is(err, unix.ENOTDIR), errors.Is(err, unix.ELOOP):
		return true, nil
	case errors.Is(err, unix.EACCES):
		return false, nil // not watchable; the reconciliation interval covers it
	case exhausted(err):
		return false, descriptorLimit()
	default:
		return false, backendFailure("open a directory for kqueue events", err)
	}
	notes := uint32(directoryNotes)
	if ignore {
		notes = ignoreNotes
	}
	if err := register(b.kq, fd, notes); err != nil {
		_ = unix.Close(fd)
		if errors.Is(err, unix.ENOMEM) {
			return false, limitReached("the kernel cannot allocate another kqueue event")
		}
		return false, backendFailure("register a descriptor with kqueue", err)
	}
	reg := &registration{path: p, ignore: ignore, fd: fd, dev: dev, ino: ino}
	regs[p], b.byFD[fd] = reg, reg
	return true, nil
}

// drop closes a registration's descriptor, which removes its events from
// the queue; b.mu is held.
func (b *kqueue) drop(reg *registration) {
	regs := b.dirs
	if reg.ignore {
		regs = b.ignores
	}
	if regs[reg.path] == reg {
		delete(regs, reg.path)
	}
	if b.byFD[reg.fd] == reg {
		delete(b.byFD, reg.fd)
		_ = unix.Close(reg.fd)
	}
}

// entryID returns the device and inode of a directory, or of a regular file
// when file is set, without following a link.
func entryID(p string, file bool) (dev, ino uint64, err error) {
	var st unix.Stat_t
	if err := unix.Lstat(p, &st); err != nil {
		return 0, 0, err
	}
	return statID(&st, file)
}

func statID(st *unix.Stat_t, file bool) (dev, ino uint64, err error) {
	want := uint16(unix.S_IFDIR)
	if file {
		want = unix.S_IFREG
	}
	if st.Mode&unix.S_IFMT != want {
		return 0, 0, unix.ENOTDIR
	}
	return uint64(uint32(st.Dev)), st.Ino, nil //nolint:gosec // G115: the device number is an opaque 32-bit value, reinterpreted.
}

// openEvents opens p for event notification only (O_EVTONLY: the volume
// stays unmountable) and returns the descriptor with the device and inode
// of what it opened.
func openEvents(p string, follow, file bool) (fd int, dev, ino uint64, err error) {
	flags := unix.O_EVTONLY | unix.O_CLOEXEC
	if !follow {
		flags |= unix.O_NOFOLLOW
	}
	if !file {
		flags |= unix.O_DIRECTORY
	}
	for {
		fd, err = unix.Open(p, flags, 0)
		if !errors.Is(err, unix.EINTR) {
			break
		}
	}
	if err != nil {
		return -1, 0, 0, err
	}
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		_ = unix.Close(fd)
		return -1, 0, 0, err
	}
	if dev, ino, err = statID(&st, file); err != nil {
		_ = unix.Close(fd)
		return -1, 0, 0, err
	}
	return fd, dev, ino, nil
}

func register(kq, fd int, notes uint32) error {
	var ev unix.Kevent_t
	unix.SetKevent(&ev, fd, unix.EVFILT_VNODE, unix.EV_ADD|unix.EV_CLEAR)
	ev.Fflags = notes
	for {
		_, err := unix.Kevent(kq, []unix.Kevent_t{ev}, nil, nil)
		if !errors.Is(err, unix.EINTR) {
			return err
		}
	}
}

// read waits for events and turns each into flags, until Close.
func (b *kqueue) read() {
	defer close(b.done)
	defer failOnPanic(b.sig)
	if b.gate != nil {
		<-b.gate
	}
	events := make([]unix.Kevent_t, eventBatch)
	for {
		n, err := unix.Kevent(b.kq, nil, events, nil)
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			b.sig.Fail(backendFailure("wait for kqueue events", err))
			return
		}
		for _, ev := range events[:n] {
			if ev.Filter == unix.EVFILT_USER {
				return
			}
			b.handle(ev)
		}
	}
}

func (b *kqueue) handle(ev unix.Kevent_t) {
	name, relevant := b.classify(ev)
	if b.trace != nil {
		b.trace(name, ev.Fflags, relevant)
	}
}

// classify signals the engine for one event. Every event of a structural
// directory or .gitignore descriptor is relevant: directories report entry
// changes only, and content writes reach no other descriptor. A lost
// descriptor is dropped, so that the next reconciliation opens the path
// again; the root's loss is fatal. An event of an unknown descriptor, a
// stale one read before its descriptor was closed, is relevant (doubt).
func (b *kqueue) classify(ev unix.Kevent_t) (string, bool) {
	fd := identFD(ev.Ident)
	lost := ev.Fflags&lossNotes != 0
	b.mu.Lock()
	reg, known := b.byFD[fd]
	isRoot := known && fd == b.rootFD
	name := "?"
	if known {
		name = reg.name()
		if lost && !isRoot {
			b.drop(reg)
		}
	}
	b.mu.Unlock()
	if isRoot && lost {
		b.sig.Fail(artifact.New(artifact.CodeWatchRootUnavailable, "the watched root was removed or moved", false))
		return name, true
	}
	b.sig.Notify()
	return name, true
}

func (b *kqueue) Close() error {
	b.mu.Lock()
	if b.closed || b.kq < 0 {
		b.closed = true
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	b.mu.Unlock()
	trigger := unix.Kevent_t{Ident: wakeIdent, Filter: unix.EVFILT_USER, Fflags: unix.NOTE_TRIGGER}
	for {
		_, err := unix.Kevent(b.kq, []unix.Kevent_t{trigger}, nil, nil)
		if !errors.Is(err, unix.EINTR) {
			break
		}
	}
	<-b.done
	b.mu.Lock()
	for fd := range b.byFD {
		_ = unix.Close(fd)
	}
	b.byFD, b.dirs, b.ignores = nil, nil, nil
	b.mu.Unlock()
	_ = unix.Close(b.kq)
	return nil
}

// refusedFilesystem names the root's filesystem type when the policy
// refuses it. A probe that fails means "unrecognized": accepted.
func refusedFilesystem(root string) string {
	var st unix.Statfs_t
	if err := unix.Statfs(root, &st); err != nil {
		return ""
	}
	return classifyDarwin(unix.ByteSliceToString(st.Fstypename[:]))
}

func rootFailure(err error) error {
	switch {
	case errors.Is(err, unix.ENOENT), errors.Is(err, unix.ENOTDIR):
		return artifact.New(artifact.CodeWatchRootUnavailable, "the watched root does not exist", false)
	case exhausted(err):
		return descriptorLimit()
	}
	return backendFailure("open the root for kqueue events", err)
}

func exhausted(err error) bool {
	return errors.Is(err, unix.EMFILE) || errors.Is(err, unix.ENFILE)
}

// descriptorLimit is A4: a native watch holds one descriptor per structural
// directory, within the process's descriptor limit.
func descriptorLimit() error {
	return limitReached("the descriptor limit is reached: a native watch holds one descriptor per watched directory (kern.maxfilesperproc, ulimit -n)")
}

// identFD converts the ident of a vnode event, the descriptor it was
// registered with, back to a descriptor.
func identFD(ident uint64) int { return int(ident) } //nolint:gosec // G115: vnode event idents are file descriptors.
