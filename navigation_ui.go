package main

import (
	"os"
	"sort"
	"strings"

	"nmf/internal/fileinfo"
	"nmf/internal/maintenance"
	"nmf/internal/ui"
)

func (fm *FileManager) OpenNewWindow() {
	fm.openWindowAtPath(fm.currentPath)
}

func (fm *FileManager) ReopenClosedWindow() {
	path, ok := nextReopenPath()
	if !ok {
		debugPrint("FileManager: No closed window path available; opening current path")
		path = fm.currentPath
	}
	fm.openWindowAtPath(path)
}

func (fm *FileManager) openWindowAtPath(path string) {
	newFM := NewFileManager(fm.runtime, path, fm.config, fm.configManager, fm.state, fm.stateManager, fm.customTheme, fm.configScript)
	newFM.window.Show()
	positionWindowNextTo(fm.window, newFM.window)
}

// ShowDirectoryTreeDialog shows the directory tree navigation dialog.
func (fm *FileManager) ShowDirectoryTreeDialog() {
	dialog := ui.NewDirectoryTreeDialog(fm.currentPath, fm.keyManager, debugPrint)
	dialog.ShowDialog(fm.window, func(selectedPath string) {
		debugPrint("FileManager: tree dialog selected path=%s focused=%s", selectedPath, focusedObjectLabel(fm.window))
		fm.LoadDirectory(selectedPath)
		fm.focusFileList("tree-dialog-selected")
	})
}

// ShowNavigationHistoryDialog shows the navigation history dialog.
func (fm *FileManager) ShowNavigationHistoryDialog() {
	fm.normalizeNavigationHistoryForRuntimeState()
	historyPaths := fm.state.GetNavigationHistory()
	openPathList, openPaths := fm.openPathsInOtherWindows()

	enhancedPaths, unpinRemovesPath := buildEnhancedNavigationHistoryPaths(
		fm.currentPath,
		openPathList,
		fm.state.NavigationHistory.Pinned,
		historyPaths,
	)

	if len(enhancedPaths) == 0 {
		debugPrint("FileManager: No navigation history available")
		return
	}

	dialog := ui.NewNavigationHistoryDialog(
		enhancedPaths,
		openPaths,
		pinnedNavigationPathMap(fm.state.NavigationHistory.Pinned),
		unpinRemovesPath,
		fm.state.NavigationHistory.LastUsed,
		fm.keyManager,
		debugPrint,
		fm.searchMatchers,
	)
	dialog.SetOnSelectedPathChanged(func(path string) {
		if openPaths[path] {
			highlightFileManagerWindowForPath(path)
			return
		}
		clearFileManagerWindowHighlights()
	})
	dialog.ShowDialog(fm.window, func(selectedPath string) {
		debugPrint("FileManager: history dialog selected path=%s focused=%s", selectedPath, focusedObjectLabel(fm.window))
		fm.LoadDirectory(selectedPath)
		fm.focusFileList("history-dialog-selected")
	}, fm.UnpinHistoryPath)
}

func pinnedNavigationPathMap(paths []string) map[string]bool {
	result := make(map[string]bool, len(paths))
	for _, path := range paths {
		if path != "" {
			result[path] = true
		}
	}
	return result
}

func buildEnhancedNavigationHistoryPaths(currentPath string, openPaths []string, pinnedPaths []string, historyPaths []string) ([]string, map[string]bool) {
	enhancedPaths := []string{}
	seen := map[string]bool{}
	if currentPath != "" {
		enhancedPaths = append(enhancedPaths, currentPath)
		seen[currentPath] = true
	}

	for _, path := range openPaths {
		if path == "" || seen[path] {
			continue
		}
		enhancedPaths = append(enhancedPaths, path)
		seen[path] = true
	}

	pinnedVisibleOnlyBecausePinned := map[string]bool{}
	for _, path := range pinnedPaths {
		if path == "" || seen[path] {
			continue
		}
		enhancedPaths = append(enhancedPaths, path)
		seen[path] = true
		pinnedVisibleOnlyBecausePinned[path] = true
	}

	for _, path := range historyPaths {
		if path == "" {
			continue
		}
		if pinnedVisibleOnlyBecausePinned[path] {
			delete(pinnedVisibleOnlyBecausePinned, path)
		}
		if !seen[path] {
			enhancedPaths = append(enhancedPaths, path)
			seen[path] = true
		}
	}

	return enhancedPaths, pinnedVisibleOnlyBecausePinned
}

func (fm *FileManager) PinCurrentHistoryPath() {
	path := canonicalNavigationHistoryPath(fm.currentPath)
	if path == "" || fm.state == nil {
		return
	}

	if !fm.state.PinNavigationPath(path) {
		debugPrint("FileManager: History path already pinned path=%s", path)
		fm.ShowMessageDialog("History Jump", "Already saved:\n"+path)
		return
	}
	if fm.stateManager != nil {
		if err := fm.stateManager.SaveAsync(fm.state); err != nil {
			debugPrint("FileManager: Error saving pinned history path: %v", err)
			fm.ShowMessageDialog("History Jump", err.Error())
			return
		}
	}
	debugPrint("FileManager: Pinned history path=%s", path)
	notifyNavigationHistoryChanged(path)
	fm.ShowMessageDialog("History Jump", "Saved:\n"+path)
}

