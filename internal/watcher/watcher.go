package watcher

import (
	"os"
	"path/filepath"
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
	mu            sync.RWMutex                 // Protects previousFiles from concurrent access
	previousFiles map[string]fileinfo.FileInfo // Previous state for comparison
	ticker        *time.Ticker
	stopChan      chan bool
	changeChan    chan *PendingChanges                     // Channel for thread-safe change communication
	stopped       bool                                     // Track if watcher is already stopped
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
		stopChan:      make(chan bool),
		changeChan:    make(chan *PendingChanges, 10), // Buffered channel
		debugPrint:    debugPrint,
	}
}

// Start begins watching the current directory for changes
func (dw *DirectoryWatcher) Start() {
	if dw.ticker != nil && !dw.stopped {
		return // Already running
	}

	dw.stopped = false

	// Recreate channels if they were closed
	if dw.stopChan == nil {
		dw.stopChan = make(chan bool)
	}
	if dw.changeChan == nil {
		dw.changeChan = make(chan *PendingChanges, 10)
	}

	dw.ticker = time.NewTicker(2 * time.Second)
	dw.updateSnapshot() // Take initial snapshot

	// Start directory monitoring goroutine
	ticker := dw.ticker // Capture ticker reference for this goroutine
	go func() {
		defer ticker.Stop() // Clean up ticker when goroutine exits
		for {
			select {
			case <-ticker.C:
				dw.checkForChanges()
			case <-dw.stopChan:
				return
			}
		}
	}()

	// Start change processing goroutine
	go func() {
		for {
			select {
			case changes, ok := <-dw.changeChan:
				if !ok {
					return // チャネルがクローズされた
				}
				// Apply data changes (binding auto-updates UI)
				dw.applyDataChanges(changes.Added, changes.Deleted, changes.Modified)
			case <-dw.stopChan:
				return
			}
		}
	}()
}

// Stop stops the directory watcher
func (dw *DirectoryWatcher) Stop() {
	if dw.stopped {
		return // Already stopped, do nothing
	}

	dw.stopped = true
	dw.ticker = nil // Just clear reference, goroutine will handle cleanup

	// Close channels safely
	close(dw.stopChan)
	dw.stopChan = nil

	// Close change channel too
	close(dw.changeChan)
	dw.changeChan = nil
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

// checkForChanges detects and handles file system changes
func (dw *DirectoryWatcher) checkForChanges() {
	// Read current directory state
	entries, err := os.ReadDir(dw.fm.GetCurrentPath())
	if err != nil {
		return // Skip this check if directory read fails
	}

	currentFiles := make(map[string]fileinfo.FileInfo)

	// Build current file map
	for _, entry := range entries {
		fullPath := filepath.Join(dw.fm.GetCurrentPath(), entry.Name())
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
		// Send changes to processing channel (if not stopped)
		if !dw.stopped && dw.changeChan != nil {
			select {
			case dw.changeChan <- &PendingChanges{
				Added:    added,
				Deleted:  deleted,
				Modified: modified,
			}:
				// Changes sent successfully
			default:
				// Channel full, skip this update
				dw.debugPrint("Change channel full, skipping update")
			}
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
	dw.debugPrint("Applying changes: %d added, %d deleted, %d modified", len(added), len(deleted), len(modified))

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
