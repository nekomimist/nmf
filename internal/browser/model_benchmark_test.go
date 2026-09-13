package browser

import (
	"fmt"
	"testing"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
)

func benchmarkListing(n int) []fileinfo.FileInfo {
	files := make([]fileinfo.FileInfo, n)
	for i := range files {
		name := fmt.Sprintf("file-%06d.txt", n-i)
		files[i] = fileinfo.FileInfo{Name: name, Path: "/audit/" + name, Size: int64(i)}
	}
	return files
}

func BenchmarkSortFiles(b *testing.B) {
	for _, n := range []int{10000, 100000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			files := benchmarkListing(n)
			cfg := config.SortConfig{SortBy: "name", SortOrder: "asc", DirectoriesFirst: true}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				_ = SortFiles(files, cfg)
			}
		})
	}
}

func BenchmarkApplyChanges(b *testing.B) {
	for _, tc := range []struct{ n, changes int }{{10000, 1}, {100000, 1}, {10000, 100}, {10000, 10000}} {
		b.Run(fmt.Sprintf("files%d_changes%d", tc.n, tc.changes), func(b *testing.B) {
			files := benchmarkListing(tc.n)
			cfg := config.SortConfig{SortBy: "name", SortOrder: "asc", DirectoriesFirst: true}
			model := New("/audit", cfg)
			model.ReplaceDirectory("/audit", files, fileinfo.StorageInfo{}, false, cfg)
			modified := append([]fileinfo.FileInfo(nil), files[len(files)-tc.changes:]...)
			for i := range modified {
				modified[i].Status = fileinfo.StatusModified
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if err := model.ApplyChanges(nil, nil, modified); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
