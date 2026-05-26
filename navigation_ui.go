package main

import (
	"os"
	"sort"
	"strings"

	"fyne.io/fyne/v2"

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
	newFM := NewFileManager(fyne.CurrentApp(), path, fm.config, fm.configManager, fm.customTheme, fm.configScript)
	newFM.window.Show()
	positionWindowNextTo(fm.window, newFM.window)
}

// ShowDirectoryTreeDialog shows the directory tree navigation dialog.
func (fm *FileManager) ShowDirectoryTreeDialog() {
	dialog := ui.NewDirectoryTreeDialog(fm.currentPath, fm.keyManager, debugPrint)
	dialog.ShowDialog(fm.window, func(selectedPath string) {
		debugPrint("FileManager: Directory selected from tree dialog: %s", selectedPath)
		fm.LoadDirectory(selectedPath)
		fm.FocusFileList()
	})
}

// ShowNavigationHistoryDialog shows the navigation history dialog.
func (fm *FileManager) ShowNavigationHistoryDialog() {
	if normalizeNavigationHistory(fm.config) {
		if err := fm.configManager.SaveAsync(fm.config); err != nil {
			debugPrint("FileManager: Error saving normalized navigation history: %v", err)
		}
	}
	historyPaths := fm.config.GetNavigationHistory()
	openPathList, openPaths := fm.openPathsInOtherWindows()

	// Add current path to the beginning of history list
	enhancedPaths := []string{}
	seen := map[string]bool{}
	if fm.currentPath != "" {
		enhancedPaths = append(enhancedPaths, fm.currentPath)
		seen[fm.currentPath] = true
	}

	// Include paths currently open in other windows even if they have not reached history yet.
	for _, path := range openPathList {
		if path == "" || seen[path] {
			continue
		}
		enhancedPaths = append(enhancedPaths, path)
		seen[path] = true
	}

	// Add existing history paths, skipping duplicates.
	for _, path := range historyPaths {
		if !seen[path] {
			enhancedPaths = append(enhancedPaths, path)
			seen[path] = true
		}
	}

	if len(enhancedPaths) == 0 {
		debugPrint("FileManager: No navigation history available")
		return
	}

	dialog := ui.NewNavigationHistoryDialog(
		enhancedPaths,
		openPaths,
		fm.config.UI.NavigationHistory.LastUsed,
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
		debugPrint("FileManager: Directory selected from history dialog: %s", selectedPath)
		fm.LoadDirectory(selectedPath)
		fm.FocusFileList()
	})
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
		debugPrint("FileManager: Directory selected from jump dialog: %s", selectedPath)
		fm.jumpToConfiguredDirectory(selectedPath)
		fm.FocusFileList()
	})
}

// ShowMaintenanceDialog opens maintenance tools for runtime state cleanup.
func (fm *FileManager) ShowMaintenanceDialog() {
	dialog := ui.NewMaintenanceDialog(fm.config, fm.keyManager, debugPrint)
	dialog.ShowDialog(fm.window, func(result maintenance.Result) (int, error) {
		removed := maintenance.Apply(fm.config, result)
		if removed == 0 {
			return 0, nil
		}
		if err := fm.configManager.SaveAsync(fm.config); err != nil {
			debugPrint("FileManager: Error saving maintenance cleanup: %v", err)
			return removed, err
		}
		debugPrint("FileManager: Maintenance cleanup removed=%d", removed)
		return removed, nil
	})
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

	resolvedPath, parsed, err := resolveDirectoryPath(path)
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
