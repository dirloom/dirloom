package watch

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/dirloom/dirloom/internal/identity"
	"github.com/dirloom/dirloom/internal/snapshot"
	"github.com/dirloom/dirloom/internal/source"
)

// Timing of Structural Watch v1. The values may be tuned within the contract
// version, with a CHANGELOG entry.
const (
	DefaultDebounce   = 200 * time.Millisecond
	MinDebounce       = 10 * time.Millisecond
	MaxDebounce       = 10 * time.Second
	MinPoll           = 100 * time.Millisecond
	MaxPoll           = time.Hour
	MaxDelayFactor    = 10
	ReconcileInterval = 60 * time.Second
	MaxBackoff        = 5 * time.Second
	StartupRetryLimit = 5 * time.Second
	OutputGrace       = 2 * time.Second
)

// Observation is one full observation of the root: its Identity Projection
// v1 records, root included, and their Fingerprint v1.
type Observation struct {
	Records     []identity.Record
	Fingerprint identity.Fingerprint
}

// NodeCount is the number of records, root included.
func (o Observation) NodeCount() int { return len(o.Records) }

// Observer observes the root.
type Observer interface {
	// CheckRoot reports whether the root still has the identity captured at
	// startup. An error ends the watch (watch_root_unavailable).
	CheckRoot() error
	// Observe scans the whole root. It never returns a partial observation,
	// and returns promptly once ctx is canceled. An error with code
	// watch_root_unavailable, or an internal error, ends the watch; any
	// other error is retried.
	Observe(ctx context.Context) (Observation, error)
}

// Signals receives a backend's notifications. Its methods never block and
// are safe for concurrent use.
type Signals interface {
	// Notify reports a relevant notification.
	Notify()
	// Overflow reports lost notifications.
	Overflow()
	// Fail reports a condition that ends the watch.
	Fail(err error)
}

// Backend delivers native notifications. A polling watch has none.
type Backend interface {
	// Start registers the control set (and, for a recursive backend, the
	// root), then delivers notifications to signals until Close.
	Start(signals Signals) error
	// Reconcile brings the per-directory registrations in line with obs and
	// reports whether they changed, which schedules a verification
	// observation. An error ends the watch.
	Reconcile(obs Observation) (changed bool, err error)
	// Close stops the notifications and releases every handle.
	Close() error
}

// Sink publishes events. A call carries one generation (its node events and
// state.changed), control events, or both; it may block. The engine calls it
// from one goroutine, one call at a time.
type Sink interface {
	Publish(events []Event) error
}

// Clock lets the engine be driven without real time in tests.
type Clock interface {
	Now() time.Time
	NewTimer(d time.Duration) Timer
}

// Timer is a one-shot timer of a Clock.
type Timer interface {
	C() <-chan time.Time
	Stop() bool
}

// Config is one watch. Exactly one of Backend (native mode) and a positive
// Poll (polling mode) is set. Zero durations take the defaults above.
type Config struct {
	Observer      Observer
	Backend       Backend
	Sink          Sink
	Clock         Clock
	Capture       snapshot.CaptureV1
	MoveDetection comparison.MoveDetection
	Debounce      time.Duration
	Poll          time.Duration
	Reconcile     time.Duration
	Grace         time.Duration
	StartupRetry  time.Duration
	MaxBackoff    time.Duration

	// trace, set by tests only, is called before each blocking wait.
	trace func(point string)
}

// Failure ends a watch: Run published it as a fatal watch.error.
type Failure struct {
	Diagnostic Diagnostic
	Internal   bool
}

func (f *Failure) Error() string { return f.Diagnostic.Message }

// OutputError reports that the sink failed; the stream is incomplete.
type OutputError struct{ Err error }

func (e *OutputError) Error() string { return "write watch events: " + e.Err.Error() }
func (e *OutputError) Unwrap() error { return e.Err }

// ErrOutputBlocked reports that the sink stayed blocked beyond the output
// grace period after an interruption or a fatal condition; the stream is
// incomplete.
var ErrOutputBlocked = errors.New("watch output blocked beyond the grace period; watch.stopped not written")

var errInterrupted = errors.New("watch interrupted")

