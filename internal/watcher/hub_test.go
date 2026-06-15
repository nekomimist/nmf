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
	closed   bool
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
	b.closed = true
	return nil
}
func (b *fakeBackend) Events() <-chan fswatcher.Event { return b.events }
func (b *fakeBackend) Errors() <-chan error           { return b.errs }

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
	if !backends[0].closed {
		t.Fatal("backend should be closed after source stops")
	}
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
