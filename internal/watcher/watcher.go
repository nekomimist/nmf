package watcher

import (
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"nmf/internal/fileinfo"
)

// FileManager interface represents the operations that the watcher needs from the file manager
type FileManager interface {
	GetCurrentPath() string
	GetFiles() []fileinfo.FileInfo
	GetUnfilteredFiles() []fileinfo.FileInfo
	UpdateFiles(files []fileinfo.FileInfo)
	RemoveFromSelections(path string)
	// ApplyChanges merges watcher-detected added/deleted/modified files into
	// the current listing. Implementations must run this only on the Fyne
	// main goroutine (see applyDataChanges, which marshals via fyne.DoAndWait).
	ApplyChanges(added, deleted, modified []fileinfo.FileInfo)
}

// DirectoryWatcher handles incremental directory change detection
type DirectoryWatcher struct {
	fm            FileManager
	hub           *WatchHub
	mu            sync.RWMutex                 // Protects watcher lifecycle state and previousFiles
	previousFiles map[string]fileinfo.FileInfo // Previous state for comparison
	pollInterval  time.Duration
	subscription  *Subscription
	stopChan      chan struct{}
	changeChan    chan *PendingChanges // Channel for thread-safe change communication
	running       bool                 // True while current watcher run is active
	runID         uint64               // Monotonically increasing watcher run generation
	// baselineGen increments whenever previousFiles is reset from the file
	// manager's list rather than advanced from a directory read. It lets a
	// change set that is already in flight notice that its baseline was
	// replaced underneath it.
	baselineGen uint64
	debugPrint  func(format string, args ...interface{}) // Debug function
}

// PendingChanges represents file changes waiting to be applied
type PendingChanges struct {
	Added    []fileinfo.FileInfo
	Deleted  []fileinfo.FileInfo
	Modified []fileinfo.FileInfo
	// BaselineGen is the baseline generation these changes were derived from.
	// It travels with them so the merge can be abandoned when the baseline was
	// reset while they sat in the queue.
	BaselineGen uint64
}

// NewDirectoryWatcher creates a new directory watcher
func NewDirectoryWatcher(fm FileManager, hub *WatchHub, debugPrint func(format string, args ...interface{})) *DirectoryWatcher {
	if hub == nil {
		hub = NewWatchHub(debugPrint)
	}
	return &DirectoryWatcher{
		fm:            fm,
		hub:           hub,
		previousFiles: make(map[string]fileinfo.FileInfo),
		pollInterval:  2 * time.Second,
		debugPrint:    debugPrint,
	}
}

// Start begins watching the current directory for changes
func (dw *DirectoryWatcher) Start() {
	dw.mu.Lock()
	if dw.running {
		dw.mu.Unlock()
		return // Already running
	}
	dw.runID++
	runID := dw.runID
	interval := dw.pollInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	path := dw.fm.GetCurrentPath()
	stopChan := make(chan struct{})
	changeChan := make(chan *PendingChanges, 10) // Buffered channel
	dw.stopChan = stopChan
	dw.changeChan = changeChan
	dw.running = true
	dw.mu.Unlock()

	dw.updateSnapshot() // Take initial snapshot
	subscription := dw.hub.Subscribe(path, interval)

	dw.mu.Lock()
	if dw.running && dw.runID == runID {
		dw.subscription = subscription
	} else {
		dw.mu.Unlock()
		subscription.Unsubscribe()
		return
	}
	dw.mu.Unlock()

	// Start directory monitoring goroutine
	go dw.watchLoop(runID, subscription.Updates, stopChan, changeChan)

	// Start change processing goroutine
	go dw.applyLoop(runID, stopChan, changeChan)
}

// SetPollInterval sets the polling interval used when Start() is called next.
func (dw *DirectoryWatcher) SetPollInterval(d time.Duration) {
	dw.mu.Lock()
	defer dw.mu.Unlock()
	dw.pollInterval = d
}

