package source

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/dirloom/dirloom/internal/filter"
	"github.com/dirloom/dirloom/internal/gitsource"
	"github.com/dirloom/dirloom/internal/gittest"
	"github.com/dirloom/dirloom/internal/tree"
)

// benchmarkRepositories caches one repository per size across -count runs.
// TestMain removes them.
var benchmarkRepositories = struct {
	sync.Mutex
	dirs map[int]string
}{dirs: make(map[int]string)}

func removeBenchmarkRepositories() {
	benchmarkRepositories.Lock()
	defer benchmarkRepositories.Unlock()
	for _, dir := range benchmarkRepositories.dirs {
		if strings.HasPrefix(dir, os.TempDir()) {
			_ = os.RemoveAll(dir)
		}
	}
}

// benchmarkRepository commits about entries tree entries as src/m<i>/f<j>.go,
// modules of nine files, with git fast-import, and checks the commit out.
func benchmarkRepository(b *testing.B, entries int) string {
	b.Helper()
	benchmarkRepositories.Lock()
	defer benchmarkRepositories.Unlock()
	if dir, ok := benchmarkRepositories.dirs[entries]; ok {
		return dir
	}
	gittest.Require(b)
	dir, err := os.MkdirTemp("", "dirloom-git-bench-")
	if err != nil {
		b.Fatal(err)
	}
	benchmarkRepositories.dirs[entries] = dir
	repo := gittest.Open(b, dir)
	repo.Git("init", "--quiet", "--initial-branch=main", "--object-format=sha1")

	var stream bytes.Buffer
	stream.WriteString("blob\nmark :1\ndata 13\npackage bench\ncommit refs/heads/main\n")
	stream.WriteString("author Dirloom <bench@dirloom.invalid> 1767225600 +0000\n")
	stream.WriteString("committer Dirloom <bench@dirloom.invalid> 1767225600 +0000\n")
	stream.WriteString("data 5\nbench\n")
	for module := 0; module < entries/10; module++ {
		for file := 0; file < 9; file++ {
			fmt.Fprintf(&stream, "M 100644 :1 src/m%05d/f%d.go\n", module, file)
		}
	}
	stream.WriteString("\n")
	repo.GitInput(stream.Bytes(), "fast-import", "--quiet")
	repo.Git("reset", "--quiet", "--hard", "main")
	return dir
}

// BenchmarkObserveGitTree measures a whole Git observation (resolution,
// on-demand tree reads, parsing, scanning and artifact construction) against
// a filesystem observation of the same checkout, and a Git observation
// limited to depth 1, which never reads the excluded subtrees. Both sides use
// the same capture: default ignores (which drop the checkout's .git) and
// hidden entries.
func BenchmarkObserveGitTree(b *testing.B) {
	capture := func() *filter.Policy { return filter.NewPolicy("", true, nil, nil, true) }
	for _, entries := range []int{1000, 10000, 100000} {
		dir := benchmarkRepository(b, entries)
		want := 1 + 1 + entries/10*10

		b.Run(fmt.Sprintf("%d/resolve", entries), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				runner, err := gitsource.NewRunner(context.Background(), dir)
				if err != nil {
					b.Fatal(err)
				}
				if _, err := runner.Resolve(context.Background(), "HEAD"); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(fmt.Sprintf("%d/git", entries), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				source := &Git{Revision: "HEAD", Dir: dir, Scan: tree.ScanOptions{RootName: "bench", FilterPolicy: capture()}}
				art, err := source.Observe(context.Background())
				if err != nil {
					b.Fatal(err)
				}
				if art.CountNodes() != want {
					b.Fatalf("%d nodes, want %d", art.CountNodes(), want)
				}
			}
		})
		b.Run(fmt.Sprintf("%d/filesystem", entries), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				scanner := tree.NewScanner(tree.ScanOptions{RootAbs: dir, RootName: "bench", FilterPolicy: capture()})
				source := Filesystem{Scan: scanner.Scan, RootAbs: dir}
				art, err := source.Observe(context.Background())
				if err != nil {
					b.Fatal(err)
				}
				if art.CountNodes() != want {
					b.Fatalf("%d nodes, want %d", art.CountNodes(), want)
				}
			}
		})
		b.Run(fmt.Sprintf("%d/git-depth-1", entries), func(b *testing.B) {
			b.ReportAllocs()
			depth := 1
			for i := 0; i < b.N; i++ {
				source := &Git{Revision: "HEAD", Dir: dir, Scan: tree.ScanOptions{RootName: "bench", MaxDepth: &depth, FilterPolicy: capture()}}
				art, err := source.Observe(context.Background())
				if err != nil {
					b.Fatal(err)
				}
				if art.CountNodes() != 2 {
					b.Fatalf("%d nodes, want 2", art.CountNodes())
				}
			}
		})
	}
}
