package watcher

import (
	"errors"
	"sync"
	"time"

	"github.com/fswatcher/fswatcher"
	"nmf/internal/fileinfo"
)

const defaultDebounceInterval = 200 * time.Millisecond

// Snapshot is a complete directory state keyed by portable display path.
type Snapshot map[string]fileinfo.FileInfo

// Subscription receives shared directory snapshots until Unsubscribe is called.
type Subscription struct {
	Updates     <-chan Snapshot
	unsubscribe func()
}

// Unsubscribe detaches the subscription from its shared path source.
func (s *Subscription) Unsubscribe() {
	if s == nil || s.unsubscribe == nil {
		return
	}
	s.unsubscribe()
	s.unsubscribe = nil
}

type watchBackend interface {
	Add(path string, op fswatcher.Op) error
	Remove(path string) error
	Close() error
	Events() <-chan fswatcher.Event
	Errors() <-chan error
}

type fswatcherBackend struct {
	w *fswatcher.Watcher
}

func newFSWatcherBackend() (watchBackend, error) {
	w, err := fswatcher.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &fswatcherBackend{w: w}, nil
}

func (b *fswatcherBackend) Add(path string, op fswatcher.Op) error { return b.w.Add(path, op) }
func (b *fswatcherBackend) Remove(path string) error               { return b.w.Remove(path) }
func (b *fswatcherBackend) Close() error                           { return b.w.Close() }
func (b *fswatcherBackend) Events() <-chan fswatcher.Event         { return b.w.Events }
func (b *fswatcherBackend) Errors() <-chan error                   { return b.w.Errors }

type listSnapshotFunc func(path string) (Snapshot, error)
type backendFactoryFunc func() (watchBackend, error)
type watchPathFunc func(path string) (string, bool)

// WatchHub shares one OS watcher or fallback poller per path across windows.
type WatchHub struct {
	mu             sync.Mutex
	sources        map[string]*watchSource
	initializing   map[string]*watchSourceInit
	nextSubscriber uint64
	newBackend     backendFactoryFunc
	listSnapshot   listSnapshotFunc
	watchPath      watchPathFunc
	debounce       time.Duration
	debugPrint     func(format string, args ...interface{})
}

type watchSourceInit struct {
	ready chan struct{}
}

// NewWatchHub creates an application-wide shared watcher hub.
func NewWatchHub(debugPrint func(format string, args ...interface{})) *WatchHub {
	return newWatchHub(debugPrint, newFSWatcherBackend, readSnapshot, resolveWatchPath, defaultDebounceInterval)
}

func newWatchHub(debugPrint func(format string, args ...interface{}), newBackend backendFactoryFunc, listSnapshot listSnapshotFunc, watchPath watchPathFunc, debounce time.Duration) *WatchHub {
	if debugPrint == nil {
		debugPrint = func(string, ...interface{}) {}
	}
	if debounce <= 0 {
		debounce = defaultDebounceInterval
	}
	return &WatchHub{
		sources:      make(map[string]*watchSource),
		initializing: make(map[string]*watchSourceInit),
		newBackend:   newBackend,
		listSnapshot: listSnapshot,
		watchPath:    watchPath,
		debounce:     debounce,
		debugPrint:   debugPrint,
	}
}

// Subscribe attaches to the shared source for path. The interval is used only
// when the source must fall back to polling.
func (h *WatchHub) Subscribe(path string, interval time.Duration) *Subscription {
	if interval <= 0 {
		interval = 2 * time.Second
	}

	for {
		h.mu.Lock()
		if src := h.sources[path]; src != nil {
			subscription := h.subscribeLocked(path, src)
			h.mu.Unlock()
			return subscription
		}
		if pending := h.initializing[path]; pending != nil {
			ready := pending.ready
			h.mu.Unlock()
			<-ready
			continue
		}
		pending := &watchSourceInit{ready: make(chan struct{})}
		h.initializing[path] = pending
		h.mu.Unlock()

		watchPath, useFSWatcher := h.watchPath(path)
		src := newWatchSource(path, watchPath, interval, useFSWatcher, h)
		src.start()

		h.mu.Lock()
		delete(h.initializing, path)
		h.sources[path] = src
		close(pending.ready)
		subscription := h.subscribeLocked(path, src)
		h.mu.Unlock()
		h.debugPrint("WatchHub: source start path=%s fallback_interval=%s", path, interval)
		return subscription
	}
}

