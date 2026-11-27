package main

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/filter"
	"github.com/dirloom/dirloom/internal/snapshot"
	"github.com/dirloom/dirloom/internal/tree"
)

func paths(n artifact.Node, out map[string]artifact.Kind) {
	out[string(n.Path)] = n.Kind
	for _, c := range n.Children {
		paths(c, out)
	}
}

// TestWideArtifactShape: the fixture has exactly the requested node count,
// relocation keeps it, and a relocated directory keeps its name and files
// under zz-moved.
func TestWideArtifactShape(t *testing.T) {
	for _, nodes := range []int{3, 10, 1002, 2500, 10_007} {
		base := wideArtifact(nodes, 0, "")
		if got := countNodes(base.Root); got != nodes {
			t.Fatalf("wideArtifact(%d) has %d nodes", nodes, got)
		}
		before := map[string]artifact.Kind{}
		paths(base.Root, before)
		for _, every := range []int{2, 10} {
			moved := wideArtifact(nodes, every, "")
			if got := countNodes(moved.Root); got != nodes {
				t.Fatalf("wideArtifact(%d, every %d) has %d nodes", nodes, every, got)
			}
			after := map[string]artifact.Kind{}
			paths(moved.Root, after)
			for path, kind := range before {
				if after[path] != kind && after["zz-moved/"+path] != kind {
					t.Fatalf("%s (%s) is neither in place nor under zz-moved", path, kind)
				}
			}
			if _, err := snapshot.Build(moved, defaultCapture()); err != nil {
				t.Fatalf("snapshot.Build(%d nodes, every %d): %v", nodes, every, err)
			}
		}
	}
}

// TestFixturesStayWithinTheirLimits: the documents and files meant to sit
// just under a limit do, and the over ones are over.
func TestFixturesStayWithinTheirLimits(t *testing.T) {
	short := shortGitIgnore()
	if len(short) > tree.MaxGitIgnoreBytes || len(short) < tree.MaxGitIgnoreBytes-64 {
		t.Fatalf("short .gitignore is %d bytes, want just under %d", len(short), tree.MaxGitIgnoreBytes)
	}
	for _, fill := range []byte{'x', '*'} {
		long := longGitIgnore(fill)
		if len(long) > tree.MaxGitIgnoreBytes {
			t.Fatalf("long .gitignore (%c) is %d bytes", fill, len(long))
		}
		if line := filter.LongGitIgnoreLine(long); line != 0 {
			t.Fatalf("long .gitignore (%c) line %d is above the line limit", fill, line)
		}
	}
	patterns := ignorePatterns(snapshot.MaxIgnorePatterns)
	if _, err := filter.NewIgnoreMatcher(patterns); err != nil {
		t.Fatalf("the %d ignore patterns do not compile: %v", len(patterns), err)
	}
	under := chainUnder*chainUnder + 2*chainUnder + len("leaf2")
	over := chainOver * chainOver
	if under > snapshot.MaxBytes || over <= snapshot.MaxBytes {
		t.Fatalf("chain path bytes: under %d, over %d, limit %d", under, over, snapshot.MaxBytes)
	}
}