// Engine runs one watch.
type Engine struct {
	cfg   Config
	clock Clock
	sig   *signals
	out   *writer
	busy  bool // a batch is in the writer

	hasState    bool
	state       Observation
	gen         uint64
	rescan      bool
	reported    map[string]bool // error codes published in the open rescan
	backoff     time.Duration
	retryAt     time.Time // zero when no retry is armed
	lastSuccess time.Time
	nextPoll    time.Time
}

// New validates cfg and returns an engine for one Run.
func New(cfg Config) (*Engine, error) {
	if cfg.Observer == nil || cfg.Sink == nil {
		return nil, artifact.InternalError("watch needs an observer and a sink")
	}
	if (cfg.Backend == nil) == (cfg.Poll <= 0) {
		return nil, artifact.InternalError("watch needs exactly one of a backend and a poll interval")
	}
	if cfg.MoveDetection == "" {
		cfg.MoveDetection = comparison.MoveDetectionStructuralV1
	}
	if !cfg.MoveDetection.Valid() {
		return nil, artifact.InternalError("unsupported move detection %q", cfg.MoveDetection)
	}
	defaults := []struct {
		value *time.Duration
		def   time.Duration
	}{
		{&cfg.Debounce, DefaultDebounce},
		{&cfg.Reconcile, ReconcileInterval},
		{&cfg.Grace, OutputGrace},
		{&cfg.StartupRetry, StartupRetryLimit},
		{&cfg.MaxBackoff, MaxBackoff},
	}
	for _, d := range defaults {
		if *d.value < 0 {
			return nil, artifact.InternalError("negative watch duration %s", *d.value)
		}
		if *d.value == 0 {
			*d.value = d.def
		}
	}
	clock := cfg.Clock
	if clock == nil {
		clock = systemClock{}
	}
	return &Engine{cfg: cfg, clock: clock, sig: newSignals(clock), reported: map[string]bool{}}, nil
}

// Run watches until ctx is canceled or a fatal condition occurs. It returns
// nil after a controlled interruption whose watch.stopped was published, a
// *Failure after a published fatal watch.error and watch.stopped, an
// *OutputError when the sink failed, or ErrOutputBlocked when the sink
// stayed blocked beyond the grace period. Run is called once.
func (e *Engine) Run(ctx context.Context) error {
	e.out = startWriter(e.cfg.Sink)
	defer e.out.close()
	// The root's identity comes first (Structural Watch v1, startup): a root
	// that is missing or not a directory is watch_root_unavailable, whatever
	// a backend would make of it.
	if err := e.cfg.Observer.CheckRoot(); err != nil {
		return e.end(failure(err, artifact.CodeWatchRootUnavailable, false))
	}
	if e.cfg.Backend != nil {
		if err := e.cfg.Backend.Start(e.sig); err != nil {
			return e.end(failure(err, artifact.CodeWatchBackendFailure, false))
		}
	}
	obs, err := e.observeAtStartup(ctx)
	if err != nil {
		return e.end(err)
	}
	now := e.clock.Now()
	e.state, e.lastSuccess, e.nextPoll = obs, now, now.Add(e.cfg.Poll)
	if err := e.reconcileRegistrations(obs); err != nil {
		return e.end(err)
	}
	e.hasState = true
	if err := e.publish(ctx, []Event{e.started()}); err != nil {
		return e.end(err)
	}
	for {
		if err := e.cycle(ctx); err != nil {
			return e.end(err)
		}
	}
}

func (e *Engine) started() Event {
	backend := BackendNative
	if e.cfg.Backend == nil {
		backend = BackendPoll
	}
	return Event{
		Kind:          KindStarted,
		HasGeneration: true,
		Metadata: Metadata{
			WatchVersion:              Version,
			Stability:                 Stability,
			ComparisonVersion:         comparison.Version,
			IdentityProjectionVersion: identity.ProjectionVersion,
			MoveDetection:             e.cfg.MoveDetection,
			Backend:                   backend,
		},
		Capture:     e.cfg.Capture.Clone(),
		Fingerprint: e.state.Fingerprint,
		NodeCount:   e.state.NodeCount(),
	}
}

