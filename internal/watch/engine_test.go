package watch

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/dirloom/dirloom/internal/snapshot"
)

const debounce = DefaultDebounce

func TestStartupPublishesStateThenStopsOnInterrupt(t *testing.T) {
	a := tree("a")
	depth := 3
	h := newHarness(t, observed(a))
	h.start(func(c *Config) { c.Capture = snapshot.CaptureV1{Depth: &depth, Ignore: []string{"*.log"}} })
	h.idle()
	if reconciles, _ := h.backend.counts(); reconciles != 1 {
		t.Fatalf("reconciles after startup = %d, want 1", reconciles)
	}
	if err := h.interrupt(); err != nil {
		t.Fatalf("Run = %v, want nil", err)
	}
	events := h.expect(true, "started", "stopped:interrupted")
	started := events[0]
	want := Metadata{WatchVersion: 1, Stability: "experimental", ComparisonVersion: 1, IdentityProjectionVersion: 1,
		MoveDetection: comparison.MoveDetectionStructuralV1, Backend: BackendNative}
	if started.Metadata != want || started.Fingerprint != a.Fingerprint || started.NodeCount != 2 ||
		*started.Capture.Depth != 3 || !reflect.DeepEqual(started.Capture.Ignore, []string{"*.log"}) {
		t.Fatalf("watch.started = %+v", started)
	}
	if stopped := events[1]; !stopped.HasGeneration || stopped.Generation != 0 || stopped.Fingerprint != a.Fingerprint {
		t.Fatalf("watch.stopped = %+v", stopped)
	}
	if _, closes := h.backend.counts(); closes != 1 {
		t.Fatalf("backend closed %d times, want 1", closes)
	}
}

func TestSignalPublishesGenerationEqualToDiff(t *testing.T) {
	before := tree("old/", "old/a", "old/b", "x")
	after := tree("new/", "new/a", "new/b", "x", "y")
	h := newHarness(t, observed(before), observed(after))
	h.start(nil)
	h.idle()
	h.backend.signals().Notify()
	h.until("debounce")
	h.clock.Advance(debounce)
	h.idle()
	events := h.expect(false, "started", "node.moved:new", "node.added:y", "state:1")
	want := changesOf(t, before, after, comparison.MoveDetectionStructuralV1)
	var got []comparison.Change
	for _, ev := range events[1:3] {
		got = append(got, ev.Change)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("node events %+v, want the diff %+v", got, want)
	}
	if state := events[3]; state.Fingerprint != after.Fingerprint || state.NodeCount != 6 || state.Summary.Total != 2 {
		t.Fatalf("state.changed = %+v", state)
	}
	if at := h.obs.observedAt(); !reflect.DeepEqual(at, []time.Duration{0, debounce}) {
		t.Fatalf("observed at %v", at)
	}
	_ = h.interrupt()
	h.expect(true, "started", "node.moved:new", "node.added:y", "state:1", "stopped:interrupted")
}

func TestNoMovesReportsRemovedAndAdded(t *testing.T) {
	h := newHarness(t, observed(tree("a")), observed(tree("b")))
	h.start(func(c *Config) { c.MoveDetection = comparison.MoveDetectionNone })
	h.idle()
	h.backend.signals().Notify()
	h.until("debounce")
	h.clock.Advance(debounce)
	h.idle()
	events := h.expect(false, "started", "node.removed:a", "node.added:b", "state:1")
	if events[0].Metadata.MoveDetection != comparison.MoveDetectionNone {
		t.Fatalf("metadata = %+v", events[0].Metadata)
	}
}

func TestUnchangedObservationPublishesNothing(t *testing.T) {
	h := newHarness(t, observed(tree("a")))
	h.start(nil)
	h.idle()
	h.backend.signals().Notify()
	h.until("debounce")
	h.clock.Advance(debounce)
	h.idle()
	h.expect(false, "started")
	if n := len(h.obs.observedAt()); n != 2 {
		t.Fatalf("observations = %d, want 2", n)
	}
}