// TestLooseObjectIDs: the loose objects are Git's, checked on the empty
// blob and the empty tree.
func TestLooseObjectIDs(t *testing.T) {
	objects := t.TempDir()
	for kind, want := range map[string]string{
		"blob": "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391",
		"tree": "4b825dc642cb6eb9a060e54bf8d69288fbee4904",
	} {
		id, err := writeLooseObject(objects, kind, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := hex.EncodeToString(id); got != want {
			t.Fatalf("empty %s id %s, want %s", kind, got, want)
		}
		if _, err := os.Stat(filepath.Join(objects, want[:2], want[2:])); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDistribution(t *testing.T) {
	if s := distribution([]float64{5, 1, 3}); s.N != 3 || s.Median != 3 || s.Min != 1 || s.Max != 5 {
		t.Fatalf("odd: %+v", s)
	}
	if s := distribution([]float64{4, 1, 3, 2}); s.Median != 2.5 {
		t.Fatalf("even: %+v", s)
	}
	if s := distribution(nil); s.N != 0 {
		t.Fatalf("empty: %+v", s)
	}
	values := []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	if p := percentile(values, 90); p != 90 {
		t.Fatalf("p90 = %v", p)
	}
	if p := percentile(values, 100); p != 100 {
		t.Fatalf("p100 = %v", p)
	}
}

func TestParseBytes(t *testing.T) {
	for text, want := range map[string]uint64{"": 0, "0": 0, "8GiB": 8 << 30, "512MiB": 512 << 20, "4KiB": 4096, "123": 123} {
		got, err := parseBytes(text)
		if err != nil || got != want {
			t.Fatalf("parseBytes(%q) = %d, %v; want %d", text, got, err, want)
		}
	}
	if _, err := parseBytes("8GB"); err == nil {
		t.Fatal("parseBytes(8GB) accepted")
	}
}

// TestScenarios: names are unique, every fixture a scenario needs exists,
// and fixture paths expand under the fixture directory.
func TestScenarios(t *testing.T) {
	known := map[string]bool{}
	for _, f := range fixtures() {
		known[f.Name] = true
	}
	seen := map[string]bool{}
	root := filepath.Join("r", "f")
	for _, s := range scenarios() {
		if seen[s.Name] {
			t.Fatalf("scenario %s twice", s.Name)
		}
		seen[s.Name] = true
		if s.Priority != "critical" && s.Priority != "high" {
			t.Fatalf("%s: priority %q", s.Name, s.Priority)
		}
		if len(s.Exits) == 0 {
			t.Fatalf("%s: no accepted exit status", s.Name)
		}
		for _, need := range s.Needs {
			if !known[need] {
				t.Fatalf("%s needs unknown fixture %s", s.Name, need)
			}
		}
		for _, arg := range s.args(root) {
			if strings.Contains(arg, "@") {
				t.Fatalf("%s: unexpanded argument %q", s.Name, arg)
			}
		}
	}
	got := scenario{Args: []string{"@a/b", "snapshot:@c", "live:@d", "git:HEAD"}}.args(root)
	want := []string{filepath.Join(root, "a", "b"), "snapshot:" + filepath.Join(root, "c"), "live:" + filepath.Join(root, "d"), "git:HEAD"}
	if !slices.Equal(got, want) {
		t.Fatalf("args = %q, want %q", got, want)
	}
}

func TestHeadTail(t *testing.T) {
	w := &headTail{limit: 4}
	for _, s := range []string{"ab", "cdef", "gh"} {
		if _, err := w.Write([]byte(s)); err != nil {
			t.Fatal(err)
		}
	}
	if n, head, tail := w.snapshot(); n != 8 || head != "abcd" || tail != "efgh" {
		t.Fatalf("snapshot = %d %q %q", n, head, tail)
	}
}

// TestBurst: three bursts leave the third burst's directory, the second's
// renamed, the first's deleted, and only the last sentinel.
func TestBurst(t *testing.T) {
	dir := t.TempDir()
	for k := 1; k <= 3; k++ {
		ops, _, err := burst(dir, k, 5, "s000"+string(rune('0'+k)))
		if err != nil {
			t.Fatal(err)
		}
		want := map[int]int{1: 7, 2: 9, 3: 15}[k]
		if ops != want {
			t.Fatalf("burst %d: %d operations, want %d", k, ops, want)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if want := []string{"m0002", "n0003", "s0003"}; !slices.Equal(names, want) {
		t.Fatalf("entries %v, want %v", names, want)
	}
	if err := resetBursts(dir); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("%d entries left after reset", len(entries))
	}
}

// fakeRun is a result for the compare tests.
func fakeRun(scenario string, ok bool, alloc, heap uint64, oom bool) result {
	return result{Label: "x", Scenario: scenario, OK: ok, OOM: oom, AllocBytes: alloc, AllocObjects: alloc / 100, HeapPeak: heap, WallNs: 1e9}
}

func campaign(runs ...result) map[string]*group {
	out := map[string]*group{}
	for _, g := range groupResults(runs) {
		out[g.Scenario] = g
	}
	return out
}

func compareFor(t *testing.T, reference, compared map[string]*group) ([]comparison, int, int) {
	t.Helper()
	gating, err := parseMeasures("alloc,objects")
	if err != nil {
		t.Fatal(err)
	}
	informative, err := parseMeasures("wall,heap")
	if err != nil {
		t.Fatal(err)
	}
	return compareCampaigns(reference, compared, compareOptions{Threshold: 0.20, Gating: gating, Informative: informative})
}

func flaggedRows(rows []comparison, flag string) []string {
	var out []string
	for _, r := range rows {
		if r.Flag == flag {
			out = append(out, r.Scenario+"/"+r.Measure+": "+r.Change)
		}
	}
	return out
}

// TestCompareFlags: what D8 must catch is flagged, and only that.
func TestCompareFlags(t *testing.T) {
	five := func(scenario string, alloc, heap uint64) []result {
		runs := make([]result, 5)
		for i := range runs {
			runs[i] = fakeRun(scenario, true, alloc, heap, false)
		}
		return runs
	}
	reference := campaign(slices.Concat(five("load-2m", 1000, 1000), five("bad-nesting", 1000, 1000), five("git-bomb", 1000, 1000))...)

	t.Run("identical", func(t *testing.T) {
		rows, flagged, rerun := compareFor(t, reference, reference)
		if flagged != 0 || rerun != 0 {
			t.Fatalf("identical campaigns: %d flagged, %d rerun: %v", flagged, rerun, rows)
		}
	})

	t.Run("allocation change is flagged, heap change is to rerun", func(t *testing.T) {
		compared := campaign(slices.Concat(five("load-2m", 1300, 1000), five("bad-nesting", 1000, 1300), five("git-bomb", 1000, 1000))...)
		rows, flagged, rerun := compareFor(t, reference, compared)
		if got := flaggedRows(rows, flagInvestigate); flagged != 2 || len(got) != 2 {
			t.Fatalf("flagged %d %v, want the bytes and the allocations of load-2m", flagged, got)
		}
		if got := flaggedRows(rows, flagRerun); rerun != 1 || len(got) != 1 || !strings.HasPrefix(got[0], "bad-nesting/") {
			t.Fatalf("rerun %d %v, want the heap of bad-nesting", rerun, got)
		}
	})

	t.Run("runs that stop failing are flagged even when the rest match", func(t *testing.T) {
		partial := five("bad-nesting", 1000, 1000)
		for i := range 3 {
			partial[i] = fakeRun("bad-nesting", false, 0, 0, true)
		}
		compared := campaign(slices.Concat(five("load-2m", 1000, 1000), partial, five("git-bomb", 1000, 1000))...)
		rows, flagged, _ := compareFor(t, reference, compared)
		got := flaggedRows(rows, flagInvestigate)
		if flagged != 1 || len(got) != 1 || got[0] != "bad-nesting/runs: fewer runs complete" {
			t.Fatalf("flagged %d %v, want bad-nesting's runs", flagged, got)
		}
		for _, r := range rows {
			if r.Scenario == "bad-nesting" && r.Measure == "runs" && r.New != "2/5 (3 out of memory)" {
				t.Fatalf("runs row %+v", r)
			}
		}
	})

	t.Run("a scenario with no completed run is flagged", func(t *testing.T) {
		failed := make([]result, 5)
		for i := range failed {
			failed[i] = fakeRun("bad-nesting", false, 0, 0, false)
		}
		compared := campaign(slices.Concat(five("load-2m", 1000, 1000), failed, five("git-bomb", 1000, 1000))...)
		_, flagged, _ := compareFor(t, reference, compared)
		if flagged != 1 {
			t.Fatalf("flagged %d, want 1", flagged)
		}
	})

	t.Run("missing and new scenarios", func(t *testing.T) {
		compared := campaign(slices.Concat(five("load-2m", 1000, 1000), five("git-bomb", 1000, 1000), five("not-in-reference", 1000, 1000))...)
		rows, flagged, _ := compareFor(t, reference, compared)
		got := flaggedRows(rows, flagInvestigate)
		if flagged != 1 || len(got) != 1 || got[0] != "bad-nesting/runs: missing" {
			t.Fatalf("flagged %d %v, want bad-nesting missing", flagged, got)
		}
		found := false
		for _, r := range rows {
			found = found || (r.Scenario == "not-in-reference" && r.Change == "new scenario" && r.Flag == "")
		}
		if !found {
			t.Fatalf("no unflagged row for the new scenario: %v", rows)
		}
	})
}

// TestFixturePaths: the runner checks every path a scenario reads.
func TestFixturePaths(t *testing.T) {
	root := filepath.Join("r", "f")
	s := scenario{Dir: "git-wide-2m", Args: []string{"diff", "snapshot:@snap/a.json", "live:@live", "@x/y", "git:HEAD"}}
	want := []string{filepath.Join(root, "git-wide-2m"), filepath.Join(root, "snap", "a.json"), filepath.Join(root, "live"), filepath.Join(root, "x", "y")}
	if got := s.fixturePaths(root); !slices.Equal(got, want) {
		t.Fatalf("fixturePaths = %q, want %q", got, want)
	}
}

// TestCompareExactThreshold: a change of exactly 20% is flagged, whatever
// the floating-point rounding of the scaled medians.
func TestCompareExactThreshold(t *testing.T) {
	reference := campaign(fakeRun("load-2m", true, 1000, 1000, false))
	compared := campaign(fakeRun("load-2m", true, 1200, 1000, false))
	rows, flagged, _ := compareFor(t, reference, compared)
	if flagged != 2 {
		t.Fatalf("flagged %d, want bytes and allocations at exactly +20%%: %v", flagged, rows)
	}
}
