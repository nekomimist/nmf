package fileinfo

import (
	"image"
	"testing"
	"time"
)

func TestIconServiceCloseIsIdempotentAndRejectsNewWork(t *testing.T) {
	service := NewIconService(nil)
	service.Close()
	service.Close()

	service.enqueue("ext", ".txt", 16)
	service.mu.RLock()
	_, pending := service.pending[iconJob{scope: "ext", key: ".txt", size: 16}]
	service.mu.RUnlock()
	if pending {
		t.Fatal("closed icon service should not retain new work")
	}
}

func TestIconPixelSizeUsesSupportedBuckets(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{input: 0, want: 16},
		{input: 16, want: 16},
		{input: 17, want: 24},
		{input: 24, want: 24},
		{input: 25, want: 32},
		{input: 64, want: 32},
	}
	for _, tt := range tests {
		if got := iconPixelSize(tt.input); got != tt.want {
			t.Errorf("iconPixelSize(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestIconServiceCacheSeparatesPixelSizes(t *testing.T) {
	service := NewIconService(nil)
	defer service.Close()

	icon16 := image.NewRGBA(image.Rect(0, 0, 16, 16))
	icon24 := image.NewRGBA(image.Rect(0, 0, 24, 24))
	service.mu.Lock()
	service.extCache[iconCacheKey{name: ".txt", size: 16}] = icon16
	service.extCache[iconCacheKey{name: ".txt", size: 24}] = icon24
	service.mu.Unlock()

	if got, ok := service.GetCachedOrRequest("file.txt", false, ".txt", 16); !ok || got != icon16 {
		t.Fatalf("16px cache lookup = (%p, %t), want (%p, true)", got, ok, icon16)
	}
	if got, ok := service.GetCachedOrRequest("file.txt", false, ".txt", 17); !ok || got != icon24 {
		t.Fatalf("24px cache lookup = (%p, %t), want (%p, true)", got, ok, icon24)
	}
}

func TestPutBoundedIconEvictsInInsertionOrder(t *testing.T) {
	cache := make(map[iconCacheKey]*image.RGBA)
	order := make([]iconCacheKey, 0, 2)
	next := 0
	keys := []iconCacheKey{
		{name: ".a", size: 16},
		{name: ".b", size: 16},
		{name: ".c", size: 16},
		{name: ".d", size: 16},
	}
	images := make([]*image.RGBA, len(keys))
	for i := range images {
		images[i] = image.NewRGBA(image.Rect(0, 0, 1, 1))
	}

	putBoundedIcon(cache, &order, &next, keys[0], images[0], 2)
	putBoundedIcon(cache, &order, &next, keys[1], images[1], 2)
	putBoundedIcon(cache, &order, &next, keys[2], images[2], 2)
	if len(cache) != 2 || cache[keys[0]] != nil || cache[keys[1]] != images[1] || cache[keys[2]] != images[2] {
		t.Fatalf("cache after first eviction = %#v", cache)
	}

	putBoundedIcon(cache, &order, &next, keys[1], images[0], 2)
	putBoundedIcon(cache, &order, &next, keys[3], images[3], 2)
	if len(cache) != 2 || cache[keys[1]] != nil || cache[keys[2]] != images[2] || cache[keys[3]] != images[3] {
		t.Fatalf("cache after update and second eviction = %#v", cache)
	}
}

func TestIconServiceCloseReleasesCachedImages(t *testing.T) {
	service := NewIconService(nil)
	service.mu.Lock()
	key := iconCacheKey{name: ".txt", size: 16}
	putBoundedIcon(service.extCache, &service.extOrder, &service.extNext, key, image.NewRGBA(image.Rect(0, 0, 16, 16)), maxExtensionIconCacheEntries)
	service.pending[iconJob{scope: "ext", key: ".png", size: 16}] = struct{}{}
	service.mu.Unlock()

	service.Close()
	service.mu.RLock()
	defer service.mu.RUnlock()
	if service.extCache != nil || service.fileCache != nil || service.extOrder != nil || service.fileOrder != nil || service.pending != nil {
		t.Fatal("Close should release decoded image caches, eviction order, and pending work")
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
