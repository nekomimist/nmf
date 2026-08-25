package browser

import (
	"sync"
	"time"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
)

// DirectorySnapshot is one successfully loaded directory listing retained in
// memory for a short revisit. UI-only state such as the cursor, marks, filter,
// and watcher status decorations deliberately does not belong here.
type DirectorySnapshot struct {
	Path         string
	Files        []fileinfo.FileInfo
	Storage      fileinfo.StorageInfo
	StorageKnown bool
	Sort         config.SortConfig
	LoadedAt     time.Time
}

// DirectoryCache is a small, process-memory-only cache of completed directory
// reads. Expiration and capacity eviction are lazy, avoiding a lifecycle
// goroutine for a cache whose owner is already short lived.
type DirectoryCache struct {
	mu         sync.Mutex
	entries    map[string]DirectorySnapshot
	ttl        time.Duration
	maxEntries int
	now        func() time.Time
}

// NewDirectoryCache creates a bounded cache. Non-positive limits disable it.
func NewDirectoryCache(ttl time.Duration, maxEntries int) *DirectoryCache {
	return newDirectoryCache(ttl, maxEntries, time.Now)
}

func newDirectoryCache(ttl time.Duration, maxEntries int, now func() time.Time) *DirectoryCache {
	if now == nil {
		now = time.Now
	}
	return &DirectoryCache{
		entries:    make(map[string]DirectorySnapshot),
		ttl:        ttl,
		maxEntries: maxEntries,
		now:        now,
	}
}

// Get returns an owned copy of a live snapshot. Reading an entry does not
// extend its lifetime: repeated visits must not keep an old listing alive
// indefinitely.
func (c *DirectoryCache) Get(path string) (DirectorySnapshot, bool) {
	if c == nil || path == "" || c.ttl <= 0 || c.maxEntries <= 0 {
		return DirectorySnapshot{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()
	c.removeExpiredLocked(now)
	snapshot, ok := c.entries[path]
	if !ok {
		return DirectorySnapshot{}, false
	}
	return cloneDirectorySnapshot(snapshot), true
}

// Put records one accepted real directory read. LoadedAt is assigned here so
// callers cannot accidentally extend a cached result by copying its old time.
func (c *DirectoryCache) Put(snapshot DirectorySnapshot) {
	if c == nil || snapshot.Path == "" || c.ttl <= 0 || c.maxEntries <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()
	c.removeExpiredLocked(now)
	snapshot.LoadedAt = now
	c.entries[snapshot.Path] = cloneDirectorySnapshot(snapshot)
	for len(c.entries) > c.maxEntries {
		c.removeOldestLocked()
	}
}

// Delete removes a path after a real read proves that a cached target no
// longer resolves to that directory (for example, parent fallback).
func (c *DirectoryCache) Delete(path string) {
	if c == nil || path == "" {
		return
	}
	c.mu.Lock()
	delete(c.entries, path)
	c.mu.Unlock()
}

func (c *DirectoryCache) removeExpiredLocked(now time.Time) {
	for path, snapshot := range c.entries {
		if snapshot.LoadedAt.IsZero() || now.Sub(snapshot.LoadedAt) >= c.ttl {
			delete(c.entries, path)
		}
	}
}

func (c *DirectoryCache) removeOldestLocked() {
	var oldestPath string
	var oldestTime time.Time
	for path, snapshot := range c.entries {
		if oldestPath == "" || snapshot.LoadedAt.Before(oldestTime) {
			oldestPath = path
			oldestTime = snapshot.LoadedAt
		}
	}
	if oldestPath != "" {
		delete(c.entries, oldestPath)
	}
}

func cloneDirectorySnapshot(snapshot DirectorySnapshot) DirectorySnapshot {
	snapshot.Files = append([]fileinfo.FileInfo(nil), snapshot.Files...)
	return snapshot
}
