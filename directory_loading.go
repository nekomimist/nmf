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
	_, file, ok := fm.browserModel().CursorFile()
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
	fm.loadDirectoryWithNavigation(path, allowParentFallback, directoryNavigation{})
}

func (fm *FileManager) loadDirectoryWithNavigation(path string, allowParentFallback bool, navigation directoryNavigation) {
	path = canonicalNavigationHistoryPath(path)
	fm.clearStatusNotice()
	currentPath := fm.GetCurrentPath()
	previousListingState := fm.directoryListingStateAfterCanceledRefresh()

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

	// Capture the sort config on the UI thread: fm.state is mutated by the
	// sort dialog on the UI thread, so the background goroutine below must
	// never read it directly (that would be a data race).
	sortCfg := fm.state.EffectiveSort(fm.config.UI.Sort)
	cacheDisplayed := currentPath != "" && currentPath != path &&
		fm.displayCachedDirectory(path, previousPath, sortCfg, refreshCursorNeighbors)
	if cacheDisplayed {
		fm.acceptDirectoryNavigation(previousPath, path, navigation)
	}

	if !cacheDisplayed {
		// Beginning a newer read cancels any revalidation that owned the current
		// cached view. If this read is canceled or fails, that old view is stale,
		// not still refreshing.
		fm.setDirectoryListingState(previousListingState)
		// Indicate busy and block input while no useful target listing is ready.
		if fm.busy != nil {
			// A repeated navigation reuses the existing busy input guard. Keep its
			// cancel callback generation-independent so Escape always cancels the
			// newest load rather than the handle that first created the guard.
			fm.busy.Begin(fmt.Sprintf("Loading %s...", path), fm.cancelActiveDirectoryLoad)
		}
	}

	// Load directory asynchronously to avoid blocking UI (applies to both local and remote paths)
	go fm.loadDirectoryAsync(handle, path, previousPath, sortCfg, allowParentFallback, refreshCursorNeighbors, directoryLoadPresentation{
		cacheDisplayed:       cacheDisplayed,
		previousListingState: previousListingState,
		navigation:           navigation,
	})
}

func (fm *FileManager) displayCachedDirectory(path string, previousPath string, sortCfg config.SortConfig, cursorNeighbors []string) bool {
	snapshot, ok := fm.ensureDirectoryCache().Get(path)
	if !ok {
		return false
	}
	files := snapshot.Files
	if snapshot.Sort != sortCfg {
		files = browser.SortFiles(files, sortCfg)
	}
	if fm.busy != nil {
		fm.busy.End()
	}
	fm.setDirectoryListingState(directoryListingCachedRefreshing)
	fm.applyDirectoryListing(path, files, snapshot.Storage, snapshot.StorageKnown, sortCfg, previousPath, cursorNeighbors, "")
	if previousPath != "" && previousPath != path {
		fm.recordNavigationHistory(previousPath)
	}
	fm.focusFileList("directory-cache-hit")
	debugPrint("FileManager: Directory cache hit path=%s files=%d", path, len(files))
	return true
}

type directoryLoadPresentation struct {
	cacheDisplayed       bool
	previousListingState directoryListingState
	navigation           directoryNavigation
}

