//go:build unix

package main

import (
	"errors"
	"runtime"

	"golang.org/x/sys/unix"
)

// selfPeaks returns the peak resident set of the current process. Unix has
// no portable peak commit, so private is 0. The reference machine of the
// baselines is Windows; this keeps the tool usable elsewhere.
func selfPeaks() (workingSet, private uint64, err error) {
	var usage unix.Rusage
	if err := unix.Getrusage(unix.RUSAGE_SELF, &usage); err != nil {
		return 0, 0, err
	}
	rss := uint64(max(usage.Maxrss, 0)) // #nosec G115 -- clamped to non-negative.
	if runtime.GOOS != "darwin" && runtime.GOOS != "ios" {
		rss *= 1024 // kilobytes everywhere but Darwin
	}
	return rss, 0, nil
}

// job has no Unix equivalent here: a memory limit needs a cgroup or a
// container set up outside the tool.
type job struct{}

func newJob(limit uint64) (*job, error) {
	if limit > 0 {
		return nil, errors.New("a memory limit needs a Windows job object; on Unix, run the tool inside a cgroup or a container")
	}
	return &job{}, nil
}

func limitSupported() bool { return false }

type measuredProcess struct{}

func openMeasured(int) (*measuredProcess, error)                  { return &measuredProcess{}, nil }
func (*job) assign(*measuredProcess) error                        { return nil }
func (*job) peakCommit() (jobPeak, processPeak uint64, err error) { return 0, 0, nil }
func (*job) terminate()                                           {}
func (*job) close()                                               {}
func (*measuredProcess) peaks() (workingSet, private uint64, err error) {
	return 0, 0, nil
}
func (*measuredProcess) close() {}