// observeAtStartup makes the first observation, retrying failures on the
// error backoff for at most StartupRetry.
func (e *Engine) observeAtStartup(ctx context.Context) (Observation, error) {
	deadline := e.clock.Now().Add(e.cfg.StartupRetry)
	for {
		if err := e.cfg.Observer.CheckRoot(); err != nil {
			return Observation{}, failure(err, artifact.CodeWatchRootUnavailable, false)
		}
		obs, err := e.cfg.Observer.Observe(ctx)
		if ctx.Err() != nil {
			return Observation{}, errInterrupted
		}
		if err == nil {
			e.backoff = 0
			return obs, nil
		}
		if fatalObservation(err) {
			return Observation{}, failure(err, artifact.CodeWatchObservationFailure, false)
		}
		e.nextBackoff()
		retry := e.clock.Now().Add(e.backoff)
		if retry.After(deadline) {
			return Observation{}, failure(err, artifact.CodeWatchObservationFailure, false)
		}
		if err := e.sleep(ctx, "startup-retry", retry); err != nil {
			return Observation{}, err
		}
	}
}

// sleep waits until t, cancellation, or a fatal signal.
func (e *Engine) sleep(ctx context.Context, point string, until time.Time) error {
	for {
		e.sig.drain()
		if err := e.sig.failure(); err != nil {
			return err
		}
		now := e.clock.Now()
		if !now.Before(until) {
			return nil
		}
		t := e.clock.NewTimer(until.Sub(now))
		e.trace(point)
		select {
		case <-t.C():
		case <-e.sig.wake:
		case <-ctx.Done():
			t.Stop()
			return errInterrupted
		}
		t.Stop()
	}
}

type trigger int

const (
	triggerSignal trigger = iota + 1
	triggerRetry
	triggerReconcile
	triggerPoll
)

// cycle waits for a trigger, observes, and publishes what changed.
func (e *Engine) cycle(ctx context.Context) error {
	trig, err := e.waitTrigger(ctx)
	if err != nil {
		return err
	}
	if err := e.openOverflowRescan(ctx); err != nil {
		return err
	}
	if trig == triggerSignal && e.cfg.Backend != nil {
		if err := e.debounce(ctx); err != nil {
			return err
		}
		if err := e.openOverflowRescan(ctx); err != nil {
			return err
		}
	}
	e.sig.clear()
	if err := e.cfg.Observer.CheckRoot(); err != nil {
		return failure(err, artifact.CodeWatchRootUnavailable, false)
	}
	obs, err := e.cfg.Observer.Observe(ctx)
	if ctx.Err() != nil {
		return errInterrupted
	}
	if err != nil {
		if fatalObservation(err) {
			return failure(err, artifact.CodeWatchObservationFailure, false)
		}
		return e.observationFailed(ctx, err)
	}
	e.retryAt, e.backoff, e.lastSuccess = time.Time{}, 0, e.clock.Now()
	if obs.Fingerprint != e.state.Fingerprint {
		if err := e.report(ctx, trig, obs); err != nil {
			return err
		}
	}
	if err := e.reconcileRegistrations(obs); err != nil {
		return err
	}
	if e.rescan && !e.sig.pending() && !e.sig.overflowed() {
		e.rescan, e.reported = false, map[string]bool{}
		return e.publish(ctx, []Event{{
			Kind:          KindRescanCompleted,
			HasGeneration: true,
			Generation:    e.gen,
			Fingerprint:   e.state.Fingerprint,
			NodeCount:     e.state.NodeCount(),
		}})
	}
	return nil
}

// waitTrigger blocks until a relevant signal, an overflow, a due timer,
// cancellation, or a fatal signal.
func (e *Engine) waitTrigger(ctx context.Context) (trigger, error) {
	for {
		e.sig.drain()
		if err := e.sig.failure(); err != nil {
			return 0, err
		}
		if ctx.Err() != nil {
			return 0, errInterrupted
		}
		if e.sig.pending() || e.sig.overflowed() {
			return triggerSignal, nil
		}
		kind, due := e.nextTimer()
		now := e.clock.Now()
		if kind != 0 && !now.Before(due) {
			e.consume(kind, now)
			return kind, nil
		}
		var timer Timer
		var fired <-chan time.Time
		if kind != 0 {
			timer = e.clock.NewTimer(due.Sub(now))
			fired = timer.C()
		}
		e.trace("trigger")
		select {
		case <-e.sig.wake:
		case <-fired:
		case <-ctx.Done():
		}
		if timer != nil {
			timer.Stop()
		}
	}
}

