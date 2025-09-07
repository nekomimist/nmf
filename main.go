package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
	"nmf/internal/jobs"
	"nmf/internal/keymanager"
	"nmf/internal/secret"
	customtheme "nmf/internal/theme"
	"nmf/internal/ui"
	"nmf/internal/watcher"
)

// Global debug flag
var debugMode bool

// Global window registry for managing multiple windows
var (
	windowRegistry sync.Map // map[fyne.Window]*FileManager
	windowCount    int32    // atomic counter for window count
)

// debugPrint prints debug messages only when debug mode is enabled
func debugPrint(format string, args ...interface{}) {
	if debugMode {
		log.Printf("DEBUG: "+format, args...)
	}
}

// FileManager is the main file manager struct
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
}

// Interface implementation for watcher.FileManager
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
			debugPrint("Filter error: %v", err)
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

// SaveCursorPosition saves the current cursor position for the given directory
func (fm *FileManager) SaveCursorPosition(dirPath string) {
	currentIdx := fm.GetCurrentCursorIndex()
	if currentIdx < 0 || currentIdx >= len(fm.files) {
		return
	}

	fileName := fm.files[currentIdx].Name
	cursorMemory := &fm.config.UI.CursorMemory

	// Clean up old entries if we exceed max entries
	if len(cursorMemory.Entries) >= cursorMemory.MaxEntries {
		fm.cleanupOldCursorEntries()
	}

	// Save the cursor position and update last used time
	cursorMemory.Entries[dirPath] = fileName
	cursorMemory.LastUsed[dirPath] = time.Now()

	// Save config to disk
	if err := fm.configManager.Save(fm.config); err != nil {
		debugPrint("Error saving cursor position config: %v", err)
	}
}

// restoreCursorPosition restores the cursor position for the given directory
func (fm *FileManager) restoreCursorPosition(dirPath string) string {
	cursorMemory := &fm.config.UI.CursorMemory

	fileName, exists := cursorMemory.Entries[dirPath]
	if !exists {
		return ""
	}

	// Update last used time
	cursorMemory.LastUsed[dirPath] = time.Now()

	return fileName
}

// cleanupOldCursorEntries removes the oldest entries when maxEntries is exceeded
func (fm *FileManager) cleanupOldCursorEntries() {
	cursorMemory := &fm.config.UI.CursorMemory

	if len(cursorMemory.Entries) < cursorMemory.MaxEntries {
		return
	}

	// Find the oldest entry
	var oldestPath string
	var oldestTime *time.Time

	for path, lastUsed := range cursorMemory.LastUsed {
		if oldestTime == nil || lastUsed.Before(*oldestTime) {
			oldestPath = path
			oldestTime = &lastUsed
		}
	}

	// Remove the oldest entry
	if oldestPath != "" {
		delete(cursorMemory.Entries, oldestPath)
		delete(cursorMemory.LastUsed, oldestPath)
	}
}

func NewFileManager(app fyne.App, path string, config *config.Config, configManager *config.Manager, customTheme *customtheme.CustomTheme) *FileManager {
	fm := &FileManager{
		window:         app.NewWindow("File Manager"),
		currentPath:    path,
		cursorPath:     "",
		selectedFiles:  make(map[string]bool),
		fileBinding:    binding.NewUntypedList(),
		config:         config,
		configManager:  configManager,
		customTheme:    customTheme,
		cursorRenderer: ui.NewCursorRenderer(config.UI.CursorStyle),
		keyManager:     keymanager.NewKeyManager(debugPrint),
	}

	// Busy overlay (hidden by default)
	fm.busyOverlay = ui.NewBusyOverlay()
	fm.busyDelay = 150 * time.Millisecond

	// Initialize async icon service and subscribe for updates
	fm.iconSvc = fileinfo.NewIconService(debugPrint)
	// Refresh the list when icons arrive (thread-safe via canvas.Refresh)
	fm.iconSvc.OnUpdated(func() {
		if fm.fileList != nil {
			canvas.Refresh(fm.fileList)
		}
	})

	// Create directory watcher
	fm.dirWatcher = watcher.NewDirectoryWatcher(fm, debugPrint)

	// Install SMB credentials provider (cached + interactive prompt fallback)
	cached := fileinfo.NewCachedCredentialsProvider(ui.NewSMBCredentialsProvider(fm.window))
	fileinfo.SetCredentialsProvider(cached)

	// Initialize OS keyring (99designs). If unavailable, continue without persistent store.
	if store, err := secret.NewKeyringStore(); err == nil {
		fileinfo.SetSecretStore(store)
	}

	// Create incremental search overlay
	fm.searchOverlay = ui.NewIncrementalSearchOverlay([]fileinfo.FileInfo{}, fm.keyManager, debugPrint)
	fm.searchHandler = keymanager.NewIncrementalSearchKeyHandler(fm, debugPrint)

	// Setup KeyManager with main screen handler
	mainHandler := keymanager.NewMainScreenKeyHandler(fm, debugPrint)
	fm.keyManager.PushHandler(mainHandler)

	fm.setupUI()
	fm.LoadDirectory(path)

	// Start watching after initial load
	fm.dirWatcher.Start()

	// Register window in global registry
	windowRegistry.Store(fm.window, fm)
	atomic.AddInt32(&windowCount, 1)

	// Set window close handler
	fm.window.SetCloseIntercept(func() {
		fm.closeWindow()
	})

	return fm
}

