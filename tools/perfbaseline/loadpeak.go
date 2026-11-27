package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/dirloom/dirloom/internal/snapshot"
)

// runLoadPeak loads one snapshot document in this process several times and
// prints, for each load, the highest live heap above the heap before it,
// sampled with runtime.ReadMemStats. This is the method of
// docs/benchmarks/v0.4-rc1-snapshot-limits.md and of BenchmarkLoadPeakMemory,
// for documents that the benchmark does not build, such as the long-named
// one.
func runLoadPeak(args []string) error {
	flags := flag.NewFlagSet("loadpeak", flag.ContinueOnError)
	file := flags.String("file", "", "snapshot document to load")
	runs := flags.Int("runs", 5, "loads")
	period := flags.Duration("sample", 2*time.Millisecond, "heap sampling period")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *file == "" {
		return errors.New("loadpeak: -file is required")
	}
	data, err := os.ReadFile(*file) // #nosec G304 -- the document given on the command line.
	if err != nil {
		return err
	}
	for run := 1; run <= *runs; run++ {
		peak, elapsed, err := loadPeakOnce(data, *period)
		if err != nil {
			return err
		}
		fmt.Printf("loadpeak go=%s run=%d bytes=%d peak-MiB=%.0f seconds=%.2f\n",
			runtime.Version(), run, len(data), float64(peak)/(1<<20), elapsed.Seconds())
	}
	return nil
}

func loadPeakOnce(data []byte, period time.Duration) (uint64, time.Duration, error) {
	runtime.GC()
	var base runtime.MemStats
	runtime.ReadMemStats(&base)
	var high atomic.Uint64
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		var m runtime.MemStats
		for {
			select {
			case <-stop:
				return
			default:
			}
			runtime.ReadMemStats(&m)
			if m.HeapAlloc > high.Load() {
				high.Store(m.HeapAlloc)
			}
			time.Sleep(period)
		}
	}()
	start := time.Now()
	_, err := snapshot.Load(bytes.NewReader(data))
	elapsed := time.Since(start)
	close(stop)
	<-done
	if err != nil {
		return 0, elapsed, err
	}
	if high.Load() < base.HeapAlloc {
		return 0, elapsed, nil
	}
	return high.Load() - base.HeapAlloc, elapsed, nil
}
