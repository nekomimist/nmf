package main

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"

	"nmf/internal/fileinfo"
)

const statusNoticeDuration = 5 * time.Second

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

	status := fmt.Sprintf("Mark: %d | Entry: %d/%d | Free: %s | Used: %s | Total: %s",
		markCount, visibleEntries, totalEntries, free, used, total)
	if fm.statusNotice == "" {
		return status
	}
	return status + "\n" + fm.statusNotice
}

func parentFallbackStatusNotice(requestedPath, openedPath string) string {
	return fmt.Sprintf("Path not found; opened nearest parent: %s → %s", requestedPath, openedPath)
}

// showStatusNotice appends a short-lived, non-modal notice to the status bar.
// The generation token prevents an older timer from clearing a newer message.
func (fm *FileManager) showStatusNotice(notice string) {
	if fm == nil || notice == "" {
		return
	}
	fm.statusNoticeGeneration++
	generation := fm.statusNoticeGeneration
	fm.statusNotice = notice
	fm.updateStatusBar()

	time.AfterFunc(statusNoticeDuration, func() {
		fyne.Do(func() {
			if fm.isWindowClosed() || fm.statusNoticeGeneration != generation {
				return
			}
			fm.statusNotice = ""
			fm.updateStatusBar()
		})
	})
}

// clearStatusNotice removes a previous navigation result when another
// directory load begins. It also invalidates that notice's expiry callback.
func (fm *FileManager) clearStatusNotice() {
	if fm == nil {
		return
	}
	fm.statusNoticeGeneration++
	if fm.statusNotice == "" {
		return
	}
	fm.statusNotice = ""
	fm.updateStatusBar()
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

// countEntriesExcludingParent relies on the sort invariant that
// sortFilesWithConfig always pins ".." at index 0.
func countEntriesExcludingParent(files []fileinfo.FileInfo) int {
	n := len(files)
	if n > 0 && files[0].Name == ".." {
		n--
	}
	return n
}
