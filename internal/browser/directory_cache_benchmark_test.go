package browser

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"nmf/internal/fileinfo"
)

// Use -benchtime=1x: this measures retained cache memory after collecting the
// original listing slices, rather than the speed of building test fixtures.
func BenchmarkDirectoryCacheRetention(b *testing.B) {
	for _, size := range []int{10000, 100000} {
		for _, windows := range []int{1, 4} {
			b.Run(fmt.Sprintf("files%d_windows%d", size, windows), func(b *testing.B) {
				var retained uint64
				for range b.N {
					runtime.GC()
					var before, after runtime.MemStats
					runtime.ReadMemStats(&before)
					caches := make([]*DirectoryCache, windows)
					for w := range windows {
						caches[w] = retentionBenchmarkCache(w, size)
					}
					runtime.GC()
					runtime.ReadMemStats(&after)
					if after.HeapAlloc > before.HeapAlloc {
						retained += after.HeapAlloc - before.HeapAlloc
					}
					runtime.KeepAlive(caches)
				}
				b.ReportMetric(float64(retained)/float64(b.N), "retained-B")
			})
		}
	}
}

func retentionBenchmarkCache(window, size int) *DirectoryCache {
	cache := NewDirectoryCache(time.Hour, 8)
	for dir := range 8 {
		path := fmt.Sprintf("/cache/window-%d/directory-%d", window, dir)
		files := make([]fileinfo.FileInfo, size)
		for i := range files {
			name := fmt.Sprintf("report-%06d.txt", i)
			files[i] = fileinfo.FileInfo{Name: name, Path: path + "/" + name}
		}
		cache.Put(DirectorySnapshot{Path: path, Files: files})
	}
	return cache
}
