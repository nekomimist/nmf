package main

import (
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
	customtheme "nmf/internal/theme"
	"nmf/internal/ui"
	"nmf/internal/watcher"
)

// FileManager is the main file manager struct.
type FileManager struct {
	mu             sync.RWMutex // Protects files and selectedFiles from concurrent access
	window         fyne.Window
	currentPath    string
	files          []fileinfo.FileInfo
	originalFiles  []fileinfo.FileInfo // Original files before filtering
	fileList       *widget.List
	fileListView   *ui.KeySink
	pathEntry      *ui.TabEntry
	cursorPath     string          // Current cursor file path
	selectedFiles  map[string]bool // Set of selected file paths
	fileBinding    binding.UntypedList
	config         *config.Config
	configManager  *config.Manager
	customTheme    *customtheme.CustomTheme                // Custom theme for colors
	cursorRenderer ui.CursorRenderer                       // Cursor display renderer
	keyManager     *keymanager.KeyManager                  // Keyboard input manager
	dirWatcher     *watcher.DirectoryWatcher               // Directory change watcher
	currentFilter  *config.FilterEntry                     // Currently applied filter
	searchOverlay  *ui.IncrementalSearchOverlay            // Incremental search overlay
	searchHandler  *keymanager.IncrementalSearchKeyHandler // Search key handler
	iconSvc        *fileinfo.IconService                   // Async icon service
	// Busy state while loading directories
	busyOverlay *ui.BusyOverlay
	busyActive  bool
	busyTimer   *time.Timer
	busyDelay   time.Duration
	busyText    string
	busyMu      sync.Mutex

	// Jobs indicator
	jobsButton    *widget.Button
	jobsBlinking  bool
	jobsBlinkOn   bool
	jobsBlinkStop chan struct{}
	jobsUnsub     func()
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

func (fm *FileManager) UpdateFiles(files []fileinfo.FileInfo) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	fm.originalFiles = files

	// Apply filter if one is active
	if fm.currentFilter != nil && fm.currentFilter.Pattern != "" {
		filtered, err := fileinfo.FilterFiles(files, fm.currentFilter.Pattern)
		if err != nil {
			debugPrint("FileManager: Filter error: %v", err)
			fm.files = files // Fall back to showing all files
		} else {
			fm.files = filtered
		}
	} else {
		fm.files = files
	}

	// Update binding to reflect all changes
	items := make([]interface{}, len(fm.files))
	for i, file := range fm.files {
		items[i] = fileinfo.ListItem{
			Index:    i,
			FileInfo: file,
		}
	}
	fm.fileBinding.Set(items)

	// Explicitly refresh on file deletions or modifications, since Fyne only auto-updates on additions.
	fm.fileList.Refresh()
}

func (fm *FileManager) RemoveFromSelections(path string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	delete(fm.selectedFiles, path)
}
