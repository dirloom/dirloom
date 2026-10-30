package source

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/gittest"
	"github.com/dirloom/dirloom/internal/identity"
	"github.com/dirloom/dirloom/internal/tree"
)

// awkwardNamesFingerprint is the Fingerprint v1 of the tree built by
// TestGitSourceAwkwardNamesArePlatformIndependent. It must be the same on
// every platform.
const awkwardNamesFingerprint = "dlm:v1:sha256:43fee2003ea7e770851eec0260b8fdc26608e9633a23589d9de29fd82324da0f"

// TestGitSourceAwkwardNamesArePlatformIndependent observes names that Git
// sorts differently from Dirloom, that sort before ".", that NFC changes, or
// that Windows cannot materialize. The tree is written as raw bytes, in an
// order Git would not use.
func TestGitSourceAwkwardNamesArePlatformIndependent(t *testing.T) {
	repo := gittest.New(t, "--object-format=sha1")
	blob := repo.Blob([]byte("x"))
	inner := repo.RawTree(gittest.Entry{Mode: "100644", Name: "x", ID: blob})
	root := repo.RawTree(
		gittest.Entry{Mode: "100644", Name: "z", ID: blob},
		gittest.Entry{Mode: "100644", Name: "Z", ID: blob},
		gittest.Entry{Mode: "40000", Name: "a", ID: inner},
		gittest.Entry{Mode: "100644", Name: "a-b", ID: blob},
		gittest.Entry{Mode: "100644", Name: "a.b", ID: blob},
		gittest.Entry{Mode: "100644", Name: "a0", ID: blob},
		gittest.Entry{Mode: "100644", Name: "-a", ID: blob},
		gittest.Entry{Mode: "100644", Name: "!b", ID: blob},
		gittest.Entry{Mode: "100755", Name: ".hidden", ID: blob},
		gittest.Entry{Mode: "100644", Name: "é", ID: blob},
		gittest.Entry{Mode: "100644", Name: "a b", ID: blob},
		gittest.Entry{Mode: "100644", Name: `back\slash`, ID: blob},
		gittest.Entry{Mode: "100644", Name: "CON", ID: blob},
		gittest.Entry{Mode: "100644", Name: "x:y", ID: blob},
		gittest.Entry{Mode: "120000", Name: "l", ID: repo.Blob([]byte(`..\up/e` + "́"))},
		gittest.Entry{Mode: "160000", Name: "m", ID: strings.Repeat("7", 40)},
	)
	art, _, err := observeGit(t, repo.Dir, root, tree.ScanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	requireNodes(t, art,
		". directory", "!b file", "-a file", ".hidden file", "CON file", "Z file",
		"a directory", "a/x file", "a b file", "a-b file", "a.b file", "a0 file",
		`back\slash file`, "é file", "l symlink -> ..\\up/é", "m directory", "x:y file", "z file",
	)
	fingerprint, err := identity.Compute(*art)
	if err != nil {
		t.Fatal(err)
	}
	if got := fingerprint.String(); got != awkwardNamesFingerprint {
		t.Fatalf("fingerprint %s, want %s", got, awkwardNamesFingerprint)
	}
}

// countdownContext is cancelled by its n-th Err call: a deterministic way to
// cancel an observation that is under way.
type countdownContext struct {
	context.Context
	mu        sync.Mutex
	remaining int
	done      chan struct{}
}

func newCountdownContext(n int) *countdownContext {
	return &countdownContext{Context: context.Background(), remaining: n, done: make(chan struct{})}
}

func (c *countdownContext) Done() <-chan struct{} { return c.done }

func (c *countdownContext) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.remaining > 0 {
		c.remaining--
		if c.remaining > 0 {
			return nil
		}
		close(c.done)
	}
	return context.Canceled
}

func TestGitSourceCancellationDuringObservation(t *testing.T) {
	repo := gittest.New(t)
	for i := 0; i < 40; i++ {
		repo.Write(fmt.Sprintf("d%02d/sub/f.txt", i), "x")
	}
	repo.CommitAll("wide")

	cancelled := 0
	for _, n := range []int{1, 5, 20, 60, 120, 100000} {
		start := time.Now()
		source := &Git{Revision: "HEAD", Dir: repo.Dir, Scan: tree.ScanOptions{RootName: "repo"}}
		art, err := source.Observe(newCountdownContext(n))
		if elapsed := time.Since(start); elapsed > 10*time.Second {
			t.Fatalf("n=%d: observation took %v", n, elapsed)
		}
		switch {
		case err == nil:
			if art.CountNodes() != 1+40*3 {
				t.Fatalf("n=%d: %d nodes", n, art.CountNodes())
			}
		case errors.Is(err, context.Canceled) || strings.Contains(err.Error(), context.Canceled.Error()):
			cancelled++
			if source.Tree() != "" {
				t.Fatalf("n=%d: a cancelled observation recorded a tree", n)
			}
		default:
			t.Fatalf("n=%d: %v", n, err)
		}
	}
	if cancelled < 3 {
		t.Fatalf("only %d observations were cancelled", cancelled)
	}
}

func TestGitSourceCorruptObjectIsUnavailable(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("src/inner/file.go", "package inner")
	repo.CommitAll("first")
	id := repo.Git("rev-parse", "HEAD:src/inner")
	path := filepath.Join(repo.Dir, ".git", "objects", id[:2], id[2:])
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not a zlib stream"), 0o600); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, _, err := observeGit(t, repo.Dir, "HEAD", tree.ScanOptions{})
	requireGitCode(t, err, artifact.CodeGitObjectUnavailable, `read directory "src/inner"`)
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("observation took %v", elapsed)
	}
	// Excluded by depth, the corrupt tree is never read.
	if _, _, err := observeGit(t, repo.Dir, "HEAD", tree.ScanOptions{MaxDepth: intPointer(2)}); err != nil {
		t.Fatal(err)
	}
}

func TestGitSourceTreeAboveLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("writes a 32 MiB object")
	}
	repo := gittest.New(t)
	huge := repo.GitInput(make([]byte, 32<<20+1), "hash-object", "-t", "tree", "-w", "--literally", "--stdin")
	start := time.Now()
	_, _, err := observeGit(t, repo.Dir, huge, tree.ScanOptions{})
	requireGitCode(t, err, artifact.CodeGitTreeInvalid, "33554433 bytes", "32 MiB limit")
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("observation took %v", elapsed)
	}
}
