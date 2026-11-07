package backend

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	"golang.org/x/sys/windows"
	"golang.org/x/text/unicode/norm"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/watch"
)

const (
	notifyBufferLen = 64 << 10
	volumePathLen   = windows.MAX_PATH + 1
)

func newNative(opts Options) watch.Backend { return &rdcw{opts: opts} }

// held counts the handles the Windows backends hold. The process handle
// count cannot tell them apart: Windows also creates handles of its own in
// the process (thread pool workers), at any time.
var held atomic.Int64

// HeldHandles returns the number of handles the Windows backends hold now:
// zero once every backend is closed. Tests check it after each watch.
func HeldHandles() int64 { return held.Load() }

func closeHandle(h windows.Handle) {
	if windows.CloseHandle(h) == nil {
		held.Add(-1)
	}
}

// dirWatch is one ReadDirectoryChangesW handle with two buffers: the next
// read is issued into one before the other is parsed.
type dirWatch struct {
	handle    windows.Handle
	event     windows.Handle
	ov        windows.Overlapped
	bufs      [2][]byte
	cur       int
	pending   bool
	recursive bool
	filter    uint32
}

// rdcw is the Windows backend: one recursive handle on the root (none at
// --depth 0) and one handle on the root's parent, which reports the root's
// own removal or rename (none for a drive root, which cannot be removed).
type rdcw struct {
	opts   Options
	sig    watch.Signals
	root   *dirWatch
	parent *dirWatch
	stop   windows.Handle
	done   chan struct{}

	mu      sync.Mutex
	started bool
	closed  bool
	set     map[string]struct{} // structural directories of the last observation, by key
	records map[string]struct{} // its records, by key; kept only with hidden entries excluded

	// Tests only: trace sees every record and its relevance; gate holds the
	// reader before its first wait.
	trace func(rec notifyRecord, control, relevant bool)
	gate  chan struct{}
}

// key folds a path for comparison with the artifact: NFC, as canonical
// paths are, and case-insensitive, as Windows names are.
func key(p string) string { return strings.ToLower(norm.NFC.String(p)) }

func (b *rdcw) Start(sig watch.Signals) error {
	if name := refusedVolume(b.opts.Root); name != "" {
		return artifact.New(artifact.CodeWatchFilesystemUnsupported,
			fmt.Sprintf("the root is on a network or virtual filesystem (%s); use --poll", name), false)
	}
	stop, err := windows.CreateEvent(nil, 1, 0, nil)
	if err == nil {
		held.Add(1)
	}
	if err != nil {
		if outOfResources(err) {
			return limitReached(handleLimit)
		}
		return backendFailure("create the stop event", err)
	}
	var root, parent *dirWatch
	if d := b.opts.Capture.Depth; d == nil || *d != 0 {
		filter := uint32(windows.FILE_NOTIFY_CHANGE_FILE_NAME | windows.FILE_NOTIFY_CHANGE_DIR_NAME)
		if b.opts.Capture.UseGitignore {
			filter |= windows.FILE_NOTIFY_CHANGE_LAST_WRITE
		}
		if !b.opts.Capture.Hidden {
			filter |= windows.FILE_NOTIFY_CHANGE_ATTRIBUTES
		}
		if root, err = openWatch(b.opts.Root, true, filter); err != nil {
			closeHandle(stop)
			return openFailure(err)
		}
	}
	if dir := filepath.Dir(b.opts.Root); dir != b.opts.Root {
		// Without the parent (unreadable), the root's identity check at each
		// observation still detects its loss.
		parent, _ = openWatch(dir, false, windows.FILE_NOTIFY_CHANGE_FILE_NAME|windows.FILE_NOTIFY_CHANGE_DIR_NAME)
	}
	b.mu.Lock()
	b.sig, b.root, b.parent, b.stop, b.started = sig, root, parent, stop, true
	b.done = make(chan struct{})
	b.mu.Unlock()
	go b.read()
	return nil
}

// openWatch opens dir with every share mode, so that the root and everything
// below it stay renamable and removable, and issues the first read.
func openWatch(dir string, recursive bool, filter uint32) (*dirWatch, error) {
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, windows.FILE_LIST_DIRECTORY,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		return nil, err
	}
	held.Add(1)
	ev, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		closeHandle(h)
		return nil, err
	}
	held.Add(1)
	w := &dirWatch{handle: h, event: ev, recursive: recursive, filter: filter,
		bufs: [2][]byte{make([]byte, notifyBufferLen), make([]byte, notifyBufferLen)}}
	w.ov.HEvent = ev
	if err := w.issue(); err != nil {
		w.close()
		return nil, err
	}
	return w, nil
}

