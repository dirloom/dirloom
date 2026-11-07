package watch

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/dirloom/dirloom/internal/identity"
)

// watchdog bounds every wait of a test on the engine. It is a safety net
// against a hung test, never a timing assumption: the engine runs on the
// fake clock.
const watchdog = 10 * time.Second

var epoch = time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)

// fakeClock only moves when a test advances it.
type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers map[*fakeTimer]struct{}
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: epoch, timers: map[*fakeTimer]struct{}{}}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) NewTimer(d time.Duration) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{clock: c, at: c.now.Add(d), ch: make(chan time.Time, 1)}
	if d <= 0 {
		t.ch <- c.now
		return t
	}
	c.timers[t] = struct{}{}
	return t
}

// Advance moves the clock and fires every timer that is due.
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	for t := range c.timers {
		if !t.at.After(c.now) {
			t.ch <- c.now
			delete(c.timers, t)
		}
	}
}

type fakeTimer struct {
	clock *fakeClock
	at    time.Time
	ch    chan time.Time
}

func (t *fakeTimer) C() <-chan time.Time { return t.ch }

func (t *fakeTimer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	_, ok := t.clock.timers[t]
	delete(t.clock.timers, t)
	return ok
}

// step is one scripted observation.
type step struct {
	obs    Observation
	err    error
	block  bool   // wait for cancellation
	during func() // runs inside Observe, before it returns
}

func observed(obs Observation) step { return step{obs: obs} }
func failing(err error) step        { return step{err: err} }

// fakeObserver plays its steps in order, then repeats the last one.
type fakeObserver struct {
	clock   *fakeClock
	entered chan struct{} // receives once per blocking step

	mu    sync.Mutex
	steps []step
	last  step
	root  error
	times []time.Time
}

func (o *fakeObserver) CheckRoot() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.root
}

func (o *fakeObserver) setRoot(err error) {
	o.mu.Lock()
	o.root = err
	o.mu.Unlock()
}

func (o *fakeObserver) Observe(ctx context.Context) (Observation, error) {
	o.mu.Lock()
	o.times = append(o.times, o.clock.Now())
	s := o.last
	if len(o.steps) > 0 {
		s, o.steps = o.steps[0], o.steps[1:]
	}
	o.last = step{obs: s.obs, err: s.err} // repeated without its side effects
	o.mu.Unlock()
	if s.during != nil {
		s.during()
	}
	if s.block {
		o.entered <- struct{}{}
		<-ctx.Done()
		return Observation{}, ctx.Err()
	}
	return s.obs, s.err
}

// observedAt returns the offsets from the epoch at which Observe was called.
func (o *fakeObserver) observedAt() []time.Duration {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]time.Duration, len(o.times))
	for i, t := range o.times {
		out[i] = t.Sub(epoch)
	}
	return out
}

type reconcileResult struct {
	changed bool
	err     error
}

// fakeBackend records its calls and plays scripted reconciliations.
type fakeBackend struct {
	startErr error

	mu         sync.Mutex
	sig        Signals
	results    []reconcileResult
	reconciles int
	closes     int
}

func (b *fakeBackend) Start(s Signals) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sig = s
	return b.startErr
}

func (b *fakeBackend) Reconcile(Observation) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.reconciles++
	if len(b.results) == 0 {
		return false, nil
	}
	r := b.results[0]
	b.results = b.results[1:]
	return r.changed, r.err
}

func (b *fakeBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closes++
	return nil
}

func (b *fakeBackend) signals() Signals {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.sig
}

func (b *fakeBackend) counts() (reconciles, closes int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.reconciles, b.closes
}

// fakeSink records published events. Batches from blockFrom on wait for
// gate; batch failAt fails; batch panicAt panics.
type fakeSink struct {
	gate      chan struct{}
	blockFrom int
	failAt    int
	panicAt   int

	mu      sync.Mutex
	batches int
	events  []Event
}

func (s *fakeSink) Publish(events []Event) error {
	s.mu.Lock()
	s.batches++
	n := s.batches
	s.mu.Unlock()
	if s.gate != nil && n >= s.blockFrom {
		<-s.gate
	}
	if n == s.failAt {
		return errors.New("broken pipe")
	}
	if n == s.panicAt {
		panic("sink bug")
	}
	s.mu.Lock()
	s.events = append(s.events, events...)
	s.mu.Unlock()
	return nil
}

func (s *fakeSink) recorded() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Event(nil), s.events...)
}

// harness runs one engine on fakes.
type harness struct {
	t       *testing.T
	clock   *fakeClock
	obs     *fakeObserver
	backend *fakeBackend
	sink    *fakeSink
	points  chan string
	cancel  context.CancelFunc
	result  chan error
}