// nextTimer returns the earliest armed timer: the error retry, the poll,
// and, in native mode with no retry or rescan pending, the reconciliation.
func (e *Engine) nextTimer() (trigger, time.Time) {
	var kind trigger
	var due time.Time
	consider := func(k trigger, t time.Time) {
		if kind == 0 || t.Before(due) {
			kind, due = k, t
		}
	}
	if !e.retryAt.IsZero() {
		consider(triggerRetry, e.retryAt)
	}
	if e.cfg.Poll > 0 {
		consider(triggerPoll, e.nextPoll)
	} else if e.retryAt.IsZero() && !e.rescan {
		consider(triggerReconcile, e.lastSuccess.Add(e.cfg.Reconcile))
	}
	return kind, due
}

func (e *Engine) consume(kind trigger, now time.Time) {
	switch kind {
	case triggerRetry:
		e.retryAt = time.Time{}
	case triggerPoll:
		e.nextPoll = e.nextPoll.Add(e.cfg.Poll)
		if !e.nextPoll.After(now) {
			e.nextPoll = now.Add(e.cfg.Poll)
		}
	}
}

// debounce waits until no relevant signal arrived for Debounce, or until
// MaxDelayFactor × Debounce after the first pending one.
func (e *Engine) debounce(ctx context.Context) error {
	for {
		e.sig.drain()
		if err := e.sig.failure(); err != nil {
			return err
		}
		first, last, ok := e.sig.times()
		if !ok {
			return nil
		}
		due := last.Add(e.cfg.Debounce)
		if limit := first.Add(MaxDelayFactor * e.cfg.Debounce); limit.Before(due) {
			due = limit
		}
		now := e.clock.Now()
		if !now.Before(due) {
			return nil
		}
		t := e.clock.NewTimer(due.Sub(now))
		e.trace("debounce")
		select {
		case <-t.C():
		case <-e.sig.wake:
		case <-ctx.Done():
			t.Stop()
			return errInterrupted
		}
		t.Stop()
	}
}

func (e *Engine) openOverflowRescan(ctx context.Context) error {
	if !e.sig.overflowed() || e.rescan {
		return nil
	}
	e.rescan = true
	return e.publish(ctx, []Event{e.rescanStarted(RescanOverflow)})
}

func (e *Engine) rescanStarted(reason RescanReason) Event {
	return Event{Kind: KindRescanStarted, HasGeneration: true, Generation: e.gen, Rescan: reason}
}

// report compares obs with the state and publishes the generation.
func (e *Engine) report(ctx context.Context, trig trigger, obs Observation) error {
	ref := comparison.SourceRef{Kind: source.KindFilesystem}
	diff, err := comparison.CompareRecords(e.state.Records, obs.Records, ref, ref, comparison.Options{MoveDetection: e.cfg.MoveDetection})
	if err != nil {
		return failure(err, artifact.CodeWatchInternalFailure, true)
	}
	if len(diff.Changes) == 0 {
		return failure(artifact.InternalError("fingerprints %s and %s differ without a structural change", e.state.Fingerprint, obs.Fingerprint), artifact.CodeWatchInternalFailure, true)
	}
	batch := make([]Event, 0, len(diff.Changes)+2)
	if trig == triggerReconcile && !e.rescan {
		missed, err := e.missedNotification(ctx)
		if err != nil {
			return err
		}
		if missed {
			e.rescan = true
			batch = append(batch, e.rescanStarted(RescanReconciliation))
		}
	}
	gen := e.gen + 1
	for _, change := range diff.Changes {
		batch = append(batch, Event{Kind: nodeKind(change.Op), HasGeneration: true, Generation: gen, Change: change})
	}
	batch = append(batch, Event{
		Kind:          KindStateChanged,
		HasGeneration: true,
		Generation:    gen,
		Fingerprint:   obs.Fingerprint,
		NodeCount:     obs.NodeCount(),
		Summary:       diff.Summary,
	})
	e.state, e.gen = obs, gen
	return e.publish(ctx, batch)
}