func (h *WatchHub) subscribeLocked(path string, src *watchSource) *Subscription {
	h.nextSubscriber++
	id := h.nextSubscriber
	subscriber := newWatchSubscriber(10)
	src.addSubscriber(id, subscriber)

	var once sync.Once
	return &Subscription{
		Updates: subscriber.updates,
		unsubscribe: func() {
			once.Do(func() {
				h.unsubscribe(path, id)
			})
		},
	}
}

func (h *WatchHub) unsubscribe(path string, id uint64) {
	h.mu.Lock()
	src := h.sources[path]
	if src == nil {
		h.mu.Unlock()
		return
	}
	empty := src.removeSubscriber(id)
	if empty {
		delete(h.sources, path)
	}
	h.mu.Unlock()

	if empty {
		go func() {
			src.stop()
			h.debugPrint("WatchHub: source stop path=%s", path)
		}()
	}
}

func readSnapshot(path string) (Snapshot, error) {
	entries, err := fileinfo.ReadDirPortable(path)
	if err != nil {
		return nil, err
	}

	currentFiles := make(Snapshot)
	for _, entry := range entries {
		fileInfo, err := fileinfo.FileInfoFromDirEntry(path, entry)
		if err != nil {
			continue
		}
		currentFiles[fileInfo.Path] = fileInfo
	}
	return currentFiles, nil
}

func resolveWatchPath(path string) (string, bool) {
	vfs, parsed, err := fileinfo.ResolveRead(path)
	if err != nil {
		return path, true
	}
	defer fileinfo.CloseVFS(vfs)
	if !vfs.Capabilities().Watch {
		return "", false
	}
	if parsed.Provider == "local" && parsed.Native != "" {
		return parsed.Native, true
	}
	if parsed.Scheme == fileinfo.SchemeFile && parsed.Native != "" {
		return parsed.Native, true
	}
	return path, true
}

type watchSource struct {
	path         string
	watchPath    string
	interval     time.Duration
	useFSWatcher bool
	hub          *WatchHub
	mu           sync.Mutex
	subscribers  map[uint64]*watchSubscriber
	stopChan     chan struct{}
	stopped      chan struct{}
	pollFallback bool
}

// watchSubscriber serializes delivery with close so a broadcast that already
// captured this subscriber cannot send to its channel after unsubscribe closes
// it.
type watchSubscriber struct {
	mu      sync.Mutex
	updates chan Snapshot
	closed  bool
}

func newWatchSubscriber(buffer int) *watchSubscriber {
	return &watchSubscriber{updates: make(chan Snapshot, buffer)}
}

func (s *watchSubscriber) send(snapshot Snapshot) (sent bool, open bool) {
	update := cloneSnapshot(snapshot)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false, false
	}
	select {
	case s.updates <- update:
		return true, true
	default:
		return false, true
	}
}

func (s *watchSubscriber) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	close(s.updates)
}

func newWatchSource(path string, watchPath string, interval time.Duration, useFSWatcher bool, hub *WatchHub) *watchSource {
	return &watchSource{
		path:         path,
		watchPath:    watchPath,
		interval:     interval,
		useFSWatcher: useFSWatcher,
		hub:          hub,
		subscribers:  make(map[uint64]*watchSubscriber),
		stopChan:     make(chan struct{}),
		stopped:      make(chan struct{}),
	}
}

