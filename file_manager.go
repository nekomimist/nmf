package main

import (
	"context"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/browser"
	"nmf/internal/config"
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
	window               fyne.Window
	browser              *browser.Model
	fileList             *widget.List
	fileListView         *ui.KeySink
	fileListItemHeight   float32
	windowHighlight      *ui.HighlightFrame
	windowActive         bool
	pathDisplay          *widget.Label
	statusLabel          *widget.Label
	cursorRefreshSeq     uint64          // Diagnostic sequence for requested cursor refreshes
	cursorItemUpdateSeq  uint64          // Latest cursor refresh sequence observed by the list UpdateItem callback
	cursorMoveDirection  int             // Pending vertical cursor movement: -1 up, 0 none, +1 down
	cursorAnchor         cursorRowAnchor // Last visible row object for shell menu positioning
	config               *config.Config
	state                *config.State
	stateManager         *config.StateManager
	initialWindowSize    fyne.Size
	customTheme          *customtheme.CustomTheme                // Custom theme for colors
	keyManager           *keymanager.KeyManager                  // Keyboard input manager
	mainKeyHandler       *keymanager.MainScreenKeyHandler        // Main screen key handler (for canvas shortcut registration)
	dirWatcher           *watcher.DirectoryWatcher               // Directory change watcher
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
	busy                 *ui.BusyController
	directoryLoader      *browser.DirectoryLoader
	viewerMu             sync.Mutex
	nextViewerID         uint64
	activeViewer         uint64
	viewerCancel         context.CancelFunc

	// Jobs indicator
	jobsButton    *widget.Button
	jobsBlinking  bool
	jobsBlinkStop chan struct{}
	jobsUnsub     func()

	// Transient non-modal notice appended to the status bar.
	statusNotice           string
	statusNoticeGeneration uint64
}

func (fm *FileManager) beginViewerLoad() (uint64, context.Context) {
	fm.viewerMu.Lock()
	defer fm.viewerMu.Unlock()
	if fm.viewerCancel != nil {
		fm.viewerCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	fm.nextViewerID++
	fm.activeViewer = fm.nextViewerID
	fm.viewerCancel = cancel
	return fm.activeViewer, ctx
}

func (fm *FileManager) finishViewerLoad(id uint64) bool {
	fm.viewerMu.Lock()
	defer fm.viewerMu.Unlock()
	if fm.activeViewer != id {
		return false
	}
	fm.activeViewer = 0
	if fm.viewerCancel != nil {
		fm.viewerCancel()
		fm.viewerCancel = nil
	}
	return true
}

func (fm *FileManager) invalidateViewerLoad(id uint64) bool {
	fm.viewerMu.Lock()
	defer fm.viewerMu.Unlock()
	if id != 0 && fm.activeViewer != id {
		return false
	}
	fm.activeViewer = 0
	if fm.viewerCancel != nil {
		fm.viewerCancel()
		fm.viewerCancel = nil
	}
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

func (fm *FileManager) browserModel() *browser.Model {
	if fm.browser == nil {
		sortConfig := config.SortConfig{SortBy: "name", SortOrder: "asc", DirectoriesFirst: true}
		if fm.state != nil && fm.config != nil {
			sortConfig = fm.state.EffectiveSort(fm.config.UI.Sort)
		}
		fm.browser = browser.New("", sortConfig)
	}
	return fm.browser
}

// Interface implementation for watcher.FileManager.
func (fm *FileManager) GetCurrentPath() string {
	return fm.browserModel().Path()
}

func (fm *FileManager) GetFiles() []fileinfo.FileInfo {
	return fm.browserModel().Files()
}

// GetUnfilteredFiles returns the complete current directory listing. The
// watcher uses this as its baseline so entries hidden by an active filter are
// not mistaken for newly added files.
func (fm *FileManager) GetUnfilteredFiles() []fileinfo.FileInfo {
	return fm.browserModel().SourceFiles()
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
	if err := fm.browserModel().ReplaceFiles(files, resort); err != nil {
		debugPrint("FileManager: Filter error: %v", err)
	}

	// widget.List is not data-bound, so it never redraws on its own; refresh
	// explicitly to reflect additions, deletions, and modifications.
	if fm.fileList != nil {
		fm.fileList.Refresh()
	}
	fm.updateStatusBar()
}

func (fm *FileManager) RemoveFromSelections(path string) {
	fm.browserModel().RemoveSelected(path)
}

// ApplyChanges merges watcher-detected added/deleted/modified files into the
// current listing. The watcher marshals this call onto Fyne's main goroutine;
// the browser model itself owns and synchronizes the mutable list state.
func (fm *FileManager) ApplyChanges(added, deleted, modified []fileinfo.FileInfo) {
	if err := fm.browserModel().ApplyChanges(added, deleted, modified); err != nil {
		debugPrint("FileManager: Filter error: %v", err)
	}
	if fm.fileList != nil {
		fm.fileList.Refresh()
	}
	fm.updateStatusBar()
}
