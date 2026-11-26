//go:build windows

package main

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// processMemoryCounters is PROCESS_MEMORY_COUNTERS (psapi.h).
type processMemoryCounters struct {
	cb                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

var procGetProcessMemoryInfo = windows.NewLazySystemDLL("kernel32.dll").NewProc("K32GetProcessMemoryInfo")

// processPeaks returns the peak working set and the peak private commit
// (PeakPagefileUsage) of a process. The counters stay readable after the
// process exits, as long as the handle is open.
func processPeaks(process windows.Handle) (workingSet, private uint64, err error) {
	var counters processMemoryCounters
	counters.cb = uint32(unsafe.Sizeof(counters))
	// #nosec G103 -- the Win32 call fills a PROCESS_MEMORY_COUNTERS that this function owns.
	r, _, callErr := procGetProcessMemoryInfo.Call(uintptr(process), uintptr(unsafe.Pointer(&counters)), uintptr(counters.cb))
	if r == 0 {
		return 0, 0, fmt.Errorf("GetProcessMemoryInfo: %w", callErr)
	}
	return uint64(counters.PeakWorkingSetSize), uint64(counters.PeakPagefileUsage), nil
}

// selfPeaks returns the peaks of the current process.
func selfPeaks() (workingSet, private uint64, err error) {
	return processPeaks(windows.CurrentProcess())
}

// job is a Windows job object that holds one measured process and the Git
// processes it starts. It always records the peak commit of the job; with a
// limit, the job refuses to commit more than the limit, so an allocation
// beyond it fails as it would on a machine without that memory.
type job struct {
	handle windows.Handle
}

func newJob(limit uint64) (*job, error) {
	handle, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("CreateJobObject: %w", err)
	}
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if limit > 0 {
		info.BasicLimitInformation.LimitFlags |= windows.JOB_OBJECT_LIMIT_JOB_MEMORY
		info.JobMemoryLimit = uintptr(limit)
	}
	// #nosec G103 -- the Win32 call reads a JOBOBJECT_EXTENDED_LIMIT_INFORMATION that this function owns.
	if _, err := windows.SetInformationJobObject(handle, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		_ = windows.CloseHandle(handle)
		return nil, fmt.Errorf("SetInformationJobObject: %w", err)
	}
	return &job{handle: handle}, nil
}

// limitSupported reports whether this platform can run a measured process
// under a memory limit.
func limitSupported() bool { return true }

// measuredProcess is an open handle on a started child process.
type measuredProcess struct {
	handle windows.Handle
}

func openMeasured(pid int) (*measuredProcess, error) {
	const access = windows.PROCESS_QUERY_LIMITED_INFORMATION | windows.PROCESS_SET_QUOTA | windows.PROCESS_TERMINATE
	// #nosec G115 -- a process id always fits in a DWORD.
	handle, err := windows.OpenProcess(access, false, uint32(pid))
	if err != nil {
		return nil, fmt.Errorf("OpenProcess: %w", err)
	}
	return &measuredProcess{handle: handle}, nil
}

func (j *job) assign(p *measuredProcess) error {
	if err := windows.AssignProcessToJobObject(j.handle, p.handle); err != nil {
		return fmt.Errorf("AssignProcessToJobObject: %w", err)
	}
	return nil
}

// peakCommit returns the peak commit of the whole job (the measured process
// and its children) and of its largest process.
func (j *job) peakCommit() (jobPeak, processPeak uint64, err error) {
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	// #nosec G103 -- the Win32 call fills a JOBOBJECT_EXTENDED_LIMIT_INFORMATION that this function owns.
	if err := windows.QueryInformationJobObject(j.handle, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), nil); err != nil {
		return 0, 0, fmt.Errorf("QueryInformationJobObject: %w", err)
	}
	return uint64(info.PeakJobMemoryUsed), uint64(info.PeakProcessMemoryUsed), nil
}

func (j *job) terminate() {
	_ = windows.TerminateJobObject(j.handle, 1)
}

func (j *job) close() {
	_ = windows.CloseHandle(j.handle)
}

func (p *measuredProcess) peaks() (workingSet, private uint64, err error) {
	return processPeaks(p.handle)
}

func (p *measuredProcess) close() {
	_ = windows.CloseHandle(p.handle)
}