func (s *watchSource) start() {
	if !s.useFSWatcher {
		s.hub.debugPrint("WatchHub: fswatcher skipped path=%s", s.path)
		s.pollFallback = true
		go s.loop(nil)
		return
	}
	backend, err := s.hub.newBackend()
	if err != nil {
		s.hub.debugPrint("WatchHub: fswatcher unavailable path=%s err=%v", s.path, err)
		s.pollFallback = true
		go s.loop(nil)
		return
	}
	if err := backend.Add(s.watchPath, fswatcher.All); err != nil {
		s.hub.debugPrint("WatchHub: fswatcher add failed path=%s watch_path=%s err=%v", s.path, s.watchPath, err)
		_ = backend.Close()
		s.pollFallback = true
		go s.loop(nil)
		return
	}
	s.hub.debugPrint("WatchHub: fswatcher watching path=%s watch_path=%s", s.path, s.watchPath)
	go s.loop(backend)
}

// stop blocks until the source goroutine has exited, including any in-flight
// backend Remove/Close or directory read. Callers on the UI thread must not
// invoke this directly; unsubscribe() runs it from a detached goroutine.
func (s *watchSource) stop() {
	close(s.stopChan)
	<-s.stopped
}

func (s *watchSource) addSubscriber(id uint64, subscriber *watchSubscriber) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscribers[id] = subscriber
}

func (s *watchSource) removeSubscriber(id uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if subscriber, ok := s.subscribers[id]; ok {
		delete(s.subscribers, id)
		subscriber.close()
	}
	return len(s.subscribers) == 0
}

func (s *watchSource) loop(backend watchBackend) {
	defer close(s.stopped)

	if backend == nil {
		s.pollLoop()
		return
	}
	fallback := s.eventLoop(backend)
	_ = backend.Remove(s.watchPath)
	_ = backend.Close()
	if fallback {
		s.pollLoop()
	}
}

func (s *watchSource) pollLoop() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.readAndBroadcast()
		case <-s.stopChan:
			return
		}
	}
}

func (s *watchSource) eventLoop(backend watchBackend) bool {
	var debounce *time.Timer
	var debounceC <-chan time.Time
	events := backend.Events()
	errorsC := backend.Errors()
	defer func() {
		if debounce != nil {
			debounce.Stop()
		}
	}()

	for {
		select {
		case _, ok := <-events:
			if !ok {
				s.hub.debugPrint("WatchHub: fswatcher events closed path=%s", s.path)
				return true
			}
			if debounce == nil {
				debounce = time.NewTimer(s.hub.debounce)
				debounceC = debounce.C
			} else {
				if !debounce.Stop() {
					select {
					case <-debounce.C:
					default:
					}
				}
				debounce.Reset(s.hub.debounce)
				debounceC = debounce.C
			}
		case err, ok := <-errorsC:
			if !ok {
				errorsC = nil
				continue
			}
			if err != nil && !errors.Is(err, fswatcher.ErrClosed) {
				s.hub.debugPrint("WatchHub: fswatcher error path=%s err=%v", s.path, err)
			}
			return true
		case <-debounceC:
			debounceC = nil
			s.readAndBroadcast()
		case <-s.stopChan:
			return false
		}
	}
}

func (s *watchSource) readAndBroadcast() {
	snapshot, err := s.hub.listSnapshot(s.path)
	if err != nil {
		s.hub.debugPrint("WatchHub: snapshot read skipped path=%s err=%v", s.path, err)
		return
	}
	s.broadcast(snapshot)
}

func (s *watchSource) broadcast(snapshot Snapshot) {
	s.mu.Lock()
	subscribers := make([]*watchSubscriber, 0, len(s.subscribers))
	for _, subscriber := range s.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	s.mu.Unlock()

	for _, subscriber := range subscribers {
		if sent, open := subscriber.send(snapshot); open && !sent {
			s.hub.debugPrint("WatchHub: subscriber channel full path=%s", s.path)
		}
	}
}

func cloneSnapshot(src Snapshot) Snapshot {
	dst := make(Snapshot, len(src))
	for path, info := range src {
		dst[path] = info
	}
	return dst
}
