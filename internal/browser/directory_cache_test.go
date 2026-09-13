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

func TestDirectoryCacheEvictsOldestListingAtFileBudget(t *testing.T) {
	now := time.Unix(300, 0)
	cache := newDirectoryCache(time.Minute, 8, func() time.Time { return now })
	cache.maxFiles = 3
	cache.Put(DirectorySnapshot{Path: "/one", Files: make([]fileinfo.FileInfo, 2)})
	now = now.Add(time.Second)
	cache.Put(DirectorySnapshot{Path: "/two", Files: make([]fileinfo.FileInfo, 1)})
	now = now.Add(time.Second)
	cache.Put(DirectorySnapshot{Path: "/three", Files: make([]fileinfo.FileInfo, 2)})
	if _, ok := cache.Get("/one"); ok {
		t.Fatal("oldest listing remained after exceeding the file budget")
	}
	for _, path := range []string{"/two", "/three"} {
		if _, ok := cache.Get(path); !ok {
			t.Fatalf("live entry %q was evicted", path)
		}
	}
}

func TestDirectoryCacheReplacementReleasesFileBudget(t *testing.T) {
	cache := NewDirectoryCache(time.Minute, 8)
	cache.maxFiles = 3
	cache.Put(DirectorySnapshot{Path: "/one", Files: make([]fileinfo.FileInfo, 2)})
	cache.Put(DirectorySnapshot{Path: "/two", Files: make([]fileinfo.FileInfo, 1)})
	cache.Put(DirectorySnapshot{Path: "/one", Files: []fileinfo.FileInfo{{Name: "replacement"}}})
	cache.Put(DirectorySnapshot{Path: "/three", Files: make([]fileinfo.FileInfo, 1)})
	for _, path := range []string{"/one", "/two", "/three"} {
		if _, ok := cache.Get(path); !ok {
			t.Fatalf("entry %q was evicted despite fitting the file budget", path)
		}
	}
	got, _ := cache.Get("/one")
	if got.Files[0].Name != "replacement" {
		t.Fatalf("replacement listing = %+v", got.Files)
	}
}

func TestDirectoryCacheOversizedReplacementDropsStaleListing(t *testing.T) {
	cache := NewDirectoryCache(time.Minute, 8)
	cache.maxFiles = 3
	cache.Put(DirectorySnapshot{Path: "/one", Files: make([]fileinfo.FileInfo, 1)})
	cache.Put(DirectorySnapshot{Path: "/two", Files: make([]fileinfo.FileInfo, 1)})
	cache.Put(DirectorySnapshot{Path: "/one", Files: make([]fileinfo.FileInfo, 4)})
	if _, ok := cache.Get("/one"); ok {
		t.Fatal("oversized replacement left a cached listing")
	}
	if _, ok := cache.Get("/two"); !ok {
		t.Fatal("oversized replacement evicted an unrelated listing")
	}
}
