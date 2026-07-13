package main

import (
	"cmp"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"fyne.io/fyne/v2/widget"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
	"nmf/internal/ui"
)

// GetCurrentCursorIndex returns the current cursor index based on cursor path.
// cursorIndex is a cache validated against cursorPath on every call: a hit
// avoids the linear scan, and any direct cursorPath assignment elsewhere
// self-heals here (cache miss falls back to scanning and re-caches).
func (fm *FileManager) GetCurrentCursorIndex() int {
	if fm.cursorPath == "" {
		return -1
	}
	if fm.cursorIndex >= 0 && fm.cursorIndex < len(fm.files) && fm.files[fm.cursorIndex].Path == fm.cursorPath {
		return fm.cursorIndex
	}
	for i, file := range fm.files {
		if file.Path == fm.cursorPath {
			fm.cursorIndex = i
			return i
		}
	}
	fm.cursorIndex = -1
	return -1
}

// SetCursorByIndex sets the cursor to the specified index.
func (fm *FileManager) SetCursorByIndex(index int) {
	beforeIndex := fm.GetCurrentCursorIndex()
	beforePath := fm.cursorPath
	if index >= 0 && index < len(fm.files) {
		fm.cursorPath = fm.files[index].Path
		fm.cursorIndex = index
	} else {
		fm.cursorPath = ""
		fm.cursorIndex = -1
	}
	debugPrint("FileManager: cursor set requested=%d before=%d after=%d changed=%t count=%d active=%t focused=%s path=%q cursor=%q",
		index, beforeIndex, fm.cursorIndex, beforeIndex != fm.cursorIndex || beforePath != fm.cursorPath,
		len(fm.files), fm.windowActive, focusedObjectLabel(fm.window), fm.currentPath, fm.cursorPath)
}

// RefreshCursor updates only the cursor display without affecting selection.
// Only safe when fm.files content has NOT changed since the last refresh;
// after a content change use refreshListAndCursor instead (see its comment).
func (fm *FileManager) RefreshCursor() {
	seq, cursorIdx := fm.beginCursorRefresh("cursor")
	if cursorIdx < 0 {
		// No cursor: refresh to clear any stale cursor decoration.
		fm.fileList.Refresh()
		fm.endCursorRefresh(seq, "cursor", cursorIdx)
		return
	}
	// Fyne v2.7.3 List.ScrollTo unconditionally ends with a full Refresh()
	// (widget/list.go:246-257), so an explicit Refresh here would double the
	// per-keypress render cost. Re-verify on Fyne upgrades.
	fm.fileList.ScrollTo(widget.ListItemID(cursorIdx))
	fm.endCursorRefresh(seq, "cursor", cursorIdx)
}

// refreshListAndCursor refreshes the list after fm.files was replaced, then
// scrolls to the cursor. The leading Refresh is load-bearing, not redundant:
// ScrollTo clamps its offset against the scroller's *current* content size,
// which only updates during a refresh/layout pass. In Fyne v2.7.3, scrolling
// first leaves the stale content size at or below the viewport, so
// internal/widget/scroller.go updateOffset silently resets the scroll offset
// to zero while the list keeps laying rows out for the requested offset —
// every row lands outside the viewport and the list looks empty until the
// next full relayout (observed on Windows with a restored cursor beyond the
// first viewport). Costs one extra viewport render per structural change;
// cursor-only moves must keep using RefreshCursor.
func (fm *FileManager) refreshListAndCursor() {
	seq, cursorIdx := fm.beginCursorRefresh("list")
	fm.fileList.Refresh()
	if cursorIdx >= 0 {
		fm.fileList.ScrollTo(widget.ListItemID(cursorIdx))
	}
	fm.endCursorRefresh(seq, "list", cursorIdx)
}

