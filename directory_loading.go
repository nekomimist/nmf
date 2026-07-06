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

	"nmf/internal/config"
	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
)

// SaveCursorPosition saves the current cursor position for the given directory.
func (fm *FileManager) SaveCursorPosition(dirPath string) {
	currentIdx := fm.GetCurrentCursorIndex()
	if currentIdx < 0 || currentIdx >= len(fm.files) {
		return
	}

	fileName := fm.files[currentIdx].Name
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
		fm.setPathDisplay(fm.currentPath)
		return false
	}

	// Handle tilde expansion for home directory
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			debugPrint("FileManager: Error getting home directory: %v", err)
			fm.setPathDisplay(fm.currentPath)
			return false
		}
		path = strings.Replace(path, "~", home, 1)
	}

	resolvedPath, parsed, err := resolveDirectoryPath(path)
	if err != nil {
		debugPrint("FileManager: Invalid path '%s': %v", inputPath, err)
		fm.setPathDisplay(fm.currentPath)
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

	// Path is valid, navigate to it.
	fm.LoadDirectory(resolvedPath)

	// Return focus to file list after successful navigation
	fm.focusFileList("path-edit-navigation")
	return true
}

// FocusFileList sets focus to the file list view.
func (fm *FileManager) FocusFileList() {
	fm.focusFileList("unspecified")
}

func (fm *FileManager) focusFileList(reason string) {
	if fm.fileListView != nil {
		before := focusedObjectLabel(fm.window)
		debugPrint("FileManager: FocusFileList start reason=%s focused=%s active=%t busy=%t path=%s", reason, before, fm.windowActive, fm.busyActive, fm.currentPath)
		fm.window.Canvas().Focus(fm.fileListView)
		fm.setWindowActive(true)
		debugPrint("FileManager: FocusFileList done reason=%s focused=%s active=%t busy=%t path=%s", reason, focusedObjectLabel(fm.window), fm.windowActive, fm.busyActive, fm.currentPath)
		return
	}
	debugPrint("FileManager: FocusFileList skipped reason=%s fileListView=nil path=%s", reason, fm.currentPath)
}

func (fm *FileManager) LoadDirectory(path string) {
	path = canonicalNavigationHistoryPath(path)

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
	debugPrint("FileManager: LoadDirectory start path=%s previous=%s focused=%s active=%t", path, previousPath, focusedObjectLabel(fm.window), fm.windowActive)
	ctx, loadID := fm.beginDirectoryLoad()

	// Indicate busy and block input while loading
	fm.beginBusy(fmt.Sprintf("Loading %s...", path), fm.cancelActiveDirectoryLoad)

	// Capture the sort config on the UI thread: fm.state is mutated by the
	// sort dialog on the UI thread, so the background goroutine below must
	// never read it directly (that would be a data race).
	sortCfg := fm.state.EffectiveSort(fm.config.UI.Sort)

	// Load directory asynchronously to avoid blocking UI (applies to both local and remote paths)
	go fm.loadDirectoryAsync(ctx, loadID, path, previousPath, sortCfg)
}

// loadDirectoryAsync lists a path in a background goroutine and applies UI updates on the main thread.
func (fm *FileManager) loadDirectoryAsync(ctx context.Context, loadID uint64, path string, previousPath string, sortCfg config.SortConfig) {
	entries, err := fileinfo.ReadDirPortableContext(ctx, path)
	if err != nil {
		if fm.ignoreCanceledDirectoryLoad(ctx, loadID, err) {
			return
		}
		log.Printf("Error reading directory: %v", err)
		fyne.Do(func() {
			if !fm.finishDirectoryLoad(loadID) {
				return
			}
			// Clear busy state on error
			fm.endBusy()
			fm.ShowMessageDialog("フォルダを開けませんでした", err.Error())
			// Revert to previous path on error and restart watcher
			if previousPath != "" {
				fm.currentPath = previousPath
				fm.setPathDisplay(previousPath)
				if fm.dirWatcher != nil && fm.shouldWatchPath(previousPath) {
					fm.dirWatcher.SetPollInterval(fm.pollIntervalForPath(previousPath))
					fm.dirWatcher.Start()
				}
			}
		})
		return
	}
	if fm.ignoreCanceledDirectoryLoad(ctx, loadID, nil) {
		return
	}

	// Build file list off the UI thread
	files := make([]fileinfo.FileInfo, 0, len(entries)+1)
	storage, storageErr := fileinfo.StatStoragePortable(path)
	if fm.ignoreCanceledDirectoryLoad(ctx, loadID, nil) {
		return
	}
	if storageErr != nil {
		debugPrint("FileManager: Storage info unavailable for %s: %v", path, storageErr)
	}

	// Add parent directory entry if not at root
	parent := fileinfo.ParentPath(path)
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
		if fm.ignoreCanceledDirectoryLoad(ctx, loadID, nil) {
			return
		}
		fi, err := fileinfo.FileInfoFromDirEntry(path, entry)
		if err != nil {
			continue
		}
		files = append(files, fi)
	}

	// Sort off the UI thread using the sort config captured before this
	// goroutine started (see LoadDirectory).
	files = sortFileInfoSlice(files, sortCfg)
	if fm.ignoreCanceledDirectoryLoad(ctx, loadID, nil) {
		return
	}

	originalFiles := make([]fileinfo.FileInfo, len(files))
	copy(originalFiles, files)

	// Apply UI updates on main thread
	fyne.Do(func() {
		if !fm.finishDirectoryLoad(loadID) {
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

		fm.currentPath = path
		fm.setPathDisplay(path)
		fm.files = files
		fm.originalFiles = originalFiles
		fm.storageInfo = storage
		fm.storageKnown = storageErr == nil
		fm.activeSort = sortCfg

		// files/originalFiles arrive pre-sorted from the background goroutine
		// above; no sort call needed here.

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
		} else {
			fm.cursorPath = ""
		}
		// Content was replaced: refresh before the cursor scroll (see
		// refreshListAndCursor) and re-query the list length even when empty.
		fm.refreshListAndCursor()
		fm.updateStatusBar()

		// Hide busy only now that list state and cursor are rendered-ready,
		// so input stays blocked until the new listing is actually usable.
		fm.endBusy()

		// Restart watcher with appropriate interval when the provider can be watched.
		if fm.dirWatcher != nil && fm.shouldWatchPath(path) {
			fm.dirWatcher.SetPollInterval(fm.pollIntervalForPath(path))
			fm.dirWatcher.Start()
		}
		fm.focusFileList("directory-load-success")
		debugPrint("FileManager: LoadDirectory done path=%s previous=%s files=%d cursor=%s index=%d focused=%s active=%t", path, previousPath, len(fm.files), fm.cursorPath, fm.GetCurrentCursorIndex(), focusedObjectLabel(fm.window), fm.windowActive)
	})
}

