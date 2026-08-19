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
	// While a notice is active it fully replaces the normal status line so
	// the label never grows past one line (see ui_setup.go's Truncation
	// setting on fm.statusLabel, which relies on this invariant).
	if fm.statusNotice != "" {
		return fm.statusNotice
	}

	stats := fm.browserModel().ListingStats()

	free := "-"
	used := "-"
	total := "-"
	if stats.StorageKnown {
		free = fileinfo.FormatFileSize(int64(stats.Storage.Free))
		used = fileinfo.FormatFileSize(int64(stats.Storage.Used))
		total = fileinfo.FormatFileSize(int64(stats.Storage.Total))
	}

	listing := fmt.Sprintf("Mark: %d | Entry: %d/%d | Free: %s | Used: %s | Total: %s",
		stats.MarkedEntries, stats.VisibleEntries, stats.TotalEntries, free, used, total)
	switch fm.directoryListingState {
	case directoryListingCachedRefreshing:
		return "Cached listing — refreshing; navigation only | " + listing
	case directoryListingCachedStale:
		return "Cached listing — refresh failed; navigation only | " + listing
	default:
		return listing
	}
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
			fm.expireStatusNotice(generation)
		})
	})
}

// expireStatusNotice clears the notice if generation still matches the most
// recently shown one, i.e. no newer notice (or clearStatusNotice) has
// superseded it since the timer was scheduled. Split out from showStatusNotice
// so the stale-timer guard can be exercised directly by tests without
// waiting for the real statusNoticeDuration.
func (fm *FileManager) expireStatusNotice(generation uint64) {
	if fm.isWindowClosed() || fm.statusNoticeGeneration != generation {
		return
	}
	fm.statusNotice = ""
	fm.updateStatusBar()
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