// beginCursorRefresh starts a diagnostic sequence without changing refresh
// behavior. cursorItemUpdateSeq is acknowledged from the List UpdateItem
// callback, which distinguishes list-row updates from later canvas painting.
func (fm *FileManager) beginCursorRefresh(route string) (uint64, int) {
	fm.cursorRefreshSeq++
	seq := fm.cursorRefreshSeq
	cursorIdx := fm.GetCurrentCursorIndex()
	listWidth, listHeight := float32(0), float32(0)
	if fm.fileList != nil {
		listWidth = fm.fileList.Size().Width
		listHeight = fm.fileList.Size().Height
	}
	debugPrint("FileManager: cursor refresh start route=%s seq=%d index=%d count=%d itemUpdateSeq=%d active=%t focused=%s list=%.0fx%.0f path=%q",
		route, seq, cursorIdx, len(fm.files), fm.cursorItemUpdateSeq, fm.windowActive,
		focusedObjectLabel(fm.window), listWidth, listHeight, fm.currentPath)
	return seq, cursorIdx
}

func (fm *FileManager) endCursorRefresh(seq uint64, route string, cursorIdx int) {
	debugPrint("FileManager: cursor refresh done route=%s seq=%d index=%d itemUpdated=%t itemUpdateSeq=%d path=%q",
		route, seq, cursorIdx, fm.cursorItemUpdateSeq >= seq, fm.cursorItemUpdateSeq, fm.currentPath)
}

// noteCursorItemUpdated records that UpdateItem rebuilt the cursor row for the
// current refresh sequence. It does not imply that the GL frame was presented.
func (fm *FileManager) noteCursorItemUpdated(index int) {
	if index != fm.GetCurrentCursorIndex() || fm.cursorRefreshSeq == 0 || fm.cursorItemUpdateSeq >= fm.cursorRefreshSeq {
		return
	}
	fm.cursorItemUpdateSeq = fm.cursorRefreshSeq
	debugPrint("FileManager: cursor item updated seq=%d index=%d active=%t focused=%s path=%q cursor=%q",
		fm.cursorItemUpdateSeq, index, fm.windowActive, focusedObjectLabel(fm.window), fm.currentPath, fm.cursorPath)
}

// FileCount returns the number of files in the current listing without copying it.
func (fm *FileManager) FileCount() int { return len(fm.files) }

// FileAt returns the file at index without copying the whole listing.
func (fm *FileManager) FileAt(index int) (fileinfo.FileInfo, bool) {
	if index < 0 || index >= len(fm.files) {
		return fileinfo.FileInfo{}, false
	}
	return fm.files[index], true
}

// GetSelectedFiles returns the map of selected files.
func (fm *FileManager) GetSelectedFiles() map[string]bool {
	return fm.selectedFiles
}

// SetFileSelected sets the selection state of a file.
func (fm *FileManager) SetFileSelected(path string, selected bool) {
	fm.selectedFiles[path] = selected
	fm.updateStatusBar()
}

// RefreshFileList refreshes the file list display.
func (fm *FileManager) RefreshFileList() {
	fm.fileList.Refresh()
	fm.updateStatusBar()
}

// CurrentSort returns the sort configuration currently used for the visible list.
func (fm *FileManager) CurrentSort() config.SortConfig {
	if fm.activeSort.SortBy == "" || fm.activeSort.SortOrder == "" {
		if fm.config == nil {
			return config.SortConfig{SortBy: "name", SortOrder: "asc", DirectoriesFirst: true}
		}
		return fm.state.EffectiveSort(fm.config.UI.Sort)
	}
	return fm.activeSort
}

// ApplyTemporarySort applies a sort configuration without changing persisted settings.
func (fm *FileManager) ApplyTemporarySort(sortConfig config.SortConfig) {
	debugPrint("FileManager: Applying temporary sort configuration: %+v", sortConfig)
	fm.applySort(sortConfig)
}

