package main

import (
	"runtime/debug"
	"runtime/metrics"
	"sync"
	"time"

	"nmf/internal/jobs"
)

const (
	idleMemoryDelay    = 60 * time.Second
	idleMemoryMinBytes = 128 << 20
)

// idleMemoryRelease schedules at most one attempt per uninterrupted idle period.
// Window lifecycle callbacks and job notifications may arrive on different goroutines.
type idleMemoryRelease struct {
	jobsActive  func() bool
	estimate    func() uint64
	release     func()
	unsubscribe func()

	mu         sync.Mutex
	background bool
	closed     bool
	attempted  bool
	releasing  bool
	timer      *time.Timer
	generation uint64
}

func newIdleMemoryRelease(manager *jobs.Manager) *idleMemoryRelease {
	r := &idleMemoryRelease{
		jobsActive: manager.HasActiveJobs,
		estimate:   estimateIdleHeapBytes,
		release:    debug.FreeOSMemory,
	}
	r.unsubscribe = manager.Subscribe(r.jobsChanged)
	return r
}

func (r *idleMemoryRelease) setBackground(background bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.background = background
	r.updateTimerLocked()
}

func (r *idleMemoryRelease) jobsChanged() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updateTimerLocked()
}

func (r *idleMemoryRelease) updateTimerLocked() {
	if r.closed {
		return
	}
	if !r.background || r.jobsActive() {
		r.stopTimerLocked()
		r.attempted = false
		return
	}
	if r.timer != nil || r.attempted || r.releasing {
		return
	}
	r.generation++
	generation := r.generation
	r.timer = time.AfterFunc(idleMemoryDelay, func() { r.onIdle(generation) })
}

func (r *idleMemoryRelease) stopTimerLocked() {
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
}

func (r *idleMemoryRelease) onIdle(generation uint64) {
	candidate := r.estimate()
	r.mu.Lock()
	// Stop cannot cancel a callback that already started. Recheck its identity
	// and the live job state after sampling, immediately before claiming it.
	if r.timer == nil || r.generation != generation {
		r.mu.Unlock()
		return
	}
	r.timer = nil
	if r.jobsActive() {
		r.mu.Unlock()
		return
	}
	r.attempted = true
	if candidate < idleMemoryMinBytes {
		r.mu.Unlock()
		debugPrint("Memory: idle release skipped candidate_bytes=%d", candidate)
		return
	}
	r.releasing = true
	r.mu.Unlock()

	started := time.Now()
	r.release()
	debugPrint("Memory: idle release candidate_bytes=%d elapsed=%s", candidate, time.Since(started))

	r.mu.Lock()
	r.releasing = false
	r.updateTimerLocked()
	r.mu.Unlock()
}

func (r *idleMemoryRelease) close() {
	r.mu.Lock()
	r.closed = true
	r.stopTimerLocked()
	unsubscribe := r.unsubscribe
	r.unsubscribe = nil
	r.mu.Unlock()
	if unsubscribe != nil {
		unsubscribe()
	}
}

func estimateIdleHeapBytes() uint64 {
	samples := [...]metrics.Sample{
		{Name: "/memory/classes/heap/free:bytes"},
		{Name: "/memory/classes/heap/objects:bytes"},
		{Name: "/gc/heap/live:bytes"},
	}
	metrics.Read(samples[:])
	free := samples[0].Value.Uint64()
	objects := samples[1].Value.Uint64()
	live := samples[2].Value.Uint64()
	// Free spans can be returned now. Growth in heap objects since the last
	// mark may contain temporary allocations that the next collection can free.
	if objects > live {
		free += objects - live
	}
	return free
}
