package main

import (
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func TestIdleMemoryReleaseCancelsOnForegroundAndRunsOnce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var releases atomic.Int32
		r := &idleMemoryRelease{
			jobsActive: func() bool { return false },
			estimate:   func() uint64 { return idleMemoryMinBytes },
			release:    func() { releases.Add(1) },
		}
		defer r.close()

		r.setBackground(true)
		time.Sleep(idleMemoryDelay - time.Second)
		r.setBackground(false)
		r.setBackground(true)
		time.Sleep(idleMemoryDelay - time.Second)
		synctest.Wait()
		if got := releases.Load(); got != 0 {
			t.Fatalf("released %d times before a full idle period", got)
		}

		time.Sleep(time.Second)
		synctest.Wait()
		if got := releases.Load(); got != 1 {
			t.Fatalf("released %d times after one idle period, want 1", got)
		}
		r.setBackground(true)
		r.jobsChanged()
		time.Sleep(2 * idleMemoryDelay)
		synctest.Wait()
		if got := releases.Load(); got != 1 {
			t.Fatalf("released %d times in the same idle period, want 1", got)
		}
	})
}

func TestIdleMemoryReleaseRestartsDelayAfterJobsFinish(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var activeJobs atomic.Bool
		var releases atomic.Int32
		r := &idleMemoryRelease{
			jobsActive: activeJobs.Load,
			estimate:   func() uint64 { return idleMemoryMinBytes },
			release:    func() { releases.Add(1) },
		}
		defer r.close()

		r.setBackground(true)
		time.Sleep(idleMemoryDelay / 2)
		activeJobs.Store(true)
		r.jobsChanged()
		time.Sleep(idleMemoryDelay)
		activeJobs.Store(false)
		r.jobsChanged()
		time.Sleep(idleMemoryDelay - time.Second)
		synctest.Wait()
		if got := releases.Load(); got != 0 {
			t.Fatalf("released %d times before jobs were idle for the full delay", got)
		}

		time.Sleep(time.Second)
		synctest.Wait()
		if got := releases.Load(); got != 1 {
			t.Fatalf("released %d times after jobs were idle, want 1", got)
		}
	})
}

func TestIdleMemoryReleaseRechecksForegroundAfterSampling(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sampling := make(chan struct{})
		finishSampling := make(chan struct{})
		var releases atomic.Int32
		r := &idleMemoryRelease{
			jobsActive: func() bool { return false },
			estimate: func() uint64 {
				close(sampling)
				<-finishSampling
				return idleMemoryMinBytes
			},
			release: func() { releases.Add(1) },
		}
		defer r.close()

		r.setBackground(true)
		<-sampling
		r.setBackground(false)
		close(finishSampling)
		synctest.Wait()
		if got := releases.Load(); got != 0 {
			t.Fatalf("released %d times after returning to the foreground", got)
		}
	})
}

func TestIdleMemoryReleaseSkipsSmallHeapAndStopsOnClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var samples, releases atomic.Int32
		unsubscribes := 0
		r := &idleMemoryRelease{
			jobsActive: func() bool { return false },
			estimate: func() uint64 {
				samples.Add(1)
				return idleMemoryMinBytes - 1
			},
			release:     func() { releases.Add(1) },
			unsubscribe: func() { unsubscribes++ },
		}
		r.setBackground(true)
		time.Sleep(idleMemoryDelay)
		synctest.Wait()
		r.jobsChanged()
		time.Sleep(idleMemoryDelay)
		synctest.Wait()
		if gotSamples, gotReleases := samples.Load(), releases.Load(); gotSamples != 1 || gotReleases != 0 {
			t.Fatalf("small heap: samples = %d, releases = %d, want 1 and 0", gotSamples, gotReleases)
		}

		r.setBackground(false)
		r.setBackground(true)
		r.close()
		r.close()
		r.jobsChanged()
		time.Sleep(idleMemoryDelay)
		synctest.Wait()
		if gotSamples, gotReleases := samples.Load(), releases.Load(); gotSamples != 1 || gotReleases != 0 || unsubscribes != 1 {
			t.Fatalf("after close: samples = %d, releases = %d, unsubscribes = %d, want 1, 0, 1", gotSamples, gotReleases, unsubscribes)
		}
	})
}
