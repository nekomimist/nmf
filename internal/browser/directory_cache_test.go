package browser

import (
	"testing"
	"time"

	"nmf/internal/fileinfo"
)

func TestDirectoryCacheReturnsOwnedCopyWithoutSlidingExpiration(t *testing.T) {
	now := time.Unix(100, 0)
	cache := newDirectoryCache(time.Minute, 2, func() time.Time { return now })
	cache.Put(DirectorySnapshot{
		Path:  "/one",
		Files: []fileinfo.FileInfo{{Name: "original"}},
	})

	now = now.Add(30 * time.Second)
	got, ok := cache.Get("/one")
	if !ok {
		t.Fatal("Get returned a miss before TTL")
	}
	got.Files[0].Name = "mutated"

	again, ok := cache.Get("/one")
	if !ok || again.Files[0].Name != "original" {
		t.Fatalf("cached snapshot = %+v, want an independent original copy", again.Files)
	}

	now = now.Add(30 * time.Second)
	if _, ok := cache.Get("/one"); ok {
		t.Fatal("Get extended the entry lifetime past its fixed TTL")
	}
}

func TestDirectoryCacheEvictsOldestEntryAtCapacity(t *testing.T) {
	now := time.Unix(200, 0)
	cache := newDirectoryCache(time.Hour, 2, func() time.Time { return now })
	cache.Put(DirectorySnapshot{Path: "/one"})
	now = now.Add(time.Second)
	cache.Put(DirectorySnapshot{Path: "/two"})
	now = now.Add(time.Second)
	cache.Put(DirectorySnapshot{Path: "/three"})

	if _, ok := cache.Get("/one"); ok {
		t.Fatal("oldest entry remained after capacity eviction")
	}
	for _, path := range []string{"/two", "/three"} {
		if _, ok := cache.Get(path); !ok {
			t.Fatalf("live entry %q was evicted", path)
		}
	}
}

func TestDirectoryCacheDelete(t *testing.T) {
	cache := NewDirectoryCache(time.Minute, 2)
	cache.Put(DirectorySnapshot{Path: "/gone"})
	cache.Delete("/gone")
	if _, ok := cache.Get("/gone"); ok {
		t.Fatal("Delete left the cache entry present")
	}
}