func (fm *FileManager) setupUI() {
	// Path entry for direct path input
	fm.pathEntry = ui.NewTabEntry()
	fm.pathEntry.SetText(fm.currentPath)
	fm.pathEntry.OnSubmitted = func(path string) {
		fm.navigateToPath(path)
	}

	// Create file list
	fm.fileList = widget.NewListWithData(
		fm.fileBinding,
		func() fyne.CanvasObject {
			// Create tappable icon (onTapped will be set in UpdateItem)
			icon := ui.NewTappableIcon(theme.FolderIcon(), nil)
			// Use RichText for colored filename display
			nameRichText := widget.NewRichTextFromMarkdown("filename")
			info := widget.NewLabel("info")

			// Left side: icon + name (with minimal spacing)
			// Size icon based on text height for consistency
			textSize := fyne.CurrentApp().Settings().Theme().Size(theme.SizeNameText)
			icon.Resize(fyne.NewSize(textSize, textSize))

			leftSide := container.NewHBox(
				icon,
				nameRichText,
			)

			// Use border container to align name left and info right
			borderContainer := container.NewBorder(
				nil, nil, leftSide, info, nil,
			)

			// Use normal container with max layout to hold content and decorations
			return container.NewMax(borderContainer)
		},
		func(item binding.DataItem, obj fyne.CanvasObject) {
			dataItem := item.(binding.Untyped)
			data, _ := dataItem.Get()
			listItem := data.(fileinfo.ListItem)
			fileInfo := listItem.FileInfo
			index := listItem.Index

			// obj is a container with border and optional cursor/selection elements
			outerContainer := obj.(*fyne.Container)

			// Find the border container (should be first element)
			var border *fyne.Container
			if len(outerContainer.Objects) > 0 {
				if container, ok := outerContainer.Objects[0].(*fyne.Container); ok {
					border = container
				}
			}

			if border != nil {
				// Find leftSide and info widgets within border
				var leftSide *fyne.Container
				var infoLabel *widget.Label

				for _, obj := range border.Objects {
					if obj == nil {
						continue
					}
					if container, ok := obj.(*fyne.Container); ok {
						leftSide = container
					} else if label, ok := obj.(*widget.Label); ok {
						infoLabel = label
					}
				}

				if leftSide != nil && infoLabel != nil && len(leftSide.Objects) >= 2 {
					// Structure is now: [icon, nameRichText]
					if icon, ok := leftSide.Objects[0].(*ui.TappableIcon); ok {
						nameRichText := leftSide.Objects[1].(*widget.RichText)

						// Set icon resource with async service (Windows uses real icons if available)
						// Default placeholders
						folderRes := theme.FolderIcon()
						fileRes := theme.FileIcon()
						if fileInfo.IsDir {
							icon.SetResource(folderRes)
						} else {
							// Desired icon size roughly equals text size
							textSize := int(fyne.CurrentApp().Settings().Theme().Size(theme.SizeNameText))
							ext := strings.ToLower(filepath.Ext(fileInfo.Name))
							if fm.iconSvc != nil {
								if res, ok := fm.iconSvc.GetCachedOrRequest(fileInfo.Path, fileInfo.IsDir, ext, textSize); ok && res != nil {
									icon.SetResource(res)
								} else {
									icon.SetResource(fileRes)
								}
							} else {
								icon.SetResource(fileRes)
							}
						}

						// Set onTapped handler for icon
						icon.SetOnTapped(func() {
							if fileInfo.IsDir {
								fm.LoadDirectory(fileInfo.Path)
							}
						})

						// Get text color based on file type
						textColor := fileinfo.GetTextColor(fileInfo.FileType, fm.customTheme)

						// Create a custom text segment with text color only
						coloredSegment := &fileinfo.ColoredTextSegment{
							Text:          fileInfo.Name,
							Color:         textColor,
							Strikethrough: fileInfo.Status == fileinfo.StatusDeleted,
						}

						nameRichText.Segments = []widget.RichTextSegment{coloredSegment}
						nameRichText.Refresh()

						if fileInfo.IsDir {
							infoLabel.SetText(fmt.Sprintf("<dir> %s %s",
								fileInfo.Modified.Format("2006-01-02"),
								fileInfo.Modified.Format("15:04:05")))
						} else {
							infoLabel.SetText(fmt.Sprintf("%s %s %s",
								fileinfo.FormatFileSize(fileInfo.Size),
								fileInfo.Modified.Format("2006-01-02"),
								fileInfo.Modified.Format("15:04:05")))
						}
					}
				}
			}

			// Handle 4 display states
			currentCursorIdx := fm.GetCurrentCursorIndex()
			isCursor := index == currentCursorIdx
			isSelected := fm.selectedFiles[fileInfo.Path]

			// Clear all decoration elements first
			outerContainer.Objects = []fyne.CanvasObject{border}

			// Add status background if file has a status (covers entire item like selection)
			statusBGColor := fileinfo.GetStatusBackgroundColor(fileInfo.Status, fm.customTheme)
			if statusBGColor != nil {
				statusBG := canvas.NewRectangle(*statusBGColor)
				statusBG.Resize(obj.Size())
				statusBG.Move(fyne.NewPos(0, 0))
				// Wrap status background in WithoutLayout container
				statusContainer := container.NewWithoutLayout(statusBG)
				outerContainer.Objects = append(outerContainer.Objects, statusContainer)
			}

			// Add selection background if selected (covers entire item)
			if isSelected {
				selectionColor := fm.customTheme.GetCustomColor("selectionBackground")
				selectionBG := canvas.NewRectangle(selectionColor)
				selectionBG.Resize(obj.Size())
				selectionBG.Move(fyne.NewPos(0, 0))
				// Wrap selection background in WithoutLayout container
				selectionContainer := container.NewWithoutLayout(selectionBG)
				outerContainer.Objects = append(outerContainer.Objects, selectionContainer)
			}

			// Add cursor if at cursor position (covers entire item like status/selection)
			if isCursor {
				cursor := fm.cursorRenderer.RenderCursor(obj.Size(), fyne.NewPos(0, 0), fm.config.UI.CursorStyle, fm.customTheme)

				// Wrap cursor in a container that won't be affected by NewMax
				cursorContainer := container.NewWithoutLayout(cursor)
				outerContainer.Objects = append(outerContainer.Objects, cursorContainer)
			}
		},
	)

	// Hide separators for compact spacing if itemSpacing is small
	if fm.config.UI.ItemSpacing <= 2 {
		fm.fileList.HideSeparators = true
	}

	// Wrap list with a generic focusable KeySink to suppress Tab traversal
	fm.fileListView = ui.NewKeySink(fm.fileList, fm.keyManager, ui.WithTabCapture(true))

	// Handle cursor movement (both mouse and keyboard)
	fm.fileList.OnSelected = func(id widget.ListItemID) {
		fm.SetCursorByIndex(id)
		// Clear list selection to avoid double cursor effect when switching back to keyboard
		fm.fileList.UnselectAll()
		// Keep focus on the KeySink so Tab does not move focus
		fm.FocusFileList()
		fm.RefreshCursor()
	}

	// Create toolbar (left side)
	toolbar := widget.NewToolbar(
		widget.NewToolbarAction(theme.NavigateBackIcon(), func() {
			parent := fileinfo.ParentPath(fm.currentPath)
			if parent != fm.currentPath {
				fm.LoadDirectory(parent)
			}
			fm.FocusFileList()
		}),
		widget.NewToolbarAction(theme.HomeIcon(), func() {
			home, _ := os.UserHomeDir()
			fm.LoadDirectory(home)
			fm.FocusFileList()
		}),
		widget.NewToolbarAction(theme.ViewRefreshIcon(), func() {
			fm.LoadDirectory(fm.currentPath)
			fm.FocusFileList()
		}),
		widget.NewToolbarAction(theme.FolderIcon(), func() {
			fm.ShowDirectoryTreeDialog()
			// focus returns after dialog closes in callback
		}),
		widget.NewToolbarAction(theme.FolderNewIcon(), func() {
			fm.OpenNewWindow()
			fm.FocusFileList()
		}),
	)

	// Jobs button on the right
	fm.jobsButton = widget.NewButton("Jobs", func() {
		fm.ShowJobsDialog()
	})
	fm.jobsButton.Importance = widget.MediumImportance

	// Layout with search overlay
	// Top row: toolbar on left, Jobs button on right
	toolbarRow := container.NewBorder(nil, nil, nil, fm.jobsButton, toolbar)
	// Subscribe to job updates to update indicator
	jobs.GetManager().Subscribe(func() { fyne.Do(fm.onJobsUpdated) })
	mainContent := container.NewBorder(
		container.NewVBox(toolbarRow, fm.pathEntry),
		nil, nil, nil,
		fm.fileListView,
	)

	// Stack main content with overlays on top (search, busy)
	content := container.NewMax(
		mainContent,
		container.NewBorder(
			fm.searchOverlay.GetContainer(), // Top overlay
			nil, nil, nil,
			nil, // Center is empty, overlay is at top
		),
		fm.busyOverlay.GetContainer(), // Highest overlay to block interactions
	)

	fm.window.SetContent(content)
	fm.window.Resize(fyne.NewSize(float32(fm.config.Window.Width), float32(fm.config.Window.Height)))

	// Initialize jobs indicator state
	fm.onJobsUpdated()

	// Ensure initial focus sits on the tabbable list view
	fm.FocusFileList()

	// Setup window close handler to properly stop DirectoryWatcher
	fm.window.SetCloseIntercept(func() {
		debugPrint("Window close intercepted - initiating cleanup for path: %s", fm.currentPath)
		if fm.dirWatcher != nil {
			debugPrint("Stopping DirectoryWatcher...")
			fm.dirWatcher.Stop()
			debugPrint("DirectoryWatcher.Stop() completed successfully")
		} else {
			debugPrint("DirectoryWatcher was nil, skipping stop")
		}
		debugPrint("Proceeding with window close")
		fm.window.Close()
	})

	// Setup keyboard handling via KeyManager
	dc, ok := (fm.window.Canvas()).(desktop.Canvas)
	if ok {
		dc.SetOnKeyDown(func(ev *fyne.KeyEvent) {
			if fm.window.Canvas().Focused() == fm.fileListView {
				return // KeySink経由で処理済み
			}
			fm.keyManager.HandleKeyDown(ev)
		})

		dc.SetOnKeyUp(func(ev *fyne.KeyEvent) {
			if fm.window.Canvas().Focused() == fm.fileListView {
				return // KeySink経由で処理済み
			}
			fm.keyManager.HandleKeyUp(ev)
		})

		fm.window.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
			fm.keyManager.HandleTypedKey(ev)
		})

		fm.window.Canvas().SetOnTypedRune(func(r rune) {
			fm.keyManager.HandleTypedRune(r)
		})
	}
}

