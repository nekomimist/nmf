package fileinfo

import (
	"image"
	"sync"
	"time"
)

const (
	maxExtensionIconCacheEntries = 256
	maxFileIconCacheEntries      = 512
)

// IconService provides asynchronous icon fetching with bounded decoded-image caches.
// - On Windows, platform-specific functions provide actual icons.
// - On other platforms, it falls back to nil (UI should use theme defaults).
type IconService struct {
	mu        sync.RWMutex
	extCache  map[iconCacheKey]*image.RGBA // key: lower-case file extension and pixel size
	fileCache map[iconCacheKey]*image.RGBA // key: full path and pixel size
	extOrder  []iconCacheKey
	fileOrder []iconCacheKey
	extNext   int
	fileNext  int
	pending   map[iconJob]struct{} // de-duplicate queued jobs
	jobs      chan iconJob
	done      chan struct{}
	closeOnce sync.Once

	// Update batching
	updMu       sync.Mutex
	updatedAny  bool
	subscribers []func()

	debugPrint func(format string, args ...interface{})
}

type iconCacheKey struct {
	name string
	size int
}

type iconJob struct {
	scope string // "ext" or "file"
	key   string // ext (".txt") or full path
	size  int    // desired size in pixels (16/24/32 etc.)
}

// NewIconService creates a new icon service with background workers.
func NewIconService(debug func(format string, args ...interface{})) *IconService {
	s := &IconService{
		extCache:   make(map[iconCacheKey]*image.RGBA, maxExtensionIconCacheEntries),
		fileCache:  make(map[iconCacheKey]*image.RGBA, maxFileIconCacheEntries),
		extOrder:   make([]iconCacheKey, 0, maxExtensionIconCacheEntries),
		fileOrder:  make([]iconCacheKey, 0, maxFileIconCacheEntries),
		pending:    make(map[iconJob]struct{}, maxFileIconCacheEntries),
		jobs:       make(chan iconJob, 256),
		done:       make(chan struct{}),
		debugPrint: debug,
	}

	// Start workers
	for i := 0; i < 2; i++ { // modest parallelism
		go s.worker()
	}

	// Start batch notifier (50ms tick)
	go s.batchNotifier()
	return s
}

// OnUpdated registers a callback called on batches of updates (no args, UI should refresh icons).
func (s *IconService) OnUpdated(f func()) {
	if f == nil || s.closed() {
		return
	}
	s.updMu.Lock()
	defer s.updMu.Unlock()
	if s.closed() {
		return
	}
	s.subscribers = append(s.subscribers, f)
}

// GetCachedOrRequest returns a cached icon if available. If not, it enqueues a background
// fetch and returns (nil, false) so the UI can display a default icon immediately.
// - Directories: always return (nil, false) and let UI use folder icon.
// - For .exe files on Windows, prefer file-specific icon. For others, prefer extension icon.
func (s *IconService) GetCachedOrRequest(path string, isDir bool, ext string, size int) (*image.RGBA, bool) {
	if s == nil || isDir || s.closed() {
		return nil, false
	}
	size = iconPixelSize(size)

	// 1) File-specific cache (platform policy decides if it’s worth fetching)
	if preferFileIcon(path, ext) {
		key := iconCacheKey{name: path, size: size}
		s.mu.RLock()
		if img, ok := s.fileCache[key]; ok {
			s.mu.RUnlock()
			return img, true
		}
		s.mu.RUnlock()
		s.enqueue("file", path, size)
		// No immediate result; fall back to extension cache/default below
	}

	// 2) Extension cache
	key := iconCacheKey{name: ext, size: size}
	s.mu.RLock()
	if img, ok := s.extCache[key]; ok {
		s.mu.RUnlock()
		return img, true
	}
	s.mu.RUnlock()

	s.enqueue("ext", ext, size)
	return nil, false
}

