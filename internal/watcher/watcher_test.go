package watcher

import (
	"testing"
	"time"

	"nmf/internal/fileinfo"
)

// mockFM is a minimal FileManager implementation for tests
type mockFM struct {
	path  string
	files []fileinfo.FileInfo
}

func (m *mockFM) GetCurrentPath() string { return m.path }
func (m *mockFM) GetFiles() []fileinfo.FileInfo {
	cp := make([]fileinfo.FileInfo, len(m.files))
	copy(cp, m.files)
	return cp
}
func (m *mockFM) UpdateFiles(files []fileinfo.FileInfo) {
	m.files = append([]fileinfo.FileInfo{}, files...)
}
func (m *mockFM) RemoveFromSelections(path string) {}

func dummyDebug(format string, args ...interface{}) {}

func fi(path string, name string, size int64, mod time.Time) fileinfo.FileInfo {
	return fileinfo.FileInfo{
		Name:     name,
		Path:     path,
		IsDir:    false,
		Size:     size,
		Modified: mod,
		FileType: fileinfo.FileTypeRegular,
		Status:   fileinfo.StatusNormal,
	}
}

func TestDetectChanges_AddedDeletedModified(t *testing.T) {
	m := &mockFM{path: "/tmp"}
	dw := NewDirectoryWatcher(m, dummyDebug)

	t1 := time.Now().Add(-time.Hour)
	t2 := time.Now()

	// previous: a (old size/time), b
	prev := []fileinfo.FileInfo{
		fi("/tmp/a.txt", "a.txt", 10, t1),
		fi("/tmp/b.txt", "b.txt", 5, t1),
	}
	m.files = prev
	dw.updateSnapshot()

	// current: a (modified), c (added)
	current := map[string]fileinfo.FileInfo{
		"/tmp/a.txt": fi("/tmp/a.txt", "a.txt", 20, t2),
		"/tmp/c.txt": fi("/tmp/c.txt", "c.txt", 1, t2),
	}

	added, deleted, modified := dw.detectChanges(current)
	if len(added) != 1 || added[0].Name != "c.txt" {
		t.Fatalf("expected 1 added c.txt, got %#v", added)
	}
	if len(deleted) != 1 || deleted[0].Name != "b.txt" || deleted[0].Status != fileinfo.StatusDeleted {
		t.Fatalf("expected 1 deleted b.txt, got %#v", deleted)
	}
	if len(modified) != 1 || modified[0].Name != "a.txt" || modified[0].Status != fileinfo.StatusModified {
		t.Fatalf("expected 1 modified a.txt, got %#v", modified)
	}
}

func TestUpdateSnapshot_ExcludesParentAndDeleted(t *testing.T) {
	m := &mockFM{path: "/tmp"}
	dw := NewDirectoryWatcher(m, dummyDebug)
	now := time.Now()
	m.files = []fileinfo.FileInfo{
		{Name: "..", Path: "/tmp/..", IsDir: true, Modified: now, FileType: fileinfo.FileTypeDirectory, Status: fileinfo.StatusNormal},
		{Name: "keep.txt", Path: "/tmp/keep.txt", Modified: now, FileType: fileinfo.FileTypeRegular, Status: fileinfo.StatusNormal},
		{Name: "gone.txt", Path: "/tmp/gone.txt", Modified: now, FileType: fileinfo.FileTypeRegular, Status: fileinfo.StatusDeleted},
	}
	dw.updateSnapshot()
	if _, ok := dw.previousFiles["/tmp/.."]; ok {
		t.Fatalf("snapshot should exclude parent entry")
	}
	if _, ok := dw.previousFiles["/tmp/gone.txt"]; ok {
		t.Fatalf("snapshot should exclude deleted status entries")
	}
	if _, ok := dw.previousFiles["/tmp/keep.txt"]; !ok {
		t.Fatalf("snapshot should include keep.txt")
	}
}
