package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"fyne.io/fyne/v2"

	"nmf/internal/browser"
	"nmf/internal/config"
	"nmf/internal/fileinfo"
)

// SaveCursorPosition saves the current cursor position for the given directory.
func (fm *FileManager) SaveCursorPosition(dirPath string) {
	currentIdx := fm.GetCurrentCursorIndex()
	file, ok := fm.FileAt(currentIdx)
	if !ok {
		return
	}

	fileName := file.Name
	cursorMemory := &fm.state.CursorMemory
	maxEntries := fm.config.UI.CursorMemory.MaxEntries

	// Clean up old entries if we exceed max entries
	if len(cursorMemory.Entries) >= maxEntries {
		fm.cleanupOldCursorEntries()
	}

	// Save the cursor position and update last used time
	cursorMemory.Entries[dirPath] = fileName
	cursorMemory.LastUsed[dirPath] = time.Now()

	// Save state to disk
	if fm.stateManager != nil {
		if err := fm.stateManager.SaveAsync(fm.state); err != nil {
			debugPrint("FileManager: Error saving cursor position state: %v", err)
		}
	}
}

// restoreCursorPosition restores the cursor position for the given directory.
func (fm *FileManager) restoreCursorPosition(dirPath string) string {
	cursorMemory := &fm.state.CursorMemory

	fileName, exists := cursorMemory.Entries[dirPath]
	if !exists {
		return ""
	}

	// Update last used time
	cursorMemory.LastUsed[dirPath] = time.Now()

	return fileName
}

// cleanupOldCursorEntries removes the oldest entries when maxEntries is exceeded.
func (fm *FileManager) cleanupOldCursorEntries() {
	cursorMemory := &fm.state.CursorMemory
	maxEntries := fm.config.UI.CursorMemory.MaxEntries

	if len(cursorMemory.Entries) < maxEntries {
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

// navigateToPath handles path edit validation and navigation.
func (fm *FileManager) navigateToPath(inputPath string) bool {
	// Trim whitespace from input
	path := strings.TrimSpace(inputPath)

	// Handle empty path - do nothing
	if path == "" {
		fm.setPathDisplay(fm.GetCurrentPath())
		return false
	}

	// Handle tilde expansion for home directory
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			debugPrint("FileManager: Error getting home directory: %v", err)
			fm.setPathDisplay(fm.GetCurrentPath())
			return false
		}
		path = strings.Replace(path, "~", home, 1)
	}

	resolvedPath, parsed, err := fileinfo.CanonicalDisplayPath(path)
	if err != nil {
		debugPrint("FileManager: Invalid path '%s': %v", inputPath, err)
		fm.setPathDisplay(fm.GetCurrentPath())
		return false
	}

	// Seed credential cache if URL contained creds.
	if parsed.Scheme == fileinfo.SchemeSMB && (parsed.User != "" || parsed.Password != "" || parsed.Domain != "") {
		fileinfo.PutCachedCredentials(parsed.Host, parsed.Share, fileinfo.Credentials{
			Domain:   parsed.Domain,
			Username: parsed.User,
			Password: parsed.Password,
		})
	}

	// Accessibility is checked by the asynchronous directory load. Path edits
	// may recover from a missing target by opening its nearest accessible parent.
	fm.loadDirectoryWithParentFallback(resolvedPath)

	// Return focus to file list after successful navigation
	fm.focusFileList("path-edit-navigation")
	return true
}

// FocusFileList sets focus to the file list view.
func (fm *FileManager) FocusFileList() {
	fm.focusFileList("unspecified")
}