// GetCurrentCursorIndex returns the current cursor index based on cursor path
func (fm *FileManager) GetCurrentCursorIndex() int {
	if fm.cursorPath == "" {
		return -1
	}
	for i, file := range fm.files {
		if file.Path == fm.cursorPath {
			return i
		}
	}
	return -1
}

// SetCursorByIndex sets the cursor to the specified index
func (fm *FileManager) SetCursorByIndex(index int) {
	if index >= 0 && index < len(fm.files) {
		fm.cursorPath = fm.files[index].Path
	} else {
		fm.cursorPath = ""
	}
}

// RefreshCursor updates only the cursor display without affecting selection
func (fm *FileManager) RefreshCursor() {
	// First refresh the list to ensure all items are updated
	fm.fileList.Refresh()

	// Then scroll to cursor position after refresh completes
	cursorIdx := fm.GetCurrentCursorIndex()
	if cursorIdx >= 0 {
		fm.fileList.ScrollTo(widget.ListItemID(cursorIdx))
	}
}

// navigateToPath handles path entry validation and navigation
func (fm *FileManager) navigateToPath(inputPath string) {
	// Trim whitespace from input
	path := strings.TrimSpace(inputPath)

	// Handle empty path - do nothing
	if path == "" {
		fm.pathEntry.SetText(fm.currentPath) // Reset to current path
		return
	}

	// Handle tilde expansion for home directory
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			debugPrint("Error getting home directory: %v", err)
			fm.pathEntry.SetText(fm.currentPath) // Reset to current path
			return
		}
		path = strings.Replace(path, "~", home, 1)
	}

	// Normalize input
	// - Windows: convert smb:// to UNC for OS calls
	// - Linux: if smb:// or //host/share is a mounted CIFS, map to mountpoint
	path = fileinfo.NormalizeInputPath(path)
	if vfs, parsed, err := fileinfo.ResolveRead(path); err == nil {
		if parsed.Scheme == fileinfo.SchemeSMB {
			// Seed credential cache if URL contained creds
			if parsed.User != "" || parsed.Password != "" || parsed.Domain != "" {
				fileinfo.PutCachedCredentials(parsed.Host, parsed.Share, fileinfo.Credentials{Domain: parsed.Domain, Username: parsed.User, Password: parsed.Password})
			}
			// For SMB remote, skip local os.Stat check and navigate using display path
			fm.LoadDirectory(parsed.Display)
			fm.FocusFileList()
			return
		}
		if parsed.Native != "" {
			path = parsed.Native
		}
		_ = vfs // reserved for future use
	} else if parsed.Scheme == fileinfo.SchemeSMB {
		debugPrint("SMB path not supported or not mounted: %s", inputPath)
		fm.pathEntry.SetText(fm.currentPath)
		return
	}

	// Convert to absolute path if it's relative
	if !filepath.IsAbs(path) {
		absPath, err := filepath.Abs(path)
		if err != nil {
			debugPrint("Error converting to absolute path: %v", err)
			fm.pathEntry.SetText(fm.currentPath) // Reset to current path
			return
		}
		path = absPath
	}

	// Validate the path exists and is a directory
	info, err := os.Stat(path)
	if err != nil {
		debugPrint("Path does not exist: %s - %v", path, err)
		fm.pathEntry.SetText(fm.currentPath) // Reset to current path
		return
	}

	if !info.IsDir() {
		debugPrint("Path is not a directory: %s", path)
		fm.pathEntry.SetText(fm.currentPath) // Reset to current path
		return
	}

	// Path is valid, navigate to it
	fm.LoadDirectory(path)

	// Return focus to file list after successful navigation
	fm.FocusFileList()
}

// FocusFileList sets focus to the file list view
func (fm *FileManager) FocusFileList() {
	if fm.fileListView != nil {
		fm.window.Canvas().Focus(fm.fileListView)
	}
}

func (fm *FileManager) LoadDirectory(path string) {
	// Save current cursor position before changing directory
	// Skip saving if already saved manually (e.g., during refresh)
	if fm.currentPath != "" && fm.currentPath != path {
		fm.SaveCursorPosition(fm.currentPath)
	}

	// Stop current directory watcher if running
	if fm.dirWatcher != nil {
		fm.dirWatcher.Stop()
	}

	// Store the previous directory for parent navigation logic
	previousPath := fm.currentPath

	// Indicate busy and block input while loading
	fm.beginBusy(fmt.Sprintf("Loading %s...", path))

	// Load directory asynchronously to avoid blocking UI (applies to both local and remote paths)
	go fm.loadDirectoryAsync(path, previousPath)
}