// missedNotification reports whether a reconciliation difference has no
// relevant signal to explain it, from the start of the observation (when the
// pending flag was cleared) until one debounce after its end.
func (e *Engine) missedNotification(ctx context.Context) (bool, error) {
	t := e.clock.NewTimer(e.cfg.Debounce)
	defer t.Stop()
	for {
		e.sig.drain()
		if err := e.sig.failure(); err != nil {
			return false, err
		}
		if e.sig.pending() {
			return false, nil
		}
		e.trace("settle")
		select {
		case <-t.C():
			return !e.sig.pending(), nil
		case <-e.sig.wake:
		case <-ctx.Done():
			return false, errInterrupted
		}
	}
}

// observationFailed publishes a non-fatal failure and arms the retry.
func (e *Engine) observationFailed(ctx context.Context, err error) error {
	diag := diagnosticOf(err, artifact.CodeWatchObservationFailure)
	var batch []Event
	if !e.reported[diag.Code] {
		e.reported[diag.Code] = true
		batch = append(batch, Event{Kind: KindError, HasGeneration: true, Generation: e.gen, Diagnostic: diag})
	}
	if !e.rescan {
		e.rescan = true
		batch = append(batch, e.rescanStarted(RescanError))
	}
	e.nextBackoff()
	e.retryAt = e.clock.Now().Add(e.backoff)
	if len(batch) == 0 {
		return nil
	}
	return e.publish(ctx, batch)
}

// nextBackoff starts the error backoff at the debounce (DefaultDebounce in
// polling mode) and doubles it up to MaxBackoff.
func (e *Engine) nextBackoff() {
	base := e.cfg.Debounce
	if e.cfg.Backend == nil {
		base = DefaultDebounce
	}
	switch {
	case e.backoff == 0:
		e.backoff = base
	case e.backoff < e.cfg.MaxBackoff/2:
		e.backoff *= 2
	default:
		e.backoff = e.cfg.MaxBackoff
	}
}

func (e *Engine) reconcileRegistrations(obs Observation) error {
	if e.cfg.Backend == nil {
		return nil
	}
	changed, err := e.cfg.Backend.Reconcile(obs)
	if err != nil {
		return failure(err, artifact.CodeWatchBackendFailure, false)
	}
	if changed {
		e.sig.verify()
	}
	return nil
}

// publish hands batch to the writer and waits for it, unless the watch is
// interrupted or fails meanwhile.
func (e *Engine) publish(ctx context.Context, batch []Event) error {
	e.out.in <- batch
	e.busy = true
	for {
		e.sig.drain()
		if err := e.sig.failure(); err != nil {
			return err
		}
		e.trace("publish")
		select {
		case err := <-e.out.done:
			e.busy = false
			if err != nil {
				return &OutputError{Err: err}
			}
			return nil
		case <-e.sig.wake:
		case <-ctx.Done():
			return errInterrupted
		}
	}
}

// end closes the backend and publishes the last events of the stream.
func (e *Engine) end(cause error) error {
	if e.cfg.Backend != nil {
		_ = e.cfg.Backend.Close()
	}
	var out *OutputError
	if errors.As(cause, &out) {
		return out
	}
	if errors.Is(cause, errInterrupted) {
		return e.stop(StopInterrupted, nil)
	}
	var f *Failure
	if !errors.As(cause, &f) {
		f = failure(cause, artifact.CodeWatchInternalFailure, true)
	}
	return e.stop(StopError, f)
}

// stop waits at most the grace period for the writer, then publishes the
// fatal error if any and watch.stopped.
func (e *Engine) stop(reason StopReason, f *Failure) error {
	deadline := e.clock.NewTimer(e.cfg.Grace)
	defer deadline.Stop()
	wait := func() error {
		e.trace("stop")
		select {
		case err := <-e.out.done:
			e.busy = false
			if err != nil {
				return &OutputError{Err: err}
			}
			return nil
		case <-deadline.C():
			return ErrOutputBlocked
		}
	}
	if e.busy {
		if err := wait(); err != nil {
			return err
		}
	}
	var batch []Event
	if f != nil {
		batch = append(batch, Event{Kind: KindError, HasGeneration: e.hasState, Generation: e.gen, Fatal: true, Diagnostic: f.Diagnostic})
	}
	stopped := Event{Kind: KindStopped, HasGeneration: e.hasState, Generation: e.gen, Stop: reason}
	if e.hasState {
		stopped.Fingerprint = e.state.Fingerprint
	}
	e.out.in <- append(batch, stopped)
	e.busy = true
	if err := wait(); err != nil {
		return err
	}
	if f != nil {
		return f
	}
	return nil
}

