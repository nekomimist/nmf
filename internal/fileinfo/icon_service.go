package fileinfo

import (
	"sync"
	"time"

	"fyne.io/fyne/v2"
)

// IconService provides asynchronous icon fetching with simple in-memory caches.
// - On Windows, platform-specific functions provide actual icons.
// - On other platforms, it falls back to nil (UI should use theme defaults).
type IconService struct {
	mu        sync.RWMutex
	extCache  map[string]fyne.Resource // key: lower-case file extension (e.g., ".txt")
	fileCache map[string]fyne.Resource // key: full path (or strategy-defined key)
	pending   map[string]struct{}      // de-duplicate queued jobs (use scope+key)
	jobs      chan iconJob

	// Update batching
	updMu       sync.Mutex
	updatedAny  bool
	subscribers []func()

	debugPrint func(format string, args ...interface{})
}

type iconJob struct {
	scope string // "ext" or "file"
	key   string // ext (".txt") or full path
	size  int    // desired size in pixels (16/24/32 etc.)
}

// NewIconService creates a new icon service with background workers.
func NewIconService(debug func(format string, args ...interface{})) *IconService {
	s := &IconService{
		extCache:   make(map[string]fyne.Resource, 256),
		fileCache:  make(map[string]fyne.Resource, 512),
		pending:    make(map[string]struct{}, 512),
		jobs:       make(chan iconJob, 256),
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
	s.updMu.Lock()
	defer s.updMu.Unlock()
	s.subscribers = append(s.subscribers, f)
}

// GetCachedOrRequest returns a cached icon if available. If not, it enqueues a background
// fetch and returns (nil, false) so the UI can display a default icon immediately.
// - Directories: always return (nil, false) and let UI use folder icon.
// - For .exe files on Windows, prefer file-specific icon. For others, prefer extension icon.
func (s *IconService) GetCachedOrRequest(path string, isDir bool, ext string, size int) (fyne.Resource, bool) {
	if isDir {
		return nil, false
	}

	// 1) File-specific cache (platform policy decides if it’s worth fetching)
	if preferFileIcon(path, ext) {
		s.mu.RLock()
		if r, ok := s.fileCache[path]; ok {
			s.mu.RUnlock()
			return r, true
		}
		s.mu.RUnlock()
		s.enqueue("file", path, size)
		// No immediate result; fall back to extension cache/default below
	}

	// 2) Extension cache
	s.mu.RLock()
	if r, ok := s.extCache[ext]; ok {
		s.mu.RUnlock()
		return r, true
	}
	s.mu.RUnlock()

	s.enqueue("ext", ext, size)
	return nil, false
}

// Close would stop workers in a more elaborate implementation; kept for API symmetry.
func (s *IconService) Close() {
	// no-op for now
}

func (s *IconService) enqueue(scope, key string, size int) {
	k := scope + "|" + key
	s.mu.Lock()
	if _, exists := s.pending[k]; exists {
		s.mu.Unlock()
		return
	}
	s.pending[k] = struct{}{}
	s.mu.Unlock()

	select {
	case s.jobs <- iconJob{scope: scope, key: key, size: size}:
	default:
		// queue full; drop silently to protect UI responsiveness
		if s.debugPrint != nil {
			s.debugPrint("IconService: job queue full, dropping %s:%s", scope, key)
		}
		s.mu.Lock()
		delete(s.pending, k)
		s.mu.Unlock()
	}
}

func (s *IconService) worker() {
	for job := range s.jobs {
		var res fyne.Resource
		var err error
		switch job.scope {
		case "ext":
			res, err = platformFetchExtIcon(job.key, job.size)
			if err == nil && res != nil {
				s.mu.Lock()
				s.extCache[job.key] = res
				s.mu.Unlock()
				s.flagUpdated()
			}
		case "file":
			res, err = platformFetchFileIcon(job.key, job.size)
			if err == nil && res != nil {
				s.mu.Lock()
				s.fileCache[job.key] = res
				s.mu.Unlock()
				s.flagUpdated()
			}
		}
		// clear pending marker
		s.mu.Lock()
		delete(s.pending, job.scope+"|"+job.key)
		s.mu.Unlock()
	}
}

func (s *IconService) flagUpdated() {
	s.updMu.Lock()
	s.updatedAny = true
	s.updMu.Unlock()
}

func (s *IconService) batchNotifier() {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
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
