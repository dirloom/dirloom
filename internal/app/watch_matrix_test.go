//go:build linux || windows || darwin

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/dirloom/dirloom/internal/identity"
	"github.com/dirloom/dirloom/internal/watch"
)

// The §15.1 matrix on real filesystems. Every scenario runs with native
// notifications and with --poll. Each step ends at a checkpoint: a
// sentinel directory is created, and the step is over when the watch's last
// state is the one dirloom fingerprint reports (convergence). At each
// checkpoint, the generations since the previous one, applied to the
// previous state as a consumer applies them, give the current state
// (structural equivalence), and each generation is the comparison of the
// two states it joins (diff equivalence). Native runs never rely on the
// reconciliation (60 s, far beyond every wait), so a change they report was
// signalled.

// stateModel is what a consumer holds: the nodes below the root.
type stateModel map[artifact.Path]comparison.NodeState

func modelOf(records []identity.Record) stateModel {
	m := stateModel{}
	for _, r := range records {
		if r.Path == artifact.RootPath {
			continue
		}
		state := comparison.NodeState{Kind: r.Kind}
		if r.Kind.HasTarget() {
			target := r.Target
			state.Target = &target
		}
		m[r.Path] = state
	}
	return m
}

func under(p, dir artifact.Path) bool {
	return p == dir || strings.HasPrefix(string(p), string(dir)+"/")
}

// apply applies the committed generations of events, each as a whole:
// moved subtrees first, then removals, then the new states.
func (m stateModel) apply(events []watch.Event) {
	var changes []comparison.Change
	for _, ev := range events {
		switch {
		case ev.Kind.IsNode():
			changes = append(changes, ev.Change)
		case ev.Kind == watch.KindStateChanged:
			m.applyGeneration(changes)
			changes = nil
		}
	}
}

func (m stateModel) applyGeneration(changes []comparison.Change) {
	moved := stateModel{}
	for _, c := range changes {
		if c.Op != comparison.OpMoved {
			continue
		}
		for p, state := range m {
			if under(p, c.From) {
				moved[c.Path+p[len(c.From):]] = state
				delete(m, p)
			}
		}
	}
	for _, c := range changes {
		if c.Op == comparison.OpRemoved {
			delete(m, c.Path)
		}
	}
	for p, state := range moved {
		m[p] = state
	}
	for _, c := range changes {
		if c.After != nil {
			m[c.Path] = *c.After
		}
	}
}

// matrixStep is one step of a scenario. quiet says that the step changes
// nothing the capture sees: no structural event may follow it. expect
// checks the comparison of the checkpoint states, sentinel excluded.
type matrixStep struct {
	name   string
	do     func(t *testing.T, root string)
	quiet  bool
	expect func(t *testing.T, changes []comparison.Change)
}

type matrixScenario struct {
	name  string
	setup func(t *testing.T, root string)
	edit  func(r *WatchRequest)
	steps []matrixStep
}

func runMatrix(t *testing.T, scenarios ...matrixScenario) {
	for _, sc := range scenarios {
		for _, mode := range []string{"native", "poll"} {
			t.Run(sc.name+"/"+mode, func(t *testing.T) { runScenario(t, sc, mode) })
		}
	}
}

