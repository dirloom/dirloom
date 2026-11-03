package watch

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/dirloom/dirloom/internal/identity"
	"github.com/dirloom/dirloom/internal/snapshot"
)

// The Watch Event Schema v1 goldens are produced by the engine on fakes from
// constructed states, never from a real filesystem run. Set
// DIRLOOM_UPDATE_GOLDEN=1 to rewrite them.
var goldenDir = filepath.Join("..", "..", "testdata", "watch", "v1", "golden")

// teeSink publishes each batch to every sink in order; the first one may
// block or fail before the encoders see the batch.
type teeSink []Sink

func (t teeSink) Publish(events []Event) error {
	for _, s := range t {
		if err := s.Publish(events); err != nil {
			return err
		}
	}
	return nil
}

// goldenOutput collects the NDJSON stream and the text rendering of a run.
type goldenOutput struct {
	ndjson, text, diagnostics bytes.Buffer
}

// with wraps edit so that the engine also publishes to both encoders.
func (g *goldenOutput) with(edit func(*Config)) func(*Config) {
	return func(c *Config) {
		if edit != nil {
			edit(c)
		}
		c.Sink = teeSink{c.Sink, NewNDJSONSink(&g.ndjson), NewTextSink(&g.text, &g.diagnostics)}
	}
}