// ShowFilterDialog displays the file filter dialog.
func (fm *FileManager) ShowFilterDialog() {
	// Get current filter entries from state
	entries := fm.state.GetFileFilterEntries()

	// Use originalFiles if available, otherwise use current files
	currentFiles := fm.originalFiles
	if len(currentFiles) == 0 {
		currentFiles = fm.files
	}

	filterDialog := ui.NewFilterDialog(entries, currentFiles, fm.keyManager, debugPrint, fm.searchMatchers)
	filterDialog.ShowDialog(fm.window, func(selectedEntry *config.FilterEntry) {
		if selectedEntry != nil {
			debugPrint("FileManager: filter dialog selected pattern=%s focused=%s", selectedEntry.Pattern, focusedObjectLabel(fm.window))
			fm.ApplyFilter(selectedEntry)
			fm.saveFilterToHistory(selectedEntry)
		}
		fm.focusFileList("filter-dialog-closed")
	}, func(pattern string) {
		if fm.state.RemoveFileFilterEntry(pattern) && fm.stateManager != nil {
			if err := fm.stateManager.SaveAsync(fm.state); err != nil {
				debugPrint("FileManager: Error saving filter history deletion: %v", err)
			}
		}
	})
}

// ApplyFilter applies a filter to the current file list.
func (fm *FileManager) ApplyFilter(entry *config.FilterEntry) {
	if entry == nil || config.EffectiveFilterPattern(entry.Pattern) == "" {
		fm.ClearFilter()
		return
	}
	effectivePattern := config.EffectiveFilterPattern(entry.Pattern)

	// Validate pattern first
	if err := fileinfo.ValidatePattern(effectivePattern); err != nil {
		debugPrint("FileManager: Invalid filter pattern '%s': %v", effectivePattern, err)
		return
	}

	fm.currentFilter = entry
	fm.state.FileFilter.Current = entry
	fm.state.FileFilter.Enabled = true
	if fm.stateManager != nil {
		if err := fm.stateManager.SaveAsync(fm.state); err != nil {
			debugPrint("FileManager: Error saving applied filter state: %v", err)
		}
	}

	// Use originalFiles if available, otherwise use current files as base
	baseFiles := fm.originalFiles
	if len(baseFiles) == 0 {
		baseFiles = fm.files
		fm.originalFiles = make([]fileinfo.FileInfo, len(fm.files))
		copy(fm.originalFiles, fm.files)
	}

	// Apply filter
	filtered, err := fileinfo.FilterFiles(baseFiles, effectivePattern)
	if err != nil {
		debugPrint("FileManager: Filter error: %v", err)
		return
	}

	fm.files = filtered
	fm.sortFilesWithConfig(fm.CurrentSort())
	fm.updateStatusBar()

	// Reset cursor to first item if available; refreshListAndCursor handles
	// both the cursor scroll and the no-match case (content was replaced).
	// cursorPath is deliberately left as-is when nothing matches so the
	// cursor can reappear when the filter is cleared.
	if len(fm.files) > 0 {
		fm.SetCursorByIndex(0)
	}
	fm.refreshListAndCursor()

	debugPrint("FileManager: Applied filter: %s (effective=%s matched %d/%d files)", entry.Pattern, effectivePattern, len(fm.files), len(baseFiles))
}

// ClearFilter completely removes the current filter (for Ctrl+Shift+F).
func (fm *FileManager) ClearFilter() {
	fm.currentFilter = nil
	fm.state.FileFilter.Current = nil // Complete clear
	fm.state.FileFilter.Enabled = false
	if fm.stateManager != nil {
		if err := fm.stateManager.SaveAsync(fm.state); err != nil {
			debugPrint("FileManager: Error saving cleared filter state: %v", err)
		}
	}

	if len(fm.originalFiles) > 0 {
		fm.files = fm.originalFiles
		fm.sortFilesWithConfig(fm.CurrentSort())

		// Update UI
		fm.fileList.Refresh()
		fm.updateStatusBar()
	}

	debugPrint("FileManager: Filter completely cleared, showing all %d files", len(fm.files))
}

// ToggleFilter toggles the current filter on/off.
func (fm *FileManager) ToggleFilter() {
	if fm.state.FileFilter.Enabled && fm.currentFilter != nil {
		fm.DisableFilter()
	} else if fm.state.FileFilter.Current != nil {
		fm.ApplyFilter(fm.state.FileFilter.Current)
	}
}

