package main

import (
	"path/filepath"
	"sort"
	"strings"

	"fyne.io/fyne/v2/widget"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
	"nmf/internal/ui"
)

// GetCurrentCursorIndex returns the current cursor index based on cursor path.
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

// SetCursorByIndex sets the cursor to the specified index.
func (fm *FileManager) SetCursorByIndex(index int) {
	if index >= 0 && index < len(fm.files) {
		fm.cursorPath = fm.files[index].Path
	} else {
		fm.cursorPath = ""
	}
}

// RefreshCursor updates only the cursor display without affecting selection.
func (fm *FileManager) RefreshCursor() {
	// First refresh the list to ensure all items are updated
	fm.fileList.Refresh()

	// Then scroll to cursor position after refresh completes
	cursorIdx := fm.GetCurrentCursorIndex()
	if cursorIdx >= 0 {
		fm.fileList.ScrollTo(widget.ListItemID(cursorIdx))
	}
}

// GetSelectedFiles returns the map of selected files.
func (fm *FileManager) GetSelectedFiles() map[string]bool {
	return fm.selectedFiles
}

// SetFileSelected sets the selection state of a file.
func (fm *FileManager) SetFileSelected(path string, selected bool) {
	fm.selectedFiles[path] = selected
}

// RefreshFileList refreshes the file list display.
func (fm *FileManager) RefreshFileList() {
	fm.fileList.Refresh()
}

// ShowFilterDialog displays the file filter dialog.
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

// ApplyFilter applies a filter to the current file list.
func (fm *FileManager) ApplyFilter(entry *config.FilterEntry) {
	if entry == nil || entry.Pattern == "" {
		fm.ClearFilter()
		return
	}

	// Validate pattern first
	if err := fileinfo.ValidatePattern(entry.Pattern); err != nil {
		debugPrint("FileManager: Invalid filter pattern '%s': %v", entry.Pattern, err)
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
		debugPrint("FileManager: Filter error: %v", err)
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

	debugPrint("FileManager: Applied filter: %s (matched %d/%d files)", entry.Pattern, len(fm.files), len(baseFiles))
}

// ClearFilter completely removes the current filter (for Ctrl+Shift+F).
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

	debugPrint("FileManager: Filter completely cleared, showing all %d files", len(fm.files))
}

// ToggleFilter toggles the current filter on/off.
func (fm *FileManager) ToggleFilter() {
	if fm.config.UI.FileFilter.Enabled && fm.currentFilter != nil {
		fm.DisableFilter()
	} else if fm.config.UI.FileFilter.Current != nil {
		fm.ApplyFilter(fm.config.UI.FileFilter.Current)
	}
}

// DisableFilter temporarily disables the current filter (for toggle functionality).
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

	debugPrint("FileManager: Filter disabled, showing all %d files", len(fm.files))
}

// saveFilterToHistory saves a filter entry to the history.
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
	if err := fm.configManager.SaveAsync(fm.config); err != nil {
		debugPrint("FileManager: Error saving filter history: %v", err)
	}
}

