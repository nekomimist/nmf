package main

import (
	"context"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/config"
	"nmf/internal/configscript"
	"nmf/internal/fileinfo"
	"nmf/internal/jobs"
	"nmf/internal/keymanager"
	"nmf/internal/search"
	customtheme "nmf/internal/theme"
	"nmf/internal/ui"
	"nmf/internal/watcher"
)

// FileManager is the main file manager struct.
type FileManager struct {
	mu                   sync.RWMutex // Protects files and selectedFiles from concurrent access
	window               fyne.Window
	currentPath          string
	files                []fileinfo.FileInfo
	originalFiles        []fileinfo.FileInfo // Original files before filtering
	fileList             *widget.List
	fileListView         *ui.KeySink
	windowHighlight      *canvas.Rectangle
	windowActive         bool
	pathDisplay          *widget.Label
	statusLabel          *widget.Label
	cursorPath           string          // Current cursor file path
	cursorIndex          int             // Cache of cursorPath's index in files; validated against cursorPath on every read in GetCurrentCursorIndex, so direct cursorPath assignments elsewhere self-heal
	cursorRefreshSeq     uint64          // Diagnostic sequence for requested cursor refreshes
	cursorItemUpdateSeq  uint64          // Latest cursor refresh sequence observed by the list UpdateItem callback
	cursorAnchor         cursorRowAnchor // Last visible row object for shell menu positioning
	selectedFiles        map[string]bool // Set of selected file paths
	storageInfo          fileinfo.StorageInfo
	storageKnown         bool
	config               *config.Config
	configManager        *config.Manager
	state                *config.State
	stateManager         *config.StateManager
	configScript         *configscript.Runtime
	initialWindowSize    fyne.Size
	activeSort           config.SortConfig
	customTheme          *customtheme.CustomTheme                // Custom theme for colors
	cursorRenderer       ui.CursorRenderer                       // Cursor display renderer
	keyManager           *keymanager.KeyManager                  // Keyboard input manager
	mainKeyHandler       *keymanager.MainScreenKeyHandler        // Main screen key handler (for canvas shortcut registration)
	dirWatcher           *watcher.DirectoryWatcher               // Directory change watcher
	currentFilter        *config.FilterEntry                     // Currently applied filter
	searchOverlay        *ui.IncrementalSearchOverlay            // Incremental search overlay
	searchHandler        *keymanager.IncrementalSearchKeyHandler // Search key handler
	searchToken          keymanager.HandlerToken                 // Token of the pushed search handler
	searchMatchers       *search.Provider                        // Shared search matcher provider
	iconSvc              *fileinfo.IconService                   // Async icon service
	runtime              *ApplicationRuntime                     // Application-scoped services
	promptTargetID       uint64
	promptUnregister     func()
	transferDestSubID    uint64
	transferDestUnsub    func()
	lifecycleMu          sync.Mutex
	closed               bool
	quitConfirmationOpen bool
	// Busy state while loading directories
	busyOverlay  *ui.BusyOverlay
	busyActive   bool
	busyTimer    *time.Timer
	busyDelay    time.Duration
	busyText     string
	busyToken    keymanager.HandlerToken
	busyMu       sync.Mutex
	loadMu       sync.Mutex
	nextLoadID   uint64
	activeLoadID uint64
	loadCancel   context.CancelFunc
	viewerMu     sync.Mutex
	nextViewerID uint64
	activeViewer uint64

	// Jobs indicator
	jobsButton    *widget.Button
	jobsBlinking  bool
	jobsBlinkStop chan struct{}
	jobsUnsub     func()
}

func (fm *FileManager) beginViewerLoad() uint64 {
	fm.viewerMu.Lock()
	defer fm.viewerMu.Unlock()
	fm.nextViewerID++
	fm.activeViewer = fm.nextViewerID
	return fm.activeViewer
}

func (fm *FileManager) finishViewerLoad(id uint64) bool {
	fm.viewerMu.Lock()
	defer fm.viewerMu.Unlock()
	if fm.activeViewer != id {
		return false
	}
	fm.activeViewer = 0
	return true
}

func (fm *FileManager) invalidateViewerLoad(id uint64) bool {
	fm.viewerMu.Lock()
	defer fm.viewerMu.Unlock()
	if id != 0 && fm.activeViewer != id {
		return false
	}
	fm.activeViewer = 0
	return true
}

func (fm *FileManager) jobManager() *jobs.Manager {
	if fm != nil && fm.runtime != nil && fm.runtime.jobManager != nil {
		return fm.runtime.jobManager
	}
	return jobs.GetManager()
}