// DisableFilter temporarily disables the current filter (for toggle functionality).
func (fm *FileManager) DisableFilter() {
	fm.currentFilter = nil
	// Keep fm.state.FileFilter.Current for toggle functionality
	fm.state.FileFilter.Enabled = false
	if fm.stateManager != nil {
		if err := fm.stateManager.SaveAsync(fm.state); err != nil {
			debugPrint("FileManager: Error saving disabled filter state: %v", err)
		}
	}

	if len(fm.originalFiles) > 0 {
		fm.files = fm.originalFiles
		fm.sortFilesWithConfig(fm.CurrentSort())

		// Update UI
		fm.fileList.Refresh()
		fm.updateStatusBar()
	}

	debugPrint("FileManager: Filter disabled, showing all %d files", len(fm.files))
}

// saveFilterToHistory saves a filter entry to the history.
func (fm *FileManager) saveFilterToHistory(entry *config.FilterEntry) {
	if entry == nil || entry.Pattern == "" || config.EffectiveFilterPattern(entry.Pattern) == "" {
		return
	}

	fm.state.AddToFileFilterHistory(entry, fm.config.UI.FileFilter.MaxEntries)

	// Save state to disk
	if fm.stateManager != nil {
		if err := fm.stateManager.SaveAsync(fm.state); err != nil {
			debugPrint("FileManager: Error saving filter history: %v", err)
		}
	}
}

// ShowIncrementalSearchDialog shows the incremental search overlay.
func (fm *FileManager) ShowIncrementalSearchDialog() {
	debugPrint("FileManager: Starting incremental search mode")

	// Update overlay with current files
	fm.searchOverlay.UpdateFiles(fm.files)

	// Set up cancellation callback. Accept is handled by IncrementalSearchKeyHandler.
	fm.searchOverlay.SetCancelCallback(func() {
		debugPrint("FileManager: Incremental search cancelled")
		// Pop the search handler and refocus main view
		fm.keyManager.BeginOwnerTransition("search.cancelCallback", func() {
			fm.keyManager.RemoveHandler(fm.searchToken)
			fm.FocusFileList()
		})
	})

	// Push the search handler and show overlay
	fm.searchToken = fm.keyManager.PushHandler(fm.searchHandler)
	fm.searchOverlay.Show(fm.window)
}

// ShowSortDialog shows the sort configuration dialog.
func (fm *FileManager) ShowSortDialog() {
	debugPrint("FileManager: Showing sort dialog")

	// Get current sort configuration
	currentConfig := fm.state.EffectiveSort(fm.config.UI.Sort)

	// Create sort dialog
	sortDialog := ui.NewSortDialog(currentConfig, fm.keyManager, debugPrint)

	// Set up apply callback
	sortDialog.SetOnApply(func(sortConfig config.SortConfig) {
		debugPrint("FileManager: Applying sort configuration: %+v", sortConfig)

		// Persist the applied sort as a state override
		appliedSort := sortConfig
		fm.state.Sort = &appliedSort

		// Save state to file
		if fm.stateManager != nil {
			if err := fm.stateManager.SaveAsync(fm.state); err != nil {
				debugPrint("FileManager: Failed to save sort state: %v", err)
			}
		}

		fm.applySort(sortConfig)

		debugPrint("FileManager: Sort configuration applied successfully")
	})

	// Set up cancel callback
	sortDialog.SetOnCancel(func() {
		debugPrint("FileManager: Sort dialog cancelled")
	})

	// Set up cleanup callback (remove key handler)
	var sortToken keymanager.HandlerToken
	sortDialog.SetOnCleanup(func() {
		debugPrint("FileManager: Cleaning up sort dialog - removing key handler")
		fm.keyManager.BeginOwnerTransition("sort.close", func() {
			fm.keyManager.RemoveHandler(sortToken)
		})
	})

	// Create and push keyboard handler
	handler := keymanager.NewSortDialogHandler(sortDialog, debugPrint)
	sortToken = fm.keyManager.PushHandler(handler)

	// Show dialog
	sortDialog.Show(fm.window, fm.keyManager)
}