// loadSMBDirectory lists SMB path on a background goroutine and applies UI updates on main thread.
// loadDirectoryAsync lists a path in a background goroutine and applies UI updates on the main thread.
func (fm *FileManager) loadDirectoryAsync(path string, previousPath string) {
	entries, err := fileinfo.ReadDirPortable(path)
	if err != nil {
		log.Printf("Error reading directory: %v", err)
		fyne.Do(func() {
			// Clear busy state on error
			fm.endBusy()
			ui.ShowMessageDialog(fm.window, "フォルダを開けませんでした", err.Error())
			// Revert to previous path on error and restart watcher
			if previousPath != "" {
				fm.currentPath = previousPath
				fm.pathEntry.SetText(previousPath)
				if fm.dirWatcher != nil {
					fm.dirWatcher.SetPollInterval(fm.pollIntervalForPath(previousPath))
					fm.dirWatcher.Start()
				}
			}
		})
		return
	}

	// Build file list off the UI thread
	files := make([]fileinfo.FileInfo, 0, len(entries)+1)

	// Add parent directory entry if not at root
	var parent string
	parent = fileinfo.ParentPath(path)
	if parent != path {
		parentInfo := fileinfo.FileInfo{
			Name:     "..",
			Path:     parent,
			IsDir:    true,
			Size:     0,
			Modified: time.Time{},
			FileType: fileinfo.FileTypeDirectory,
			Status:   fileinfo.StatusNormal,
		}
		files = append(files, parentInfo)
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		var fullPath string
		fullPath = fileinfo.JoinPath(path, entry.Name())
		fileType := fileinfo.DetermineFileType(fullPath, entry.Name(), entry.IsDir())
		fi := fileinfo.FileInfo{
			Name:     entry.Name(),
			Path:     fullPath,
			IsDir:    entry.IsDir(),
			Size:     info.Size(),
			Modified: info.ModTime(),
			FileType: fileType,
			Status:   fileinfo.StatusNormal,
		}
		files = append(files, fi)
	}

	// Apply UI updates on main thread
	fyne.Do(func() {
		// Clear busy state on success just before applying changes
		fm.endBusy()
		// Stop existing watcher (if any) before applying
		if fm.dirWatcher != nil {
			fm.dirWatcher.Stop()
		}

		// Add previous path to navigation history before changing directory
		if previousPath != "" && previousPath != path {
			fm.config.AddToNavigationHistory(previousPath)
			if err := fm.configManager.Save(fm.config); err != nil {
				debugPrint("Error saving navigation history: %v", err)
			}
		}

		fm.currentPath = path
		fm.pathEntry.SetText(path)
		fm.files = files

		// Sort and build items
		fm.sortFiles()
		items := make([]interface{}, 0, len(fm.files))
		for i, f := range fm.files {
			items = append(items, fileinfo.ListItem{Index: i, FileInfo: f})
		}
		fm.fileBinding.Set(items)

		// Clear selections and restore cursor
		fm.selectedFiles = make(map[string]bool)
		if len(fm.files) > 0 {
			parentPrev := fileinfo.ParentPath(previousPath)
			if parentPrev == path && previousPath != "" {
				dirName := fileinfo.BaseName(previousPath)
				cursorSet := false
				for i, f := range fm.files {
					if f.Name == dirName {
						fm.SetCursorByIndex(i)
						cursorSet = true
						break
					}
				}
				if !cursorSet {
					fm.SetCursorByIndex(0)
				}
			} else {
				saved := fm.restoreCursorPosition(path)
				cursorSet := false
				if saved != "" {
					for i, f := range fm.files {
						if f.Name == saved {
							fm.SetCursorByIndex(i)
							cursorSet = true
							break
						}
					}
				}
				if !cursorSet {
					fm.SetCursorByIndex(0)
				}
			}
			fm.RefreshCursor()
		} else {
			fm.cursorPath = ""
		}

		// Restart watcher with appropriate interval
		if fm.dirWatcher != nil {
			fm.dirWatcher.SetPollInterval(fm.pollIntervalForPath(path))
			fm.dirWatcher.Start()
		}
	})
}

// beginBusy shows the busy overlay and pushes a swallowing key handler
func (fm *FileManager) beginBusy(text string) {
	fm.busyMu.Lock()
	defer fm.busyMu.Unlock()

	if fm.busyActive {
		// Already busy: update label if visible, and store latest text
		fm.busyText = text
		fm.busyOverlay.Show(fm.window, text)
		return
	}

	fm.busyActive = true
	fm.busyText = text

	// Block keys immediately to avoid reentrancy
	fm.keyManager.PushHandler(keymanager.NewBusyKeyHandler())

	// Delay overlay to prevent flicker on very fast operations
	if fm.busyTimer != nil {
		fm.busyTimer.Stop()
	}
	d := fm.busyDelay
	fm.busyTimer = time.AfterFunc(d, func() {
		fyne.Do(func() {
			fm.busyMu.Lock()
			active := fm.busyActive
			text := fm.busyText
			fm.busyMu.Unlock()
			if active {
				fm.busyOverlay.Show(fm.window, text)
			}
		})
	})
}

// endBusy hides the busy overlay and pops the swallowing key handler
func (fm *FileManager) endBusy() {
	fm.busyMu.Lock()
	if !fm.busyActive {
		fm.busyMu.Unlock()
		return
	}
	fm.busyActive = false
	if fm.busyTimer != nil {
		fm.busyTimer.Stop()
		fm.busyTimer = nil
	}
	fm.busyMu.Unlock()

	// Hide overlay (if visible) and pop guard
	fm.busyOverlay.Hide()
	fm.keyManager.PopHandler()
}

// Path helpers moved to internal/fileinfo (JoinPath/ParentPath/BaseName)

// pollIntervalForPath returns the recommended watcher polling interval for a path.
// Remote (SMB) paths get a longer interval to reduce load/latency impact.
func (fm *FileManager) pollIntervalForPath(p string) time.Duration {
	if strings.HasPrefix(strings.ToLower(p), "smb://") {
		return 4 * time.Second
	}
	return 2 * time.Second
}

func (fm *FileManager) OpenNewWindow() {
	newFM := NewFileManager(fyne.CurrentApp(), fm.currentPath, fm.config, fm.configManager, fm.customTheme)
	newFM.window.Show()
}

