package watcher

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/fswatcher/fswatcher"
	"nmf/internal/fileinfo"
)

type fakeBackend struct {
	events   chan fswatcher.Event
	errs     chan error
	addErr   error
	addCount int
	addPath  string

	closeMu sync.Mutex
	closed  bool

	// closeBlock, if non-nil, makes Close() block until the channel is closed.
	closeBlock chan struct{}
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{
		events: make(chan fswatcher.Event, 10),
		errs:   make(chan error, 10),
	}
}

func (b *fakeBackend) Add(path string, _ fswatcher.Op) error {
	b.addCount++
	b.addPath = path
	return b.addErr
}

func (b *fakeBackend) Remove(string) error { return nil }
func (b *fakeBackend) Close() error {
	if b.closeBlock != nil {
		<-b.closeBlock
	}
	b.closeMu.Lock()
	b.closed = true
	b.closeMu.Unlock()
	return nil
}
func (b *fakeBackend) Events() <-chan fswatcher.Event { return b.events }
func (b *fakeBackend) Errors() <-chan error           { return b.errs }

func (b *fakeBackend) isClosed() bool {
	b.closeMu.Lock()
	defer b.closeMu.Unlock()
	return b.closed
}

// waitFor polls cond until it reports true or timeout elapses, failing the
// test on timeout.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for condition")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestWatchHubSharesSourceForSamePath(t *testing.T) {
	var backends []*fakeBackend
	hub := newWatchHub(dummyDebug, func() (watchBackend, error) {
		backend := newFakeBackend()
		backends = append(backends, backend)
		return backend, nil
	}, func(string) (Snapshot, error) {
		return Snapshot{}, nil
	}, func(path string) (string, bool) {
		return path, true
	}, time.Millisecond)

	first := hub.Subscribe("/tmp/share", time.Second)
	second := hub.Subscribe("/tmp/share", time.Second)

	if len(backends) != 1 {
		t.Fatalf("backend count = %d, want 1", len(backends))
	}
	if backends[0].addCount != 1 {
		t.Fatalf("Add count = %d, want 1", backends[0].addCount)
	}

	first.Unsubscribe()
	hub.mu.Lock()
	_, stillPresent := hub.sources["/tmp/share"]
	hub.mu.Unlock()
	if !stillPresent {
		t.Fatal("source should stay active while one subscriber remains")
	}

	second.Unsubscribe()
	hub.mu.Lock()
	_, stillPresent = hub.sources["/tmp/share"]
	hub.mu.Unlock()
	if stillPresent {
		t.Fatal("source should be removed after last subscriber leaves")
	}
	// Source teardown (including backend Close) runs on a detached goroutine,
	// so wait for it instead of asserting it happened synchronously.
	waitFor(t, time.Second, backends[0].isClosed)
}

func TestWatchSubscriberRejectsStaleBroadcastAfterUnsubscribe(t *testing.T) {
	hub := newWatchHub(dummyDebug, nil, nil, nil, time.Millisecond)
	source := newWatchSource("/tmp/stale", "", time.Second, false, hub)
	subscriber := newWatchSubscriber(1)
	source.addSubscriber(1, subscriber)

	// Model broadcast after it has captured a subscriber reference but before
	// it sends: unsubscribe removes and closes the subscriber first.
	source.mu.Lock()
	captured := source.subscribers[1]
	source.mu.Unlock()
	if empty := source.removeSubscriber(1); !empty {
		t.Fatal("source should have no subscribers after removal")
	}

	if sent, open := captured.send(Snapshot{"/tmp/stale/file": {Path: "/tmp/stale/file"}}); sent || open {
		t.Fatalf("stale send = (sent=%t, open=%t), want closed subscriber", sent, open)
	}
	if _, ok := <-subscriber.updates; ok {
		t.Fatal("subscriber updates channel should be closed")
	}
}

// TestWatchHubUnsubscribeDoesNotBlockOnHangingClose verifies that Unsubscribe
// returns promptly even when the backend's Close() blocks indefinitely, and
// that a fresh Subscribe for the same path works while the old source is
// still tearing down in the background.
func TestWatchHubUnsubscribeDoesNotBlockOnHangingClose(t *testing.T) {
	firstBackend := newFakeBackend()
	firstBackend.closeBlock = make(chan struct{})

	backendCount := 0
	var secondBackend *fakeBackend
	newBackend := func() (watchBackend, error) {
		backendCount++
		if backendCount == 1 {
			return firstBackend, nil
		}
		secondBackend = newFakeBackend()
		return secondBackend, nil
	}

	hub := newWatchHub(dummyDebug, newBackend, func(string) (Snapshot, error) {
		return Snapshot{}, nil
	}, func(path string) (string, bool) {
		return path, true
	}, time.Millisecond)

	sub := hub.Subscribe("/tmp/hang", time.Second)

	done := make(chan struct{})
	go func() {
		sub.Unsubscribe()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Unsubscribe blocked on hanging backend Close")
	}

	hub.mu.Lock()
	_, stillPresent := hub.sources["/tmp/hang"]
	hub.mu.Unlock()
	if stillPresent {
		t.Fatal("source should already be removed from hub map")
	}

	fresh := hub.Subscribe("/tmp/hang", time.Second)
	defer fresh.Unsubscribe()

	waitFor(t, time.Second, func() bool {
		return secondBackend != nil && secondBackend.addCount == 1
	})

	// Release the hung Close and wait for the old source goroutine to
	// finish so the test doesn't leak it.
	close(firstBackend.closeBlock)
	waitFor(t, time.Second, firstBackend.isClosed)
}