// ShowIncrementalSearchDialog shows the incremental search overlay.
func (fm *FileManager) ShowIncrementalSearchDialog() {
	debugPrint("FileManager: Starting incremental search mode")

	// Update overlay with current files
	fm.searchOverlay.UpdateFiles(fm.files)

	// Set up callbacks
	fm.searchOverlay.SetCallback(func(selectedFile *fileinfo.FileInfo) {
		// Navigate to selected file/directory
		if selectedFile.IsDir {
			// For directories, navigate into them
			targetPath := fileinfo.JoinPath(fm.currentPath, selectedFile.Name)
			debugPrint("FileManager: Incremental search navigating to directory %s", targetPath)
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
		debugPrint("FileManager: Incremental search cancelled")
		// Pop the search handler and refocus main view
		fm.keyManager.PopHandler()
		fm.FocusFileList()
	})

	// Push the search handler and show overlay
	fm.keyManager.PushHandler(fm.searchHandler)
	fm.searchOverlay.Show(fm.window)
}

// ShowSortDialog shows the sort configuration dialog.
func (fm *FileManager) ShowSortDialog() {
	debugPrint("FileManager: Showing sort dialog")

	// Get current sort configuration
	currentConfig := fm.config.UI.Sort

	// Create sort dialog
	sortDialog := ui.NewSortDialog(currentConfig, debugPrint)

	// Set up apply callback
	sortDialog.SetOnApply(func(sortConfig config.SortConfig) {
		debugPrint("FileManager: Applying sort configuration: %+v", sortConfig)

		// Store current cursor file name to restore position after sorting
		var currentFile string
		cursorIndex := fm.GetCurrentCursorIndex()
		if cursorIndex >= 0 && cursorIndex < len(fm.files) {
			currentFile = fm.files[cursorIndex].Name
			debugPrint("FileManager: Storing current cursor file: %s", currentFile)
		}

		// Update configuration
		fm.config.UI.Sort = sortConfig

		// Save configuration to file
		if err := fm.configManager.SaveAsync(fm.config); err != nil {
			debugPrint("FileManager: Failed to save sort configuration: %v", err)
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
					debugPrint("FileManager: Restored cursor to file: %s at index %d", currentFile, i)
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

		debugPrint("FileManager: Sort configuration applied successfully")
	})

	// Set up cancel callback
	sortDialog.SetOnCancel(func() {
		debugPrint("FileManager: Sort dialog cancelled")
	})

	// Set up cleanup callback (pop key handler)
	sortDialog.SetOnCleanup(func() {
		debugPrint("FileManager: Cleaning up sort dialog - popping key handler")
		fm.keyManager.PopHandler()
	})

	// Create and push keyboard handler
	handler := keymanager.NewSortDialogHandler(sortDialog, debugPrint)
	fm.keyManager.PushHandler(handler)

	// Show dialog
	sortDialog.Show(fm.window, handler)
}

// FocusPathEntry focuses the path entry widget.
func (fm *FileManager) FocusPathEntry() {
	debugPrint("FileManager: Focusing path entry")
	if fm.pathEntry != nil {
		fm.window.Canvas().Focus(fm.pathEntry)
		fm.pathEntry.FocusGained()
	}
}

// ShowIncrementalSearchOverlay shows the search overlay.
func (fm *FileManager) ShowIncrementalSearchOverlay() {
	fm.ShowIncrementalSearchDialog()
}

// HideIncrementalSearchOverlay hides the search overlay.
func (fm *FileManager) HideIncrementalSearchOverlay() {
	if fm.searchOverlay != nil {
		fm.searchOverlay.Hide()
	}
}

// IsIncrementalSearchVisible returns whether the search overlay is visible.
func (fm *FileManager) IsIncrementalSearchVisible() bool {
	return fm.searchOverlay != nil && fm.searchOverlay.IsVisible()
}

// AddSearchCharacter adds a character to the search term.
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

// RemoveLastSearchCharacter removes the last character from search term.
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

// NextSearchMatch moves to the next matching file.
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

// PreviousSearchMatch moves to the previous matching file.
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

// SelectCurrentSearchMatch selects the current search match.
func (fm *FileManager) SelectCurrentSearchMatch() {
	if fm.searchOverlay != nil {
		fm.searchOverlay.SelectCurrentMatch()
	}
}

// GetCurrentSearchMatch returns the current search match.
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
		debugPrint("FileManager: Failed to open file '%s': %v", file.Path, err)
		ui.ShowMessageDialog(fm.window, "ファイルを開けませんでした", err.Error())
		return
	}
}

// SetCursorToFile sets the cursor to the specified file.
func (fm *FileManager) SetCursorToFile(file *fileinfo.FileInfo) {
	for i, f := range fm.files {
		if f.Name == file.Name {
			fm.SetCursorByIndex(i)
			fm.RefreshCursor()
			break
		}
	}
}

// sortFiles sorts the fm.files slice according to the configuration.
func (fm *FileManager) sortFiles() {
	sortConfig := fm.config.UI.Sort

	debugPrint("FileManager: Sorting files: sortBy=%s, order=%s, dirFirst=%t",
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

// sortSlice sorts a slice of FileInfo according to the sort configuration.
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