// onJobsUpdated updates the Jobs indicator based on queue state
func (fm *FileManager) onJobsUpdated() {
	mgr := jobs.GetManager()
	snaps := mgr.List()
	var hasError, hasPending, hasRunning bool
	for _, s := range snaps {
		switch s.Status {
		case jobs.StatusFailed:
			hasError = true
		case jobs.StatusPending:
			hasPending = true
		case jobs.StatusRunning:
			hasRunning = true
		}
	}

	if fm.jobsButton == nil {
		return
	}

	// Visual policy:
	// - Error or Pending: blink
	// - Running only: highlight but no blink
	if hasError || hasPending {
		fm.jobsButton.Importance = widget.HighImportance
		if !fm.jobsBlinking {
			fm.startJobsBlink()
		}
	} else {
		if fm.jobsBlinking {
			fm.stopJobsBlink()
		}
		if hasRunning {
			fm.jobsButton.Importance = widget.HighImportance
		} else {
			fm.jobsButton.Importance = widget.MediumImportance
		}
	}
	fm.jobsButton.Refresh()
}

func (fm *FileManager) startJobsBlink() {
	if fm.jobsBlinking {
		return
	}
	fm.jobsBlinking = true
	fm.jobsBlinkOn = true
	fm.jobsBlinkStop = make(chan struct{})
	go func(stop <-chan struct{}) {
		ticker := time.NewTicker(600 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				fm.jobsBlinkOn = !fm.jobsBlinkOn
				// Toggle importance to create a blink effect
				fyne.Do(func() {
					if fm.jobsButton != nil {
						if fm.jobsBlinkOn {
							fm.jobsButton.Importance = widget.HighImportance
						} else {
							fm.jobsButton.Importance = widget.MediumImportance
						}
						fm.jobsButton.Refresh()
					}
				})
			case <-stop:
				return
			}
		}
	}(fm.jobsBlinkStop)
}

func (fm *FileManager) stopJobsBlink() {
	if !fm.jobsBlinking {
		return
	}
	close(fm.jobsBlinkStop)
	fm.jobsBlinking = false
	fm.jobsBlinkOn = false
}

// ShowCopyDialog shows the copy UI (simulation only)
func (fm *FileManager) ShowCopyDialog() { fm.showCopyMoveDialog(ui.OpCopy) }

// ShowMoveDialog shows the move UI (simulation only)
func (fm *FileManager) ShowMoveDialog() { fm.showCopyMoveDialog(ui.OpMove) }

// showCopyMoveDialog builds targets and destination candidates then shows dialog
func (fm *FileManager) showCopyMoveDialog(op ui.Operation) {
	// Determine targets: marked files if any; otherwise cursor item
	targets := fm.collectTargets()
	if len(targets) == 0 {
		debugPrint("No valid target for %s", string(op))
		return
	}

	// Build destination candidates: other windows' directories first, then history without duplicates
	dest := fm.buildDestinationCandidates()
	if len(dest) == 0 {
		debugPrint("No destination candidates available")
	}

	// We need full source paths for jobs, not only names — compute now
	srcPaths := fm.collectTargetPaths()
	dlg := ui.NewCopyMoveDialog(op, targets, dest, fm.config.UI.NavigationHistory.LastUsed, fm.keyManager, debugPrint)
	dlg.ShowDialog(fm.window, func(selectedDest string) {
		mgr := jobs.GetManager()
		if op == ui.OpCopy {
			mgr.EnqueueCopy(srcPaths, selectedDest)
		} else {
			mgr.EnqueueMove(srcPaths, selectedDest)
		}
		// Feedback
		ui.ShowMessageDialog(fm.window, strings.Title(string(op)), fmt.Sprintf("Queued %d item(s) to:\n%s", len(srcPaths), selectedDest))
		fm.FocusFileList()
	})
}

// collectTargets returns display names of targets based on selection or cursor
func (fm *FileManager) collectTargets() []string {
	// Gather selected files
	var selected []string
	for p, sel := range fm.selectedFiles {
		if !sel {
			continue
		}
		// Find matching file to ensure it still exists in list and to skip parent/invalid
		for _, fi := range fm.files {
			if fi.Path == p {
				if fi.Name == ".." || fi.Status == fileinfo.StatusDeleted {
					continue
				}
				selected = append(selected, fi.Name)
				break
			}
		}
	}
	if len(selected) > 0 {
		return selected
	}
	// Fall back to cursor
	idx := fm.GetCurrentCursorIndex()
	if idx >= 0 && idx < len(fm.files) {
		fi := fm.files[idx]
		if fi.Name != ".." && fi.Status != fileinfo.StatusDeleted {
			return []string{fi.Name}
		}
	}
	return nil
}

// collectTargetPaths returns absolute/native source file paths
func (fm *FileManager) collectTargetPaths() []string {
	var selected []string
	for p, sel := range fm.selectedFiles {
		if !sel {
			continue
		}
		for _, fi := range fm.files {
			if fi.Path == p {
				if fi.Name == ".." || fi.Status == fileinfo.StatusDeleted {
					continue
				}
				selected = append(selected, fi.Path)
				break
			}
		}
	}
	if len(selected) > 0 {
		return selected
	}
	idx := fm.GetCurrentCursorIndex()
	if idx >= 0 && idx < len(fm.files) {
		fi := fm.files[idx]
		if fi.Name != ".." && fi.Status != fileinfo.StatusDeleted {
			return []string{fi.Path}
		}
	}
	return nil
}

// ShowJobsDialog opens the job queue view
func (fm *FileManager) ShowJobsDialog() {
	dlg := ui.NewJobsDialog(fm.keyManager, debugPrint)
	dlg.ShowDialog(fm.window)
}

// buildDestinationCandidates composes other windows' dirs then history without dups
func (fm *FileManager) buildDestinationCandidates() []string {
	// Collect from other windows
	seen := map[string]struct{}{}
	var candidates []string
	windowRegistry.Range(func(k, v any) bool {
		if other, ok := v.(*FileManager); ok {
			if other == fm {
				return true
			}
			if other.currentPath != "" {
				if _, ok := seen[other.currentPath]; !ok {
					candidates = append(candidates, other.currentPath)
					seen[other.currentPath] = struct{}{}
				}
			}
		}
		return true
	})

	// Optionally include current path after other windows
	if fm.currentPath != "" {
		if _, ok := seen[fm.currentPath]; !ok {
			candidates = append(candidates, fm.currentPath)
			seen[fm.currentPath] = struct{}{}
		}
	}

	// Append navigation history skipping dups
	for _, p := range fm.config.GetNavigationHistory() {
		if _, ok := seen[p]; ok {
			continue
		}
		candidates = append(candidates, p)
		seen[p] = struct{}{}
	}
	return candidates
}

// ShowDirectoryTreeDialog shows the directory tree navigation dialog
func (fm *FileManager) ShowDirectoryTreeDialog() {
	dialog := ui.NewDirectoryTreeDialog(fm.currentPath, fm.keyManager, debugPrint)
	dialog.ShowDialog(fm.window, func(selectedPath string) {
		debugPrint("Directory selected from tree dialog: %s", selectedPath)
		fm.LoadDirectory(selectedPath)
		fm.FocusFileList()
	})
}