type cursorRowAnchor struct {
	path   string
	object fyne.CanvasObject
}

// Interface implementation for watcher.FileManager.
func (fm *FileManager) GetCurrentPath() string {
	return fm.currentPath
}

func (fm *FileManager) GetFiles() []fileinfo.FileInfo {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	// Return a copy to prevent external modifications
	result := make([]fileinfo.FileInfo, len(fm.files))
	copy(result, fm.files)
	return result
}

// UpdateFiles replaces the current listing with files and always re-sorts.
// It implements the watcher.FileManager interface; ApplyChanges is the sole
// production caller, and it goes through updateFiles directly so it can skip
// the re-sort when safe. Keep this exported entry point always-sorting so any
// other future caller gets the conservative, always-correct behavior.
func (fm *FileManager) UpdateFiles(files []fileinfo.FileInfo) {
	fm.updateFiles(files, true)
}

// updateFiles applies files as the new listing. resort is false only when the
// caller has already proven the update cannot change relative order (see the
// sortAffected computation in ApplyChanges).
func (fm *FileManager) updateFiles(files []fileinfo.FileInfo, resort bool) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	fm.originalFiles = make([]fileinfo.FileInfo, len(files))
	copy(fm.originalFiles, files)

	// Apply filter if one is active
	if fm.currentFilter != nil && config.EffectiveFilterPattern(fm.currentFilter.Pattern) != "" {
		filtered, err := fileinfo.FilterFiles(files, config.EffectiveFilterPattern(fm.currentFilter.Pattern))
		if err != nil {
			debugPrint("FileManager: Filter error: %v", err)
			fm.files = files // Fall back to showing all files
		} else {
			fm.files = filtered
		}
	} else {
		fm.files = files
	}

	if resort {
		fm.sortFilesWithConfig(fm.CurrentSort())
	}

	// widget.List is not data-bound, so it never redraws on its own; refresh
	// explicitly to reflect additions, deletions, and modifications.
	fm.fileList.Refresh()
	fm.updateStatusBar()
}

func (fm *FileManager) RemoveFromSelections(path string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	delete(fm.selectedFiles, path)
}

// ApplyChanges merges watcher-detected added/deleted/modified files into the
// current listing. Must only run on the Fyne main goroutine: the watcher
// marshals into this call via fyne.DoAndWait (internal/watcher/watcher.go
// applyDataChanges), since fm.files/fm.selectedFiles are otherwise mutated
// without synchronization from UI-thread code such as SetFileSelected.
func (fm *FileManager) ApplyChanges(added, deleted, modified []fileinfo.FileInfo) {
	files := fm.GetFiles()

	// Handle deleted files - mark as deleted but keep in list
	for _, deletedFile := range deleted {
		for i, file := range files {
			if file.Path == deletedFile.Path {
				files[i].Status = fileinfo.StatusDeleted
				// Remove from selections if selected
				fm.RemoveFromSelections(deletedFile.Path)
				break
			}
		}
	}

	// Handle modified files - update status
	typeFlipped := false
	for _, modifiedFile := range modified {
		for i, file := range files {
			if file.Path == modifiedFile.Path {
				if file.IsDir != modifiedFile.IsDir {
					typeFlipped = true
				}
				files[i] = modifiedFile
				break
			}
		}
	}

	// Handle added files - append to end
	for _, addedFile := range added {
		files = append(files, addedFile)
	}

	// Skip the re-sort when this merge cannot change relative order.
	// Adds/deletes always change the member set, which can change order under
	// any sort key, so those always re-sort. A modify-only merge only changes
	// order under "size" or "modified", since those are the only keys whose
	// comparison value a plain content modification can change; under
	// "name"/"extension" (and any other key), a modify event never changes
	// the file's name, so its position within its DirectoriesFirst group is
	// correct by construction and the ".."-pinning invariant
	// (sortFilesWithConfig always pins ".." at index 0) still holds untouched.
	// The one exception is typeFlipped: if a path's IsDir changed (e.g. a
	// file removed and replaced by a same-named directory between polls),
	// sortFileInfoSlice's DirectoriesFirst grouping puts it in a different
	// group regardless of sort key, so that always forces a re-sort too. This
	// event is rare and the re-sort itself is cheap, so we don't bother
	// gating it on whether DirectoriesFirst is actually enabled.
	sortAffected := len(added) > 0 || len(deleted) > 0 || typeFlipped
	if !sortAffected {
		switch fm.CurrentSort().SortBy {
		case "size", "modified":
			sortAffected = true
		}
	}

	fm.updateFiles(files, sortAffected)
}