func (fm *FileManager) focusFileList(reason string) {
	currentPath := fm.GetCurrentPath()
	busy := fm.busy != nil && fm.busy.Active()
	if fm.fileListView != nil {
		before := focusedObjectLabel(fm.window)
		debugPrint("FileManager: FocusFileList start reason=%s focused=%s active=%t busy=%t path=%s", reason, before, fm.windowActive, busy, currentPath)
		fm.window.Canvas().Focus(fm.fileListView)
		fm.setWindowActive(true)
		debugPrint("FileManager: FocusFileList done reason=%s focused=%s active=%t busy=%t path=%s", reason, focusedObjectLabel(fm.window), fm.windowActive, busy, currentPath)
		return
	}
	debugPrint("FileManager: FocusFileList skipped reason=%s fileListView=nil path=%s", reason, currentPath)
}

// LoadDirectory opens path without recovering from a missing destination. Most
// navigation sources retain this behavior so an unexpected directory failure
// is reported directly to the user.
func (fm *FileManager) LoadDirectory(path string) {
	fm.loadDirectory(path, false)
}

// loadDirectoryWithParentFallback opens path and, only when it does not exist,
// tries its parents until it finds one that can be listed. It is deliberately
// limited to user-entered paths and navigation-history selections.
func (fm *FileManager) loadDirectoryWithParentFallback(path string) {
	fm.loadDirectory(path, true)
}

func (fm *FileManager) loadDirectory(path string, allowParentFallback bool) {
	path = canonicalNavigationHistoryPath(path)
	fm.clearStatusNotice()
	currentPath := fm.GetCurrentPath()

	// A refresh replaces the list wholesale. Keep the surrounding entries as
	// a transient fallback before starting the asynchronous read, so a cursor
	// on a file which disappeared since the watcher's last update does not
	// unexpectedly land on the first row. Prefer the following row on a tie:
	// it occupies the deleted row's former screen position after the refresh.
	var refreshCursorNeighbors []string
	if currentPath != "" && currentPath == path {
		refreshCursorNeighbors = fm.cursorNeighborPaths()
	}

	// Save current cursor position before changing directory
	// Skip saving if already saved manually (e.g., during refresh)
	if currentPath != "" && currentPath != path {
		fm.SaveCursorPosition(currentPath)
	}

	// Stop current directory watcher if running
	if fm.dirWatcher != nil {
		fm.dirWatcher.Stop()
	}

	// Store the previous directory for parent navigation logic
	previousPath := currentPath
	debugPrint("FileManager: LoadDirectory start path=%s previous=%s fallback=%t focused=%s active=%t", path, previousPath, allowParentFallback, focusedObjectLabel(fm.window), fm.windowActive)
	if fm.directoryLoader == nil {
		fm.directoryLoader = browser.NewDirectoryLoader()
	}
	handle := fm.directoryLoader.Begin()

	// Indicate busy and block input while loading
	if fm.busy != nil {
		fm.busy.Begin(fmt.Sprintf("Loading %s...", path), func() {
			fm.cancelDirectoryLoad(handle.ID)
		})
	}

	// Capture the sort config on the UI thread: fm.state is mutated by the
	// sort dialog on the UI thread, so the background goroutine below must
	// never read it directly (that would be a data race).
	sortCfg := fm.state.EffectiveSort(fm.config.UI.Sort)

	// Load directory asynchronously to avoid blocking UI (applies to both local and remote paths)
	go fm.loadDirectoryAsync(handle, path, previousPath, sortCfg, allowParentFallback, refreshCursorNeighbors)
}