func TestWatchHubFallsBackToPollingWhenAddFails(t *testing.T) {
	var listMu sync.Mutex
	listCount := 0
	backend := newFakeBackend()
	backend.addErr = errors.New("watch unsupported")
	hub := newWatchHub(dummyDebug, func() (watchBackend, error) {
		return backend, nil
	}, func(path string) (Snapshot, error) {
		listMu.Lock()
		listCount++
		listMu.Unlock()
		return Snapshot{
			path + "/file.txt": {
				Name:     "file.txt",
				Path:     path + "/file.txt",
				FileType: fileinfo.FileTypeRegular,
				Status:   fileinfo.StatusNormal,
			},
		}, nil
	}, func(path string) (string, bool) {
		return path, true
	}, time.Millisecond)

	sub := hub.Subscribe("/tmp/fallback", 5*time.Millisecond)
	defer sub.Unsubscribe()

	select {
	case snapshot := <-sub.Updates:
		if _, ok := snapshot["/tmp/fallback/file.txt"]; !ok {
			t.Fatalf("snapshot = %#v, want fallback file", snapshot)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for fallback polling snapshot")
	}

	hub.mu.Lock()
	src := hub.sources["/tmp/fallback"]
	hub.mu.Unlock()
	if src == nil || !src.pollFallback {
		t.Fatal("source should record polling fallback")
	}
	listMu.Lock()
	got := listCount
	listMu.Unlock()
	if got == 0 {
		t.Fatal("fallback poller should read at least once")
	}
}

func TestWatchHubDebouncesEventSnapshots(t *testing.T) {
	backend := newFakeBackend()
	var listMu sync.Mutex
	listCount := 0
	hub := newWatchHub(dummyDebug, func() (watchBackend, error) {
		return backend, nil
	}, func(path string) (Snapshot, error) {
		listMu.Lock()
		listCount++
		listMu.Unlock()
		return Snapshot{
			path + "/created.txt": {
				Name:     "created.txt",
				Path:     path + "/created.txt",
				FileType: fileinfo.FileTypeRegular,
				Status:   fileinfo.StatusNormal,
			},
		}, nil
	}, func(path string) (string, bool) {
		return path, true
	}, 5*time.Millisecond)

	first := hub.Subscribe("/tmp/events", time.Second)
	defer first.Unsubscribe()
	second := hub.Subscribe("/tmp/events", time.Second)
	defer second.Unsubscribe()

	backend.events <- fswatcher.Event{Name: "/tmp/events/a", Op: fswatcher.Create}
	backend.events <- fswatcher.Event{Name: "/tmp/events/b", Op: fswatcher.Write}
	backend.events <- fswatcher.Event{Name: "/tmp/events/c", Op: fswatcher.Remove}

	for i, sub := range []*Subscription{first, second} {
		select {
		case snapshot := <-sub.Updates:
			if _, ok := snapshot["/tmp/events/created.txt"]; !ok {
				t.Fatalf("subscriber %d snapshot = %#v, want created file", i, snapshot)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("timed out waiting for subscriber %d snapshot", i)
		}
	}

	time.Sleep(20 * time.Millisecond)
	listMu.Lock()
	got := listCount
	listMu.Unlock()
	if got != 1 {
		t.Fatalf("snapshot reads = %d, want 1 debounced read", got)
	}
}

func TestWatchHubUsesNativeWatchPath(t *testing.T) {
	backend := newFakeBackend()
	hub := newWatchHub(dummyDebug, func() (watchBackend, error) {
		return backend, nil
	}, func(string) (Snapshot, error) {
		return Snapshot{}, nil
	}, func(string) (string, bool) {
		return `\\server\share\dir`, true
	}, time.Millisecond)

	sub := hub.Subscribe("smb://server/share/dir", time.Second)
	sub.Unsubscribe()

	if backend.addPath != `\\server\share\dir` {
		t.Fatalf("Add path = %q, want native UNC path", backend.addPath)
	}
}

func TestWatchHubSkipsFSWatcherForUnwatchablePath(t *testing.T) {
	var backendCreated bool
	hub := newWatchHub(dummyDebug, func() (watchBackend, error) {
		backendCreated = true
		return newFakeBackend(), nil
	}, func(path string) (Snapshot, error) {
		return Snapshot{
			path + "/file.txt": {
				Name:     "file.txt",
				Path:     path + "/file.txt",
				FileType: fileinfo.FileTypeRegular,
				Status:   fileinfo.StatusNormal,
			},
		}, nil
	}, func(string) (string, bool) {
		return "", false
	}, time.Millisecond)

	sub := hub.Subscribe("smb://server/share/dir", 5*time.Millisecond)
	defer sub.Unsubscribe()

	select {
	case <-sub.Updates:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for polling snapshot")
	}
	if backendCreated {
		t.Fatal("backend should not be created for unwatchable path")
	}
}
