package app

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/dirloom/dirloom/internal/identity"
	"github.com/dirloom/dirloom/internal/source"
	"github.com/dirloom/dirloom/internal/watch"
)

// watchWait bounds a wait for the polling engine to report a state. It is a
// guard against a hung test; a state normally arrives within one or two
// poll intervals.
const watchWait = 15 * time.Second

// eventLog is a sink that records events and wakes waiters.
type eventLog struct {
	mu      sync.Mutex
	events  []watch.Event
	changed chan struct{}
}

func newEventLog() *eventLog { return &eventLog{changed: make(chan struct{}, 1)} }

func (l *eventLog) Publish(events []watch.Event) error {
	l.mu.Lock()
	l.events = append(l.events, events...)
	l.mu.Unlock()
	select {
	case l.changed <- struct{}{}:
	default:
	}
	return nil
}

func (l *eventLog) recorded() []watch.Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]watch.Event(nil), l.events...)
}

// waitFor blocks until pred holds for the recorded events.
func (l *eventLog) waitFor(t *testing.T, what string, pred func([]watch.Event) bool) {
	t.Helper()
	deadline := time.After(watchWait)
	for {
		if pred(l.recorded()) {
			return
		}
		select {
		case <-l.changed:
		case <-deadline:
			t.Fatalf("no %s within %s; events: %d", what, watchWait, len(l.recorded()))
		}
	}
}

// waitState blocks until the last committed state has fingerprint fp.
func (l *eventLog) waitState(t *testing.T, fp identity.Fingerprint) {
	t.Helper()
	l.waitFor(t, "state "+fp.String(), func(events []watch.Event) bool {
		state, ok := lastState(events)
		return ok && state == fp
	})
}

func lastState(events []watch.Event) (identity.Fingerprint, bool) {
	for i := len(events) - 1; i >= 0; i-- {
		switch events[i].Kind {
		case watch.KindStarted, watch.KindStateChanged:
			return events[i].Fingerprint, true
		}
	}
	return identity.Fingerprint{}, false
}

// recordingObserver keeps every successful observation by fingerprint, for
// the diff-equivalence property.
type recordingObserver struct {
	watch.Observer
	observed chan struct{}
	mu       sync.Mutex
	seen     map[identity.Fingerprint][]identity.Record
	count    int
}

func (r *recordingObserver) Observe(ctx context.Context) (watch.Observation, error) {
	obs, err := r.Observer.Observe(ctx)
	r.mu.Lock()
	r.count++
	if err == nil {
		r.seen[obs.Fingerprint] = obs.Records
	}
	r.mu.Unlock()
	select {
	case r.observed <- struct{}{}:
	default:
	}
	return obs, err
}

// waitObservations blocks until n more observations have completed.
func (r *recordingObserver) waitObservations(t *testing.T, n int) {
	t.Helper()
	r.mu.Lock()
	target := r.count + n
	r.mu.Unlock()
	deadline := time.After(watchWait)
	for {
		r.mu.Lock()
		done := r.count >= target
		r.mu.Unlock()
		if done {
			return
		}
		select {
		case <-r.observed:
		case <-deadline:
			t.Fatalf("fewer than %d observations within %s", n, watchWait)
		}
	}
}

func (r *recordingObserver) records(fp identity.Fingerprint) ([]identity.Record, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records, ok := r.seen[fp]
	return records, ok
}

type runningWatch struct {
	log    *eventLog
	obs    *recordingObserver
	cancel context.CancelFunc
	result chan error
}

func startWatch(t *testing.T, request WatchRequest) *runningWatch {
	t.Helper()
	log := newEventLog()
	cfg, err := watchConfig(request, log)
	if err != nil {
		t.Fatalf("watchConfig: %v", err)
	}
	rec := &recordingObserver{Observer: cfg.Observer, observed: make(chan struct{}, 1), seen: map[identity.Fingerprint][]identity.Record{}}
	cfg.Observer = rec
	engine, err := watch.New(cfg)
	if err != nil {
		t.Fatalf("watch.New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := &runningWatch{log: log, obs: rec, cancel: cancel, result: make(chan error, 1)}
	go func() { w.result <- engine.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-w.result:
		case <-time.After(watchWait):
		}
	})
	return w
}

