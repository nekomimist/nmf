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
	"nmf/internal/keymanager"
	customtheme "nmf/internal/theme"
	"nmf/internal/ui"
	"nmf/internal/watcher"
)

// Global debug flag
var debugMode bool

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

	// Create directory watcher
	fm.dirWatcher = watcher.NewDirectoryWatcher(fm, debugPrint)

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

						// Set icon resource
						if fileInfo.IsDir {
							icon.SetResource(theme.FolderIcon())
						} else {
							icon.SetResource(theme.FileIcon())
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

	// Create toolbar
	toolbar := widget.NewToolbar(
		widget.NewToolbarAction(theme.NavigateBackIcon(), func() {
			parent := filepath.Dir(fm.currentPath)
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

	// Layout with search overlay
	mainContent := container.NewBorder(
		container.NewVBox(toolbar, fm.pathEntry),
		nil, nil, nil,
		fm.fileListView,
	)

	// Stack main content with search overlay on top
	content := container.NewMax(
		mainContent,
		container.NewBorder(
			fm.searchOverlay.GetContainer(), // Top overlay
			nil, nil, nil,
			nil, // Center is empty, overlay is at top
		),
	)

	fm.window.SetContent(content)
	fm.window.Resize(fyne.NewSize(float32(fm.config.Window.Width), float32(fm.config.Window.Height)))

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

	// Add current path to navigation history before changing directory
	if fm.currentPath != "" && fm.currentPath != path {
		fm.config.AddToNavigationHistory(fm.currentPath)
		// Save config to persist navigation history
		if err := fm.configManager.Save(fm.config); err != nil {
			debugPrint("Error saving navigation history: %v", err)
		}
	}

	// Stop current directory watcher if running
	if fm.dirWatcher != nil {
		fm.dirWatcher.Stop()
	}

	// Store the previous directory for parent navigation logic
	previousPath := fm.currentPath

	fm.currentPath = path
	fm.pathEntry.SetText(path)
	fm.files = []fileinfo.FileInfo{}

	// Read directory
	entries, err := os.ReadDir(path)
	if err != nil {
		log.Printf("Error reading directory: %v", err)
		return
	}

	// Convert to ListItem (FileInfo with index)
	items := make([]interface{}, 0, len(entries)+1)
	index := 0

	// Add parent directory entry if not at root
	parent := filepath.Dir(path)
	if parent != path {
		parentInfo := fileinfo.FileInfo{
			Name:     "..",
			Path:     parent,
			IsDir:    true,
			Size:     0,
			Modified: time.Time{},
			FileType: fileinfo.FileTypeDirectory, // Parent directory is always a directory
			Status:   fileinfo.StatusNormal,
		}

		listItem := fileinfo.ListItem{
			Index:    index,
			FileInfo: parentInfo,
		}

		fm.files = append(fm.files, parentInfo)
		items = append(items, listItem)
		index++
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		fullPath := filepath.Join(path, entry.Name())
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

		listItem := fileinfo.ListItem{
			Index:    index,
			FileInfo: fileInfo,
		}

		fm.files = append(fm.files, fileInfo)
		items = append(items, listItem)
		index++
	}

	// Sort files according to configuration
	fm.sortFiles()

	// Rebuild items after sorting
	items = make([]interface{}, 0, len(fm.files))
	for i, file := range fm.files {
		listItem := fileinfo.ListItem{
			Index:    i,
			FileInfo: file,
		}
		items = append(items, listItem)
	}

	// Update binding
	fm.fileBinding.Set(items)

	// Clear selections when changing directory
	fm.selectedFiles = make(map[string]bool)

	// Restore cursor position or set default
	if len(fm.files) > 0 {
		// Check if we're navigating to parent directory
		parent := filepath.Dir(previousPath)
		if parent == path && previousPath != "" {
			// We're going to parent directory, try to position cursor on the directory we came from
			dirName := filepath.Base(previousPath)
			cursorSet := false
			for i, file := range fm.files {
				if file.Name == dirName {
					fm.SetCursorByIndex(i)
					cursorSet = true
					break
				}
			}
			if !cursorSet {
				fm.SetCursorByIndex(0)
			}
		} else {
			// Try to restore saved cursor position
			savedFileName := fm.restoreCursorPosition(path)
			cursorSet := false
			if savedFileName != "" {
				for i, file := range fm.files {
					if file.Name == savedFileName {
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
		// Refresh cursor display immediately
		fm.RefreshCursor()
	} else {
		fm.cursorPath = ""
	}

	// Restart directory watcher for new path
	if fm.dirWatcher != nil {
		fm.dirWatcher.Start()
	}
}

func (fm *FileManager) OpenNewWindow() {
	newFM := NewFileManager(fyne.CurrentApp(), fm.currentPath, fm.config, fm.configManager, fm.customTheme)
	newFM.window.Show()
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
	if len(historyPaths) == 0 {
		debugPrint("No navigation history available")
		return
	}

	dialog := ui.NewNavigationHistoryDialog(
		historyPaths,
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
			targetPath := filepath.Join(fm.currentPath, selectedFile.Name)
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

// OpenFile opens a file or navigates to directory
func (fm *FileManager) OpenFile(file *fileinfo.FileInfo) {
	if file.IsDir {
		targetPath := filepath.Join(fm.currentPath, file.Name)
		fm.LoadDirectory(targetPath)
	}
	// For regular files, we don't open them, just set cursor
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
	configManager := config.NewManager()
	config, err := configManager.Load()
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	app := app.New()

	// Apply custom theme
	customTheme := customtheme.NewCustomTheme(config)
	app.Settings().SetTheme(customTheme)

	fm := NewFileManager(app, startPath, config, configManager, customTheme)
	fm.window.ShowAndRun()
}