// ShowPathEditDialog opens the path edit dialog.
func (fm *FileManager) ShowPathEditDialog() {
	debugPrint("FileManager: Opening path edit dialog")
	dlg := ui.NewLineEditDialog(ui.LineEditDialogOptions{
		Title:       "Edit Path",
		Prompt:      "Path:",
		InitialText: fm.currentPath,
		InitialSelection: &ui.LineEditSelection{
			Start: 0,
			End:   utf8.RuneCountInString(fm.currentPath),
		},
		ConfirmText: "Open",
		Width:       760,
	}, fm.keyManager, fm.config.UI.KeyBindings)
	dlg.ShowDialog(fm.window, func(path string) bool {
		debugPrint("FileManager: path edit accepted input=%s focused=%s", path, focusedObjectLabel(fm.window))
		return fm.navigateToPath(path)
	})
}

func (fm *FileManager) setPathDisplay(path string) {
	if fm.pathDisplay != nil {
		fm.pathDisplay.SetText(path)
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

// AcceptIncrementalSearchOverlay hides the search overlay after accepting a match.
func (fm *FileManager) AcceptIncrementalSearchOverlay() {
	if fm.searchOverlay != nil {
		fm.searchOverlay.HideAccepted()
	}
	fm.keyManager.BeginOwnerTransition("search.acceptCallback", func() {
		fm.keyManager.RemoveHandler(fm.searchToken)
		fm.FocusFileList()
	})
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
	if fileinfo.IsSupportedArchive(file.Path) {
		fm.LoadDirectory(fileinfo.ArchiveRootPath(file.Path))
		return
	}
	if dir, ok, err := fileinfo.ResolveShortcutNavigationDir(file.Path); ok {
		fm.LoadDirectory(dir)
		return
	} else if err != nil {
		debugPrint("FileManager: Windows shortcut navigation skipped path=%s err=%v", file.Path, err)
	}

	// Regular file: try to open with associated application
	if err := fileinfo.OpenWithDefaultApp(file.Path); err != nil {
		debugPrint("FileManager: Failed to open file '%s': %v", file.Path, err)
		fm.resetKeyStateAfterExternalOpen("open-file-error")
		fm.ShowMessageDialog("ファイルを開けませんでした", err.Error())
		return
	}
}

// OpenFileDefaultApp opens a file with the system default app, or navigates into a directory.
func (fm *FileManager) OpenFileDefaultApp(file *fileinfo.FileInfo) {
	if file == nil {
		return
	}
	if file.IsDir {
		fm.LoadDirectory(file.Path)
		return
	}
	if err := fileinfo.OpenWithDefaultApp(file.Path); err != nil {
		debugPrint("FileManager: Failed to open file with default app '%s': %v", file.Path, err)
		fm.resetKeyStateAfterExternalOpen("open-default-app-error")
		fm.ShowMessageDialog("ファイルを開けませんでした", err.Error())
		return
	}
}

func (fm *FileManager) resetKeyStateAfterExternalOpen(label string) {
	if fm == nil || fm.keyManager == nil {
		return
	}
	fm.keyManager.ResetTransientState(label)
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

func (fm *FileManager) sortFilesWithConfig(sortConfig config.SortConfig) {
	debugPrint("FileManager: Sorting files: sortBy=%s, order=%s, dirFirst=%t",
		sortConfig.SortBy, sortConfig.SortOrder, sortConfig.DirectoriesFirst)

	fm.files = sortFileInfoSlice(fm.files, sortConfig)
}

// sortFileInfoSlice returns files reordered per sortConfig. It pins ".." at
// index 0 (if present) and, when DirectoriesFirst is set, sorts directories
// and regular files as separate groups; otherwise sorts everything but the
// parent entry together. It touches no FileManager state, so it is safe to
// call from a background goroutine (see loadDirectoryAsync).
func sortFileInfoSlice(files []fileinfo.FileInfo, sortConfig config.SortConfig) []fileinfo.FileInfo {
	if len(files) <= 1 {
		return files // No need to sort 0 or 1 items
	}

	// If DirectoriesFirst is enabled, separate directories and files
	if sortConfig.DirectoriesFirst {
		var dirs []fileinfo.FileInfo
		var regularFiles []fileinfo.FileInfo

		for _, file := range files {
			if file.Name == ".." {
				// Keep parent directory at the very top
				continue
			} else if file.IsDir {
				dirs = append(dirs, file)
			} else {
				regularFiles = append(regularFiles, file)
			}
		}

		// Sort directories and files separately
		sortSlice(dirs, sortConfig)
		sortSlice(regularFiles, sortConfig)

		// Rebuild the files slice: parent directory first, then sorted directories, then sorted files
		newFiles := make([]fileinfo.FileInfo, 0, len(files))
		for _, file := range files {
			if file.Name == ".." {
				newFiles = append(newFiles, file)
				break
			}
		}
		newFiles = append(newFiles, dirs...)
		newFiles = append(newFiles, regularFiles...)

		return newFiles
	}

	// Sort all files together (except parent directory)
	var parentDir *fileinfo.FileInfo
	var regularFiles []fileinfo.FileInfo

	for _, file := range files {
		if file.Name == ".." {
			parentDir = &file
		} else {
			regularFiles = append(regularFiles, file)
		}
	}

	sortSlice(regularFiles, sortConfig)

	// Rebuild with parent directory first if it exists
	newFiles := make([]fileinfo.FileInfo, 0, len(files))
	if parentDir != nil {
		newFiles = append(newFiles, *parentDir)
	}
	newFiles = append(newFiles, regularFiles...)

	return newFiles
}

func (fm *FileManager) applySort(sortConfig config.SortConfig) {
	currentPath := fm.cursorPath
	fm.activeSort = sortConfig

	fm.sortFilesWithConfig(sortConfig)

	if currentPath != "" {
		fm.cursorPath = currentPath
		if fm.GetCurrentCursorIndex() >= 0 {
			debugPrint("FileManager: Restored cursor to path: %s", currentPath)
		} else {
			fm.SetCursorByIndex(0)
		}
	} else if len(fm.files) > 0 {
		fm.SetCursorByIndex(0)
	}

	fm.RefreshCursor()
}

// sortKey precomputes the lowercase comparison keys for a file so sortSlice
// avoids recomputing strings.ToLower/filepath.Ext on every comparison.
type sortKey struct {
	file      fileinfo.FileInfo
	lowerName string
	lowerExt  string
}

// sortSlice sorts a slice of FileInfo according to the sort configuration.
// It decorates each entry with precomputed comparison keys, sorts the
// decorated slice once, then writes the reordered files back. It touches no
// FileManager state, so it is safe to call from a background goroutine.
func sortSlice(files []fileinfo.FileInfo, sortConfig config.SortConfig) {
	if len(files) <= 1 {
		return
	}

	keys := make([]sortKey, len(files))
	for i, file := range files {
		k := sortKey{file: file, lowerName: strings.ToLower(file.Name)}
		if sortConfig.SortBy == "extension" {
			k.lowerExt = strings.ToLower(filepath.Ext(file.Name))
		}
		keys[i] = k
	}

	desc := sortConfig.SortOrder == "desc"

	slices.SortFunc(keys, func(a, b sortKey) int {
		var c int
		switch sortConfig.SortBy {
		case "size":
			c = cmp.Compare(a.file.Size, b.file.Size)
		case "modified":
			c = a.file.Modified.Compare(b.file.Modified)
		case "extension":
			// Files without extensions come first
			switch {
			case a.lowerExt == "" && b.lowerExt != "":
				c = -1
			case a.lowerExt != "" && b.lowerExt == "":
				c = 1
			default:
				c = cmp.Compare(a.lowerExt, b.lowerExt)
				// If extensions are the same, sort by name
				if c == 0 {
					c = cmp.Compare(a.lowerName, b.lowerName)
				}
			}
		default:
			// "name" and unknown SortBy default to name sorting
			c = cmp.Compare(a.lowerName, b.lowerName)
		}

		if desc {
			c = -c
		}
		return c
	})

	for i, k := range keys {
		files[i] = k.file
	}
}
