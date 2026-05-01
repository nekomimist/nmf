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
	UpdateFiles(files []fileinfo.FileInfo)
	RemoveFromSelections(path string)
}

// DirectoryWatcher handles incremental directory change detection
type DirectoryWatcher struct {
	fm            FileManager
	mu            sync.RWMutex                 // Protects watcher lifecycle state and previousFiles
	previousFiles map[string]fileinfo.FileInfo // Previous state for comparison
	ticker        *time.Ticker
	pollInterval  time.Duration
	stopChan      chan struct{}
	changeChan    chan *PendingChanges                     // Channel for thread-safe change communication
	running       bool                                     // True while current watcher run is active
	runID         uint64                                   // Monotonically increasing watcher run generation
	debugPrint    func(format string, args ...interface{}) // Debug function
}

// PendingChanges represents file changes waiting to be applied
type PendingChanges struct {
	Added    []fileinfo.FileInfo
	Deleted  []fileinfo.FileInfo
	Modified []fileinfo.FileInfo
}

// NewDirectoryWatcher creates a new directory watcher
func NewDirectoryWatcher(fm FileManager, debugPrint func(format string, args ...interface{})) *DirectoryWatcher {
	return &DirectoryWatcher{
		fm:            fm,
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
	ticker := time.NewTicker(interval)
	stopChan := make(chan struct{})
	changeChan := make(chan *PendingChanges, 10) // Buffered channel
	dw.ticker = ticker
	dw.stopChan = stopChan
	dw.changeChan = changeChan
	dw.running = true
	dw.mu.Unlock()

	dw.updateSnapshot() // Take initial snapshot

	// Start directory monitoring goroutine
	go dw.watchLoop(runID, ticker, stopChan, changeChan)

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
	ticker := dw.ticker
	dw.running = false
	dw.ticker = nil
	dw.stopChan = nil
	dw.changeChan = nil
	dw.mu.Unlock()

	// Stop ticker before signaling goroutines to exit.
	if ticker != nil {
		ticker.Stop()
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

	// Take snapshot of current files (excluding ".." entry and deleted files)
	for _, file := range dw.fm.GetFiles() {
		if file.Name != ".." && file.Status != fileinfo.StatusDeleted {
			dw.previousFiles[file.Path] = file
		}
	}
}

// RefreshSnapshot resets the watcher baseline to the file manager's current list.
func (dw *DirectoryWatcher) RefreshSnapshot() {
	dw.updateSnapshot()
}

func (dw *DirectoryWatcher) watchLoop(runID uint64, ticker *time.Ticker, stopChan <-chan struct{}, changeChan chan<- *PendingChanges) {
	for {
		select {
		case <-ticker.C:
			dw.checkForChanges(runID, changeChan)
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

// checkForChanges detects and handles file system changes.
func (dw *DirectoryWatcher) checkForChanges(runID uint64, changeChan chan<- *PendingChanges) {
	if !dw.isCurrentRun(runID) {
		return
	}

	// Read current directory state
	cur := dw.fm.GetCurrentPath()
	entries, err := fileinfo.ReadDirPortable(cur)
	if err != nil {
		return // Skip this check if directory read fails
	}

	currentFiles := make(map[string]fileinfo.FileInfo)

	// Build current file map
	for _, entry := range entries {
		fullPath := fileinfo.JoinPath(cur, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		fileType := fileinfo.DetermineFileType(fullPath, entry.Name(), entry.IsDir())
		fileInfo := fileinfo.FileInfo{
			Name:     entry.Name(),
			Path:     fullPath,
			IsDir:    entry.IsDir(),
			Size:     info.Size(),
			Modified: info.ModTime(),
			FileType: fileType,
			Status:   fileinfo.StatusNormal,
		}
		currentFiles[fullPath] = fileInfo
	}

	// Detect changes
	added, deleted, modified := dw.detectChanges(currentFiles)

	// Apply changes if any detected
	if len(added) > 0 || len(deleted) > 0 || len(modified) > 0 {
		if !dw.isCurrentRun(runID) {
			return
		}

		select {
		case changeChan <- &PendingChanges{
			Added:    added,
			Deleted:  deleted,
			Modified: modified,
		}:
			// Changes sent successfully
		default:
			// Channel full, skip this update
			dw.debugPrint("DirectoryWatcher: Change channel full, skipping update")
		}
	}
}

// detectChanges compares current and previous states to find differences
func (dw *DirectoryWatcher) detectChanges(currentFiles map[string]fileinfo.FileInfo) (added, deleted, modified []fileinfo.FileInfo) {
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
			if !file.Modified.Equal(prevFile.Modified) || file.Size != prevFile.Size {
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

	return added, deleted, modified
}

// applyDataChanges applies detected changes to the file manager data (thread-safe)
func (dw *DirectoryWatcher) applyDataChanges(added, deleted, modified []fileinfo.FileInfo) {
	dw.debugPrint("DirectoryWatcher: Applying changes: %d added, %d deleted, %d modified", len(added), len(deleted), len(modified))

	// Get current files (this is now thread-safe as GetFiles() returns a copy)
	files := dw.fm.GetFiles()
	needsUpdate := len(added) > 0 || len(deleted) > 0 || len(modified) > 0

	// Handle deleted files - mark as deleted but keep in list
	for _, deletedFile := range deleted {
		for i, file := range files {
			if file.Path == deletedFile.Path {
				files[i].Status = fileinfo.StatusDeleted
				// Remove from selections if selected (already thread-safe)
				dw.fm.RemoveFromSelections(deletedFile.Path)
				break
			}
		}
	}

	// Handle modified files - update status
	for _, modifiedFile := range modified {
		for i, file := range files {
			if file.Path == modifiedFile.Path {
				files[i] = modifiedFile
				break
			}
		}
	}

	// Handle added files - append to end
	for _, addedFile := range added {
		files = append(files, addedFile)
	}

	// UI update must happen on the main thread
	if needsUpdate {
		fyne.Do(func() {
			dw.fm.UpdateFiles(files)
			// Update snapshot after UI update to ensure consistency
			dw.updateSnapshot()
		})
	}
}

// applyPendingChanges applies a queued change set only when it belongs to the active watcher run.
func (dw *DirectoryWatcher) applyPendingChanges(runID uint64, changes *PendingChanges) {
	if changes == nil || !dw.isCurrentRun(runID) {
		return
	}
	// Apply data changes (binding auto-updates UI)
	dw.applyDataChanges(changes.Added, changes.Deleted, changes.Modified)
}
