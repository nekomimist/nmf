package main

import (
	"os"
	"strings"

	"fyne.io/fyne/v2"

	"nmf/internal/fileinfo"
	"nmf/internal/ui"
)

func (fm *FileManager) OpenNewWindow() {
	newFM := NewFileManager(fyne.CurrentApp(), fm.currentPath, fm.config, fm.configManager, fm.customTheme)
	newFM.window.Show()
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
	historyPaths := fm.config.GetNavigationHistory()

	// Add current path to the beginning of history list
	enhancedPaths := []string{}
	if fm.currentPath != "" {
		enhancedPaths = append(enhancedPaths, fm.currentPath)
	}

	// Add existing history paths, but skip duplicates of current path
	for _, path := range historyPaths {
		if path != fm.currentPath {
			enhancedPaths = append(enhancedPaths, path)
		}
	}

	if len(enhancedPaths) == 0 {
		debugPrint("FileManager: No navigation history available")
		return
	}

	dialog := ui.NewNavigationHistoryDialog(
		enhancedPaths,
		fm.config.UI.NavigationHistory.LastUsed,
		fm.keyManager,
		debugPrint,
	)
	dialog.ShowDialog(fm.window, func(selectedPath string) {
		debugPrint("FileManager: Directory selected from history dialog: %s", selectedPath)
		fm.LoadDirectory(selectedPath)
		fm.FocusFileList()
	})
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

func (fm *FileManager) jumpToConfiguredDirectory(inputPath string) {
	path := strings.TrimSpace(inputPath)
	if path == "" {
		return
	}

	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			debugPrint("FileManager: Error getting home directory: %v", err)
			ui.ShowMessageDialog(fm.window, "フォルダを開けませんでした", err.Error())
			return
		}
		path = strings.Replace(path, "~", home, 1)
	}

	resolvedPath, parsed, err := resolveDirectoryPath(path)
	if err != nil {
		debugPrint("FileManager: Invalid directory jump path '%s': %v", inputPath, err)
		ui.ShowMessageDialog(fm.window, "フォルダを開けませんでした", err.Error())
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