// loadDirectoryAsync asks the widget-free loader to read a path in a
// background goroutine, then applies an accepted result on the main thread.
// refreshCursorNeighbors is supplied only for a same-directory reload. It is
// variadic to keep direct test callers concise when no refresh fallback is
// relevant.
func (fm *FileManager) loadDirectoryAsync(handle browser.DirectoryLoadHandle, path string, previousPath string, sortCfg config.SortConfig, allowParentFallback bool, refreshCursorNeighbors ...[]string) {
	var cursorNeighbors []string
	if len(refreshCursorNeighbors) > 0 {
		cursorNeighbors = refreshCursorNeighbors[0]
	}
	discarded := func(err error) bool {
		if handle.Context != nil && handle.Context.Err() != nil {
			debugPrint("FileManager: LoadDirectory canceled id=%d err=%v", handle.ID, handle.Context.Err())
			return true
		}
		if err != nil && errors.Is(err, context.Canceled) {
			debugPrint("FileManager: LoadDirectory canceled id=%d err=%v", handle.ID, err)
			return true
		}
		if fm.directoryLoader == nil || !fm.directoryLoader.Active(handle.ID) {
			debugPrint("FileManager: LoadDirectory stale id=%d path=%s", handle.ID, fm.GetCurrentPath())
			return true
		}
		return false
	}

	// Keep the busy overlay naming the ancestor currently being probed as the
	// fallback walk advances, instead of the (already-rejected) requested
	// path. This callback runs on the background goroutine, so the overlay
	// mutation itself must be dispatched through fyne.Do.
	onCandidate := func(candidate string) {
		fyne.Do(func() {
			if discarded(nil) {
				return
			}
			if fm.busy != nil {
				fm.busy.UpdateText(fmt.Sprintf("Loading %s...", candidate))
			}
		})
	}
	result, err := fm.directoryLoader.Load(handle, browser.DirectoryLoadRequest{
		Path:                path,
		AllowParentFallback: allowParentFallback,
		Sort:                sortCfg,
	}, onCandidate)
	if err != nil {
		if discarded(err) {
			return
		}
		log.Printf("Error reading directory: %v", err)
		fyne.Do(func() {
			if !fm.directoryLoader.Finish(handle.ID) {
				return
			}
			// Clear busy state on error
			if fm.busy != nil {
				fm.busy.End()
			}
			fm.ShowMessageDialog("フォルダを開けませんでした", err.Error())
			// Revert to previous path on error and restart watcher
			if previousPath != "" {
				fm.browserModel().SetPath(previousPath)
				fm.setPathDisplay(previousPath)
				if fm.dirWatcher != nil && fm.shouldWatchPath(previousPath) {
					fm.dirWatcher.SetPollInterval(fm.pollIntervalForPath(previousPath))
					fm.dirWatcher.Start()
				}
			}
		})
		return
	}
	if discarded(nil) {
		return
	}

	path = result.Path
	if result.StorageErr != nil {
		debugPrint("FileManager: Storage info unavailable for %s: %v", path, result.StorageErr)
	}

	// Apply UI updates on main thread
	fyne.Do(func() {
		if !fm.directoryLoader.Finish(handle.ID) {
			return
		}
		// Stop existing watcher (if any) before applying
		if fm.dirWatcher != nil {
			fm.dirWatcher.Stop()
		}

		// Add previous path to navigation history before changing directory
		if previousPath != "" && previousPath != path {
			fm.recordNavigationHistory(previousPath)
		}

		fm.browserModel().ReplaceDirectory(path, result.Files, result.Storage, result.StorageErr == nil, sortCfg)
		fm.setPathDisplay(path)
		loadedFiles := fm.GetFiles()

		// The loader returns a pre-sorted listing; no sort call is needed here.

		// Restore cursor for the newly replaced listing.
		if len(loadedFiles) > 0 {
			if isParentDirectoryNavigation(previousPath, path) {
				dirName := fileinfo.BaseName(previousPath)
				cursorSet := false
				for i, f := range loadedFiles {
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
					for i, f := range loadedFiles {
						if f.Name == saved {
							fm.SetCursorByIndex(i)
							cursorSet = true
							break
						}
					}
				}
				if !cursorSet {
					for _, neighborPath := range cursorNeighbors {
						for i, f := range loadedFiles {
							if f.Path != neighborPath {
								continue
							}
							fm.SetCursorByIndex(i)
							cursorSet = true
							debugPrint("FileManager: refresh cursor fallback path=%s index=%d", neighborPath, i)
							break
						}
						if cursorSet {
							break
						}
					}
				}
				if !cursorSet {
					fm.SetCursorByIndex(0)
				}
			}
		} else {
			fm.browserModel().SetCursorPath("")
		}
		// Content was replaced: refresh before the cursor scroll (see
		// refreshListAndCursor) and re-query the list length even when empty.
		fm.refreshListAndCursor()
		fm.updateStatusBar()
		if result.UsedParentFallback {
			fm.showStatusNotice(parentFallbackStatusNotice(result.RequestedPath, path))
			debugPrint("FileManager: LoadDirectory fallback requested=%s opened=%s", result.RequestedPath, path)
		}

		// Hide busy only now that list state and cursor are rendered-ready,
		// so input stays blocked until the new listing is actually usable.
		if fm.busy != nil {
			fm.busy.End()
		}

		// Restart watcher with appropriate interval when the provider can be watched.
		if fm.dirWatcher != nil && fm.shouldWatchPath(path) {
			fm.dirWatcher.SetPollInterval(fm.pollIntervalForPath(path))
			fm.dirWatcher.Start()
		}
		fm.focusFileList("directory-load-success")
		debugPrint("FileManager: LoadDirectory done path=%s previous=%s files=%d cursor=%s index=%d focused=%s active=%t", path, previousPath, fm.FileCount(), fm.browserModel().CursorPath(), fm.GetCurrentCursorIndex(), focusedObjectLabel(fm.window), fm.windowActive)
	})
}