// ShowNavigationHistoryDialog shows the navigation history dialog
func (fm *FileManager) ShowNavigationHistoryDialog() {
	historyPaths := fm.config.GetNavigationHistory()

	// Add current path to the beginning of history list
	enhancedPaths := []string{}
	if fm.currentPath != "" {
		enhancedPaths = append(enhancedPaths, fm.currentPath)
	}

	// Add existing history paths, but skip duplicates of current path
	for _, path := range historyPaths {
		if path != fm.currentPath {
			enhancedPaths = append(enhancedPaths, path)
		}
	}

	if len(enhancedPaths) == 0 {
		debugPrint("No navigation history available")
		return
	}

	dialog := ui.NewNavigationHistoryDialog(
		enhancedPaths,
		fm.config.UI.NavigationHistory.LastUsed,
		fm.keyManager,
		debugPrint,
	)
	dialog.ShowDialog(fm.window, func(selectedPath string) {
		debugPrint("Directory selected from history dialog: %s", selectedPath)
		fm.LoadDirectory(selectedPath)
		fm.FocusFileList()
	})
}

// GetSelectedFiles returns the map of selected files
func (fm *FileManager) GetSelectedFiles() map[string]bool {
	return fm.selectedFiles
}

// SetFileSelected sets the selection state of a file
func (fm *FileManager) SetFileSelected(path string, selected bool) {
	fm.selectedFiles[path] = selected
}

// RefreshFileList refreshes the file list display
func (fm *FileManager) RefreshFileList() {
	fm.fileList.Refresh()
}

// ShowFilterDialog displays the file filter dialog
func (fm *FileManager) ShowFilterDialog() {
	// Get current filter entries from config
	entries := fm.config.UI.FileFilter.Entries

	// Use originalFiles if available, otherwise use current files
	currentFiles := fm.originalFiles
	if len(currentFiles) == 0 {
		currentFiles = fm.files
	}

	filterDialog := ui.NewFilterDialog(entries, currentFiles, fm.keyManager, debugPrint)
	filterDialog.ShowDialog(fm.window, func(selectedEntry *config.FilterEntry) {
		if selectedEntry != nil {
			fm.ApplyFilter(selectedEntry)
			fm.saveFilterToHistory(selectedEntry)
		}
	})
}

// ApplyFilter applies a filter to the current file list
func (fm *FileManager) ApplyFilter(entry *config.FilterEntry) {
	if entry == nil || entry.Pattern == "" {
		fm.ClearFilter()
		return
	}

	// Validate pattern first
	if err := fileinfo.ValidatePattern(entry.Pattern); err != nil {
		debugPrint("Invalid filter pattern '%s': %v", entry.Pattern, err)
		return
	}

	fm.currentFilter = entry
	fm.config.UI.FileFilter.Current = entry
	fm.config.UI.FileFilter.Enabled = true

	// Use originalFiles if available, otherwise use current files as base
	baseFiles := fm.originalFiles
	if len(baseFiles) == 0 {
		baseFiles = fm.files
		fm.originalFiles = make([]fileinfo.FileInfo, len(fm.files))
		copy(fm.originalFiles, fm.files)
	}

	// Apply filter
	filtered, err := fileinfo.FilterFiles(baseFiles, entry.Pattern)
	if err != nil {
		debugPrint("Filter error: %v", err)
		return
	}

	fm.files = filtered

	// Update UI
	items := make([]interface{}, len(fm.files))
	for i, file := range fm.files {
		items[i] = fileinfo.ListItem{
			Index:    i,
			FileInfo: file,
		}
	}
	fm.fileBinding.Set(items)

	// Reset cursor to first item if available
	if len(fm.files) > 0 {
		fm.SetCursorByIndex(0)
		fm.RefreshCursor()
	}

	debugPrint("Applied filter: %s (matched %d/%d files)", entry.Pattern, len(fm.files), len(baseFiles))
}

// ClearFilter completely removes the current filter (for Ctrl+Shift+F)
func (fm *FileManager) ClearFilter() {
	fm.currentFilter = nil
	fm.config.UI.FileFilter.Current = nil // Complete clear
	fm.config.UI.FileFilter.Enabled = false

	if len(fm.originalFiles) > 0 {
		fm.files = fm.originalFiles

		// Update UI
		items := make([]interface{}, len(fm.files))
		for i, file := range fm.files {
			items[i] = fileinfo.ListItem{
				Index:    i,
				FileInfo: file,
			}
		}
		fm.fileBinding.Set(items)
	}

	debugPrint("Filter completely cleared, showing all %d files", len(fm.files))
}

// ToggleFilter toggles the current filter on/off
func (fm *FileManager) ToggleFilter() {
	if fm.config.UI.FileFilter.Enabled && fm.currentFilter != nil {
		fm.DisableFilter()
	} else if fm.config.UI.FileFilter.Current != nil {
		fm.ApplyFilter(fm.config.UI.FileFilter.Current)
	}
}

// DisableFilter temporarily disables the current filter (for toggle functionality)
func (fm *FileManager) DisableFilter() {
	fm.currentFilter = nil
	// Keep fm.config.UI.FileFilter.Current for toggle functionality
	fm.config.UI.FileFilter.Enabled = false

	if len(fm.originalFiles) > 0 {
		fm.files = fm.originalFiles

		// Update UI
		items := make([]interface{}, len(fm.files))
		for i, file := range fm.files {
			items[i] = fileinfo.ListItem{
				Index:    i,
				FileInfo: file,
			}
		}
		fm.fileBinding.Set(items)
	}

	debugPrint("Filter disabled, showing all %d files", len(fm.files))
}

// saveFilterToHistory saves a filter entry to the history
func (fm *FileManager) saveFilterToHistory(entry *config.FilterEntry) {
	if entry == nil || entry.Pattern == "" {
		return
	}

	filterConfig := &fm.config.UI.FileFilter

	// Update existing entry or add new one
	found := false
	for i := range filterConfig.Entries {
		if filterConfig.Entries[i].Pattern == entry.Pattern {
			filterConfig.Entries[i].LastUsed = entry.LastUsed
			filterConfig.Entries[i].UseCount = entry.UseCount
			found = true
			break
		}
	}

	if !found {
		// Add new entry at the beginning
		filterConfig.Entries = append([]config.FilterEntry{*entry}, filterConfig.Entries...)

		// Trim to max entries
		if len(filterConfig.Entries) > filterConfig.MaxEntries {
			filterConfig.Entries = filterConfig.Entries[:filterConfig.MaxEntries]
		}
	}

	// Save config to disk
	if err := fm.configManager.Save(fm.config); err != nil {
		debugPrint("Error saving filter history: %v", err)
	}
}

