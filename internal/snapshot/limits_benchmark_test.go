package snapshot

import (
	"bytes"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dirloom/dirloom/internal/artifact"
)

// BenchmarkLoadPeakMemory measures the peak live heap of Load on valid
// snapshots up to the node limit, the measurement behind the reader limits
// (decision D6 of the v0.4.0-rc1 plan). Run it explicitly:
//
//	go test -run '^$' -bench BenchmarkLoadPeakMemory -benchtime 1x ./internal/snapshot/
func BenchmarkLoadPeakMemory(b *testing.B) {
	for _, nodes := range []int{100_000, 1_000_000, MaxNodes} {
		b.Run(fmt.Sprintf("nodes=%d", nodes), func(b *testing.B) {
			data := peakBenchmarkSnapshot(b, nodes)
			b.SetBytes(int64(len(data)))
			b.ResetTimer()
			var peak uint64
			for range b.N {
				peak = max(peak, livePeak(func() {
					if _, err := Load(bytes.NewReader(data)); err != nil {
						b.Fatal(err)
					}
				}))
			}
			b.ReportMetric(float64(peak)/(1<<20), "peak-MiB")
			b.ReportMetric(float64(peak)/float64(len(data)), "peak/bytes")
		})
	}
}

// peakBenchmarkSnapshot returns a valid snapshot of exactly nodes nodes, the
// root and directories included: up to 1000 files per directory, about 82
// bytes per node.
func peakBenchmarkSnapshot(b *testing.B, nodes int) []byte {
	b.Helper()
	root := artifact.Node{Path: artifact.RootPath, Name: ".", Kind: artifact.KindDirectory}
	count := 1
	for d := 0; count < nodes; d++ {
		dname := fmt.Sprintf("d%05d", d)
		dir := artifact.Node{Path: artifact.Path(dname), Name: dname, Kind: artifact.KindDirectory}
		count++
		for f := 0; f < 1000 && count < nodes; f++ {
			fname := fmt.Sprintf("file-%06d.txt", f)
			dir.Children = append(dir.Children, artifact.Node{Path: artifact.Path(dname + "/" + fname), Name: fname, Kind: artifact.KindFile})
			count++
		}
		root.Children = append(root.Children, dir)
	}
	doc, err := Build(artifact.Artifact{Root: root}, CaptureV1{Ignore: []string{}})
	if err != nil {
		b.Fatal(err)
	}
	data, err := Marshal(doc)
	if err != nil {
		b.Fatal(err)
	}
	return data
}

// livePeak runs f and returns the highest live heap above the starting one,
// sampled every 2 ms.
func livePeak(f func()) uint64 {
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
			time.Sleep(2 * time.Millisecond)
		}
	}()
	f()
	close(stop)
	<-done
	if high.Load() < base.HeapAlloc {
		return 0
	}
	return high.Load() - base.HeapAlloc
}