func (w *dirWatch) issue() error {
	if err := windows.ResetEvent(w.event); err != nil {
		return err
	}
	buf := w.bufs[w.cur]
	err := windows.ReadDirectoryChanges(w.handle, &buf[0], notifyBufferLen, w.recursive, w.filter, nil, &w.ov, 0)
	if err != nil && !errors.Is(err, windows.ERROR_IO_PENDING) {
		return err
	}
	w.pending = true
	return nil
}

// complete collects the finished read, issues the next one into the other
// buffer at once, and returns the data, the read error and the issue error.
func (w *dirWatch) complete() (data []byte, overflow bool, readErr, issueErr error) {
	var n uint32
	readErr = windows.GetOverlappedResult(w.handle, &w.ov, &n, false)
	w.pending = false
	done := w.bufs[w.cur]
	if errors.Is(readErr, windows.ERROR_OPERATION_ABORTED) {
		return nil, false, readErr, nil
	}
	w.cur ^= 1
	issueErr = w.issue()
	switch {
	case errors.Is(readErr, windows.ERROR_NOTIFY_ENUM_DIR), readErr == nil && n == 0:
		return nil, true, nil, issueErr
	case readErr != nil:
		return nil, false, readErr, issueErr
	}
	return done[:n], false, nil, issueErr
}

func (w *dirWatch) close() {
	if w.pending {
		_ = windows.CancelIoEx(w.handle, &w.ov)
		var n uint32
		_ = windows.GetOverlappedResult(w.handle, &w.ov, &n, true)
		w.pending = false
	}
	closeHandle(w.handle)
	closeHandle(w.event)
}

// Reconcile records the structural set of obs. A recursive handle needs no
// registration, but a directory that just entered the set may have had
// events dropped before this call, so a gain schedules a verification.
func (b *rdcw) Reconcile(obs watch.Observation) (bool, error) {
	if b.root == nil {
		return false, nil
	}
	set := make(map[string]struct{})
	for _, p := range watch.StructuralSet(obs.Records, b.opts.Capture.Depth) {
		set[key(string(p))] = struct{}{}
	}
	var records map[string]struct{}
	if !b.opts.Capture.Hidden {
		records = make(map[string]struct{}, len(obs.Records))
		for _, r := range obs.Records {
			records[key(string(r.Path))] = struct{}{}
		}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	changed := false
	for k := range set {
		if _, ok := b.set[k]; !ok {
			changed = true
			break
		}
	}
	b.set, b.records = set, records
	return changed, nil
}

// read waits on the handles and turns their records into flags, until Close.
func (b *rdcw) read() {
	defer close(b.done)
	defer failOnPanic(b.sig)
	if b.gate != nil {
		<-b.gate
	}
	handles := []windows.Handle{b.stop}
	watches := []*dirWatch{nil}
	for _, w := range []*dirWatch{b.root, b.parent} {
		if w != nil {
			handles, watches = append(handles, w.event), append(watches, w)
		}
	}
	for {
		r, err := windows.WaitForMultipleObjects(handles, false, windows.INFINITE)
		if err != nil {
			b.sig.Fail(backendFailure("wait for directory changes", err))
			return
		}
		i := int(r - windows.WAIT_OBJECT_0)
		if i <= 0 || i >= len(handles) {
			return
		}
		w := watches[i]
		data, overflow, readErr, issueErr := w.complete()
		if errors.Is(readErr, windows.ERROR_OPERATION_ABORTED) {
			return
		}
		if w == b.root {
			if !b.rootRecords(data, overflow, readErr, issueErr) {
				return
			}
			continue
		}
		if !b.parentRecords(data, overflow, readErr, issueErr) {
			handles = append(handles[:i], handles[i+1:]...)
			watches = append(watches[:i], watches[i+1:]...)
		}
	}
}

// rootRecords handles a read of the root; false ends the reader.
func (b *rdcw) rootRecords(data []byte, overflow bool, readErr, issueErr error) bool {
	switch {
	case overflow:
		b.sig.Overflow()
	case readErr != nil:
		b.sig.Fail(rootReadFailure(readErr))
		return false
	default:
		if err := parseNotify(data, func(rec notifyRecord) {
			relevant := b.relevant(rec)
			if relevant {
				b.sig.Notify()
			}
			if b.trace != nil {
				b.trace(rec, false, relevant)
			}
		}); err != nil {
			b.sig.Overflow() // an unreadable batch is lost notifications
		}
	}
	if issueErr != nil {
		b.sig.Fail(rootReadFailure(issueErr))
		return false
	}
	return true
}

// parentRecords handles a read of the root's parent: the root's removal or
// rename is fatal; anything uncertain schedules an observation, whose
// identity check decides. False stops watching the parent.
func (b *rdcw) parentRecords(data []byte, overflow bool, readErr, issueErr error) bool {
	name := key(filepath.Base(b.opts.Root))
	switch {
	case overflow, readErr != nil:
		b.sig.Notify()
	default:
		if err := parseNotify(data, func(rec notifyRecord) {
			mine := key(rec.path) == name
			if mine && (rec.action == actionRemoved || rec.action == actionRenamedOld) {
				b.sig.Fail(artifact.New(artifact.CodeWatchRootUnavailable, "the watched root was removed or moved", false))
			} else if mine {
				b.sig.Notify()
			}
			if b.trace != nil {
				b.trace(rec, true, mine)
			}
		}); err != nil {
			b.sig.Notify()
		}
	}
	return readErr == nil && issueErr == nil
}

// relevant applies the Structural Watch v1 relevance rules to a record of
// the recursive root handle.
func (b *rdcw) relevant(rec notifyRecord) bool {
	if strings.ContainsRune(rec.path, utf8.RuneError) || mayBeShortName(rec.path) {
		return true // a name that cannot be placed in the last state
	}
	k := key(rec.path)
	b.mu.Lock()
	_, inSet := b.set[path.Dir(k)]
	_, inState := b.records[k]
	b.mu.Unlock()
	if !inSet {
		// An excluded subtree, beyond the depth limit, or below a directory
		// that entered the tree since the last observation, whose own
		// creation was relevant and whose registration is verified.
		return false
	}
	switch rec.action {
	case actionAdded, actionRemoved, actionRenamedOld, actionRenamedNew:
		return true
	case actionModified:
		base := path.Base(rec.path)
		if b.opts.Capture.UseGitignore && strings.EqualFold(base, ".gitignore") {
			return true
		}
		if b.opts.Capture.Hidden || strings.HasPrefix(base, ".") {
			return false // a dot name is hidden whatever its attributes
		}
		return b.hiddenMayDisagree(rec.path, inState)
	}
	return true
}

// hiddenMayDisagree reports whether an entry's hidden attribute may
// disagree with its membership in the last state, with one attribute read.
func (b *rdcw) hiddenMayDisagree(rel string, inState bool) bool {
	p, err := windows.UTF16PtrFromString(filepath.Join(b.opts.Root, filepath.FromSlash(rel)))
	if err != nil {
		return true
	}
	attrs, err := windows.GetFileAttributes(p)
	if err != nil {
		return true // vanished or unreadable: doubt
	}
	if b.opts.Capture.DirsOnly && attrs&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return false
	}
	hidden := attrs&windows.FILE_ATTRIBUTE_HIDDEN != 0
	return hidden == inState
}