func TestDebounceWaitsForQuiet(t *testing.T) {
	h := newHarness(t, observed(tree("a")))
	h.start(nil)
	h.idle()
	sig := h.backend.signals()
	sig.Notify()
	h.until("debounce")
	h.clock.Advance(150 * time.Millisecond)
	sig.Notify()
	h.until("debounce")
	h.clock.Advance(200 * time.Millisecond)
	h.idle()
	if at := h.obs.observedAt(); !reflect.DeepEqual(at, []time.Duration{0, 350 * time.Millisecond}) {
		t.Fatalf("observed at %v, want 0 and 350ms", at)
	}
}

func TestDebounceMaxDelayUnderContinuousActivity(t *testing.T) {
	h := newHarness(t, observed(tree("a")))
	h.start(nil)
	h.idle()
	sig := h.backend.signals()
	sig.Notify()
	h.until("debounce")
	for i := 0; i < 13; i++ {
		h.clock.Advance(150 * time.Millisecond)
		sig.Notify()
		h.until("debounce")
	}
	h.clock.Advance(50 * time.Millisecond)
	h.idle()
	if at := h.obs.observedAt(); !reflect.DeepEqual(at, []time.Duration{0, MaxDelayFactor * debounce}) {
		t.Fatalf("observed at %v, want 0 and %v", at, MaxDelayFactor*debounce)
	}
}

func TestSignalDuringObservationCausesAnotherCycle(t *testing.T) {
	h := newHarness(t, observed(tree("a")))
	h.obs.steps = append(h.obs.steps, step{obs: tree("a", "b"), during: func() { h.backend.signals().Notify() }})
	h.start(nil)
	h.idle()
	h.backend.signals().Notify()
	h.until("debounce")
	h.clock.Advance(debounce)
	h.until("debounce")
	h.clock.Advance(debounce)
	h.idle()
	h.expect(false, "started", "node.added:b", "state:1")
	if n := len(h.obs.observedAt()); n != 3 {
		t.Fatalf("observations = %d, want 3", n)
	}
}

func TestOverflowOpensRescan(t *testing.T) {
	h := newHarness(t, observed(tree("a")), observed(tree("a", "b")))
	h.start(nil)
	h.idle()
	h.backend.signals().Overflow()
	h.idle()
	h.expect(false, "started", "rescan:overflow", "node.added:b", "state:1", "completed")
	if at := h.obs.observedAt(); !reflect.DeepEqual(at, []time.Duration{0, 0}) {
		t.Fatalf("observed at %v: an overflow does not wait for the debounce", at)
	}
	h.backend.signals().Overflow()
	h.idle()
	h.expect(false, "started", "rescan:overflow", "node.added:b", "state:1", "completed", "rescan:overflow", "completed")
}

func TestReconciliationIsSilentWhenUnchanged(t *testing.T) {
	h := newHarness(t, observed(tree("a")))
	h.start(nil)
	h.idle()
	h.clock.Advance(ReconcileInterval)
	h.idle()
	h.clock.Advance(ReconcileInterval)
	h.idle()
	h.expect(false, "started")
	if at := h.obs.observedAt(); !reflect.DeepEqual(at, []time.Duration{0, ReconcileInterval, 2 * ReconcileInterval}) {
		t.Fatalf("observed at %v", at)
	}
}

func TestReconciliationReportsMissedChange(t *testing.T) {
	h := newHarness(t, observed(tree("a")), observed(tree("a", "b")))
	h.start(nil)
	h.idle()
	h.clock.Advance(ReconcileInterval)
	h.until("settle")
	h.clock.Advance(debounce)
	h.idle()
	h.expect(false, "started", "rescan:reconciliation", "node.added:b", "state:1", "completed")
}

