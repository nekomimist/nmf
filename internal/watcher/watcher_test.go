package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"nmf/internal/fileinfo"
)

// mockFM is a minimal FileManager implementation for tests
type mockFM struct {
	path            string
	files           []fileinfo.FileInfo
	unfilteredFiles []fileinfo.FileInfo
	applied         []PendingChanges
	notify          chan PendingChanges
}

func (m *mockFM) GetCurrentPath() string { return m.path }
func (m *mockFM) GetUnfilteredFiles() []fileinfo.FileInfo {
	files := m.unfilteredFiles
	if files == nil {
		files = m.files
	}
	cp := make([]fileinfo.FileInfo, len(files))
	copy(cp, files)
	return cp
}
func (m *mockFM) ApplyChanges(added, deleted, modified []fileinfo.FileInfo) {
	changes := PendingChanges{Added: added, Deleted: deleted, Modified: modified}
	m.applied = append(m.applied, changes)
	if m.notify != nil {
		m.notify <- changes
	}
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

func TestUpdateSnapshotUsesUnfilteredFiles(t *testing.T) {
	visible := fi("/tmp/image.png", "image.png", 10, time.Now())
	hidden := fi("/tmp/notes.txt", "notes.txt", 20, time.Now())
	m := &mockFM{
		path:            "/tmp",
		files:           []fileinfo.FileInfo{visible},
		unfilteredFiles: []fileinfo.FileInfo{visible, hidden},
	}
	dw := NewDirectoryWatcher(m, nil, dummyDebug)

	dw.updateSnapshot()

	if len(dw.previousFiles) != 2 {
		t.Fatalf("snapshot contains %d files, want complete unfiltered listing", len(dw.previousFiles))
	}
	if _, ok := dw.previousFiles[hidden.Path]; !ok {
		t.Fatalf("snapshot omitted filtered-out path %q", hidden.Path)
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

	added, deleted, modified, _ := dw.detectChanges(snapshotUpdate{files: current, sequence: dw.hub.sequence.Add(1)})
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

	added, deleted, modified, _ := dw.detectChanges(snapshotUpdate{files: current, sequence: dw.hub.sequence.Add(1)})
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
	m := &mockFM{path: t.TempDir()}
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
	if dw.subscription != nil {
		t.Fatalf("subscription should be nil after Stop")
	}
}

func TestWatchLoopCommitsLatestSnapshotAndReconcilesRegistration(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "created-before-watch"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	m := &mockFM{path: dir, notify: make(chan PendingChanges, 1)}
	dw := NewDirectoryWatcher(m, nil, dummyDebug)
	dw.Start()
	defer dw.Stop()
	select {
	case changes := <-m.notify:
		if len(changes.Added) != 1 || changes.Added[0].Name != "created-before-watch" {
			t.Fatalf("initial reconciliation = %+v", changes)
		}
	case <-time.After(time.Second):
		t.Fatal("watcher did not reconcile the original listing")
	}
}

func TestApplySnapshotRequiresSameRunAndBaseline(t *testing.T) {
	now := time.Now()
	existing := fi("/tmp/a", "a", 10, now)
	added := fi("/tmp/b", "b", 10, now)
	fm := &mockFM{files: []fileinfo.FileInfo{existing}}
	dw := NewDirectoryWatcher(fm, nil, dummyDebug)
	dw.running = true
	dw.runID = 1
	dw.updateSnapshot()
	read := snapshotUpdate{files: Snapshot{existing.Path: existing, added.Path: added}, sequence: dw.hub.sequence.Add(1)}
	a, d, m, gen := dw.detectChanges(read)
	changes := &PendingChanges{Added: a, Deleted: d, Modified: m, BaselineGen: gen}
	dw.applySnapshotOnUI(0, changes, read.files)
	if len(fm.applied) != 0 {
		t.Fatal("old run applied a delta")
	}
	fm.files = append(fm.files, added)
	dw.RefreshSnapshot()
	dw.applySnapshotOnUI(1, changes, read.files)
	if len(fm.applied) != 0 {
		t.Fatal("old baseline applied a delta")
	}
	read.files = Snapshot{existing.Path: existing}
	a, d, m, _ = dw.detectChanges(read)
	if len(a)+len(d)+len(m) != 0 {
		t.Fatal("read started before a reset must be ignored")
	}
	read.sequence = dw.hub.sequence.Add(1)
	a, d, m, gen = dw.detectChanges(read)
	if len(d) != 1 || d[0].Path != added.Path {
		t.Fatalf("expected deletion, got %+v", d)
	}
	dw.applySnapshotOnUI(1, &PendingChanges{Added: a, Deleted: d, Modified: m, BaselineGen: gen}, read.files)
	if len(fm.applied) != 1 {
		t.Fatal("current delta was not applied")
	}
	a, d, m, _ = dw.detectChanges(read)
	if len(a)+len(d)+len(m) != 0 {
		t.Fatal("accepted snapshot was not promoted")
	}
}