// end waits for the engine to stop by itself.
func (w *runningWatch) end(t *testing.T) error {
	t.Helper()
	select {
	case err := <-w.result:
		w.result <- err
		return err
	case <-time.After(watchWait):
		t.Fatal("the watch did not end")
		return nil
	}
}

func (w *runningWatch) interrupt(t *testing.T) error {
	t.Helper()
	w.cancel()
	return w.end(t)
}

func pollRequest(root string) WatchRequest {
	return WatchRequest{
		Inspect: InspectRequest{Root: root, UseDefaultIgnores: true, UseGitIgnore: true},
		Poll:    watch.MinPoll,
	}
}

func fingerprintOf(t *testing.T, request InspectRequest) identity.Fingerprint {
	t.Helper()
	result, err := Fingerprint(context.Background(), request)
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	return result.Fingerprint
}

func mkdirAll(t *testing.T, paths ...string) {
	t.Helper()
	for _, p := range paths {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func writeFiles(t *testing.T, paths ...string) {
	t.Helper()
	for _, p := range paths {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// assertNoRoot checks that no diagnostic discloses the root's absolute path.
func assertNoRoot(t *testing.T, events []watch.Event, root string) {
	t.Helper()
	quoted := strconv.Quote(root)
	for _, ev := range events {
		msg := ev.Diagnostic.Message
		if strings.Contains(msg, root) || strings.Contains(msg, quoted[1:len(quoted)-1]) {
			t.Fatalf("diagnostic discloses the root: %q", msg)
		}
	}
}

// checkDiffEquivalence verifies that every generation's node events are the
// comparison of the two observed states it joins.
func checkDiffEquivalence(t *testing.T, events []watch.Event, rec *recordingObserver) {
	t.Helper()
	var prev identity.Fingerprint
	var nodes []comparison.Change
	generations := 0
	for _, ev := range events {
		switch {
		case ev.Kind == watch.KindStarted:
			prev = ev.Fingerprint
		case ev.Kind.IsNode():
			nodes = append(nodes, ev.Change)
		case ev.Kind == watch.KindStateChanged:
			before, ok1 := rec.records(prev)
			after, ok2 := rec.records(ev.Fingerprint)
			if !ok1 || !ok2 {
				t.Fatalf("generation %d joins states that were not observed", ev.Generation)
			}
			ref := comparison.SourceRef{Kind: source.KindFilesystem}
			diff, err := comparison.CompareRecords(before, after, ref, ref, comparison.Options{MoveDetection: comparison.MoveDetectionStructuralV1})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(nodes, diff.Changes) {
				t.Fatalf("generation %d: node events %+v, diff %+v", ev.Generation, nodes, diff.Changes)
			}
			prev, nodes = ev.Fingerprint, nil
			generations++
		}
	}
	if generations == 0 {
		t.Fatal("no generation to check")
	}
}

// listing returns the directories (root first) and files below root.
func listing(t *testing.T, root string) (dirs, files []string) {
	t.Helper()
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			dirs = append(dirs, p)
		} else {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return dirs, files
}

// mutate applies one structural change, chosen by rng; every choice changes
// the fingerprint.
func mutate(t *testing.T, rng *rand.Rand, root string, step int) string {
	t.Helper()
	dirs, files := listing(t, root)
	pick := func(list []string) string { return list[rng.IntN(len(list))] }
	inner := dirs[1:]
	op := rng.IntN(6)
	switch {
	case op == 2 && len(files) > 0:
		f := pick(files)
		if err := os.Remove(f); err != nil {
			t.Fatal(err)
		}
		return "remove file"
	case op == 3 && len(inner) > 0:
		d := pick(inner)
		if err := os.RemoveAll(d); err != nil {
			t.Fatal(err)
		}
		return "remove tree"
	case op == 4 && len(files) > 0:
		f := pick(files)
		if err := os.Rename(f, filepath.Join(filepath.Dir(f), fmt.Sprintf("r%d", step))); err != nil {
			t.Fatal(err)
		}
		return "rename file"
	case op == 5 && len(inner) > 0:
		src := pick(inner)
		var targets []string
		for _, d := range dirs {
			if rel, err := filepath.Rel(src, d); err != nil || rel == "." || !strings.HasPrefix(rel, "..") {
				continue
			}
			targets = append(targets, d)
		}
		if len(targets) > 0 {
			if err := os.Rename(src, filepath.Join(pick(targets), fmt.Sprintf("m%d", step))); err != nil {
				t.Fatal(err)
			}
			return "move tree"
		}
	case op == 1:
		mkdirAll(t, filepath.Join(pick(dirs), fmt.Sprintf("d%d", step)))
		return "mkdir"
	}
	writeFiles(t, filepath.Join(pick(dirs), fmt.Sprintf("f%d", step)))
	return "create file"
}

func TestWatchPollingMatchesDiffAndConverges(t *testing.T) {
	for _, seed := range []uint64{1, 2, 3} {
		t.Run(strconv.FormatUint(seed, 10), func(t *testing.T) {
			root := t.TempDir()
			mkdirAll(t, filepath.Join(root, "a", "b"), filepath.Join(root, "c"))
			writeFiles(t, filepath.Join(root, "a", "x"), filepath.Join(root, "a", "b", "y"), filepath.Join(root, "c", "z"), filepath.Join(root, "top"))
			request := pollRequest(root)
			w := startWatch(t, request)
			w.log.waitState(t, fingerprintOf(t, request.Inspect))
			rng := rand.New(rand.NewPCG(seed, 0xa8))
			var ops []string
			for step := 0; step < 25; step++ {
				ops = append(ops, mutate(t, rng, root, step))
				w.log.waitState(t, fingerprintOf(t, request.Inspect))
			}
			if err := w.interrupt(t); err != nil {
				t.Fatalf("Run = %v", err)
			}
			events := w.log.recorded()
			if err := watch.ValidateStream(events, true); err != nil {
				t.Fatalf("invalid stream after %v: %v", ops, err)
			}
			checkDiffEquivalence(t, events, w.obs)
			if events[0].Metadata.Backend != watch.BackendPoll {
				t.Fatalf("metadata = %+v", events[0].Metadata)
			}
		})
	}
}

func TestWatchStartedMatchesFingerprintAndCapture(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, "a", "b", "c"), filepath.Join(root, "node_modules", "x"))
	writeFiles(t, filepath.Join(root, "a", "keep"), filepath.Join(root, "a", "drop.log"), filepath.Join(root, ".hidden"))
	depth := 2
	request := pollRequest(root)
	request.Inspect.MaxDepth = &depth
	request.Inspect.IgnorePatterns = []string{"*.log"}
	w := startWatch(t, request)
	want := fingerprintOf(t, request.Inspect)
	w.log.waitState(t, want)
	_ = w.interrupt(t)
	started := w.log.recorded()[0]
	if started.Fingerprint != want {
		t.Fatalf("watch.started fingerprint %s, dirloom fingerprint %s", started.Fingerprint, want)
	}
	if capture := CaptureFromInspect(request.Inspect); !reflect.DeepEqual(started.Capture, capture) {
		t.Fatalf("capture %+v, want %+v", started.Capture, capture)
	}
	if started.NodeCount != 4 { // ., a, a/b, a/keep
		t.Fatalf("node count %d, want 4", started.NodeCount)
	}
}

func TestWatchDepthZeroReportsNoChildren(t *testing.T) {
	root := t.TempDir()
	depth := 0
	request := pollRequest(root)
	request.Inspect.MaxDepth = &depth
	w := startWatch(t, request)
	w.log.waitState(t, fingerprintOf(t, request.Inspect))
	mkdirAll(t, filepath.Join(root, "child"))
	w.obs.waitObservations(t, 3) // three polls after the change; nothing may be reported
	_ = w.interrupt(t)
	events := w.log.recorded()
	if len(events) != 2 || events[1].Kind != watch.KindStopped {
		t.Fatalf("events at --depth 0: %+v", events)
	}
}

func TestWatchRootLossEndsTheWatch(t *testing.T) {
	for _, tc := range []struct {
		name string
		lose func(root string) error
	}{
		{"removed", os.RemoveAll},
		{"renamed", func(root string) error { return os.Rename(root, root+"-moved") }},
		{"replaced", func(root string) error {
			if err := os.Rename(root, root+"-old"); err != nil {
				return err
			}
			return os.Mkdir(root, 0o755)
		}},
		{"recreated", func(root string) error {
			if err := os.RemoveAll(root); err != nil {
				return err
			}
			return os.Mkdir(root, 0o755)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "root")
			mkdirAll(t, filepath.Join(root, "a"))
			if tc.name == "recreated" {
				if id, _ := watch.IdentityOf(root); id.Birth == 0 && id.Handle == "" && runtime.GOOS != "windows" {
					t.Skip("neither a birth time nor a file handle on this filesystem: a recreated root may reuse its inode")
				}
			}
			request := pollRequest(root)
			w := startWatch(t, request)
			w.log.waitState(t, fingerprintOf(t, request.Inspect))
			if err := tc.lose(root); err != nil {
				t.Fatal(err)
			}
			var f *watch.Failure
			if err := w.end(t); !errors.As(err, &f) || f.Diagnostic.Code != artifact.CodeWatchRootUnavailable {
				t.Fatalf("Run = %v, want watch_root_unavailable", err)
			}
			events := w.log.recorded()
			if err := watch.ValidateStream(events, true); err != nil {
				t.Fatal(err)
			}
			if last := events[len(events)-2]; last.Kind != watch.KindError || !last.Fatal {
				t.Fatalf("events end with %+v", last)
			}
			assertNoRoot(t, events, root)
		})
	}
}

func TestWatchStartupFailures(t *testing.T) {
	base := t.TempDir()
	file := filepath.Join(base, "file")
	writeFiles(t, file)
	for _, tc := range []struct {
		name, root, message string
	}{
		{"missing", filepath.Join(base, "missing"), "the watched root does not exist"},
		{"file", file, "the watched root is not a directory"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := startWatch(t, pollRequest(tc.root))
			var f *watch.Failure
			if err := w.end(t); !errors.As(err, &f) || f.Diagnostic.Code != artifact.CodeWatchRootUnavailable || f.Diagnostic.Message != tc.message {
				t.Fatalf("Run = %v", err)
			}
			events := w.log.recorded()
			if len(events) != 2 || events[0].HasGeneration || events[1].Kind != watch.KindStopped {
				t.Fatalf("startup failure stream %+v", events)
			}
			assertNoRoot(t, events, tc.root)
		})
	}
}