func runScenario(t *testing.T, sc matrixScenario, mode string) {
	root := t.TempDir()
	if sc.setup != nil {
		sc.setup(t, root)
	}
	request := nativeRequest(root)
	if mode == "poll" {
		request = pollRequest(root)
	}
	if sc.edit != nil {
		sc.edit(&request)
	}
	w := startWatch(t, request)
	fp := fingerprintOf(t, request.Inspect)
	w.log.waitState(t, fp)
	before, _ := w.obs.records(fp)
	model := modelOf(before)
	mark := len(w.log.recorded())
	for i, step := range sc.steps {
		step.do(t, root)
		sentinel := artifact.Path(fmt.Sprintf("zz-checkpoint-%02d", i))
		mkdirAll(t, filepath.Join(root, string(sentinel)))
		fp = fingerprintOf(t, request.Inspect)
		w.log.waitState(t, fp)
		after, ok := w.obs.records(fp)
		if !ok {
			t.Fatalf("step %q: the state %s was not observed", step.name, fp)
		}
		events := w.log.recorded()
		since := events[mark:]
		mark = len(events)
		model.apply(since)
		if want := modelOf(after); !reflect.DeepEqual(model, want) {
			t.Fatalf("step %q: the generations do not compose to the checkpoint state\n got %v\nwant %v", step.name, model, want)
		}
		changes := checkpointChanges(t, before, after, request.MoveDetection, sentinel)
		if step.quiet {
			if len(changes) != 0 {
				t.Fatalf("step %q is not invisible to the capture: %+v", step.name, changes)
			}
			for _, ev := range since {
				if ev.Kind.IsNode() && ev.Change.Path != sentinel {
					t.Fatalf("step %q: structural event %s %s", step.name, ev.Kind, ev.Change.Path)
				}
			}
		}
		if step.expect != nil {
			step.expect(t, changes)
		}
		before = after
	}
	if err := w.interrupt(t); err != nil {
		t.Fatalf("Run = %v", err)
	}
	events := w.log.recorded()
	if err := watch.ValidateStream(events, true); err != nil {
		t.Fatal(err)
	}
	checkDiffEquivalence(t, events, w.obs)
	for _, ev := range events {
		if ev.Kind == watch.KindError {
			t.Fatalf("unexpected %s: %+v", ev.Kind, ev.Diagnostic)
		}
		if mode == "native" && ev.Kind == watch.KindRescanStarted {
			t.Fatalf("a native run needed a rescan (%s)", ev.Rescan)
		}
	}
}

// checkpointChanges compares two checkpoint states, without the sentinel.
func checkpointChanges(t *testing.T, before, after []identity.Record, moves comparison.MoveDetection, sentinel artifact.Path) []comparison.Change {
	t.Helper()
	if moves == "" {
		moves = comparison.MoveDetectionStructuralV1
	}
	ref := comparison.SourceRef{Kind: "filesystem"}
	diff, err := comparison.CompareRecords(before, after, ref, ref, comparison.Options{MoveDetection: moves})
	if err != nil {
		t.Fatal(err)
	}
	var out []comparison.Change
	for _, c := range diff.Changes {
		if c.Path != sentinel {
			out = append(out, c)
		}
	}
	return out
}

// describeChanges renders changes as "ADDED a", "MOVED b<-a (subtree,2)",
// "CHANGED c (file->directory)", sorted.
func describeChanges(changes []comparison.Change) []string {
	out := make([]string, 0, len(changes))
	for _, c := range changes {
		switch c.Op {
		case comparison.OpMoved:
			out = append(out, fmt.Sprintf("MOVED %s<-%s (%s,%d)", c.Path, c.From, c.Evidence, c.Nodes))
		case comparison.OpChanged:
			target := func(s *comparison.NodeState) string {
				if s.Target == nil {
					return string(s.Kind)
				}
				return string(s.Kind) + ":" + *s.Target
			}
			out = append(out, fmt.Sprintf("CHANGED %s (%s->%s)", c.Path, target(c.Before), target(c.After)))
		default:
			out = append(out, fmt.Sprintf("%s %s", c.Op, c.Path))
		}
	}
	sort.Strings(out)
	return out
}

func expectChanges(want ...string) func(t *testing.T, changes []comparison.Change) {
	return func(t *testing.T, changes []comparison.Change) {
		t.Helper()
		sort.Strings(want)
		if got := describeChanges(changes); strings.Join(got, "; ") != strings.Join(want, "; ") {
			t.Fatalf("changes\n got %v\nwant %v", got, want)
		}
	}
}