// ShowIncrementalSearchDialog shows the incremental search overlay
func (fm *FileManager) ShowIncrementalSearchDialog() {
	debugPrint("Starting incremental search mode")

	// Update overlay with current files
	fm.searchOverlay.UpdateFiles(fm.files)

	// Set up callbacks
	fm.searchOverlay.SetCallback(func(selectedFile *fileinfo.FileInfo) {
		// Navigate to selected file/directory
		if selectedFile.IsDir {
			// For directories, navigate into them
			targetPath := fileinfo.JoinPath(fm.currentPath, selectedFile.Name)
			debugPrint("Incremental search: navigating to directory %s", targetPath)
			fm.LoadDirectory(targetPath)
		} else {
			// For files, set cursor to them
			fm.SetCursorToFile(selectedFile)
		}

		// Pop the search handler and refocus main view
		fm.keyManager.PopHandler()
		fm.FocusFileList()
	})

	fm.searchOverlay.SetCancelCallback(func() {
		debugPrint("Incremental search cancelled")
		// Pop the search handler and refocus main view
		fm.keyManager.PopHandler()
		fm.FocusFileList()
	})

	// Push the search handler and show overlay
	fm.keyManager.PushHandler(fm.searchHandler)
	fm.searchOverlay.Show(fm.window)
}

// ShowSortDialog shows the sort configuration dialog
func (fm *FileManager) ShowSortDialog() {
	debugPrint("Showing sort dialog")

	// Get current sort configuration
	currentConfig := fm.config.UI.Sort

	// Create sort dialog
	sortDialog := ui.NewSortDialog(currentConfig, debugPrint)

	// Set up apply callback
	sortDialog.SetOnApply(func(sortConfig config.SortConfig) {
		debugPrint("Applying sort configuration: %+v", sortConfig)

		// Store current cursor file name to restore position after sorting
		var currentFile string
		cursorIndex := fm.GetCurrentCursorIndex()
		if cursorIndex >= 0 && cursorIndex < len(fm.files) {
			currentFile = fm.files[cursorIndex].Name
			debugPrint("Storing current cursor file: %s", currentFile)
		}

		// Update configuration
		fm.config.UI.Sort = sortConfig

		// Save configuration to file
		if err := fm.configManager.Save(fm.config); err != nil {
			debugPrint("Failed to save sort configuration: %v", err)
		}

		// Re-sort current files and refresh display
		fm.sortFiles()

		// Rebuild items after sorting
		items := make([]interface{}, 0, len(fm.files))
		for i, file := range fm.files {
			listItem := fileinfo.ListItem{
				Index:    i,
				FileInfo: file,
			}
			items = append(items, listItem)
		}
		fm.fileBinding.Set(items)

		// Restore cursor position to the same file if possible
		if currentFile != "" {
			for i, file := range fm.files {
				if file.Name == currentFile {
					fm.SetCursorByIndex(i)
					debugPrint("Restored cursor to file: %s at index %d", currentFile, i)
					break
				}
			}
		} else {
			// If no previous cursor file, set cursor to first item
			if len(fm.files) > 0 {
				fm.SetCursorByIndex(0)
			}
		}

		// Force UI refresh
		fm.fileList.Refresh()

		debugPrint("Sort configuration applied successfully")
	})

	// Set up cancel callback
	sortDialog.SetOnCancel(func() {
		debugPrint("Sort dialog cancelled")
	})

	// Set up cleanup callback (pop key handler)
	sortDialog.SetOnCleanup(func() {
		debugPrint("Cleaning up sort dialog - popping key handler")
		fm.keyManager.PopHandler()
	})

	// Create and push keyboard handler
	handler := keymanager.NewSortDialogHandler(sortDialog, debugPrint)
	fm.keyManager.PushHandler(handler)

	// Show dialog
	sortDialog.Show(fm.window, handler)
}

// FocusPathEntry focuses the path entry widget
func (fm *FileManager) FocusPathEntry() {
	debugPrint("Focusing path entry")
	if fm.pathEntry != nil {
		fm.window.Canvas().Focus(fm.pathEntry)
		fm.pathEntry.FocusGained()
	}
}

// IncrementalSearchInterface implementation methods

// ShowIncrementalSearchOverlay shows the search overlay
func (fm *FileManager) ShowIncrementalSearchOverlay() {
	fm.ShowIncrementalSearchDialog()
}

// HideIncrementalSearchOverlay hides the search overlay
func (fm *FileManager) HideIncrementalSearchOverlay() {
	if fm.searchOverlay != nil {
		fm.searchOverlay.Hide()
	}
}

// IsIncrementalSearchVisible returns whether the search overlay is visible
func (fm *FileManager) IsIncrementalSearchVisible() bool {
	return fm.searchOverlay != nil && fm.searchOverlay.IsVisible()
}

// AddSearchCharacter adds a character to the search term
func (fm *FileManager) AddSearchCharacter(char rune) {
	if fm.searchOverlay != nil {
		fm.searchOverlay.AddCharacter(char)
		// Update cursor to current match
		currentMatch := fm.searchOverlay.GetCurrentMatch()
		if currentMatch != nil {
			fm.SetCursorToFile(currentMatch)
		}
	}
}

// RemoveLastSearchCharacter removes the last character from search term
func (fm *FileManager) RemoveLastSearchCharacter() {
	if fm.searchOverlay != nil {
		fm.searchOverlay.RemoveLastCharacter()
		// Update cursor to current match
		currentMatch := fm.searchOverlay.GetCurrentMatch()
		if currentMatch != nil {
			fm.SetCursorToFile(currentMatch)
		}
	}
}

// NextSearchMatch moves to the next matching file
func (fm *FileManager) NextSearchMatch() {
	if fm.searchOverlay != nil {
		fm.searchOverlay.NextMatch()
		// Update cursor to current match
		currentMatch := fm.searchOverlay.GetCurrentMatch()
		if currentMatch != nil {
			fm.SetCursorToFile(currentMatch)
		}
	}
}

// PreviousSearchMatch moves to the previous matching file
func (fm *FileManager) PreviousSearchMatch() {
	if fm.searchOverlay != nil {
		fm.searchOverlay.PreviousMatch()
		// Update cursor to current match
		currentMatch := fm.searchOverlay.GetCurrentMatch()
		if currentMatch != nil {
			fm.SetCursorToFile(currentMatch)
		}
	}
}

// SelectCurrentSearchMatch selects the current search match
func (fm *FileManager) SelectCurrentSearchMatch() {
	if fm.searchOverlay != nil {
		fm.searchOverlay.SelectCurrentMatch()
	}
}

// GetCurrentSearchMatch returns the current search match
func (fm *FileManager) GetCurrentSearchMatch() *fileinfo.FileInfo {
	if fm.searchOverlay != nil {
		return fm.searchOverlay.GetCurrentMatch()
	}
	return nil
}

// OpenFile opens a file with the system default app or navigates into a directory.
func (fm *FileManager) OpenFile(file *fileinfo.FileInfo) {
	if file == nil {
		return
	}
	if file.IsDir {
		// Use the path provided in listing to handle parent (..) and SMB display paths correctly
		fm.LoadDirectory(file.Path)
		return
	}

	// Regular file: try to open with associated application
	if err := fileinfo.OpenWithDefaultApp(file.Path); err != nil {
		debugPrint("Failed to open file '%s': %v", file.Path, err)
		ui.ShowMessageDialog(fm.window, "ファイルを開けませんでした", err.Error())
		return
	}
}