func newHarness(t *testing.T, steps ...step) *harness {
	t.Helper()
	clock := newFakeClock()
	return &harness{
		t:       t,
		clock:   clock,
		obs:     &fakeObserver{clock: clock, steps: steps, entered: make(chan struct{}, 1)},
		backend: &fakeBackend{},
		sink:    &fakeSink{},
		points:  make(chan string, 4096),
		result:  make(chan error, 1),
	}
}

// start runs the engine in native mode, or in polling mode when edit sets a
// poll interval.
func (h *harness) start(edit func(*Config)) {
	h.t.Helper()
	cfg := Config{
		Observer: h.obs,
		Backend:  h.backend,
		Sink:     h.sink,
		Clock:    h.clock,
		trace:    func(p string) { h.points <- p },
	}
	if edit != nil {
		edit(&cfg)
	}
	if cfg.Poll > 0 {
		cfg.Backend = nil
	}
	e, err := New(cfg)
	if err != nil {
		h.t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	h.t.Cleanup(cancel)
	go func() { h.result <- e.Run(ctx) }()
}

// until consumes trace points up to the next occurrence of point.
func (h *harness) until(point string) {
	h.t.Helper()
	timeout := time.After(watchdog)
	for {
		select {
		case p := <-h.points:
			if p == point {
				return
			}
		case err := <-h.result:
			h.t.Fatalf("engine ended (%v) while waiting for %q", err, point)
		case <-timeout:
			h.t.Fatalf("no %q point within the watchdog", point)
		}
	}
}

// idle waits until the engine waits for a trigger.
func (h *harness) idle() { h.t.Helper(); h.until("trigger") }

func (h *harness) finish() error {
	h.t.Helper()
	select {
	case err := <-h.result:
		return err
	case <-time.After(watchdog):
		h.t.Fatal("engine did not end within the watchdog")
		return nil
	}
}

func (h *harness) interrupt() error {
	h.t.Helper()
	h.cancel()
	return h.finish()
}

func (h *harness) entered() {
	h.t.Helper()
	select {
	case <-h.obs.entered:
	case <-time.After(watchdog):
		h.t.Fatal("observation did not start within the watchdog")
	}
}

// expect compares the recorded events, described compactly, and validates
// the stream.
func (h *harness) expect(complete bool, want ...string) []Event {
	h.t.Helper()
	events := h.sink.recorded()
	got := describe(events)
	if strings.Join(got, " ") != strings.Join(want, " ") {
		h.t.Fatalf("events:\n got  %v\n want %v", got, want)
	}
	if err := ValidateStream(events, complete); err != nil {
		h.t.Fatalf("invalid stream: %v", err)
	}
	return events
}

// describe renders events as "started", "node.added:b", "state:1",
// "rescan:overflow", "completed", "error:<code>" ("!" when fatal),
// "stopped:<reason>".
func describe(events []Event) []string {
	out := make([]string, 0, len(events))
	for _, ev := range events {
		switch {
		case ev.Kind == KindStarted:
			out = append(out, "started")
		case ev.Kind.IsNode():
			out = append(out, fmt.Sprintf("%s:%s", ev.Kind, ev.Change.Path))
		case ev.Kind == KindStateChanged:
			out = append(out, fmt.Sprintf("state:%d", ev.Generation))
		case ev.Kind == KindRescanStarted:
			out = append(out, "rescan:"+string(ev.Rescan))
		case ev.Kind == KindRescanCompleted:
			out = append(out, "completed")
		case ev.Kind == KindError:
			s := "error:" + ev.Diagnostic.Code
			if ev.Fatal {
				s += "!"
			}
			out = append(out, s)
		case ev.Kind == KindStopped:
			out = append(out, "stopped:"+string(ev.Stop))
		}
	}
	return out
}

// tree builds an observation: the root plus each path, a directory when it
// ends with "/".
func tree(paths ...string) Observation {
	records := []identity.Record{{Path: artifact.RootPath, Kind: artifact.KindDirectory}}
	for _, p := range paths {
		kind := artifact.KindFile
		if strings.HasSuffix(p, "/") {
			kind, p = artifact.KindDirectory, strings.TrimSuffix(p, "/")
		}
		records = append(records, identity.Record{Path: artifact.Path(p), Kind: kind})
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Path < records[j].Path })
	fp, err := identity.FingerprintRecords(records)
	if err != nil {
		panic(err)
	}
	return Observation{Records: records, Fingerprint: fp}
}

func readFailure(msg string) error { return artifact.SourceReadFailure(errors.New(msg)) }

func codedError(code string) error { return artifact.New(code, code+" message", false) }

// changesOf runs the comparison engine directly, for diff equivalence.
func changesOf(t *testing.T, a, b Observation, moves comparison.MoveDetection) []comparison.Change {
	t.Helper()
	ref := comparison.SourceRef{Kind: "filesystem"}
	diff, err := comparison.CompareRecords(a.Records, b.Records, ref, ref, comparison.Options{MoveDetection: moves})
	if err != nil {
		t.Fatalf("CompareRecords: %v", err)
	}
	return diff.Changes
}
