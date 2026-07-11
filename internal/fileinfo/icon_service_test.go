package fileinfo

import (
	"testing"
	"time"
)

func TestIconServiceCloseIsIdempotentAndRejectsNewWork(t *testing.T) {
	service := NewIconService(nil)
	service.Close()
	service.Close()

	service.enqueue("ext", ".txt", 16)
	service.mu.RLock()
	_, pending := service.pending["ext|.txt"]
	service.mu.RUnlock()
	if pending {
		t.Fatal("closed icon service should not retain new work")
	}
}

func TestIconServiceCloseReleasesSubscribers(t *testing.T) {
	service := NewIconService(nil)
	called := make(chan struct{}, 1)
	service.OnUpdated(func() { called <- struct{}{} })
	service.Close()
	service.flagUpdated()

	select {
	case <-called:
		t.Fatal("closed icon service invoked an update subscriber")
	case <-time.After(75 * time.Millisecond):
	}

	service.updMu.Lock()
	defer service.updMu.Unlock()
	if len(service.subscribers) != 0 {
		t.Fatalf("subscriber count = %d, want 0 after Close", len(service.subscribers))
	}
}