func TestReconciliationDifferenceExplainedBySignal(t *testing.T) {
	h := newHarness(t, observed(tree("a")), observed(tree("a", "b")))
	h.start(nil)
	h.idle()
	h.clock.Advance(ReconcileInterval)
	h.until("settle")
	h.backend.signals().Notify()
	h.until("debounce")
	h.clock.Advance(debounce)
	h.idle()
	h.expect(false, "started", "node.added:b", "state:1")
	if n := len(h.obs.observedAt()); n != 3 {
		t.Fatalf("observations = %d, want 3", n)
	}
}

func TestReconciliationRestartsAfterEveryObservation(t *testing.T) {
	h := newHarness(t, observed(tree("a")))
	h.start(nil)
	h.idle()
	h.clock.Advance(ReconcileInterval - time.Second)
	h.backend.signals().Notify()
	h.until("debounce")
	h.clock.Advance(debounce)
	h.idle()
	h.clock.Advance(time.Second)
	h.backend.signals().Notify() // wakes the engine, which must not reconcile yet
	h.until("debounce")
	h.clock.Advance(debounce)
	h.idle()
	want := []time.Duration{0, ReconcileInterval - time.Second + debounce, ReconcileInterval + 2*debounce}
	if at := h.obs.observedAt(); !reflect.DeepEqual(at, want) {
		t.Fatalf("observed at %v, want %v", at, want)
	}
}

func TestObservationFailuresOpenOneRescanAndBackOff(t *testing.T) {
	h := newHarness(t,
		observed(tree("a")),
		failing(readFailure("entry no longer exists")),
		failing(readFailure("entry no longer exists")),
		failing(codedError(artifact.CodeCollision)),
		observed(tree("a", "b")),
	)
	h.start(nil)
	h.idle()
	h.backend.signals().Notify()
	h.until("debounce")
	h.clock.Advance(debounce)
	for _, wait := range []time.Duration{200, 400, 800} {
		h.idle()
		h.clock.Advance(wait * time.Millisecond)
	}
	h.idle()
	events := h.expect(false, "started", "error:source_read_failure", "rescan:error", "error:canonical_path_collision",
		"node.added:b", "state:1", "completed")
	if events[1].Fatal || events[1].Diagnostic.Message != "entry no longer exists" || events[1].Generation != 0 {
		t.Fatalf("watch.error = %+v", events[1])
	}
	want := []time.Duration{0, 200, 400, 800, 1600}
	for i := range want {
		want[i] *= time.Millisecond
	}
	if at := h.obs.observedAt(); !reflect.DeepEqual(at, want) {
		t.Fatalf("observed at %v, want %v", at, want)
	}
}

func TestErrorBackoffDoublesUpToTheCap(t *testing.T) {
	steps := []step{observed(tree("a"))}
	for i := 0; i < 7; i++ {
		steps = append(steps, failing(readFailure("busy")))
	}
	steps = append(steps, observed(tree("a")))
	h := newHarness(t, steps...)
	h.start(nil)
	h.idle()
	h.backend.signals().Notify()
	h.until("debounce")
	h.clock.Advance(debounce)
	gaps := []time.Duration{200, 400, 800, 1600, 3200, 5000, 5000}
	for _, gap := range gaps {
		h.idle()
		h.clock.Advance(gap * time.Millisecond)
	}
	h.idle()
	at := h.obs.observedAt()
	for i, gap := range gaps {
		if got := at[i+2] - at[i+1]; got != gap*time.Millisecond {
			t.Fatalf("gap %d = %v, want %v (observed at %v)", i, got, gap*time.Millisecond, at)
		}
	}
	h.expect(false, "started", "error:source_read_failure", "rescan:error", "completed")
}

func TestFailureDuringOverflowRescanKeepsItOpen(t *testing.T) {
	h := newHarness(t, observed(tree("a")), failing(readFailure("busy")), observed(tree("a", "b")))
	h.start(nil)
	h.idle()
	h.backend.signals().Overflow()
	h.idle()
	h.clock.Advance(debounce)
	h.idle()
	h.expect(false, "started", "rescan:overflow", "error:source_read_failure", "node.added:b", "state:1", "completed")
}

