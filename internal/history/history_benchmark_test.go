package history

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/dirloom/dirloom/internal/gittest"
	"github.com/dirloom/dirloom/internal/source"
	"github.com/dirloom/dirloom/internal/tree"
)

// benchmarkRepositories caches one repository per (entries, commits) across
// -count runs. TestMain removes them.
var benchmarkRepositories = struct {
	sync.Mutex
	dirs map[string]string
}{dirs: make(map[string]string)}

func removeBenchmarkRepositories() {
	benchmarkRepositories.Lock()
	defer benchmarkRepositories.Unlock()
	for _, dir := range benchmarkRepositories.dirs {
		if strings.HasPrefix(dir, os.TempDir()) {
			_ = os.RemoveAll(dir)
		}
	}
}

// benchmarkRepository builds, with git fast-import, a first commit of about
// entries files (src/m<i>/f<j>.go, modules of nine files) followed by
// commits-1 commits that each add one file, delete one file in another
// module and, every fifth commit, also rewrite README.md (contents only).
func benchmarkRepository(b *testing.B, entries, commits int) string {
	b.Helper()
	key := fmt.Sprintf("%d/%d", entries, commits)
	benchmarkRepositories.Lock()
	defer benchmarkRepositories.Unlock()
	if dir, ok := benchmarkRepositories.dirs[key]; ok {
		return dir
	}
	gittest.Require(b)
	dir, err := os.MkdirTemp("", "dirloom-history-bench-")
	if err != nil {
		b.Fatal(err)
	}
	benchmarkRepositories.dirs[key] = dir
	repo := gittest.Open(b, dir)
	repo.Git("init", "--quiet", "--initial-branch=main", "--object-format=sha1")

	modules := entries / 10
	var stream bytes.Buffer
	stream.WriteString("blob\nmark :1\ndata 13\npackage bench\n")
	stream.WriteString("blob\nmark :2\ndata 7\nreadme1\n")
	stream.WriteString("blob\nmark :3\ndata 7\nreadme2\n")
	for c := 0; c < commits; c++ {
		fmt.Fprintf(&stream, "commit refs/heads/main\ncommitter Dirloom <bench@dirloom.invalid> %d +0000\ndata 5\nc%04d\n", 1767225600+60*c, c)
		if c == 0 {
			stream.WriteString("M 100644 :2 README.md\n")
			for module := 0; module < modules; module++ {
				for file := 0; file < 9; file++ {
					fmt.Fprintf(&stream, "M 100644 :1 src/m%05d/f%d.go\n", module, file)
				}
			}
			continue
		}
		fmt.Fprintf(&stream, "M 100644 :1 src/m%05d/added%04d.go\n", c%modules, c)
		fmt.Fprintf(&stream, "D src/m%05d/f%d.go\n", (c+modules/2)%modules, (c/modules)%9)
		if c%5 == 0 {
			fmt.Fprintf(&stream, "M 100644 :%d README.md\n", 2+(c/5)%2)
		}
	}
	stream.WriteString("\n")
	repo.GitInput(stream.Bytes(), "fast-import", "--quiet")
	return dir
}

// BenchmarkHistoryWalk measures whole histories: resolution, one batch
// process, every commit and tree read, projections, fingerprints and
// summaries.
func BenchmarkHistoryWalk(b *testing.B) {
	for _, tc := range []struct{ entries, commits int }{
		{1000, 20}, {1000, 100}, {10000, 20}, {10000, 100}, {100000, 20},
	} {
		dir := benchmarkRepository(b, tc.entries, 101)
		b.Run(fmt.Sprintf("%d-entries/%d-commits", tc.entries, tc.commits), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				result, err := Walk(context.Background(), Request{Dir: dir, Limit: tc.commits})
				if err != nil {
					b.Fatal(err)
				}
				if len(result.Entries) != tc.commits || result.End != EndLimit {
					b.Fatalf("%d entries, end %s", len(result.Entries), result.End)
				}
			}
		})
	}
}

// BenchmarkHistoryVersusDiff obtains the same 20 summaries with 20 separate
// comparisons of git:<parent> and git:<commit>, as 20 dirloom diff runs
// would.
func BenchmarkHistoryVersusDiff(b *testing.B) {
	for _, entries := range []int{1000, 10000} {
		dir := benchmarkRepository(b, entries, 101)
		b.Run(fmt.Sprintf("%d-entries/20-diffs", entries), func(b *testing.B) {
			b.ReportAllocs()
			gitSource := func(revision string) *source.Git {
				return &source.Git{Revision: revision, Dir: dir, Scan: tree.ScanOptions{RootName: "git"}}
			}
			for i := 0; i < b.N; i++ {
				for k := 0; k < 20; k++ {
					diff, err := comparison.Compare(context.Background(), gitSource(fmt.Sprintf("HEAD~%d", k+1)), gitSource(fmt.Sprintf("HEAD~%d", k)),
						comparison.Options{MoveDetection: comparison.MoveDetectionStructuralV1})
					if err != nil {
						b.Fatal(err)
					}
					if diff.Summary.Total == 0 {
						b.Fatalf("pair %d has no structural change", k)
					}
				}
			}
		})
	}
}

// BenchmarkHistoryRetainedEntries reports the heap a finished history keeps
// per entry, for --limit 100 and 1000 over a 1,000-entry tree.
func BenchmarkHistoryRetainedEntries(b *testing.B) {
	dir := benchmarkRepository(b, 1000, 1001)
	for _, limit := range []int{100, 1000} {
		b.Run(fmt.Sprintf("1000-entries/limit-%d", limit), func(b *testing.B) {
			var retained float64
			for i := 0; i < b.N; i++ {
				var before, after runtime.MemStats
				runtime.GC()
				runtime.ReadMemStats(&before)
				result, err := Walk(context.Background(), Request{Dir: dir, Limit: limit})
				if err != nil {
					b.Fatal(err)
				}
				runtime.GC()
				runtime.ReadMemStats(&after)
				if len(result.Entries) != limit {
					b.Fatalf("%d entries", len(result.Entries))
				}
				retained += float64(int64(after.HeapAlloc)-int64(before.HeapAlloc)) / float64(limit)
				runtime.KeepAlive(result)
			}
			b.ReportMetric(retained/float64(b.N), "retained-B/entry")
		})
	}
}
