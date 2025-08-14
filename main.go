package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"
	"os"
	"path/filepath"
	"strings"
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
	window         fyne.Window
	currentPath    string
	files          []fileinfo.FileInfo
	fileList       *widget.List
	fileListView   *ui.KeySink
	pathEntry      *ui.TabEntry
	cursorPath     string          // Current cursor file path
	selectedFiles  map[string]bool // Set of selected file paths
	fileBinding    binding.UntypedList
	config         *config.Config
	configManager  *config.Manager
	cursorRenderer ui.CursorRenderer         // Cursor display renderer
	keyManager     *keymanager.KeyManager    // Keyboard input manager
	dirWatcher     *watcher.DirectoryWatcher // Directory change watcher
}

// Interface implementation for watcher.FileManager
func (fm *FileManager) GetCurrentPath() string {
	return fm.currentPath
}

func (fm *FileManager) GetFiles() []fileinfo.FileInfo {
	return fm.files
}

func (fm *FileManager) UpdateFiles(files []fileinfo.FileInfo) {
	fm.files = files

	// Update binding to reflect all changes (this auto-refreshes UI)
	items := make([]interface{}, len(fm.files))
	for i, file := range fm.files {
		items[i] = fileinfo.ListItem{
			Index:    i,
			FileInfo: file,
		}
	}
	fm.fileBinding.Set(items)
}

func (fm *FileManager) RemoveFromSelections(path string) {
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

func NewFileManager(app fyne.App, path string, config *config.Config, configManager *config.Manager) *FileManager {
	fm := &FileManager{
		window:         app.NewWindow("File Manager"),
		currentPath:    path,
		cursorPath:     "",
		selectedFiles:  make(map[string]bool),
		fileBinding:    binding.NewUntypedList(),
		config:         config,
		configManager:  configManager,
		cursorRenderer: ui.NewCursorRenderer(config.UI.CursorStyle),
		keyManager:     keymanager.NewKeyManager(debugPrint),
	}

	// Create directory watcher
	fm.dirWatcher = watcher.NewDirectoryWatcher(fm, debugPrint)

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
						textColor := fileinfo.GetTextColor(fileInfo.FileType, fm.config.UI.FileColors)

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
			statusBGColor := fileinfo.GetStatusBackgroundColor(fileInfo.Status)
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
				selectionBG := canvas.NewRectangle(color.RGBA{R: 100, G: 150, B: 200, A: 100})
				selectionBG.Resize(obj.Size())
				selectionBG.Move(fyne.NewPos(0, 0))
				// Wrap selection background in WithoutLayout container
				selectionContainer := container.NewWithoutLayout(selectionBG)
				outerContainer.Objects = append(outerContainer.Objects, selectionContainer)
			}

			// Add cursor if at cursor position (covers entire item like status/selection)
			if isCursor {
				cursor := fm.cursorRenderer.RenderCursor(obj.Size(), fyne.NewPos(0, 0), fm.config.UI.CursorStyle)

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
		if fm.fileListView != nil {
			fm.window.Canvas().Focus(fm.fileListView)
		}
		fm.RefreshCursor()
	}

	// Create toolbar
	toolbar := widget.NewToolbar(
		widget.NewToolbarAction(theme.NavigateBackIcon(), func() {
			parent := filepath.Dir(fm.currentPath)
			if parent != fm.currentPath {
				fm.LoadDirectory(parent)
			}
			if fm.fileListView != nil {
				fm.window.Canvas().Focus(fm.fileListView)
			}
		}),
		widget.NewToolbarAction(theme.HomeIcon(), func() {
			home, _ := os.UserHomeDir()
			fm.LoadDirectory(home)
			if fm.fileListView != nil {
				fm.window.Canvas().Focus(fm.fileListView)
			}
		}),
		widget.NewToolbarAction(theme.ViewRefreshIcon(), func() {
			fm.LoadDirectory(fm.currentPath)
			if fm.fileListView != nil {
				fm.window.Canvas().Focus(fm.fileListView)
			}
		}),
		widget.NewToolbarAction(theme.FolderIcon(), func() {
			fm.ShowDirectoryTreeDialog()
			// focus returns after dialog closes in callback
		}),
		widget.NewToolbarAction(theme.FolderNewIcon(), func() {
			fm.OpenNewWindow()
			if fm.fileListView != nil {
				fm.window.Canvas().Focus(fm.fileListView)
			}
		}),
	)

	// Layout without overlay
	content := container.NewBorder(
		container.NewVBox(toolbar, fm.pathEntry),
		nil, nil, nil,
		fm.fileListView,
	)

	fm.window.SetContent(content)
	fm.window.Resize(fyne.NewSize(float32(fm.config.Window.Width), float32(fm.config.Window.Height)))

	// Ensure initial focus sits on the tabbable list view
	if fm.fileListView != nil {
		fm.window.Canvas().Focus(fm.fileListView)
	}

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
			debugPrint("KeyDown")
			fm.keyManager.HandleKeyDown(ev)
		})

		dc.SetOnKeyUp(func(ev *fyne.KeyEvent) {
			debugPrint("KeyUp")
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

	// Remove focus from path entry after successful navigation
	fm.window.Canvas().Unfocus()
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
	newFM := NewFileManager(fyne.CurrentApp(), fm.currentPath, fm.config, fm.configManager)
	newFM.window.Show()
}

// ShowDirectoryTreeDialog shows the directory tree navigation dialog
func (fm *FileManager) ShowDirectoryTreeDialog() {
	dialog := ui.NewDirectoryTreeDialog(fm.currentPath, fm.keyManager, debugPrint)
	dialog.ShowDialog(fm.window, func(selectedPath string) {
		debugPrint("Directory selected from tree dialog: %s", selectedPath)
		fm.LoadDirectory(selectedPath)
		if fm.fileListView != nil {
			fm.window.Canvas().Focus(fm.fileListView)
		}
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
		if fm.fileListView != nil {
			fm.window.Canvas().Focus(fm.fileListView)
		}
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

	fm := NewFileManager(app, startPath, config, configManager)
	fm.window.ShowAndRun()
}