func (fm *FileManager) beginDirectoryLoad() (context.Context, uint64) {
	ctx, cancel := context.WithCancel(context.Background())
	fm.loadMu.Lock()
	if fm.loadCancel != nil {
		fm.loadCancel()
	}
	fm.nextLoadID++
	loadID := fm.nextLoadID
	fm.activeLoadID = loadID
	fm.loadCancel = cancel
	fm.loadMu.Unlock()
	return ctx, loadID
}

func (fm *FileManager) finishDirectoryLoad(loadID uint64) bool {
	fm.loadMu.Lock()
	defer fm.loadMu.Unlock()
	if fm.activeLoadID != loadID {
		return false
	}
	fm.activeLoadID = 0
	fm.loadCancel = nil
	return true
}

func (fm *FileManager) directoryLoadActive(loadID uint64) bool {
	fm.loadMu.Lock()
	defer fm.loadMu.Unlock()
	return fm.activeLoadID == loadID
}

func (fm *FileManager) ignoreCanceledDirectoryLoad(ctx context.Context, loadID uint64, err error) bool {
	if ctx != nil && ctx.Err() != nil {
		debugPrint("FileManager: LoadDirectory canceled id=%d err=%v", loadID, ctx.Err())
		return true
	}
	if err != nil && errors.Is(err, context.Canceled) {
		debugPrint("FileManager: LoadDirectory canceled id=%d err=%v", loadID, err)
		return true
	}
	if !fm.directoryLoadActive(loadID) {
		debugPrint("FileManager: LoadDirectory stale id=%d path=%s", loadID, fm.currentPath)
		return true
	}
	return false
}

func (fm *FileManager) cancelDirectoryLoad(loadID uint64) {
	var cancel context.CancelFunc
	fm.loadMu.Lock()
	if fm.activeLoadID == 0 || fm.activeLoadID != loadID {
		fm.loadMu.Unlock()
		return
	}
	cancel = fm.loadCancel
	fm.activeLoadID = 0
	fm.loadCancel = nil
	fm.loadMu.Unlock()

	if cancel != nil {
		cancel()
	}
	fm.endBusy()
	if fm.dirWatcher != nil && fm.shouldWatchPath(fm.currentPath) {
		fm.dirWatcher.SetPollInterval(fm.pollIntervalForPath(fm.currentPath))
		fm.dirWatcher.Start()
	}
	fm.focusFileList("directory-load-cancel")
	debugPrint("FileManager: LoadDirectory cancel id=%d path=%s", loadID, fm.currentPath)
}

func (fm *FileManager) cancelActiveDirectoryLoad() {
	fm.loadMu.Lock()
	loadID := fm.activeLoadID
	fm.loadMu.Unlock()
	if loadID != 0 {
		fm.cancelDirectoryLoad(loadID)
	}
}

// beginBusy shows the busy overlay and pushes a swallowing key handler.
func (fm *FileManager) beginBusy(text string, onCancel ...func()) {
	fm.busyMu.Lock()
	defer fm.busyMu.Unlock()

	if fm.busyActive {
		// Already busy: update label if visible, and store latest text
		fm.busyText = text
		fm.busyOverlay.Show(fm.window, text)
		debugPrint("FileManager: busy update text=%q focused=%s", text, focusedObjectLabel(fm.window))
		return
	}

	fm.busyActive = true
	fm.busyText = text
	debugPrint("FileManager: busy begin text=%q focused=%s", text, focusedObjectLabel(fm.window))

	// Block keys immediately to avoid reentrancy
	var cancel func()
	if len(onCancel) > 0 {
		cancel = onCancel[0]
	}
	fm.busyToken = fm.keyManager.PushHandler(keymanager.NewBusyKeyHandler(cancel))

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

// endBusy hides the busy overlay and pops the swallowing key handler.
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
	busyToken := fm.busyToken
	fm.busyToken = 0
	fm.busyMu.Unlock()

	// Hide overlay (if visible) and remove guard
	fm.busyOverlay.Hide()
	fm.keyManager.RemoveHandler(busyToken)
	debugPrint("FileManager: busy end focused=%s active=%t path=%s", focusedObjectLabel(fm.window), fm.windowActive, fm.currentPath)
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
	vfs, _, err := fileinfo.ResolveRead(p)
	if err != nil {
		return false
	}
	return vfs.Capabilities().Watch
}