func TestStartupRetriesSilently(t *testing.T) {
	h := newHarness(t, failing(readFailure("busy")), failing(readFailure("busy")), observed(tree("a")))
	h.start(nil)
	h.until("startup-retry")
	h.clock.Advance(200 * time.Millisecond)
	h.until("startup-retry")
	h.clock.Advance(400 * time.Millisecond)
	h.idle()
	h.expect(false, "started")
}

func TestStartupFailureAfterRetryLimit(t *testing.T) {
	h := newHarness(t, failing(readFailure("busy")))
	h.start(nil)
	for _, wait := range []time.Duration{200, 400, 800, 1600} {
		h.until("startup-retry")
		h.clock.Advance(wait * time.Millisecond)
	}
	var f *Failure
	if err := h.finish(); !errors.As(err, &f) || f.Diagnostic.Code != artifact.CodeSourceRead || f.Internal {
		t.Fatalf("Run = %v, want a source_read_failure Failure", err)
	}
	events := h.expect(true, "error:source_read_failure!", "stopped:error")
	if events[0].HasGeneration || events[1].HasGeneration {
		t.Fatalf("a startup failure has no generation: %+v", events)
	}
	if _, closes := h.backend.counts(); closes != 1 {
		t.Fatalf("backend closed %d times", closes)
	}
}

func TestStartupRootUnavailableIsImmediate(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(h *harness)
	}{
		{"observe", func(h *harness) { h.obs.steps = []step{failing(codedError(artifact.CodeWatchRootUnavailable))} }},
		{"check", func(h *harness) { h.obs.root = codedError(artifact.CodeWatchRootUnavailable) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, observed(tree("a")))
			tc.edit(h)
			h.start(nil)
			var f *Failure
			if err := h.finish(); !errors.As(err, &f) || f.Diagnostic.Code != artifact.CodeWatchRootUnavailable {
				t.Fatalf("Run = %v", err)
			}
			h.expect(true, "error:watch_root_unavailable!", "stopped:error")
		})
	}
}

func TestBackendStartFailureIsAStartupFailure(t *testing.T) {
	h := newHarness(t, observed(tree("a")))
	h.backend.startErr = codedError(artifact.CodeWatchFilesystemUnsupported)
	h.start(nil)
	var f *Failure
	if err := h.finish(); !errors.As(err, &f) || f.Diagnostic.Code != artifact.CodeWatchFilesystemUnsupported {
		t.Fatalf("Run = %v", err)
	}
	h.expect(true, "error:watch_filesystem_unsupported!", "stopped:error")
	if n := len(h.obs.observedAt()); n != 0 {
		t.Fatalf("observed %d times before a refused backend", n)
	}
}

func TestRootLossEndsTheWatch(t *testing.T) {
	a := tree("a")
	h := newHarness(t, observed(a))
	h.start(nil)
	h.idle()
	h.obs.setRoot(codedError(artifact.CodeWatchRootUnavailable))
	h.backend.signals().Notify()
	h.until("debounce")
	h.clock.Advance(debounce)
	var f *Failure
	if err := h.finish(); !errors.As(err, &f) || f.Diagnostic.Code != artifact.CodeWatchRootUnavailable || f.Internal {
		t.Fatalf("Run = %v", err)
	}
	events := h.expect(true, "started", "error:watch_root_unavailable!", "stopped:error")
	if stopped := events[2]; stopped.Generation != 0 || stopped.Fingerprint != a.Fingerprint {
		t.Fatalf("watch.stopped = %+v", stopped)
	}
}

func TestBackendFailureEndsTheWatch(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code string
	}{
		{codedError(artifact.CodeWatchLimitReached), artifact.CodeWatchLimitReached},
		{errors.New("read failed"), artifact.CodeWatchBackendFailure},
	} {
		t.Run(tc.code, func(t *testing.T) {
			h := newHarness(t, observed(tree("a")))
			h.start(nil)
			h.idle()
			h.backend.signals().Fail(tc.err)
			var f *Failure
			if err := h.finish(); !errors.As(err, &f) || f.Diagnostic.Code != tc.code {
				t.Fatalf("Run = %v", err)
			}
			h.expect(true, "started", "error:"+tc.code+"!", "stopped:error")
		})
	}
}

