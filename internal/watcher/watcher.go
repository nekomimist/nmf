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
	GetUnfilteredFiles() []fileinfo.FileInfo
	// ApplyChanges merges watcher-detected added/deleted/modified files into
	// the current listing. Implementations must run this only on the Fyne
	// main goroutine.
	ApplyChanges(added, deleted, modified []fileinfo.FileInfo)
}

// DirectoryWatcher handles incremental directory change detection
type DirectoryWatcher struct {
	fm               FileManager
	hub              *WatchHub
	mu               sync.RWMutex                 // Protects watcher lifecycle state and previousFiles
	previousFiles    map[string]fileinfo.FileInfo // Previous state for comparison
	pollInterval     time.Duration
	subscription     *Subscription
	stopChan         chan struct{}
	baselineSequence uint64 // ignore reads started before the latest local baseline reset
	running          bool   // True while current watcher run is active
	runID            uint64 // Monotonically increasing watcher run generation

}

// PendingChanges represents file changes waiting to be applied
type PendingChanges struct {
	Added    []fileinfo.FileInfo
	Deleted  []fileinfo.FileInfo
	Modified []fileinfo.FileInfo
	// BaselineGen is the baseline generation these changes were derived from.
	// It travels with them so the merge can be abandoned when the baseline was
	// reset before the UI applied them.
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
	dw.stopChan = stopChan
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

	go dw.watchLoop(runID, subscription.Updates, stopChan)
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

	files := dw.fm.GetUnfilteredFiles()
	dw.previousFiles = make(map[string]fileinfo.FileInfo, len(files))
	dw.baselineSequence = dw.hub.sequence.Add(1)

	// Take a snapshot of the complete directory listing (excluding ".." and
	// deleted entries). A visible list may be filtered and would make every
	// hidden file look newly added on the next directory read.
	for _, file := range files {
		if file.Name != ".." && file.Status != fileinfo.StatusDeleted {
			dw.previousFiles[file.Path] = file
		}
	}
}

// RefreshSnapshot resets the watcher baseline to the file manager's current list.
func (dw *DirectoryWatcher) RefreshSnapshot() {
	dw.updateSnapshot()
	dw.mu.RLock()
	sub := dw.subscription
	dw.mu.RUnlock()
	if sub != nil {
		sub.reconcile()
	}
}

func (dw *DirectoryWatcher) watchLoop(runID uint64, updates <-chan snapshotUpdate, stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case update, ok := <-updates:
			if !ok || !dw.isCurrentRun(runID) {
				return
			}
			added, deleted, modified, gen := dw.detectChanges(update)
			if len(added)+len(deleted)+len(modified) == 0 {
				continue
			}
			changes := PendingChanges{Added: added, Deleted: deleted, Modified: modified, BaselineGen: gen}
			fyne.DoAndWait(func() { dw.applySnapshotOnUI(runID, &changes, update.files) })
		}
	}
}

// isCurrentRun reports whether runID still matches the currently active watcher run.
func (dw *DirectoryWatcher) isCurrentRun(runID uint64) bool {
	dw.mu.RLock()
	defer dw.mu.RUnlock()
	return dw.running && dw.runID == runID
}

// detectChanges compares current and previous states to find differences. It
// also reports the baseline generation the comparison used, so the caller can
// tell whether that baseline is still current by the time it acts on the
// result.
func (dw *DirectoryWatcher) detectChanges(update snapshotUpdate) (added, deleted, modified []fileinfo.FileInfo, baselineGen uint64) {
	dw.mu.RLock()
	previous := dw.previousFiles
	baselineGen = dw.baselineSequence
	dw.mu.RUnlock()
	if update.sequence < baselineGen {
		return nil, nil, nil, baselineGen
	}
	currentFiles := update.files

	// Find added files
	for path, file := range currentFiles {
		if prevFile, exists := previous[path]; !exists {
			file.Status = fileinfo.StatusAdded
			added = append(added, file)
		} else {
			// Check for modifications
			if !file.Modified.Equal(prevFile.Modified) || file.Size != prevFile.Size || file.IsDir != prevFile.IsDir || file.FileType != prevFile.FileType || file.HiddenAttribute != prevFile.HiddenAttribute {
				file.Status = fileinfo.StatusModified
				modified = append(modified, file)
			}
		}
	}

	// Find deleted files
	for path, file := range previous {
		if _, exists := currentFiles[path]; !exists {
			file.Status = fileinfo.StatusDeleted
			deleted = append(deleted, file)
		}
	}

	return added, deleted, modified, baselineGen
}

// Commit the accepted snapshot and its UI delta in one UI turn. A reset
// that ran after detection invalidates the delta and requests a fresh read.
func (dw *DirectoryWatcher) applySnapshotOnUI(runID uint64, changes *PendingChanges, files Snapshot) {
	dw.mu.Lock()
	if !dw.running || dw.runID != runID || dw.baselineSequence != changes.BaselineGen {
		dw.mu.Unlock()
		return
	}
	dw.previousFiles = files
	dw.mu.Unlock()
	dw.fm.ApplyChanges(changes.Added, changes.Deleted, changes.Modified)
}