func (fm *FileManager) UnpinHistoryPath(path string) bool {
	path = canonicalNavigationHistoryPath(path)
	if path == "" || fm.state == nil {
		return false
	}
	if !fm.state.UnpinNavigationPath(path) {
		return false
	}
	if fm.stateManager != nil {
		if err := fm.stateManager.SaveAsync(fm.state); err != nil {
			debugPrint("FileManager: Error saving unpinned history path: %v", err)
			fm.ShowMessageDialog("History Jump", err.Error())
			return false
		}
	}
	debugPrint("FileManager: Unpinned history path=%s", path)
	notifyNavigationHistoryChanged(path)
	return true
}

func (fm *FileManager) openPathsInOtherWindows() ([]string, map[string]bool) {
	openPaths := map[string]bool{}
	windowRegistry.Range(func(k, v any) bool {
		other, ok := v.(*FileManager)
		if !ok || other == fm || other.currentPath == "" {
			return true
		}
		openPaths[other.currentPath] = true
		return true
	})
	pathList := make([]string, 0, len(openPaths))
	for path := range openPaths {
		pathList = append(pathList, path)
	}
	sort.Strings(pathList)
	return pathList, openPaths
}

// ShowDirectoryJumpDialog shows manually configured directory jump targets.
func (fm *FileManager) ShowDirectoryJumpDialog() {
	entries := fm.config.UI.DirectoryJumps.Entries
	if len(entries) == 0 {
		debugPrint("FileManager: No directory jumps configured")
		return
	}

	dialog := ui.NewDirectoryJumpDialog(entries, fm.keyManager, debugPrint)
	dialog.ShowDialog(fm.window, func(selectedPath string) {
		debugPrint("FileManager: jump dialog selected path=%s focused=%s", selectedPath, focusedObjectLabel(fm.window))
		fm.jumpToConfiguredDirectory(selectedPath)
		fm.focusFileList("jump-dialog-selected")
	})
}

// ShowMaintenanceDialog opens maintenance tools for runtime state cleanup.
func (fm *FileManager) ShowMaintenanceDialog() {
	fm.normalizeNavigationHistoryForRuntimeState()
	dialog := ui.NewMaintenanceDialog(fm.state, fm.keyManager, debugPrint)
	dialog.ShowDialog(fm.window, func(result maintenance.Result) (int, error) {
		removed := maintenance.Apply(fm.state, result)
		if removed == 0 {
			return 0, nil
		}
		if fm.stateManager != nil {
			if err := fm.stateManager.SaveAsync(fm.state); err != nil {
				debugPrint("FileManager: Error saving maintenance cleanup: %v", err)
				return removed, err
			}
		}
		debugPrint("FileManager: Maintenance cleanup removed=%d", removed)
		return removed, nil
	})
}

func (fm *FileManager) normalizeNavigationHistoryForRuntimeState() {
	if normalizeNavigationHistory(fm.state) && fm.stateManager != nil {
		if err := fm.stateManager.SaveAsync(fm.state); err != nil {
			debugPrint("FileManager: Error saving normalized navigation history: %v", err)
		}
	}
}

func (fm *FileManager) recordNavigationHistory(path string) {
	path = canonicalNavigationHistoryPath(path)
	if path == "" || fm.state == nil {
		return
	}

	fm.state.AddToNavigationHistory(path, fm.config.UI.NavigationHistory.MaxEntries)
	if fm.stateManager != nil {
		if err := fm.stateManager.SaveAsync(fm.state); err != nil {
			debugPrint("FileManager: Error saving navigation history: %v", err)
		}
	}
	notifyNavigationHistoryChanged(path)
}

func (fm *FileManager) jumpToConfiguredDirectory(inputPath string) {
	path := strings.TrimSpace(inputPath)
	if path == "" {
		return
	}

	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			debugPrint("FileManager: Error getting home directory: %v", err)
			fm.ShowMessageDialog("フォルダを開けませんでした", err.Error())
			return
		}
		path = strings.Replace(path, "~", home, 1)
	}

	resolvedPath, parsed, err := fileinfo.CanonicalDisplayPath(path)
	if err != nil {
		debugPrint("FileManager: Invalid directory jump path '%s': %v", inputPath, err)
		fm.ShowMessageDialog("フォルダを開けませんでした", err.Error())
		return
	}

	if parsed.Scheme == fileinfo.SchemeSMB && (parsed.User != "" || parsed.Password != "" || parsed.Domain != "") {
		fileinfo.PutCachedCredentials(parsed.Host, parsed.Share, fileinfo.Credentials{
			Domain:   parsed.Domain,
			Username: parsed.User,
			Password: parsed.Password,
		})
	}

	fm.LoadDirectory(resolvedPath)
}