// Close stops workers/notifier and releases update callbacks. An in-flight
// platform icon fetch is allowed to return, but its result is discarded.
func (s *IconService) Close() {
	if s == nil {
		return
	}
	s.closeOnce.Do(func() {
		close(s.done)
		s.mu.Lock()
		s.extCache = nil
		s.fileCache = nil
		s.extOrder = nil
		s.fileOrder = nil
		s.extNext = 0
		s.fileNext = 0
		s.pending = nil
		s.mu.Unlock()
		s.updMu.Lock()
		s.updatedAny = false
		s.subscribers = nil
		s.updMu.Unlock()
	})
}

func (s *IconService) closed() bool {
	if s == nil {
		return true
	}
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

func (s *IconService) enqueue(scope, key string, size int) {
	if s.closed() {
		return
	}
	job := iconJob{scope: scope, key: key, size: iconPixelSize(size)}
	s.mu.Lock()
	if s.closed() {
		s.mu.Unlock()
		return
	}
	if _, exists := s.pending[job]; exists {
		s.mu.Unlock()
		return
	}
	s.pending[job] = struct{}{}
	s.mu.Unlock()

	select {
	case <-s.done:
		s.mu.Lock()
		delete(s.pending, job)
		s.mu.Unlock()
	case s.jobs <- job:
	default:
		// queue full; drop silently to protect UI responsiveness
		if s.debugPrint != nil {
			s.debugPrint("IconService: job queue full, dropping %s:%s", scope, key)
		}
		s.mu.Lock()
		delete(s.pending, job)
		s.mu.Unlock()
	}
}

func (s *IconService) worker() {
	for {
		var job iconJob
		select {
		case <-s.done:
			return
		case job = <-s.jobs:
		}
		if s.closed() {
			return
		}
		var img *image.RGBA
		var err error
		stored := false
		switch job.scope {
		case "ext":
			img, err = platformFetchExtIcon(job.key, job.size)
			if err == nil && img != nil {
				s.mu.Lock()
				if !s.closed() {
					putBoundedIcon(s.extCache, &s.extOrder, &s.extNext, iconCacheKey{name: job.key, size: job.size}, img, maxExtensionIconCacheEntries)
					stored = true
				}
				s.mu.Unlock()
			}
		case "file":
			img, err = platformFetchFileIcon(job.key, job.size)
			if err == nil && img != nil {
				s.mu.Lock()
				if !s.closed() {
					putBoundedIcon(s.fileCache, &s.fileOrder, &s.fileNext, iconCacheKey{name: job.key, size: job.size}, img, maxFileIconCacheEntries)
					stored = true
				}
				s.mu.Unlock()
			}
		}
		if stored {
			s.flagUpdated()
		}
		// clear pending marker
		s.mu.Lock()
		delete(s.pending, job)
		s.mu.Unlock()
	}
}

func iconPixelSize(size int) int {
	if size <= 16 {
		return 16
	}
	if size <= 24 {
		return 24
	}
	return 32
}

func putBoundedIcon(cache map[iconCacheKey]*image.RGBA, order *[]iconCacheKey, next *int, key iconCacheKey, img *image.RGBA, limit int) {
	if cache == nil || img == nil || limit <= 0 {
		return
	}
	if _, exists := cache[key]; exists {
		cache[key] = img
		return
	}
	if len(*order) < limit {
		*order = append(*order, key)
	} else {
		delete(cache, (*order)[*next])
		(*order)[*next] = key
		*next = (*next + 1) % limit
	}
	cache[key] = img
}

func (s *IconService) flagUpdated() {
	if s.closed() {
		return
	}
	s.updMu.Lock()
	if s.closed() {
		s.updMu.Unlock()
		return
	}
	s.updatedAny = true
	s.updMu.Unlock()
}

func (s *IconService) batchNotifier() {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
		}
		s.updMu.Lock()
		if !s.updatedAny {
			s.updMu.Unlock()
			continue
		}
		s.updatedAny = false
		subs := append([]func(){}, s.subscribers...)
		s.updMu.Unlock()
		for _, f := range subs {
			// UI must marshal to main thread
			f()
		}
	}
}