func TestRegistrationChangeSchedulesVerification(t *testing.T) {
	h := newHarness(t, observed(tree("a")), observed(tree("a", "b")))
	h.backend.results = []reconcileResult{{changed: true}}
	h.start(nil)
	h.until("debounce")
	h.clock.Advance(debounce)
	h.idle()
	h.expect(false, "started", "node.added:b", "state:1")
	if reconciles, _ := h.backend.counts(); reconciles != 2 {
		t.Fatalf("reconciles = %d, want 2", reconciles)
	}
}

func TestRegistrationErrorEndsTheWatch(t *testing.T) {
	t.Run("startup", func(t *testing.T) {
		h := newHarness(t, observed(tree("a")))
		h.backend.results = []reconcileResult{{err: codedError(artifact.CodeWatchLimitReached)}}
		h.start(nil)
		if err := h.finish(); err == nil {
			t.Fatal("Run = nil")
		}
		h.expect(true, "error:watch_limit_reached!", "stopped:error")
	})
	t.Run("running", func(t *testing.T) {
		h := newHarness(t, observed(tree("a")), observed(tree("a", "d/")))
		h.backend.results = []reconcileResult{{}, {err: codedError(artifact.CodeWatchLimitReached)}}
		h.start(nil)
		h.idle()
		h.backend.signals().Notify()
		h.until("debounce")
		h.clock.Advance(debounce)
		if err := h.finish(); err == nil {
			t.Fatal("Run = nil")
		}
		h.expect(true, "started", "node.added:d", "state:1", "error:watch_limit_reached!", "stopped:error")
	})
}

func TestBrokenInvariantIsInternal(t *testing.T) {
	a := tree("a")
	forged := Observation{Records: a.Records, Fingerprint: tree("b").Fingerprint}
	h := newHarness(t, observed(a), observed(forged))
	h.start(nil)
	h.idle()
	h.backend.signals().Notify()
	h.until("debounce")
	h.clock.Advance(debounce)
	var f *Failure
	if err := h.finish(); !errors.As(err, &f) || !f.Internal || f.Diagnostic.Code != artifact.CodeInternal {
		t.Fatalf("Run = %v, want an internal Failure", err)
	}
	h.expect(true, "started", "error:internal_invariant_failure!", "stopped:error")
}

func TestInterruptDuringObservationPublishesNoGeneration(t *testing.T) {
	h := newHarness(t, observed(tree("a")), step{block: true})
	h.start(nil)
	h.idle()
	h.backend.signals().Notify()
	h.until("debounce")
	h.clock.Advance(debounce)
	h.entered()
	if err := h.interrupt(); err != nil {
		t.Fatalf("Run = %v", err)
	}
	h.expect(true, "started", "stopped:interrupted")
}

func TestInterruptBeforeTheFirstState(t *testing.T) {
	h := newHarness(t, step{block: true})
	h.start(nil)
	h.entered()
	if err := h.interrupt(); err != nil {
		t.Fatalf("Run = %v", err)
	}
	events := h.expect(true, "stopped:interrupted")
	if events[0].HasGeneration {
		t.Fatalf("watch.stopped = %+v", events[0])
	}
}

// blockGeneration starts a watch whose sink blocks from the first
// generation on, and stops while that generation is being written.
func blockGeneration(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t, observed(tree("a")), observed(tree("a", "b")))
	h.sink.gate, h.sink.blockFrom = make(chan struct{}), 2
	h.start(nil)
	h.idle()
	h.backend.signals().Notify()
	h.until("debounce")
	h.clock.Advance(debounce)
	h.until("publish")
	return h
}

