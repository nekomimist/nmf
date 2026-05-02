package main

import (
	"fmt"

	"nmf/internal/fileinfo"
)

func (fm *FileManager) updateStatusBar() {
	if fm.statusLabel == nil {
		return
	}
	fm.statusLabel.SetText(fm.statusBarText())
}

func (fm *FileManager) statusBarText() string {
	markCount := countMarkedFiles(fm.selectedFiles)
	visibleEntries := countEntriesExcludingParent(fm.files)
	totalEntries := countEntriesExcludingParent(fm.originalFiles)
	if totalEntries == 0 && len(fm.originalFiles) == 0 {
		totalEntries = visibleEntries
	}

	free := "-"
	used := "-"
	total := "-"
	if fm.storageKnown {
		free = fileinfo.FormatFileSize(int64(fm.storageInfo.Free))
		used = fileinfo.FormatFileSize(int64(fm.storageInfo.Used))
		total = fileinfo.FormatFileSize(int64(fm.storageInfo.Total))
	}

	return fmt.Sprintf("Mark: %d | Entry: %d/%d | Free: %s | Used: %s | Total: %s",
		markCount, visibleEntries, totalEntries, free, used, total)
}

func countMarkedFiles(selected map[string]bool) int {
	count := 0
	for _, marked := range selected {
		if marked {
			count++
		}
	}
	return count
}

func countEntriesExcludingParent(files []fileinfo.FileInfo) int {
	count := 0
	for _, file := range files {
		if file.Name != ".." {
			count++
		}
	}
	return count
}
