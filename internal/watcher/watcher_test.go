package watcher

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"nmf/internal/fileinfo"
)

// mockFM is a minimal FileManager implementation for tests
type mockFM struct {
	path          string
	files         []fileinfo.FileInfo
	selectedFiles map[string]bool
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
func (m *mockFM) RemoveFromSelections(path string) {
	delete(m.selectedFiles, path)
}

// ApplyChanges mirrors FileManager.ApplyChanges (file_manager.go) against the
// mock's own files field, so tests exercise the same merge semantics.
func (m *mockFM) ApplyChanges(added, deleted, modified []fileinfo.FileInfo) {
	files := m.GetFiles()

	for _, deletedFile := range deleted {
		for i, file := range files {
			if file.Path == deletedFile.Path {
				files[i].Status = fileinfo.StatusDeleted
				m.RemoveFromSelections(deletedFile.Path)
				break
			}
		}
	}

	for _, modifiedFile := range modified {
		for i, file := range files {
			if file.Path == modifiedFile.Path {
				files[i] = modifiedFile
				break
			}
		}
	}

	for _, addedFile := range added {
		files = append(files, addedFile)
	}

	m.UpdateFiles(files)
}

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
	dw := NewDirectoryWatcher(m, nil, dummyDebug)

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

// TestDetectChanges_IsDirFlipWithSameMtimeSize verifies that a path whose
// IsDir changed (e.g. "beta" was removed and replaced by a same-named
// directory between polls) is reported as modified even when Modified and
// Size happen to be identical, since neither reliably changes across a
// file-to-directory swap.
func TestDetectChanges_IsDirFlipWithSameMtimeSize(t *testing.T) {
	m := &mockFM{path: "/tmp"}
	dw := NewDirectoryWatcher(m, nil, dummyDebug)

	t1 := time.Now().Add(-time.Hour)

	prev := []fileinfo.FileInfo{
		fi("/tmp/beta", "beta", 5, t1),
	}
	m.files = prev
	dw.updateSnapshot()

	// current: beta is now a directory with identical Modified/Size.
	betaAsDir := fi("/tmp/beta", "beta", 5, t1)
	betaAsDir.IsDir = true
	betaAsDir.FileType = fileinfo.FileTypeDirectory
	current := map[string]fileinfo.FileInfo{
		"/tmp/beta": betaAsDir,
	}

	added, deleted, modified := dw.detectChanges(current)
	if len(added) != 0 || len(deleted) != 0 {
		t.Fatalf("expected no added/deleted, got added=%#v deleted=%#v", added, deleted)
	}
	if len(modified) != 1 || modified[0].Name != "beta" || !modified[0].IsDir || modified[0].Status != fileinfo.StatusModified {
		t.Fatalf("expected 1 modified beta reported as directory, got %#v", modified)
	}
}

func TestUpdateSnapshot_ExcludesParentAndDeleted(t *testing.T) {
	m := &mockFM{path: "/tmp"}
	dw := NewDirectoryWatcher(m, nil, dummyDebug)
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

func TestStartStop_IdempotentAndRapidCycles(t *testing.T) {
	m := &mockFM{path: "."}
	dw := NewDirectoryWatcher(m, nil, dummyDebug)

	for i := 0; i < 20; i++ {
		dw.Start()
		time.Sleep(2 * time.Millisecond)
		dw.Stop()
		// Stop must remain safe when called repeatedly.
		dw.Stop()
	}

	dw.mu.RLock()
	defer dw.mu.RUnlock()
	if dw.running {
		t.Fatalf("watcher should not be running after Stop")
	}
	if dw.stopChan != nil {
		t.Fatalf("stopChan should be nil after Stop")
	}
	if dw.changeChan != nil {
		t.Fatalf("changeChan should be nil after Stop")
	}
	if dw.subscription != nil {
		t.Fatalf("subscription should be nil after Stop")
	}
}

func TestApplyPendingChanges_IgnoresStaleRun(t *testing.T) {
	m := &mockFM{path: "."}
	dw := NewDirectoryWatcher(m, nil, dummyDebug)

	dw.Start()
	dw.mu.RLock()
	runID := dw.runID
	dw.mu.RUnlock()

	dw.Stop()

	dw.applyPendingChanges(runID, &PendingChanges{
		Added: []fileinfo.FileInfo{
			fi("./new.txt", "new.txt", 1, time.Now()),
		},
	})

	if len(m.files) != 0 {
		t.Fatalf("stale run changes should be ignored, got %d files", len(m.files))
	}
}

// TestApplyDataChanges_MergesAddedDeletedModified exercises applyDataChanges'
// new path (fyne.Do -> fm.ApplyChanges -> updateSnapshot). It runs the call
// from a spawned goroutine (not the test's own goroutine) so the fyne test
// driver treats it as an off-main-thread call and executes it synchronously,
// matching how applyLoop invokes it in production.
func TestApplyDataChanges_MergesAddedDeletedModified(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	now := time.Now()
	m := &mockFM{
		path:          "/tmp",
		selectedFiles: map[string]bool{"/tmp/b.txt": true},
		files: []fileinfo.FileInfo{
			fi("/tmp/a.txt", "a.txt", 10, now),
			fi("/tmp/b.txt", "b.txt", 5, now),
		},
	}
	dw := NewDirectoryWatcher(m, nil, dummyDebug)

	added := []fileinfo.FileInfo{fi("/tmp/c.txt", "c.txt", 1, now)}
	deleted := []fileinfo.FileInfo{fi("/tmp/b.txt", "b.txt", 5, now)}
	modified := []fileinfo.FileInfo{fi("/tmp/a.txt", "a.txt", 99, now)}

	done := make(chan struct{})
	go func() {
		defer close(done)
		dw.applyDataChanges(added, deleted, modified)
	}()
	<-done

	if len(m.files) != 3 {
		t.Fatalf("merged files = %#v, want 3 entries", m.files)
	}
	var gotA, gotB, gotC bool
	for _, f := range m.files {
		switch f.Path {
		case "/tmp/a.txt":
			gotA = true
			if f.Size != 99 {
				t.Fatalf("modified a.txt not replaced: %#v", f)
			}
		case "/tmp/b.txt":
			gotB = true
			if f.Status != fileinfo.StatusDeleted {
				t.Fatalf("deleted b.txt should have StatusDeleted: %#v", f)
			}
		case "/tmp/c.txt":
			gotC = true
		}
	}
	if !gotA || !gotB || !gotC {
		t.Fatalf("merged files missing expected entries: %#v", m.files)
	}
	if m.selectedFiles["/tmp/b.txt"] {
		t.Fatalf("deleted file should be removed from selections")
	}
}

func TestApplyDataChanges_NoopWhenAllEmpty(t *testing.T) {
	m := &mockFM{path: "/tmp", files: []fileinfo.FileInfo{fi("/tmp/a.txt", "a.txt", 1, time.Now())}}
	dw := NewDirectoryWatcher(m, nil, dummyDebug)

	// All three slices empty: must return before reaching fyne.Do, so this is
	// safe to call directly without a running app.
	dw.applyDataChanges(nil, nil, nil)

	if len(m.files) != 1 {
		t.Fatalf("files should be untouched, got %#v", m.files)
	}
}