// Stop stops the directory watcher
func (dw *DirectoryWatcher) Stop() {
	dw.mu.Lock()
	if !dw.running {
		dw.mu.Unlock()
		return // Already stopped, do nothing
	}
	stopChan := dw.stopChan
	subscription := dw.subscription
	dw.running = false
	dw.subscription = nil
	dw.stopChan = nil
	dw.changeChan = nil
	dw.mu.Unlock()

	// Unsubscribe detaches from the shared source immediately; when this was
	// the last subscriber for the path, the source itself shuts down in the
	// background rather than blocking this call.
	if subscription != nil {
		subscription.Unsubscribe()
	}
	if stopChan != nil {
		close(stopChan)
	}
}

// updateSnapshot updates the current file snapshot
func (dw *DirectoryWatcher) updateSnapshot() {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	dw.previousFiles = make(map[string]fileinfo.FileInfo)
	dw.baselineGen++

	// Take a snapshot of the complete directory listing (excluding ".." and
	// deleted entries). A visible list may be filtered and would make every
	// hidden file look newly added on the next directory read.
	for _, file := range dw.fm.GetUnfilteredFiles() {
		if file.Name != ".." && file.Status != fileinfo.StatusDeleted {
			dw.previousFiles[file.Path] = file
		}
	}
}

// RefreshSnapshot resets the watcher baseline to the file manager's current list.
func (dw *DirectoryWatcher) RefreshSnapshot() {
	dw.updateSnapshot()
}

func (dw *DirectoryWatcher) watchLoop(runID uint64, updates <-chan Snapshot, stopChan <-chan struct{}, changeChan chan<- *PendingChanges) {
	for {
		select {
		case snapshot, ok := <-updates:
			if !ok {
				return
			}
			dw.queueSnapshotChanges(runID, snapshot, changeChan)
		case <-stopChan:
			return
		}
	}
}

func (dw *DirectoryWatcher) applyLoop(runID uint64, stopChan <-chan struct{}, changeChan <-chan *PendingChanges) {
	for {
		select {
		case changes := <-changeChan:
			dw.applyPendingChanges(runID, changes)
		case <-stopChan:
			return
		}
	}
}

// isCurrentRun reports whether runID still matches the currently active watcher run.
func (dw *DirectoryWatcher) isCurrentRun(runID uint64) bool {
	dw.mu.RLock()
	defer dw.mu.RUnlock()
	return dw.running && dw.runID == runID
}

// queueSnapshotChanges detects and queues file system changes from a shared snapshot.
func (dw *DirectoryWatcher) queueSnapshotChanges(runID uint64, currentFiles Snapshot, changeChan chan<- *PendingChanges) {
	if !dw.isCurrentRun(runID) {
		return
	}

	// Detect changes. baselineGen identifies the baseline they were derived
	// from, so a reset that lands while this change set is in flight can be
	// told apart from the steady state.
	added, deleted, modified, baselineGen := dw.detectChanges(currentFiles)

	// Apply changes if any detected
	if len(added) > 0 || len(deleted) > 0 || len(modified) > 0 {
		if !dw.isCurrentRun(runID) {
			return
		}

		select {
		case changeChan <- &PendingChanges{
			Added:       added,
			Deleted:     deleted,
			Modified:    modified,
			BaselineGen: baselineGen,
		}:
			// Advance the expected baseline only after the change set is queued.
			// This prevents a burst of snapshots from deriving duplicate adds or
			// deletes while the first UI application is still pending.
			dw.advanceSnapshot(runID, baselineGen, currentFiles)
		default:
			// Channel full, skip this update
			dw.debugPrint("DirectoryWatcher: Change channel full, skipping update")
		}
	}
}

// advanceSnapshot promotes the directory read that produced the queued change
// set to be the new baseline.
//
// baselineGen is the generation the change set was derived from. Detection and
// this promotion are separate critical sections with a channel send between
// them, so RefreshSnapshot can install a newer baseline in the gap; without the
// generation check this would overwrite it with the older read. The UI's reset
// then wins, and the next directory read reconciles against it -- otherwise a
// file the UI had just added would be missing from the baseline and reported as
// "added" a second time.
func (dw *DirectoryWatcher) advanceSnapshot(runID, baselineGen uint64, files Snapshot) {
	dw.mu.Lock()
	live := dw.running && dw.runID == runID
	currentGen := dw.baselineGen
	if live && currentGen == baselineGen {
		// Each subscriber receives its own cloned snapshot from WatchHub. After
		// the watcher goroutine receives it, ownership transfers here and no
		// second copy is needed.
		dw.previousFiles = files
	}
	dw.mu.Unlock()

	if live && currentGen != baselineGen {
		dw.debugPrint("DirectoryWatcher: baseline reset during detection, keeping gen=%d", currentGen)
	}
}