// SetCursorToFile sets the cursor to the specified file
func (fm *FileManager) SetCursorToFile(file *fileinfo.FileInfo) {
	for i, f := range fm.files {
		if f.Name == file.Name {
			fm.SetCursorByIndex(i)
			fm.RefreshCursor()
			break
		}
	}
}

// sortFiles sorts the fm.files slice according to the configuration
func (fm *FileManager) sortFiles() {
	sortConfig := fm.config.UI.Sort

	debugPrint("Sorting files: sortBy=%s, order=%s, dirFirst=%t",
		sortConfig.SortBy, sortConfig.SortOrder, sortConfig.DirectoriesFirst)

	if len(fm.files) <= 1 {
		return // No need to sort 0 or 1 items
	}

	// If DirectoriesFirst is enabled, separate directories and files
	if sortConfig.DirectoriesFirst {
		var dirs []fileinfo.FileInfo
		var files []fileinfo.FileInfo

		for _, file := range fm.files {
			if file.Name == ".." {
				// Keep parent directory at the very top
				continue
			} else if file.IsDir {
				dirs = append(dirs, file)
			} else {
				files = append(files, file)
			}
		}

		// Sort directories and files separately
		fm.sortSlice(dirs, sortConfig)
		fm.sortSlice(files, sortConfig)

		// Rebuild the files slice: parent directory first, then sorted directories, then sorted files
		newFiles := []fileinfo.FileInfo{}
		for _, file := range fm.files {
			if file.Name == ".." {
				newFiles = append(newFiles, file)
				break
			}
		}
		newFiles = append(newFiles, dirs...)
		newFiles = append(newFiles, files...)

		fm.files = newFiles
	} else {
		// Sort all files together (except parent directory)
		var parentDir *fileinfo.FileInfo
		var regularFiles []fileinfo.FileInfo

		for _, file := range fm.files {
			if file.Name == ".." {
				parentDir = &file
			} else {
				regularFiles = append(regularFiles, file)
			}
		}

		fm.sortSlice(regularFiles, sortConfig)

		// Rebuild with parent directory first if it exists
		newFiles := []fileinfo.FileInfo{}
		if parentDir != nil {
			newFiles = append(newFiles, *parentDir)
		}
		newFiles = append(newFiles, regularFiles...)

		fm.files = newFiles
	}
}

// sortSlice sorts a slice of FileInfo according to the sort configuration
func (fm *FileManager) sortSlice(files []fileinfo.FileInfo, sortConfig config.SortConfig) {
	sort.Slice(files, func(i, j int) bool {
		fileI := files[i]
		fileJ := files[j]

		var less bool

		switch sortConfig.SortBy {
		case "name":
			less = strings.ToLower(fileI.Name) < strings.ToLower(fileJ.Name)
		case "size":
			less = fileI.Size < fileJ.Size
		case "modified":
			less = fileI.Modified.Before(fileJ.Modified)
		case "extension":
			extI := strings.ToLower(filepath.Ext(fileI.Name))
			extJ := strings.ToLower(filepath.Ext(fileJ.Name))
			// Files without extensions come first
			if extI == "" && extJ != "" {
				less = true
			} else if extI != "" && extJ == "" {
				less = false
			} else {
				less = extI < extJ
				// If extensions are the same, sort by name
				if extI == extJ {
					less = strings.ToLower(fileI.Name) < strings.ToLower(fileJ.Name)
				}
			}
		default:
			// Default to name sorting
			less = strings.ToLower(fileI.Name) < strings.ToLower(fileJ.Name)
		}

		// Apply sort order (asc/desc)
		if sortConfig.SortOrder == "desc" {
			less = !less
		}

		return less
	})
}

func main() {
	// Parse command line flags
	var startPath string
	flag.BoolVar(&debugMode, "d", false, "Enable debug mode")
	flag.StringVar(&startPath, "path", "", "Starting directory path")
	flag.Parse()

	// If no path specified via flag, check remaining arguments
	if startPath == "" && flag.NArg() > 0 {
		startPath = flag.Arg(0)
	}

	// If still no path, use current working directory
	if startPath == "" {
		pwd, err := os.Getwd()
		if err != nil {
			log.Fatalf("Error getting current directory: %v", err)
		}
		startPath = pwd
	} else {
		// Validate the path exists and is a directory
		if info, err := os.Stat(startPath); err != nil {
			log.Fatalf("Error accessing path '%s': %v", startPath, err)
		} else if !info.IsDir() {
			log.Fatalf("Path '%s' is not a directory", startPath)
		}

		// Convert to absolute path
		if absPath, err := filepath.Abs(startPath); err == nil {
			startPath = absPath
		}
	}

	// Load configuration
	configManager := config.NewManager(debugPrint)
	config, err := configManager.Load()
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	app := app.New()

	// Apply custom theme
	customTheme := customtheme.NewCustomTheme(config, debugPrint)
	app.Settings().SetTheme(customTheme)

	// Install debug logger for jobs package (prints only in -d mode)
	jobs.SetDebug(debugPrint)

	fm := NewFileManager(app, startPath, config, configManager, customTheme)
	fm.window.ShowAndRun()
}

// closeWindow handles window closing logic
func (fm *FileManager) closeWindow() {
	// Remove from registry
	windowRegistry.Delete(fm.window)
	remaining := atomic.AddInt32(&windowCount, -1)

	debugPrint("Window closed, remaining windows: %d", remaining)

	// Stop directory watcher
	if fm.dirWatcher != nil {
		fm.dirWatcher.Stop()
	}

	// Stop blinking indicator if active
	fm.stopJobsBlink()

	// Close the window
	fm.window.Close()

	// If this was the last window, quit the application
	if remaining == 0 {
		debugPrint("Last window closed, quitting application")
		fyne.CurrentApp().Quit()
	}
}

// QuitApplication handles application quit logic with confirmation dialog
func (fm *FileManager) QuitApplication() {
	currentCount := atomic.LoadInt32(&windowCount)
	debugPrint("QuitApplication called, current window count: %d", currentCount)

	if currentCount > 1 {
		// Multiple windows open, just close current window
		fm.closeWindow()
	} else {
		// Last window, show confirmation dialog
		fm.showQuitConfirmationDialog()
	}
}

// showQuitConfirmationDialog shows a confirmation dialog before quitting
func (fm *FileManager) showQuitConfirmationDialog() {
	dialog := ui.NewQuitConfirmDialog(fm.keyManager, debugPrint)
	dialog.ShowDialog(fm.window, func(confirmed bool) {
		if confirmed {
			debugPrint("User confirmed quit")
			fm.closeWindow()
		} else {
			debugPrint("User cancelled quit")
		}
		// Return focus to file list after dialog closes
		fm.FocusFileList()
	})
}