func TestBlockedOutputEndsAfterTheGracePeriod(t *testing.T) {
	h := blockGeneration(t)
	h.cancel()
	h.until("stop")
	h.clock.Advance(OutputGrace)
	if err := h.finish(); !errors.Is(err, ErrOutputBlocked) {
		t.Fatalf("Run = %v, want ErrOutputBlocked", err)
	}
	h.expect(false, "started")
	if _, closes := h.backend.counts(); closes != 1 {
		t.Fatalf("backend closed %d times", closes)
	}
	close(h.sink.gate)
}

func TestBlockedOutputReleasedWithinTheGracePeriod(t *testing.T) {
	h := blockGeneration(t)
	h.cancel()
	h.until("stop")
	close(h.sink.gate)
	if err := h.finish(); err != nil {
		t.Fatalf("Run = %v", err)
	}
	h.expect(true, "started", "node.added:b", "state:1", "stopped:interrupted")
}

func TestFatalSignalWhileOutputIsBlocked(t *testing.T) {
	h := blockGeneration(t)
	h.backend.signals().Fail(codedError(artifact.CodeWatchBackendFailure))
	h.until("stop")
	h.clock.Advance(OutputGrace)
	if err := h.finish(); !errors.Is(err, ErrOutputBlocked) {
		t.Fatalf("Run = %v, want ErrOutputBlocked", err)
	}
	close(h.sink.gate)
}

func TestOutputFailureEndsWithoutMoreOutput(t *testing.T) {
	h := newHarness(t, observed(tree("a")), observed(tree("a", "b")))
	h.sink.failAt = 2
	h.start(nil)
	h.idle()
	h.backend.signals().Notify()
	h.until("debounce")
	h.clock.Advance(debounce)
	var out *OutputError
	if err := h.finish(); !errors.As(err, &out) {
		t.Fatalf("Run = %v, want an OutputError", err)
	}
	h.expect(false, "started")
	if _, closes := h.backend.counts(); closes != 1 {
		t.Fatalf("backend closed %d times", closes)
	}
}

func TestPollingObservesEveryInterval(t *testing.T) {
	h := newHarness(t, observed(tree("a")), observed(tree("a")), failing(readFailure("busy")), observed(tree("a", "b")))
	h.start(func(c *Config) { c.Poll = time.Second })
	h.idle()
	h.clock.Advance(time.Second)
	h.idle()
	h.clock.Advance(time.Second)
	h.idle()
	h.clock.Advance(DefaultDebounce)
	h.idle()
	events := h.expect(false, "started", "error:source_read_failure", "rescan:error", "node.added:b", "state:1", "completed")
	if events[0].Metadata.Backend != BackendPoll {
		t.Fatalf("metadata = %+v", events[0].Metadata)
	}
	want := []time.Duration{0, time.Second, 2 * time.Second, 2*time.Second + DefaultDebounce}
	if at := h.obs.observedAt(); !reflect.DeepEqual(at, want) {
		t.Fatalf("observed at %v, want %v", at, want)
	}
	h.clock.Advance(ReconcileInterval - time.Second)
	h.idle()
	h.expect(false, "started", "error:source_read_failure", "rescan:error", "node.added:b", "state:1", "completed")
}

func TestNewRejectsInvalidConfigs(t *testing.T) {
	obs, sink := &fakeObserver{}, &fakeSink{}
	for _, tc := range []struct {
		name string
		cfg  Config
	}{
		{"no observer", Config{Sink: sink, Poll: time.Second}},
		{"no sink", Config{Observer: obs, Poll: time.Second}},
		{"neither backend nor poll", Config{Observer: obs, Sink: sink}},
		{"both backend and poll", Config{Observer: obs, Sink: sink, Backend: &fakeBackend{}, Poll: time.Second}},
		{"move detection", Config{Observer: obs, Sink: sink, Poll: time.Second, MoveDetection: "fuzzy"}},
		{"negative duration", Config{Observer: obs, Sink: sink, Poll: time.Second, Grace: -time.Second}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := New(tc.cfg); !artifact.IsInternal(err) {
				t.Fatalf("New = %v, want an internal error", err)
			}
		})
	}
}
