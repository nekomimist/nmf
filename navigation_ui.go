package main

import (
	"fyne.io/fyne/v2"

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
		debugPrint("Directory selected from tree dialog: %s", selectedPath)
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
		debugPrint("No navigation history available")
		return
	}

	dialog := ui.NewNavigationHistoryDialog(
		enhancedPaths,
		fm.config.UI.NavigationHistory.LastUsed,
		fm.keyManager,
		debugPrint,
	)
	dialog.ShowDialog(fm.window, func(selectedPath string) {
		debugPrint("Directory selected from history dialog: %s", selectedPath)
		fm.LoadDirectory(selectedPath)
		fm.FocusFileList()
	})
}