// loadDirectoryAsync asks the widget-free loader to read a path in a
// background goroutine, then applies an accepted result on the main thread.
// refreshCursorNeighbors is supplied only for a same-directory reload.
func (fm *FileManager) loadDirectoryAsync(handle browser.DirectoryLoadHandle, path string, previousPath string, sortCfg config.SortConfig, allowParentFallback bool, cursorNeighbors []string, presentation directoryLoadPresentation) {
	requestedPath := path
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
			if presentation.cacheDisplayed {
				if fileinfo.IsNotExist(err) {
					fm.ensureDirectoryCache().Delete(requestedPath)
				}
				fm.setDirectoryListingState(directoryListingCachedStale)
				fm.ShowMessageDialog("フォルダを更新できませんでした", err.Error())
				return
			}
			fm.rejectDirectoryNavigation(presentation.navigation)
			fallbackState := presentation.previousListingState
			if fallbackState == directoryListingCachedRefreshing {
				fallbackState = directoryListingCachedStale
			}
			fm.setDirectoryListingState(fallbackState)
			fm.ShowMessageDialog("フォルダを開けませんでした", err.Error())
			// Revert to previous path on error and restart watcher
			if previousPath != "" {
				fm.browserModel().SetPath(previousPath)
				fm.setPathDisplay(previousPath)
				if fallbackState == directoryListingFresh && fm.dirWatcher != nil && fm.shouldWatchPath(previousPath) {
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

		if result.Path != result.RequestedPath {
			fm.ensureDirectoryCache().Delete(result.RequestedPath)
		}
		fm.ensureDirectoryCache().Put(browser.DirectorySnapshot{
			Path:         result.Path,
			Files:        result.Files,
			Storage:      result.Storage,
			StorageKnown: result.StorageErr == nil,
			Sort:         sortCfg,
		})

		// A cache hit records the navigation when it displays the provisional
		// target, because the user may continue elsewhere before revalidation.
		if !presentation.cacheDisplayed && previousPath != "" && previousPath != path {
			fm.recordNavigationHistory(previousPath)
		}

		preferredCursorPath := ""
		if presentation.cacheDisplayed && requestedPath == path {
			preferredCursorPath = fm.browserModel().CursorPath()
		}
		fm.setDirectoryListingState(directoryListingFresh)
		fm.applyDirectoryListing(path, result.Files, result.Storage, result.StorageErr == nil, sortCfg, previousPath, cursorNeighbors, preferredCursorPath)
		if !presentation.cacheDisplayed {
			fm.acceptDirectoryNavigation(previousPath, path, presentation.navigation)
		}
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

// applyDirectoryListing replaces one complete listing and restores a useful
// cursor. preferredCursorPath is used when a user moved within a provisional
// cached listing while its real read was running.
func (fm *FileManager) applyDirectoryListing(path string, files []fileinfo.FileInfo, storage fileinfo.StorageInfo, storageKnown bool, sortCfg config.SortConfig, previousPath string, cursorNeighbors []string, preferredCursorPath string) {
	fm.browserModel().ReplaceDirectory(path, files, storage, storageKnown, sortCfg)
	fm.setPathDisplay(path)
	loadedFiles := fm.GetFiles()

	cursorSet := false
	if preferredCursorPath != "" {
		for i, file := range loadedFiles {
			if file.Path == preferredCursorPath {
				fm.SetCursorByIndex(i)
				cursorSet = true
				break
			}
		}
	}
	if len(loadedFiles) > 0 && !cursorSet {
		if isParentDirectoryNavigation(previousPath, path) {
			dirName := fileinfo.BaseName(previousPath)
			for i, file := range loadedFiles {
				if file.Name == dirName {
					fm.SetCursorByIndex(i)
					cursorSet = true
					break
				}
			}
		} else {
			saved := fm.restoreCursorPosition(path)
			if saved != "" {
				for i, file := range loadedFiles {
					if file.Name == saved {
						fm.SetCursorByIndex(i)
						cursorSet = true
						break
					}
				}
			}
			if !cursorSet {
				for _, neighborPath := range cursorNeighbors {
					for i, file := range loadedFiles {
						if file.Path != neighborPath {
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
		}
		if !cursorSet {
			fm.SetCursorByIndex(0)
		}
	} else if len(loadedFiles) == 0 {
		fm.browserModel().SetCursorPath("")
	}

	// Content was replaced: refresh before the cursor scroll (see
	// refreshListAndCursor) and re-query the list length even when empty.
	fm.refreshListAndCursor()
	fm.updateStatusBar()
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
	return fm.browserModel().CursorNeighborPaths()
}

func (fm *FileManager) cancelActiveDirectoryLoad() {
	if fm.directoryLoader == nil || !fm.directoryLoader.CancelActive() {
		return
	}
	if fm.busy != nil {
		fm.busy.End()
	}
	currentPath := fm.GetCurrentPath()
	if fm.directoryListingState == directoryListingFresh && fm.dirWatcher != nil && fm.shouldWatchPath(currentPath) {
		fm.dirWatcher.SetPollInterval(fm.pollIntervalForPath(currentPath))
		fm.dirWatcher.Start()
	}
	fm.focusFileList("directory-load-cancel")
	debugPrint("FileManager: LoadDirectory cancel path=%s", currentPath)
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