func isParentDirectoryNavigation(previousPath, path string) bool {
	return previousPath != "" && previousPath != path && fileinfo.ParentPath(previousPath) == path
}

// cursorNeighborPaths returns the ordinary files nearest to the cursor in
// the current visible order. The row after the cursor is considered before
// the row before it for an equal distance, preserving the on-screen position
// when the cursor's row is removed by a refresh. Deleted rows and the parent
// entry cannot be useful cursor restoration targets after a reload.
func (fm *FileManager) cursorNeighborPaths() []string {
	cursorIndex := fm.GetCurrentCursorIndex()
	if cursorIndex < 0 {
		return nil
	}

	files := fm.GetFiles()
	neighbors := make([]string, 0, len(files)-1)
	for distance := 1; distance < len(files); distance++ {
		for _, index := range []int{cursorIndex + distance, cursorIndex - distance} {
			if index < 0 || index >= len(files) {
				continue
			}
			file := files[index]
			if file.Name == ".." || file.Status == fileinfo.StatusDeleted {
				continue
			}
			neighbors = append(neighbors, file.Path)
		}
	}
	return neighbors
}

func (fm *FileManager) cancelDirectoryLoad(loadID uint64) {
	if fm.directoryLoader == nil || !fm.directoryLoader.Cancel(loadID) {
		return
	}
	if fm.busy != nil {
		fm.busy.End()
	}
	currentPath := fm.GetCurrentPath()
	if fm.dirWatcher != nil && fm.shouldWatchPath(currentPath) {
		fm.dirWatcher.SetPollInterval(fm.pollIntervalForPath(currentPath))
		fm.dirWatcher.Start()
	}
	fm.focusFileList("directory-load-cancel")
	debugPrint("FileManager: LoadDirectory cancel id=%d path=%s", loadID, currentPath)
}

// pollIntervalForPath returns the recommended watcher polling interval for a path.
// Remote (SMB) paths get a longer interval to reduce load/latency impact.
func (fm *FileManager) pollIntervalForPath(p string) time.Duration {
	if fileinfo.IsArchivePath(p) {
		return 0
	}
	if strings.HasPrefix(strings.ToLower(p), "smb://") {
		return 4 * time.Second
	}
	return 2 * time.Second
}

func (fm *FileManager) shouldWatchPath(p string) bool {
	if fileinfo.IsArchivePath(p) {
		return false
	}
	vfs, _, err := fileinfo.ResolveRead(p)
	if err != nil {
		return false
	}
	defer fileinfo.CloseVFS(vfs)
	return vfs.Capabilities().Watch
}
