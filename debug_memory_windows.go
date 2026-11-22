package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var getProcessMemoryInfo = windows.NewLazySystemDLL("kernel32.dll").NewProc("K32GetProcessMemoryInfo")

// Layout of PROCESS_MEMORY_COUNTERS_EX2 in psapi.h.
type processMemoryCounters struct {
	size, pageFaultCount                     uint32
	peakWorkingSet, workingSet               uintptr
	quotaPeakPagedPool, quotaPagedPool       uintptr
	quotaPeakNonpagedPool, quotaNonpagedPool uintptr
	pagefileUsage, peakPagefileUsage         uintptr
	privateUsage, privateWorkingSet          uintptr
	sharedCommitUsage                        uint64
}

func logDebugProcessMemory(phase string) {
	var m processMemoryCounters
	m.size = uint32(unsafe.Sizeof(m))
	// Older Windows versions may fill only the original EX counters.
	m.privateWorkingSet = ^uintptr(0)
	ok, _, err := getProcessMemoryInfo.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&m)), uintptr(m.size))
	if ok == 0 {
		debugPrint("Memory: phase=%s Windows counters unavailable: %v", phase, err)
		return
	}
	if m.privateWorkingSet == ^uintptr(0) {
		debugPrint("Memory: phase=%s working_set_bytes=%d private_commit_bytes=%d", phase, m.workingSet, m.privateUsage)
		return
	}
	debugPrint("Memory: phase=%s working_set_bytes=%d private_working_set_bytes=%d private_commit_bytes=%d",
		phase, m.workingSet, m.privateWorkingSet, m.privateUsage)
}