// check compares the outputs with name.ndjson, name.txt and, when the text
// rendering wrote diagnostics, name.stderr.txt.
func (g *goldenOutput) check(t *testing.T, name string) {
	t.Helper()
	files := map[string][]byte{name + ".ndjson": g.ndjson.Bytes(), name + ".txt": g.text.Bytes()}
	stderr := filepath.Join(goldenDir, name+".stderr.txt")
	if g.diagnostics.Len() > 0 {
		files[name+".stderr.txt"] = g.diagnostics.Bytes()
	} else if os.Getenv("DIRLOOM_UPDATE_GOLDEN") == "1" {
		_ = os.Remove(stderr)
	} else if _, err := os.Stat(stderr); err == nil {
		t.Errorf("%s exists but the run wrote no diagnostic", stderr)
	}
	for file, got := range files {
		path := filepath.Join(goldenDir, file)
		if os.Getenv("DIRLOOM_UPDATE_GOLDEN") == "1" {
			if err := os.MkdirAll(goldenDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, got, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		want, err := os.ReadFile(path) // #nosec G304 -- fixed test golden path.
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s\n got %q\nwant %q", file, got, want)
		}
	}
}

// state builds an observation from entries: "dir/" is a directory, "link ->
// target" a symlink, anything else a file.
func state(entries ...string) Observation {
	records := []identity.Record{{Path: artifact.RootPath, Kind: artifact.KindDirectory}}
	for _, e := range entries {
		switch {
		case strings.Contains(e, " -> "):
			p, target, _ := strings.Cut(e, " -> ")
			records = append(records, identity.Record{Path: artifact.Path(p), Kind: artifact.KindSymlink, Target: target})
		case strings.HasSuffix(e, "/"):
			records = append(records, identity.Record{Path: artifact.Path(strings.TrimSuffix(e, "/")), Kind: artifact.KindDirectory})
		default:
			records = append(records, identity.Record{Path: artifact.Path(e), Kind: artifact.KindFile})
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Path < records[j].Path })
	fp, err := identity.FingerprintRecords(records)
	if err != nil {
		panic(err)
	}
	return Observation{Records: records, Fingerprint: fp}
}

var (
	goldenBefore = state("docs/", "docs/guide.md", "shared/", "shared/auth/", "shared/auth/login.go", "shared/auth/token.go",
		"src/", "current -> docs")
	goldenAfter = state("docs/", "shared/", "src/", "src/auth/", "src/auth/login.go", "src/auth/token.go", "src/main.go",
		"current -> src", "notes\tdraft.md")
	goldenCapture = func() snapshot.CaptureV1 {
		depth := 4
		return snapshot.CaptureV1{Depth: &depth, UseDefaultIgnores: true, UseGitignore: true, Ignore: []string{"*.log", "dist/"}}
	}
)

// signal makes the backend report a change and lets the debounce expire.
func (h *harness) signal() {
	h.t.Helper()
	h.backend.signals().Notify()
	h.until("debounce")
	h.clock.Advance(debounce)
	h.idle()
}

func TestGoldenGenerations(t *testing.T) {
	for _, tc := range []struct {
		name  string
		moves comparison.MoveDetection
		want  []string
	}{
		{"generations", comparison.MoveDetectionStructuralV1, []string{"started",
			"node.changed:current", "node.removed:docs/guide.md", "node.added:notes\tdraft.md", "node.moved:src/auth", "node.added:src/main.go", "state:1",
			"node.removed:src/main.go", "state:2", "stopped:interrupted"}},
		{"no-moves", comparison.MoveDetectionNone, []string{"started",
			"node.changed:current", "node.removed:docs/guide.md", "node.added:notes\tdraft.md", "node.removed:shared/auth",
			"node.removed:shared/auth/login.go", "node.removed:shared/auth/token.go", "node.added:src/auth",
			"node.added:src/auth/login.go", "node.added:src/auth/token.go", "node.added:src/main.go", "state:1",
			"node.removed:src/main.go", "state:2", "stopped:interrupted"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			last := state("docs/", "shared/", "src/", "src/auth/", "src/auth/login.go", "src/auth/token.go",
				"current -> src", "notes\tdraft.md")
			var g goldenOutput
			h := newHarness(t, observed(goldenBefore), observed(goldenAfter), observed(last))
			h.start(g.with(func(c *Config) { c.Capture, c.MoveDetection = goldenCapture(), tc.moves }))
			h.idle()
			h.signal()
			h.signal()
			if err := h.interrupt(); err != nil {
				t.Fatal(err)
			}
			h.expect(true, tc.want...)
			g.check(t, tc.name)
		})
	}
}

func TestGoldenRescans(t *testing.T) {
	t.Run("rescan-overflow", func(t *testing.T) {
		var g goldenOutput
		h := newHarness(t, observed(state("a")), observed(state("a", "b")), observed(state("a", "b")))
		h.start(g.with(nil))
		h.idle()
		h.backend.signals().Overflow()
		h.idle()
		h.backend.signals().Overflow()
		h.idle()
		_ = h.interrupt()
		h.expect(true, "started", "rescan:overflow", "node.added:b", "state:1", "completed", "rescan:overflow", "completed", "stopped:interrupted")
		g.check(t, "rescan-overflow")
	})
	t.Run("rescan-reconciliation", func(t *testing.T) {
		var g goldenOutput
		h := newHarness(t, observed(state("a")), observed(state("a", "b")))
		h.start(g.with(nil))
		h.idle()
		h.clock.Advance(ReconcileInterval)
		h.until("settle")
		h.clock.Advance(debounce)
		h.idle()
		_ = h.interrupt()
		h.expect(true, "started", "rescan:reconciliation", "node.added:b", "state:1", "completed", "stopped:interrupted")
		g.check(t, "rescan-reconciliation")
	})
	t.Run("error-recovery", func(t *testing.T) {
		var g goldenOutput
		h := newHarness(t, observed(state("a")),
			failing(readFailure(`read metadata for "tmp/x": entry no longer exists`)),
			observed(state("a")))
		h.start(g.with(nil))
		h.idle()
		h.backend.signals().Notify()
		h.until("debounce")
		h.clock.Advance(debounce)
		h.idle()
		h.clock.Advance(debounce)
		h.idle()
		_ = h.interrupt()
		h.expect(true, "started", "error:source_read_failure", "rescan:error", "completed", "stopped:interrupted")
		g.check(t, "error-recovery")
	})
}

func TestGoldenEndings(t *testing.T) {
	t.Run("root-removed", func(t *testing.T) {
		var g goldenOutput
		h := newHarness(t, observed(state("a")))
		h.start(g.with(nil))
		h.idle()
		h.obs.setRoot(artifact.New(artifact.CodeWatchRootUnavailable, "the watched root was removed or moved", false))
		h.backend.signals().Notify()
		h.until("debounce")
		h.clock.Advance(debounce)
		var f *Failure
		if err := h.finish(); !errors.As(err, &f) {
			t.Fatalf("Run = %v", err)
		}
		h.expect(true, "started", "error:watch_root_unavailable!", "stopped:error")
		g.check(t, "root-removed")
	})
	t.Run("broken-invariant", func(t *testing.T) {
		var g goldenOutput
		a := state("a")
		h := newHarness(t, observed(a), observed(Observation{Records: a.Records, Fingerprint: state("b").Fingerprint}))
		h.start(g.with(nil))
		h.idle()
		h.backend.signals().Notify()
		h.until("debounce")
		h.clock.Advance(debounce)
		var f *Failure
		if err := h.finish(); !errors.As(err, &f) || !f.Internal {
			t.Fatalf("Run = %v", err)
		}
		h.expect(true, "started", "error:internal_invariant_failure!", "stopped:error")
		g.check(t, "broken-invariant")
	})
	t.Run("startup-failure", func(t *testing.T) {
		var g goldenOutput
		h := newHarness(t, observed(state("a")))
		h.backend.startErr = artifact.New(artifact.CodeWatchFilesystemUnsupported,
			"the root is on a network or virtual filesystem (nfs); use --poll", false)
		h.start(g.with(nil))
		var f *Failure
		if err := h.finish(); !errors.As(err, &f) {
			t.Fatalf("Run = %v", err)
		}
		h.expect(true, "error:watch_filesystem_unsupported!", "stopped:error")
		g.check(t, "startup-failure")
	})
	t.Run("interrupted-before-state", func(t *testing.T) {
		var g goldenOutput
		h := newHarness(t, step{block: true})
		h.start(g.with(nil))
		h.entered()
		if err := h.interrupt(); err != nil {
			t.Fatal(err)
		}
		h.expect(true, "stopped:interrupted")
		g.check(t, "interrupted-before-state")
	})
	t.Run("output-grace-expiry", func(t *testing.T) {
		// The output blocks while the first generation is written; the
		// interruption then waits the grace period and gives up: the stream
		// is incomplete, without watch.stopped.
		var g goldenOutput
		h := newHarness(t, observed(state("a")), observed(state("a", "b")))
		h.sink.gate, h.sink.blockFrom = make(chan struct{}), 2
		h.start(g.with(nil))
		h.idle()
		h.backend.signals().Notify()
		h.until("debounce")
		h.clock.Advance(debounce)
		h.until("publish")
		h.cancel()
		h.until("stop")
		h.clock.Advance(OutputGrace)
		if err := h.finish(); !errors.Is(err, ErrOutputBlocked) {
			t.Fatalf("Run = %v, want ErrOutputBlocked", err)
		}
		g.check(t, "output-grace-expiry")
		close(h.sink.gate) // after the outputs were read: the writer may go on
	})
}