func TestWatchObservationFailureRecovers(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("APFS does not keep two names that differ only in Unicode normalization")
	}
	root := t.TempDir()
	writeFiles(t, filepath.Join(root, "é"))
	request := pollRequest(root)
	w := startWatch(t, request)
	w.log.waitState(t, fingerprintOf(t, request.Inspect))
	decomposed := filepath.Join(root, "é")
	writeFiles(t, decomposed)
	w.log.waitFor(t, "a rescan for the collision", func(events []watch.Event) bool {
		last := events[len(events)-1]
		return last.Kind == watch.KindRescanStarted && last.Rescan == watch.RescanError
	})
	if err := os.Remove(decomposed); err != nil {
		t.Fatal(err)
	}
	w.log.waitFor(t, "the end of the rescan", func(events []watch.Event) bool {
		return events[len(events)-1].Kind == watch.KindRescanCompleted
	})
	if err := w.interrupt(t); err != nil {
		t.Fatalf("Run = %v", err)
	}
	events := w.log.recorded()
	if err := watch.ValidateStream(events, true); err != nil {
		t.Fatal(err)
	}
	if ev := events[1]; ev.Kind != watch.KindError || ev.Fatal || ev.Diagnostic.Code != artifact.CodeCollision {
		t.Fatalf("watch.error = %+v", ev)
	}
	assertNoRoot(t, events, root)
}

func TestWatchUsageErrors(t *testing.T) {
	negative := -1
	for _, tc := range []struct {
		name    string
		request WatchRequest
	}{
		{"poll too short", WatchRequest{Poll: 50 * time.Millisecond}},
		{"poll too long", WatchRequest{Poll: 2 * time.Hour}},
		{"negative poll", WatchRequest{Poll: -time.Second}},
		{"debounce with poll", WatchRequest{Poll: time.Second, Debounce: time.Second}},
		{"debounce too short", WatchRequest{Debounce: 5 * time.Millisecond}},
		{"debounce too long", WatchRequest{Debounce: time.Minute}},
		{"negative depth", WatchRequest{Poll: time.Second, Inspect: InspectRequest{MaxDepth: &negative}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			log := newEventLog()
			if err := Watch(context.Background(), tc.request, log); !WatchUsage(err) {
				t.Fatalf("Watch = %v, want a usage error", err)
			}
			if n := len(log.recorded()); n != 0 {
				t.Fatalf("%d events published for a usage error", n)
			}
		})
	}
}