// detectChanges compares current and previous states to find differences. It
// also reports the baseline generation the comparison used, so the caller can
// tell whether that baseline is still current by the time it acts on the
// result.
func (dw *DirectoryWatcher) detectChanges(currentFiles map[string]fileinfo.FileInfo) (added, deleted, modified []fileinfo.FileInfo, baselineGen uint64) {
	dw.mu.RLock()
	defer dw.mu.RUnlock()

	// Find added files
	for path, file := range currentFiles {
		if _, exists := dw.previousFiles[path]; !exists {
			file.Status = fileinfo.StatusAdded
			added = append(added, file)
		} else {
			// Check for modifications
			prevFile := dw.previousFiles[path]
			if !file.Modified.Equal(prevFile.Modified) || file.Size != prevFile.Size || file.IsDir != prevFile.IsDir {
				file.Status = fileinfo.StatusModified
				modified = append(modified, file)
			}
		}
	}

	// Find deleted files
	for path, file := range dw.previousFiles {
		if _, exists := currentFiles[path]; !exists {
			file.Status = fileinfo.StatusDeleted
			deleted = append(deleted, file)
		}
	}

	return added, deleted, modified, dw.baselineGen
}

// applyDataChanges applies detected changes to the file manager data. The
// merge itself (fm.ApplyChanges) runs inside fyne.DoAndWait so changes remain
// ordered and confined to the Fyne main goroutine, alongside all other
// fm.files/fm.selectedFiles access; the previous background-goroutine merge
// (calling GetFiles/RemoveFromSelections here directly) raced with UI-thread
// code such as SetFileSelected, which mutates fm.selectedFiles without a lock.
func (dw *DirectoryWatcher) applyDataChanges(runID uint64, changes *PendingChanges) {
	if changes == nil {
		return
	}
	if len(changes.Added) == 0 && len(changes.Deleted) == 0 && len(changes.Modified) == 0 {
		return
	}
	dw.debugPrint("DirectoryWatcher: Applying changes: %d added, %d deleted, %d modified",
		len(changes.Added), len(changes.Deleted), len(changes.Modified))

	fyne.DoAndWait(func() {
		dw.applyDataChangesOnUI(runID, changes)
	})
}

// applyDataChangesOnUI merges a change set into the file manager's list. The
// baseline check belongs here rather than only at queue time: RefreshSnapshot
// runs on this same goroutine, so checking it immediately before the merge is
// what makes the two atomic with respect to each other. A change set that
// outlived its baseline is dropped -- the UI has already merged whatever it
// created, and the next directory read reconciles anything genuinely missing.
// Applying it anyway would restamp the entry the UI just added with
// StatusAdded and could overwrite fresh metadata with the older read's.
func (dw *DirectoryWatcher) applyDataChangesOnUI(runID uint64, changes *PendingChanges) {
	if changes == nil || !dw.canApply(runID, changes.BaselineGen) {
		return
	}
	dw.fm.ApplyChanges(changes.Added, changes.Deleted, changes.Modified)
}

// canApply reports whether a change set still belongs to the active run and to
// the current baseline.
func (dw *DirectoryWatcher) canApply(runID, baselineGen uint64) bool {
	dw.mu.RLock()
	live := dw.running && dw.runID == runID
	currentGen := dw.baselineGen
	dw.mu.RUnlock()

	if !live {
		return false
	}
	if currentGen != baselineGen {
		dw.debugPrint("DirectoryWatcher: dropping changes from baseline gen=%d, current=%d", baselineGen, currentGen)
		return false
	}
	return true
}

// applyPendingChanges applies a queued change set only when it belongs to the active watcher run.
func (dw *DirectoryWatcher) applyPendingChanges(runID uint64, changes *PendingChanges) {
	if changes == nil || !dw.isCurrentRun(runID) {
		return
	}
	// Apply data changes (binding auto-updates UI)
	dw.applyDataChanges(runID, changes)
}