func (e *Engine) trace(point string) {
	if e.cfg.trace != nil {
		e.cfg.trace(point)
	}
}

// fatalObservation reports whether an observation error ends the watch.
func fatalObservation(err error) bool {
	var typed *artifact.Error
	return artifact.IsInternal(err) || (errors.As(err, &typed) && typed.Code == artifact.CodeWatchRootUnavailable)
}

// failure turns err into a *Failure, keeping its typed code when it has
// one. Internal marks an exit-6 failure.
func failure(err error, fallback string, internal bool) *Failure {
	var f *Failure
	if errors.As(err, &f) {
		return f
	}
	return &Failure{Diagnostic: diagnosticOf(err, fallback), Internal: internal || artifact.IsInternal(err)}
}

func diagnosticOf(err error, fallback string) Diagnostic {
	var typed *artifact.Error
	if errors.As(err, &typed) && typed.Code != "" {
		return Diagnostic{Code: typed.Code, Message: typed.Message}
	}
	return Diagnostic{Code: fallback, Message: err.Error()}
}

// signals holds the backend's flags. Notifications only set flags and wake
// the engine: memory never grows with the notification rate.
type signals struct {
	clock Clock
	wake  chan struct{}

	mu       sync.Mutex
	pend     bool
	first    time.Time
	last     time.Time
	overflow bool
	fatal    *Failure
}

func newSignals(clock Clock) *signals {
	return &signals{clock: clock, wake: make(chan struct{}, 1)}
}

func (s *signals) Notify() {
	s.mu.Lock()
	s.mark()
	s.mu.Unlock()
	s.poke()
}

func (s *signals) Overflow() {
	s.mu.Lock()
	s.overflow = true
	s.mu.Unlock()
	s.poke()
}

func (s *signals) Fail(err error) {
	if err == nil {
		err = errors.New("watch backend failed without an error")
	}
	s.mu.Lock()
	if s.fatal == nil {
		s.fatal = failure(err, artifact.CodeWatchBackendFailure, false)
	}
	s.mu.Unlock()
	s.poke()
}

// verify schedules a verification observation. The engine reads the flag
// itself, so it does not wake.
func (s *signals) verify() {
	s.mu.Lock()
	s.mark()
	s.mu.Unlock()
}

func (s *signals) mark() {
	now := s.clock.Now()
	if !s.pend {
		s.pend, s.first = true, now
	}
	s.last = now
}

func (s *signals) poke() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// drain removes a pending wake-up; callers check the flags right after.
func (s *signals) drain() {
	select {
	case <-s.wake:
	default:
	}
}

func (s *signals) pending() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pend
}

func (s *signals) overflowed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.overflow
}

func (s *signals) times() (first, last time.Time, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.first, s.last, s.pend
}

func (s *signals) failure() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fatal == nil {
		return nil
	}
	return s.fatal
}

// clear forgets the pending signal and overflow before an observation.
func (s *signals) clear() {
	s.mu.Lock()
	s.pend, s.overflow = false, false
	s.mu.Unlock()
}

// writer publishes batches on its own goroutine, so that a blocked sink
// never prevents the engine from reacting to an interruption.
type writer struct {
	in   chan []Event
	done chan error
}

func startWriter(sink Sink) *writer {
	w := &writer{in: make(chan []Event), done: make(chan error, 1)}
	go func() {
		for batch := range w.in {
			w.done <- sink.Publish(batch)
		}
	}()
	return w
}

// close lets the goroutine end once its current batch, if any, returns.
func (w *writer) close() { close(w.in) }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func (systemClock) NewTimer(d time.Duration) Timer { return systemTimer{time.NewTimer(d)} }

type systemTimer struct{ t *time.Timer }

func (t systemTimer) C() <-chan time.Time { return t.t.C }
func (t systemTimer) Stop() bool          { return t.t.Stop() }