func join(root string, parts ...string) string {
	return filepath.Join(append([]string{root}, parts...)...)
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestWatchMatrixCreationsAndRemovals(t *testing.T) {
	runMatrix(t,
		matrixScenario{name: "create", steps: []matrixStep{
			{name: "file", do: func(t *testing.T, root string) { writeFiles(t, join(root, "f")) }, expect: expectChanges("ADDED f")},
			{name: "directory", do: func(t *testing.T, root string) { mkdirAll(t, join(root, "d")) }, expect: expectChanges("ADDED d")},
			{name: "nested burst", do: func(t *testing.T, root string) {
				mkdirAll(t, join(root, "a", "b", "c", "d"))
				writeFiles(t, join(root, "a", "f"), join(root, "a", "b", "f"), join(root, "a", "b", "c", "f"), join(root, "a", "b", "c", "d", "f"))
			}, expect: expectChanges("ADDED a", "ADDED a/b", "ADDED a/b/c", "ADDED a/b/c/d", "ADDED a/f", "ADDED a/b/f", "ADDED a/b/c/f", "ADDED a/b/c/d/f")},
			{name: "inside the burst", do: func(t *testing.T, root string) { writeFiles(t, join(root, "a", "b", "c", "d", "late")) },
				expect: expectChanges("ADDED a/b/c/d/late")},
		}},
		matrixScenario{name: "remove", setup: func(t *testing.T, root string) {
			mkdirAll(t, join(root, "tree", "sub"))
			writeFiles(t, join(root, "f"), join(root, "tree", "x"), join(root, "tree", "sub", "y"))
		}, steps: []matrixStep{
			{name: "file", do: func(t *testing.T, root string) { must(t, os.Remove(join(root, "f"))) }, expect: expectChanges("REMOVED f")},
			{name: "tree", do: func(t *testing.T, root string) { must(t, os.RemoveAll(join(root, "tree"))) },
				expect: expectChanges("REMOVED tree", "REMOVED tree/sub", "REMOVED tree/sub/y", "REMOVED tree/x")},
		}},
	)
}

func movesSetup(t *testing.T, root string) {
	mkdirAll(t, join(root, "src", "lib"), join(root, "dst"))
	writeFiles(t, join(root, "name.txt"), join(root, "src", "lib", "a.go"), join(root, "src", "lib", "b.go"))
}

func TestWatchMatrixMovesAndReplacements(t *testing.T) {
	rename := func(from, to string) func(t *testing.T, root string) {
		return func(t *testing.T, root string) {
			must(t, os.Rename(join(root, filepath.FromSlash(from)), join(root, filepath.FromSlash(to))))
		}
	}
	runMatrix(t,
		matrixScenario{name: "moves", setup: movesSetup, steps: []matrixStep{
			// Structural move detection matches names, never contents: a file
			// renamed in place is a removal and an addition, as dirloom diff
			// reports it, and a relocated file keeps its name.
			{name: "rename in place", do: rename("name.txt", "renamed.txt"), expect: expectChanges("ADDED renamed.txt", "REMOVED name.txt")},
			{name: "relocate a file", do: rename("renamed.txt", "dst/renamed.txt"), expect: expectChanges("MOVED dst/renamed.txt<-renamed.txt (name,1)")},
			{name: "move a directory", do: rename("src/lib", "dst/lib"), expect: expectChanges("MOVED dst/lib<-src/lib (subtree,3)")},
			{name: "change inside it", do: func(t *testing.T, root string) { writeFiles(t, join(root, "dst", "lib", "c.go")) },
				expect: expectChanges("ADDED dst/lib/c.go")},
		}},
		matrixScenario{name: "no-moves", setup: movesSetup, edit: func(r *WatchRequest) { r.MoveDetection = comparison.MoveDetectionNone },
			steps: []matrixStep{
				{name: "relocate a file", do: rename("name.txt", "dst/name.txt"), expect: expectChanges("ADDED dst/name.txt", "REMOVED name.txt")},
				{name: "move a directory", do: rename("src/lib", "dst/lib"), expect: expectChanges(
					"ADDED dst/lib", "ADDED dst/lib/a.go", "ADDED dst/lib/b.go", "REMOVED src/lib", "REMOVED src/lib/a.go", "REMOVED src/lib/b.go")},
			}},
		matrixScenario{name: "replace", setup: func(t *testing.T, root string) { writeFiles(t, join(root, "x")) }, steps: []matrixStep{
			{name: "file by directory", do: func(t *testing.T, root string) {
				must(t, os.Remove(join(root, "x")))
				mkdirAll(t, join(root, "x"))
			}, expect: expectChanges("CHANGED x (file->directory)")},
		}},
		matrixScenario{name: "recreate", setup: func(t *testing.T, root string) { mkdirAll(t, join(root, "d")) }, steps: []matrixStep{
			{name: "removed and recreated at once", do: func(t *testing.T, root string) {
				must(t, os.RemoveAll(join(root, "d")))
				mkdirAll(t, join(root, "d"))
			}, expect: expectChanges()},
			{name: "change inside the new one", do: func(t *testing.T, root string) { writeFiles(t, join(root, "d", "x")) }, expect: expectChanges("ADDED d/x")},
			{name: "removed", do: func(t *testing.T, root string) { must(t, os.RemoveAll(join(root, "d"))) }, expect: expectChanges("REMOVED d", "REMOVED d/x")},
			{name: "recreated", do: func(t *testing.T, root string) { mkdirAll(t, join(root, "d")) }, expect: expectChanges("ADDED d")},
			{name: "change inside it", do: func(t *testing.T, root string) { writeFiles(t, join(root, "d", "y")) }, expect: expectChanges("ADDED d/y")},
		}},
		matrixScenario{name: "move registered", setup: func(t *testing.T, root string) {
			mkdirAll(t, join(root, "a", "inner"), join(root, "b"))
			writeFiles(t, join(root, "a", "x"))
		}, steps: []matrixStep{
			{name: "move", do: rename("a", "b/a"), expect: expectChanges("MOVED b/a<-a (subtree,3)")},
			{name: "changes at the new path", do: func(t *testing.T, root string) {
				writeFiles(t, join(root, "b", "a", "new"), join(root, "b", "a", "inner", "deep"))
			}, expect: expectChanges("ADDED b/a/inner/deep", "ADDED b/a/new")},
			{name: "the old path again", do: func(t *testing.T, root string) { mkdirAll(t, join(root, "a")) }, expect: expectChanges("ADDED a")},
		}},
	)
}

func TestWatchMatrixContentAndEditorSaves(t *testing.T) {
	setup := func(t *testing.T, root string) { writeFiles(t, join(root, "f"), join(root, "doc.txt")) }
	runMatrix(t, matrixScenario{name: "content", setup: setup, steps: []matrixStep{
		{name: "append", quiet: true, do: func(t *testing.T, root string) {
			f, err := os.OpenFile(join(root, "f"), os.O_APPEND|os.O_WRONLY, 0)
			must(t, err)
			_, _ = f.WriteString("more")
			must(t, f.Close())
		}},
		{name: "truncate", quiet: true, do: func(t *testing.T, root string) { must(t, os.Truncate(join(root, "f"), 0)) }},
		{name: "rewrite", quiet: true, do: func(t *testing.T, root string) { must(t, os.WriteFile(join(root, "f"), []byte("new"), 0o644)) }},
		{name: "chmod", quiet: true, do: func(t *testing.T, root string) {
			must(t, os.Chmod(join(root, "f"), 0o600))
			must(t, os.Chmod(join(root, "f"), 0o644))
		}},
		{name: "touch", quiet: true, do: func(t *testing.T, root string) {
			later := time.Now().Add(time.Hour)
			must(t, os.Chtimes(join(root, "f"), later, later))
		}},
		// An editor's save may be seen half done: intermediate generations
		// are allowed, the net change is none.
		{name: "save through a temporary file", expect: expectChanges(), do: func(t *testing.T, root string) {
			must(t, os.WriteFile(join(root, "doc.txt.tmp"), []byte("v2"), 0o644))
			must(t, os.Rename(join(root, "doc.txt.tmp"), join(root, "doc.txt")))
		}},
		{name: "save with a backup", expect: expectChanges(), do: func(t *testing.T, root string) {
			must(t, os.Rename(join(root, "doc.txt"), join(root, "doc.txt~")))
			must(t, os.WriteFile(join(root, "doc.txt"), []byte("v3"), 0o644))
			must(t, os.Remove(join(root, "doc.txt~")))
		}},
		{name: "save in place", expect: expectChanges(), do: func(t *testing.T, root string) {
			must(t, os.WriteFile(join(root, "doc.txt"), []byte("v4"), 0o644))
		}},
	}})
}

func TestWatchMatrixCaptureRules(t *testing.T) {
	churn := func(dir string) func(t *testing.T, root string) {
		return func(t *testing.T, root string) {
			base := join(root, filepath.FromSlash(dir))
			mkdirAll(t, filepath.Join(base, "new"))
			writeFiles(t, filepath.Join(base, "file"), filepath.Join(base, "new", "file"))
			must(t, os.RemoveAll(filepath.Join(base, "new")))
			must(t, os.Rename(filepath.Join(base, "file"), filepath.Join(base, "renamed")))
		}
	}
	runMatrix(t,
		matrixScenario{name: "ignored", setup: func(t *testing.T, root string) {
			mkdirAll(t, join(root, "node_modules", "pkg"), join(root, ".git", "objects"), join(root, "build", "out"), join(root, "deep", "a", "b"))
		}, edit: func(r *WatchRequest) {
			depth := 2
			r.Inspect.MaxDepth, r.Inspect.IgnorePatterns = &depth, []string{"build"}
		}, steps: []matrixStep{
			{name: "node_modules", quiet: true, do: churn("node_modules/pkg")},
			{name: ".git", quiet: true, do: churn(".git/objects")},
			{name: "an ignored directory", quiet: true, do: churn("build/out")},
			{name: "beyond --depth", quiet: true, do: churn("deep/a/b")},
			{name: "within --depth", do: func(t *testing.T, root string) { writeFiles(t, join(root, "deep", "visible")) },
				expect: expectChanges("ADDED deep/visible")},
		}},
		matrixScenario{name: "dirs-only", edit: func(r *WatchRequest) { r.Inspect.DirectoriesOnly = true }, steps: []matrixStep{
			{name: "file", quiet: true, do: func(t *testing.T, root string) { writeFiles(t, join(root, "f")) }},
			{name: "directory", do: func(t *testing.T, root string) { mkdirAll(t, join(root, "d")) }, expect: expectChanges("ADDED d")},
		}},
		matrixScenario{name: "gitignore", setup: func(t *testing.T, root string) {
			mkdirAll(t, join(root, "build"), join(root, "sub"))
			writeFiles(t, join(root, ".gitignore"), join(root, "build", "out.txt"), join(root, "sub", ".gitignore"), join(root, "sub", "tmp.log"), join(root, "tmp.log"))
		}, steps: []matrixStep{
			{name: "a pattern added", do: func(t *testing.T, root string) {
				must(t, os.WriteFile(join(root, ".gitignore"), []byte("build/\n"), 0o644))
			}, expect: expectChanges("REMOVED build", "REMOVED build/out.txt")},
			{name: "the pattern removed", do: func(t *testing.T, root string) {
				must(t, os.WriteFile(join(root, ".gitignore"), []byte("\n"), 0o644))
			}, expect: expectChanges("ADDED build", "ADDED build/out.txt")},
			{name: "a nested pattern", do: func(t *testing.T, root string) {
				must(t, os.WriteFile(join(root, "sub", ".gitignore"), []byte("*.log\n"), 0o644))
			}, expect: expectChanges("REMOVED sub/tmp.log")},
		}},
		matrixScenario{name: "no-gitignore", setup: func(t *testing.T, root string) {
			mkdirAll(t, join(root, "build"))
			writeFiles(t, join(root, ".gitignore"), join(root, "build", "out.txt"))
		}, edit: func(r *WatchRequest) { r.Inspect.UseGitIgnore = false }, steps: []matrixStep{
			{name: "a pattern added", quiet: true, do: func(t *testing.T, root string) {
				must(t, os.WriteFile(join(root, ".gitignore"), []byte("build/\n"), 0o644))
			}},
		}},
		matrixScenario{name: "hidden excluded", steps: []matrixStep{
			{name: "dot entries", quiet: true, do: func(t *testing.T, root string) {
				mkdirAll(t, join(root, ".hidden-dir"))
				writeFiles(t, join(root, ".env"), join(root, ".hidden-dir", "x"))
			}},
		}},
		matrixScenario{name: "hidden included", edit: func(r *WatchRequest) { r.Inspect.IncludeHidden = true }, steps: []matrixStep{
			{name: "dot entries", do: func(t *testing.T, root string) {
				mkdirAll(t, join(root, ".hidden-dir"))
				writeFiles(t, join(root, ".env"), join(root, ".hidden-dir", "x"))
			}, expect: expectChanges("ADDED .hidden-dir", "ADDED .hidden-dir/x", "ADDED .env")},
			{name: "removed", do: func(t *testing.T, root string) { must(t, os.Remove(join(root, ".env"))) }, expect: expectChanges("REMOVED .env")},
		}},
	)
}

func TestWatchMatrixNames(t *testing.T) {
	scenarios := []matrixScenario{{name: "nfd", steps: []matrixStep{
		{name: "a decomposed name", do: func(t *testing.T, root string) { writeFiles(t, join(root, "cafe"+string(rune(0x0301)))) },
			expect: expectChanges("ADDED caf" + string(rune(0x00e9)))},
	}}}
	if runtime.GOOS != "linux" {
		scenarios = append(scenarios, matrixScenario{name: "case only", setup: func(t *testing.T, root string) { writeFiles(t, join(root, "Readme.md")) },
			steps: []matrixStep{
				{name: "rename", do: func(t *testing.T, root string) { must(t, os.Rename(join(root, "Readme.md"), join(root, "README.md"))) },
					expect: expectChanges("ADDED README.md", "REMOVED Readme.md")},
			}})
	}
	runMatrix(t, scenarios...)
}

func TestWatchMatrixSymlinks(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink("a", filepath.Join(root, "probe")); err != nil {
		t.Skipf("symbolic links are not allowed here: %v", err)
	}
	runMatrix(t, matrixScenario{name: "symlink", setup: func(t *testing.T, root string) {
		mkdirAll(t, join(root, "a"), join(root, "b"))
		must(t, os.Symlink("a", join(root, "link")))
	}, steps: []matrixStep{
		{name: "retarget", do: func(t *testing.T, root string) {
			must(t, os.Remove(join(root, "link")))
			must(t, os.Symlink("b", join(root, "link")))
		}, expect: expectChanges("CHANGED link (symlink:a->symlink:b)")},
		// A link is never entered: a change in its target is reported under
		// the target only.
		{name: "a change in its target", do: func(t *testing.T, root string) { writeFiles(t, join(root, "b", "x")) },
			expect: expectChanges("ADDED b/x")},
	}})
}

// deafBackend drops every notification of the backend it wraps: only the
// reconciliation can find a change (the matrix's "notification dropped by a
// hook").
type deafBackend struct{ watch.Backend }

func (b deafBackend) Start(sig watch.Signals) error { return b.Backend.Start(deafSignals{sig}) }

type deafSignals struct{ watch.Signals }

func (deafSignals) Notify() {}

// describeEvents renders events compactly: "started", "added:<path>",
// "state", "rescan:<reason>", "completed", "error:<code>" ("!" when fatal),
// "stopped:<reason>".
func describeEvents(events []watch.Event) string {
	out := make([]string, 0, len(events))
	for _, ev := range events {
		switch {
		case ev.Kind == watch.KindStarted:
			out = append(out, "started")
		case ev.Kind.IsNode():
			out = append(out, strings.ToLower(string(ev.Change.Op))+":"+string(ev.Change.Path))
		case ev.Kind == watch.KindStateChanged:
			out = append(out, "state")
		case ev.Kind == watch.KindRescanStarted:
			out = append(out, "rescan:"+string(ev.Rescan))
		case ev.Kind == watch.KindRescanCompleted:
			out = append(out, "completed")
		case ev.Kind == watch.KindError:
			s := "error:" + ev.Diagnostic.Code
			if ev.Fatal {
				s += "!"
			}
			out = append(out, s)
		case ev.Kind == watch.KindStopped:
			out = append(out, "stopped:"+string(ev.Stop))
		}
	}
	return strings.Join(out, " ")
}

func TestWatchMatrixReconciliation(t *testing.T) {
	t.Run("a dropped notification", func(t *testing.T) {
		root := t.TempDir()
		request := nativeRequest(root)
		w := startWatchWith(t, request, func(c *watch.Config) {
			c.Backend, c.Reconcile = deafBackend{c.Backend}, 300*time.Millisecond
		})
		w.log.waitState(t, fingerprintOf(t, request.Inspect))
		// The startup's verification observation (or a first reconciliation)
		// is over: from now on only a reconciliation observes.
		w.obs.waitObservations(t, 1)
		writeFiles(t, join(root, "missed"))
		w.log.waitState(t, fingerprintOf(t, request.Inspect))
		w.log.waitFor(t, "the end of the rescan", func(events []watch.Event) bool {
			return events[len(events)-1].Kind == watch.KindRescanCompleted
		})
		_ = w.interrupt(t)
		events := w.log.recorded()
		if got := describeEvents(events); got != "started rescan:reconciliation added:missed state completed stopped:interrupted" {
			t.Fatalf("events: %s", got)
		}
		if err := watch.ValidateStream(events, true); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("a quiet tree", func(t *testing.T) {
		root := t.TempDir()
		mkdirAll(t, join(root, "a"))
		request := nativeRequest(root)
		w := startWatchWith(t, request, func(c *watch.Config) { c.Reconcile = 100 * time.Millisecond })
		w.log.waitState(t, fingerprintOf(t, request.Inspect))
		w.obs.waitObservations(t, 5) // five reconciliations
		_ = w.interrupt(t)
		if got := describeEvents(w.log.recorded()); got != "started stopped:interrupted" {
			t.Fatalf("a quiet tree reported: %s", got)
		}
	})
}

func TestWatchMatrixDepthZeroNative(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	mkdirAll(t, join(root, "a"))
	depth := 0
	request := nativeRequest(root)
	request.Inspect.MaxDepth = &depth
	w := startWatch(t, request)
	w.log.waitState(t, fingerprintOf(t, request.Inspect))
	mkdirAll(t, join(root, "child", "grandchild"))
	writeFiles(t, join(root, "a", "f"))
	must(t, os.RemoveAll(root))
	if err := w.end(t); err == nil {
		t.Fatal("the watch survived its root")
	}
	if got := describeEvents(w.log.recorded()); got != "started error:watch_root_unavailable! stopped:error" {
		t.Fatalf("events at --depth 0: %s", got)
	}
}

func TestWatchMatrixRootLossNative(t *testing.T) {
	for _, tc := range []struct {
		name string
		lose func(root string) error
	}{
		{"removed", os.RemoveAll},
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
			mkdirAll(t, join(root, "a"))
			request := nativeRequest(root)
			w := startWatch(t, request)
			w.log.waitState(t, fingerprintOf(t, request.Inspect))
			must(t, tc.lose(root))
			if err := w.end(t); err == nil {
				t.Fatal("the watch survived the loss of its root")
			}
			events := w.log.recorded()
			if got := describeEvents(events); got != "started error:watch_root_unavailable! stopped:error" {
				t.Fatalf("events: %s", got)
			}
			assertNoRoot(t, events, root)
		})
	}
}

func TestWatchMatrixObservationFailureNative(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("APFS does not keep two names that differ only in Unicode normalization")
	}
	root := t.TempDir()
	composed := "caf" + string(rune(0x00e9))
	writeFiles(t, join(root, composed))
	request := nativeRequest(root)
	w := startWatch(t, request)
	w.log.waitState(t, fingerprintOf(t, request.Inspect))
	decomposed := join(root, "cafe"+string(rune(0x0301)))
	writeFiles(t, decomposed)
	w.log.waitFor(t, "a rescan for the collision", func(events []watch.Event) bool {
		last := events[len(events)-1]
		return last.Kind == watch.KindRescanStarted && last.Rescan == watch.RescanError
	})
	must(t, os.Remove(decomposed))
	w.log.waitFor(t, "the end of the rescan", func(events []watch.Event) bool {
		return events[len(events)-1].Kind == watch.KindRescanCompleted
	})
	_ = w.interrupt(t)
	events := w.log.recorded()
	if got := describeEvents(events); got != "started error:canonical_path_collision rescan:error completed stopped:interrupted" {
		t.Fatalf("events: %s", got)
	}
	assertNoRoot(t, events, root)
}
