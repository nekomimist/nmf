package main

import (
	"context"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/config"
	"nmf/internal/configscript"
	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
	"nmf/internal/search"
	customtheme "nmf/internal/theme"
	"nmf/internal/ui"
	"nmf/internal/watcher"
)

// FileManager is the main file manager struct.
type FileManager struct {
	mu                sync.RWMutex // Protects files and selectedFiles from concurrent access
	window            fyne.Window
	currentPath       string
	files             []fileinfo.FileInfo
	originalFiles     []fileinfo.FileInfo // Original files before filtering
	fileList          *widget.List
	fileListView      *ui.KeySink
	windowHighlight   *canvas.Rectangle
	windowActive      bool
	pathDisplay       *widget.Label
	statusLabel       *widget.Label
	cursorPath        string          // Current cursor file path
	cursorAnchor      cursorRowAnchor // Last visible row object for shell menu positioning
	selectedFiles     map[string]bool // Set of selected file paths
	storageInfo       fileinfo.StorageInfo
	storageKnown      bool
	fileBinding       binding.UntypedList
	config            *config.Config
	configManager     *config.Manager
	configScript      *configscript.Runtime
	initialWindowSize fyne.Size
	activeSort        config.SortConfig
	customTheme       *customtheme.CustomTheme                // Custom theme for colors
	cursorRenderer    ui.CursorRenderer                       // Cursor display renderer
	keyManager        *keymanager.KeyManager                  // Keyboard input manager
	dirWatcher        *watcher.DirectoryWatcher               // Directory change watcher
	currentFilter     *config.FilterEntry                     // Currently applied filter
	searchOverlay     *ui.IncrementalSearchOverlay            // Incremental search overlay
	searchHandler     *keymanager.IncrementalSearchKeyHandler // Search key handler
	searchToken       keymanager.HandlerToken                 // Token of the pushed search handler
	searchMatchers    *search.Provider                        // Shared search matcher provider
	iconSvc           *fileinfo.IconService                   // Async icon service
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

	// Jobs indicator
	jobsButton    *widget.Button
	jobsBlinking  bool
	jobsBlinkOn   bool
	jobsBlinkStop chan struct{}
	jobsUnsub     func()
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

func (fm *FileManager) UpdateFiles(files []fileinfo.FileInfo) {
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

	fm.sortFilesWithConfig(fm.CurrentSort())

	// Update binding to reflect all changes
	fm.rebuildFileBinding()

	// Explicitly refresh on file deletions or modifications, since Fyne only auto-updates on additions.
	fm.fileList.Refresh()
	fm.updateStatusBar()
}

func (fm *FileManager) RemoveFromSelections(path string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	delete(fm.selectedFiles, path)
}
