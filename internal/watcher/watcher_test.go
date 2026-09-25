package watcher

import (
	"reflect"
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
}

func (m *mockFM) GetCurrentPath() string { return m.path }
func (m *mockFM) GetFiles() []fileinfo.FileInfo {
	cp := make([]fileinfo.FileInfo, len(m.files))
	copy(cp, m.files)
	return cp
}
func (m *mockFM) GetUnfilteredFiles() []fileinfo.FileInfo {
	files := m.unfilteredFiles
	if files == nil {
		files = m.files
	}
	cp := make([]fileinfo.FileInfo, len(files))
	copy(cp, files)
	return cp
}
func (m *mockFM) UpdateFiles(files []fileinfo.FileInfo) {
	m.files = append([]fileinfo.FileInfo{}, files...)
}
func (m *mockFM) RemoveFromSelections(string) {}

func (m *mockFM) ApplyChanges(added, deleted, modified []fileinfo.FileInfo) {
	m.applied = append(m.applied, PendingChanges{Added: added, Deleted: deleted, Modified: modified})
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

	added, deleted, modified, _ := dw.detectChanges(current)
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

	added, deleted, modified, _ := dw.detectChanges(current)
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

	if len(m.applied) != 0 {
		t.Fatalf("stale run dispatched changes: %+v", m.applied)
	}
}

func TestApplyDataChangesOnUIRechecksRunGeneration(t *testing.T) {
	m := &mockFM{path: "."}
	dw := NewDirectoryWatcher(m, nil, dummyDebug)
	dw.running = true
	dw.runID = 2

	dw.applyDataChangesOnUI(1, &PendingChanges{
		Added: []fileinfo.FileInfo{fi("./stale.txt", "stale.txt", 1, time.Now())},
	})

	if len(m.applied) != 0 {
		t.Fatalf("stale UI callback dispatched changes: %+v", m.applied)
	}
}

func TestQueueSnapshotChangesAdvancesExpectedBaseline(t *testing.T) {
	m := &mockFM{path: "."}
	dw := NewDirectoryWatcher(m, nil, dummyDebug)
	dw.running = true
	dw.runID = 1
	queued := make(chan *PendingChanges, 2)
	now := time.Now()

	dw.queueSnapshotChanges(1, Snapshot{
		"./a.txt": fi("./a.txt", "a.txt", 1, now),
	}, queued)
	dw.queueSnapshotChanges(1, Snapshot{
		"./a.txt": fi("./a.txt", "a.txt", 1, now),
		"./b.txt": fi("./b.txt", "b.txt", 1, now),
	}, queued)

	first := <-queued
	second := <-queued
	if len(first.Added) != 1 || first.Added[0].Name != "a.txt" {
		t.Fatalf("first snapshot changes = %#v, want only a.txt added", first)
	}
	if len(second.Added) != 1 || second.Added[0].Name != "b.txt" {
		t.Fatalf("second snapshot changes = %#v, want only b.txt added", second)
	}
}

func TestApplyDataChangesDispatchesCurrentChanges(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	m := &mockFM{path: "/tmp"}
	dw := NewDirectoryWatcher(m, nil, dummyDebug)
	dw.running = true
	dw.runID = 1
	dw.updateSnapshot()
	now := time.Unix(1, 0)
	want := PendingChanges{
		Added:    []fileinfo.FileInfo{fi("/tmp/new", "new", 1, now)},
		Deleted:  []fileinfo.FileInfo{fi("/tmp/gone", "gone", 2, now)},
		Modified: []fileinfo.FileInfo{fi("/tmp/changed", "changed", 3, now)},
	}
	changes := PendingChanges{
		Added:       append([]fileinfo.FileInfo(nil), want.Added...),
		Deleted:     append([]fileinfo.FileInfo(nil), want.Deleted...),
		Modified:    append([]fileinfo.FileInfo(nil), want.Modified...),
		BaselineGen: dw.baselineGen,
	}
	// Exercise the same UI dispatch used by the watcher worker.
	done := make(chan struct{})
	go func() {
		defer close(done)
		dw.applyDataChanges(1, &changes)
		dw.applyDataChanges(1, &PendingChanges{BaselineGen: dw.baselineGen})
	}()
	<-done
	if !reflect.DeepEqual(m.applied, []PendingChanges{want}) {
		t.Fatalf("dispatched changes = %+v, want exactly %+v", m.applied, want)
	}
}

