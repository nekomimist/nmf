package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"fyne.io/fyne/v2"

	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
	"nmf/internal/ui"
)

// SaveCursorPosition saves the current cursor position for the given directory.
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
	if err := fm.configManager.SaveAsync(fm.config); err != nil {
		debugPrint("FileManager: Error saving cursor position config: %v", err)
	}
}

// restoreCursorPosition restores the cursor position for the given directory.
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

// cleanupOldCursorEntries removes the oldest entries when maxEntries is exceeded.
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

// navigateToPath handles path entry validation and navigation.
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
			debugPrint("FileManager: Error getting home directory: %v", err)
			fm.pathEntry.SetText(fm.currentPath) // Reset to current path
			return
		}
		path = strings.Replace(path, "~", home, 1)
	}

	resolvedPath, parsed, err := resolveDirectoryPath(path)
	if err != nil {
		debugPrint("FileManager: Invalid path '%s': %v", inputPath, err)
		fm.pathEntry.SetText(fm.currentPath) // Reset to current path
		return
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
	fm.FocusFileList()
}

// FocusFileList sets focus to the file list view.
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
		info, err := entry.Info()
		if err != nil {
			continue
		}
		fullPath := fileinfo.JoinPath(path, entry.Name())
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
			if err := fm.configManager.SaveAsync(fm.config); err != nil {
					debugPrint("FileManager: Error saving navigation history: %v", err)
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

// beginBusy shows the busy overlay and pushes a swallowing key handler.
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
	fm.busyMu.Unlock()

	// Hide overlay (if visible) and pop guard
	fm.busyOverlay.Hide()
	fm.keyManager.PopHandler()
}

// pollIntervalForPath returns the recommended watcher polling interval for a path.
// Remote (SMB) paths get a longer interval to reduce load/latency impact.
func (fm *FileManager) pollIntervalForPath(p string) time.Duration {
	if strings.HasPrefix(strings.ToLower(p), "smb://") {
		return 4 * time.Second
	}
	return 2 * time.Second
}