func (b *rdcw) Close() error {
	b.mu.Lock()
	if b.closed || !b.started {
		b.closed = true
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	b.mu.Unlock()
	_ = windows.SetEvent(b.stop)
	<-b.done
	for _, w := range []*dirWatch{b.root, b.parent} {
		if w != nil {
			w.close()
		}
	}
	closeHandle(b.stop)
	return nil
}

// refusedVolume names the root's drive type when the policy refuses it. A
// probe that fails (a subst drive) means "unrecognized": accepted.
func refusedVolume(root string) string {
	p, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return ""
	}
	vol := make([]uint16, volumePathLen)
	if err := windows.GetVolumePathName(p, &vol[0], volumePathLen); err != nil {
		return ""
	}
	return classifyWindowsDrive(windows.GetDriveType(&vol[0]))
}

func openFailure(err error) error {
	switch {
	case errors.Is(err, windows.ERROR_FILE_NOT_FOUND), errors.Is(err, windows.ERROR_PATH_NOT_FOUND):
		return artifact.New(artifact.CodeWatchRootUnavailable, "the watched root does not exist", false)
	case outOfResources(err):
		return limitReached(handleLimit)
	}
	return backendFailure("open the root for directory changes", err)
}

// rootReadFailure maps a failed read of the root: a directory removed while
// its handle is open refuses access.
func rootReadFailure(err error) error {
	switch {
	case errors.Is(err, windows.ERROR_ACCESS_DENIED):
		return artifact.New(artifact.CodeWatchRootUnavailable, "the watched root was removed", false)
	case outOfResources(err):
		return limitReached(handleLimit)
	}
	return backendFailure("read directory changes", err)
}

const handleLimit = "the system cannot allocate another handle or buffer for directory changes"

// outOfResources reports a handle, memory or quota the system cannot
// allocate (Structural Watch v1: watch_limit_reached).
func outOfResources(err error) bool {
	for _, limit := range []error{windows.ERROR_TOO_MANY_OPEN_FILES, windows.ERROR_NOT_ENOUGH_MEMORY,
		windows.ERROR_NO_SYSTEM_RESOURCES, windows.ERROR_NOT_ENOUGH_QUOTA} {
		if errors.Is(err, limit) {
			return true
		}
	}
	return false
}
