package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/dirloom/dirloom/internal/identity"
	"github.com/dirloom/dirloom/internal/watch"
	"github.com/dirloom/dirloom/internal/watch/backend"
)

// WatchRequest is one structural watch: the root and Capture Semantics as
// Inspect resolves them, the move detection (empty means structural-v1), and
// the trigger: native notifications with Debounce (0 means the default), or
// polling every Poll.
type WatchRequest struct {
	Inspect       InspectRequest
	MoveDetection comparison.MoveDetection
	Debounce      time.Duration
	Poll          time.Duration
}

// WatchUsageError reports an invalid watch request. The CLI maps it to exit 2
// with a human diagnostic on stderr, before any output.
type WatchUsageError struct {
	Message string
}

func (e *WatchUsageError) Error() string { return e.Message }

// WatchUsage reports whether err is an invalid watch request (exit 2).
func WatchUsage(err error) bool {
	var usage *WatchUsageError
	return errors.As(err, &usage)
}

// Watch validates the request, then runs Structural Watch v1 and publishes
// its events to sink until ctx is canceled or a fatal condition occurs. It
// returns a *WatchUsageError before touching the filesystem, or what
// watch.Engine.Run returns.
func Watch(ctx context.Context, request WatchRequest, sink watch.Sink) error {
	cfg, err := watchConfig(request, sink)
	if err != nil {
		return err
	}
	engine, err := watch.New(cfg)
	if err != nil {
		return err
	}
	return engine.Run(ctx)
}

func watchConfig(request WatchRequest, sink watch.Sink) (watch.Config, error) {
	if err := validateWatch(request); err != nil {
		return watch.Config{}, err
	}
	observer := newWatchObserver(request.Inspect)
	cfg := watch.Config{
		Observer:      observer,
		Sink:          sink,
		Capture:       CaptureFromInspect(request.Inspect),
		MoveDetection: request.MoveDetection,
		Debounce:      request.Debounce,
		Poll:          request.Poll,
	}
	if request.Poll == 0 {
		cfg.Backend = backend.New(backend.Options{Root: observer.root, Capture: cfg.Capture})
	}
	return cfg, nil
}

func validateWatch(request WatchRequest) error {
	usage := func(format string, args ...any) error {
		return &WatchUsageError{Message: fmt.Sprintf(format, args...)}
	}
	switch {
	case request.Poll < 0 || (request.Poll > 0 && (request.Poll < watch.MinPoll || request.Poll > watch.MaxPoll)):
		return usage("--poll must be between %s and %s, received %s", watch.MinPoll, watch.MaxPoll, request.Poll)
	case request.Poll > 0 && request.Debounce != 0:
		return usage("--debounce applies to native notifications and cannot be combined with --poll")
	case request.Debounce != 0 && (request.Debounce < watch.MinDebounce || request.Debounce > watch.MaxDebounce):
		return usage("--debounce must be between %s and %s, received %s", watch.MinDebounce, watch.MaxDebounce, request.Debounce)
	case request.Inspect.MaxDepth != nil && *request.Inspect.MaxDepth < 0:
		return usage("depth must be a non-negative integer")
	case request.Inspect.OutputPath != "":
		return artifact.InternalError("a watch writes no file")
	}
	return nil
}

// watchObserver observes the live root through the same filesystem source
// as dirloom fingerprint, and checks that the root keeps the identity it had
// at startup.
type watchObserver struct {
	request InspectRequest
	root    string // absolute and cleaned
	id      watch.Identity
	err     error // the root was unusable at startup
}

func newWatchObserver(request InspectRequest) *watchObserver {
	root := request.Root
	if root == "" {
		root = "."
	}
	o := &watchObserver{request: request, root: absRoot(root)}
	o.id, o.err = o.identity()
	if o.err == nil {
		o.err = o.readable()
	}
	return o
}

func rootUnavailable(message string) error {
	return artifact.New(artifact.CodeWatchRootUnavailable, message, false)
}

// identity reads the root's identity. Messages never name the root: its
// absolute path is not disclosed.
func (o *watchObserver) identity() (watch.Identity, error) {
	id, err := watch.IdentityOf(o.root)
	switch {
	case err == nil:
		return id, nil
	case errors.Is(err, os.ErrNotExist):
		return watch.Identity{}, rootUnavailable("the watched root does not exist")
	case errors.Is(err, watch.ErrNotDirectory):
		return watch.Identity{}, rootUnavailable("the watched root is not a directory")
	default:
		return watch.Identity{}, rootUnavailable("the watched root cannot be opened")
	}
}

func (o *watchObserver) readable() error {
	f, err := os.Open(o.root)
	if err == nil {
		_, err = f.Readdirnames(1)
		if errors.Is(err, io.EOF) {
			err = nil
		}
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
	}
	if err != nil {
		return rootUnavailable("the watched root cannot be read")
	}
	return nil
}

func (o *watchObserver) CheckRoot() error {
	if o.err != nil {
		return o.err
	}
	id, err := o.identity()
	if err != nil {
		return err
	}
	if id != o.id {
		return rootUnavailable("the watched root was replaced or moved")
	}
	return nil
}

func (o *watchObserver) Observe(ctx context.Context) (watch.Observation, error) {
	art, err := NewFilesystemSource(o.request).Observe(ctx)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return watch.Observation{}, ctxErr
		}
		if rootErr := o.CheckRoot(); rootErr != nil {
			return watch.Observation{}, rootErr
		}
		return watch.Observation{}, o.scrub(err)
	}
	records, err := identity.ProjectV1(*art)
	if err != nil {
		return watch.Observation{}, o.scrub(err)
	}
	fp, err := identity.FingerprintRecords(records)
	if err != nil {
		return watch.Observation{}, o.scrub(err)
	}
	return watch.Observation{Records: records, Fingerprint: fp}, nil
}

// scrub removes the root's absolute path from a diagnostic: watch messages
// name paths relative to the root only.
func (o *watchObserver) scrub(err error) error {
	var typed *artifact.Error
	if !errors.As(err, &typed) {
		return artifact.SourceReadFailure(errors.New(o.scrubText(err.Error())))
	}
	scrubbed := *typed
	scrubbed.Message = o.scrubText(typed.Message)
	scrubbed.Paths = make([]string, len(typed.Paths))
	for i, p := range typed.Paths {
		scrubbed.Paths[i] = o.scrubText(p)
	}
	return &scrubbed
}

func (o *watchObserver) scrubText(s string) string {
	roots := []string{o.root}
	if r := o.request.Root; filepath.IsAbs(r) && r != o.root {
		roots = append(roots, r)
	}
	for _, r := range roots {
		quoted := strconv.Quote(r)
		s = strings.ReplaceAll(s, quoted[1:len(quoted)-1], ".")
		s = strings.ReplaceAll(s, r, ".")
	}
	return s
}
