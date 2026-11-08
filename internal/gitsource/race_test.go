package gitsource

import (
	"context"
	"slices"
	"sync"
	"testing"

	"github.com/dirloom/dirloom/internal/gittest"
)

// The tests of this file give the race detector the concurrent paths of the
// package (v0.4.0-rc1 plan, LOT 5e). CI runs them under -race on Linux,
// Windows and macOS.

// TestConcurrentSessions opens sessions on one repository from several
// goroutines at once, with an empty version cache: the process-wide version
// check and the batch processes must not share unguarded state, and every
// session reads the same tree.
func TestConcurrentSessions(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("a.txt", "alpha")
	repo.Write("dir/b.txt", "beta")
	repo.CommitAll("first")
	versionChecks.Lock()
	clear(versionChecks.results)
	versionChecks.Unlock()

	const sessions = 8
	names := make([][]string, sessions)
	errs := make([]error, sessions)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range sessions {
		wg.Go(func() {
			<-start
			names[i], errs[i] = readRoot(context.Background(), repo.Dir)
		})
	}
	close(start)
	wg.Wait()

	for i := range sessions {
		if errs[i] != nil {
			t.Fatalf("session %d: %v", i, errs[i])
		}
		if !slices.Equal(names[i], []string{"a.txt", "dir"}) {
			t.Fatalf("session %d read %v", i, names[i])
		}
	}
}

// readRoot lists the root tree of HEAD through a session of its own.
func readRoot(ctx context.Context, dir string) (names []string, err error) {
	session, err := OpenSession(ctx, dir)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := session.Close(); err == nil {
			err = closeErr
		}
	}()
	resolved, err := session.Resolve(ctx, "HEAD")
	if err != nil {
		return nil, err
	}
	backend, err := session.Backend()
	if err != nil {
		return nil, err
	}
	entries, err := backend.ReadDir(resolved.Tree)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names, nil
}

// TestBatchCloseRacesCancellation cancels the context of a batch while Close
// runs: the shutdown started by the cancellation and the one Close starts
// must meet, the process be reaped once, and Close return without error.
// The two calls are released together, so the runs cover both orders.
func TestBatchCloseRacesCancellation(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("a.txt", "alpha")
	repo.CommitAll("first")
	tree := repo.Git("rev-parse", "HEAD^{tree}")
	runner := realRunner(t, repo)

	for range 20 {
		ctx, cancel := context.WithCancel(context.Background())
		batch, err := runner.StartBatch(ctx, SHA1)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		if _, err := batch.read(tree, treeObject); err != nil {
			cancel()
			t.Fatal(err)
		}

		start := make(chan struct{})
		var closeErr error
		var wg sync.WaitGroup
		wg.Go(func() {
			<-start
			cancel()
		})
		wg.Go(func() {
			<-start
			closeErr = batch.Close()
		})
		close(start)
		wg.Wait()

		if closeErr != nil {
			t.Fatalf("close during cancellation: %v", closeErr)
		}
		if batch.cmd.ProcessState == nil {
			t.Fatal("cat-file was not reaped")
		}
		if _, err := batch.read(tree, treeObject); err == nil {
			t.Fatal("read after close succeeded")
		}
	}
}
