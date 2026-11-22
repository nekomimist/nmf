package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"time"
)

func logDebugMemory(phase string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	debugPrint("Memory: phase=%s heap_alloc_bytes=%d heap_idle_unreleased_bytes=%d heap_released_bytes=%d go_sys_minus_released_bytes=%d total_alloc_bytes=%d num_gc=%d",
		phase, m.HeapAlloc, m.HeapIdle-m.HeapReleased, m.HeapReleased, m.Sys-m.HeapReleased, m.TotalAlloc, m.NumGC)
	logDebugProcessMemory(phase)
}

func writeDebugHeapProfile(dir, trigger string) (string, error) {
	logDebugMemory("before_gc")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating heap profile directory: %w", err)
	}
	f, err := os.CreateTemp(dir, "nmf-heap-"+trigger+"-"+time.Now().Format("20060102-150405")+"-*.pprof")
	if err != nil {
		return "", fmt.Errorf("creating heap profile: %w", err)
	}
	runtime.GC()
	logDebugMemory("after_gc")
	err = pprof.WriteHeapProfile(f)
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("writing heap profile %q: %w", f.Name(), err)
	}
	return f.Name(), nil
}