// A RefreshSnapshot that lands between detection and the baseline promotion
// must win. Otherwise the promotion installs the older directory read, the
// entry the UI just created is missing from the baseline, and the next poll
// reports it as added a second time.
func TestAdvanceSnapshotKeepsBaselineResetDuringDetection(t *testing.T) {
	m := &mockFM{path: "/tmp"}
	dw := NewDirectoryWatcher(m, nil, dummyDebug)
	dw.running = true

	now := time.Now()
	existing := fi("/tmp/a.txt", "a.txt", 10, now)
	m.files = []fileinfo.FileInfo{existing}
	dw.updateSnapshot()

	// A directory read taken before the user created anything.
	staleRead := Snapshot{"/tmp/a.txt": existing}
	_, _, _, baselineGen := dw.detectChanges(staleRead)

	// The user creates a directory; the UI merges it and resets the baseline.
	created := fi("/tmp/new-dir", "new-dir", 0, now)
	created.IsDir = true
	m.files = append(m.files, created)
	dw.RefreshSnapshot()

	// The in-flight change set now tries to promote its own, older read.
	dw.advanceSnapshot(dw.runID, baselineGen, staleRead)

	if _, ok := dw.previousFiles["/tmp/new-dir"]; !ok {
		t.Fatal("baseline reset was overwritten by the older directory read")
	}

	// The next poll sees the directory and must not report it as added.
	nextRead := Snapshot{"/tmp/a.txt": existing, "/tmp/new-dir": created}
	added, _, _, _ := dw.detectChanges(nextRead)
	if len(added) != 0 {
		t.Fatalf("added = %#v, want none", added)
	}
}

// The steady state still advances: without an intervening reset the directory
// read becomes the baseline, so the same change is not reported twice.
func TestAdvanceSnapshotPromotesReadWhenBaselineUnchanged(t *testing.T) {
	m := &mockFM{path: "/tmp"}
	dw := NewDirectoryWatcher(m, nil, dummyDebug)
	dw.running = true

	now := time.Now()
	existing := fi("/tmp/a.txt", "a.txt", 10, now)
	m.files = []fileinfo.FileInfo{existing}
	dw.updateSnapshot()

	added := fi("/tmp/b.txt", "b.txt", 3, now)
	read := Snapshot{"/tmp/a.txt": existing, "/tmp/b.txt": added}
	gotAdded, _, _, baselineGen := dw.detectChanges(read)
	if len(gotAdded) != 1 {
		t.Fatalf("added = %#v, want b.txt", gotAdded)
	}

	dw.advanceSnapshot(dw.runID, baselineGen, read)

	gotAdded, _, _, _ = dw.detectChanges(read)
	if len(gotAdded) != 0 {
		t.Fatalf("added = %#v, want none after the baseline advanced", gotAdded)
	}
}

// The queue-time generation check is not enough on its own: a change set can be
// queued, then the UI can merge the same creation itself and reset the
// baseline, and only then does the apply loop get to it. Applying it at that
// point restamps the entry the UI added with StatusAdded.
func TestApplyDataChangesOnUIDropsChangesFromAResetBaseline(t *testing.T) {
	now := time.Now()
	existing := fi("/tmp/a.txt", "a.txt", 10, now)
	m := &mockFM{path: "/tmp", files: []fileinfo.FileInfo{existing}}
	dw := NewDirectoryWatcher(m, nil, dummyDebug)
	dw.running = true
	dw.runID = 1
	dw.updateSnapshot()

	// The watcher detects the new file and queues it.
	created := fi("/tmp/new.txt", "new.txt", 3, now)
	created.Status = fileinfo.StatusAdded
	read := Snapshot{"/tmp/a.txt": existing, "/tmp/new.txt": created}
	added, deleted, modified, baselineGen := dw.detectChanges(read)
	if len(added) != 1 {
		t.Fatalf("added = %#v, want new.txt", added)
	}
	queued := &PendingChanges{Added: added, Deleted: deleted, Modified: modified, BaselineGen: baselineGen}

	// Before the apply loop gets to it, the UI merges its own creation as a
	// normal entry and resets the baseline.
	uiCreated := fi("/tmp/new.txt", "new.txt", 3, now)
	m.files = append(m.files, uiCreated)
	dw.RefreshSnapshot()

	dw.applyDataChangesOnUI(1, queued)

	if len(m.applied) != 0 {
		t.Fatalf("reset baseline dispatched stale changes: %+v", m.applied)
	}
}
