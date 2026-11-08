package output

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestWriteFileConcurrentWriters replaces one destination from several
// writers at once, as runs with the same output path would (v0.4.0-rc1
// plan, LOT 5e). A replacement may fail, but a reader only ever sees one
// complete payload, never a mix or a truncation, the destination ends as
// one of the payloads, and no temporary file survives. CI runs it under
// -race on Linux, Windows and macOS.
func TestWriteFileConcurrentWriters(t *testing.T) {
	t.Run("writers", func(t *testing.T) {
		result := contend(t, 0)
		if result.written == 0 {
			t.Fatalf("no replacement succeeded out of %d", contendWriters*contendRounds)
		}
	})
	// On Windows, a replacement fails while a reader has the destination
	// open, so this case requires integrity only, not a success.
	t.Run("writers and readers", func(t *testing.T) {
		result := contend(t, 2)
		if result.reads == 0 {
			t.Fatal("no read succeeded")
		}
	})
}

const (
	contendWriters = 6
	contendRounds  = 8
)

type contention struct {
	written, refused, reads int64
}

// contend runs contendWriters writers of contendRounds payloads each on one
// destination, and readers that read it until the writers are done. It
// fails the test on a torn read, a torn destination or a leaked temporary
// file, and returns the counts.
func contend(t *testing.T, readers int) contention {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "tree.txt")

	// Payloads differ in content and size, so that a mix or a truncation
	// is never a valid payload.
	valid := make(map[[sha256.Size]byte]bool)
	payload := func(writer, round int) []byte {
		line := fmt.Sprintf("writer %d round %d\n", writer, round)
		return []byte(strings.Repeat(line, 2000+writer*500+round*7))
	}
	initial := payload(contendWriters, 0)
	valid[sha256.Sum256(initial)] = true
	for writer := range contendWriters {
		for round := range contendRounds {
			valid[sha256.Sum256(payload(writer, round))] = true
		}
	}
	if err := WriteFile(path, initial); err != nil {
		t.Fatal(err)
	}

	var written, refused, reads atomic.Int64
	var refusals sync.Map
	var torn atomic.Int64
	start := make(chan struct{})
	var writing sync.WaitGroup
	for writer := range contendWriters {
		writing.Go(func() {
			<-start
			for round := range contendRounds {
				if err := WriteFile(path, payload(writer, round)); err != nil {
					refused.Add(1)
					refusals.Store(err.Error(), true)
					continue
				}
				written.Add(1)
			}
		})
	}
	done := make(chan struct{})
	var reading sync.WaitGroup
	for range readers {
		reading.Go(func() {
			<-start
			for {
				select {
				case <-done:
					return
				default:
				}
				// A read may fail while a replacement runs; a read that
				// succeeds must give a whole payload.
				if data, err := os.ReadFile(path); err == nil {
					reads.Add(1)
					if !valid[sha256.Sum256(data)] {
						torn.Store(int64(len(data)) + 1)
					}
				}
				time.Sleep(time.Millisecond)
			}
		})
	}
	close(start)
	writing.Wait()
	close(done)
	reading.Wait()

	if size := torn.Load(); size != 0 {
		t.Fatalf("a reader saw %d bytes that are no complete payload", size-1)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !valid[sha256.Sum256(data)] {
		t.Fatalf("the destination holds %d bytes that are no complete payload", len(data))
	}
	matches, err := filepath.Glob(filepath.Join(root, ".tree.txt.tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files leaked: %#v", matches)
	}
	t.Logf("%d replacements, %d refused, %d complete reads", written.Load(), refused.Load(), reads.Load())
	refusals.Range(func(message, _ any) bool {
		t.Logf("refused: %s", message)
		return true
	})
	return contention{written: written.Load(), refused: refused.Load(), reads: reads.Load()}
}
