package main

import (
	"sync/atomic"

	"fyne.io/fyne/v2"

	"nmf/internal/jobs"
	"nmf/internal/ui"
)

// closeWindow handles window closing logic.
func (fm *FileManager) closeWindow() {
	if !fm.beginWindowClose() {
		return
	}

	// Invalidate background work before releasing window-owned UI resources.
	fm.invalidateActiveDirectoryLoad()
	fm.invalidateViewerLoad(0)
	fm.endBusy()

	recordReopenPath(fm.currentPath)
	clearFileManagerWindowHighlights()

	// Remove from registry
	unregisterFileManagerWindow(fm)
	remaining := atomic.AddInt32(&windowCount, -1)

	debugPrint("WindowLifecycle: Window closed, remaining windows: %d", remaining)

	// Stop directory watcher
	if fm.dirWatcher != nil {
		fm.dirWatcher.Stop()
	}
	if fm.iconSvc != nil {
		fm.iconSvc.Close()
	}

	// Stop blinking indicator if active
	fm.stopJobsBlink()

	// Unsubscribe from jobs updates for this window.
	if fm.jobsUnsub != nil {
		fm.jobsUnsub()
		fm.jobsUnsub = nil
	}
	if fm.promptUnregister != nil {
		fm.promptUnregister()
		fm.promptUnregister = nil
	}
	fm.releaseTransferDestinationSubscription(0)

	// Close the window
	if fm.window != nil {
		fm.window.Close()
	}

	// If this was the last window, quit the application
	if remaining == 0 {
		debugPrint("WindowLifecycle: Last window closed, quitting application")
		if fm.runtime != nil {
			fm.runtime.Close()
		}
		if app := fyne.CurrentApp(); app != nil {
			app.Quit()
		}
	}
}

func (fm *FileManager) installTransferDestinationSubscription(unsubscribe func()) (uint64, bool) {
	if fm == nil || unsubscribe == nil {
		return 0, false
	}
	fm.lifecycleMu.Lock()
	if fm.closed {
		fm.lifecycleMu.Unlock()
		unsubscribe()
		return 0, false
	}
	previous := fm.transferDestUnsub
	fm.transferDestSubID++
	id := fm.transferDestSubID
	fm.transferDestUnsub = unsubscribe
	fm.lifecycleMu.Unlock()
	if previous != nil {
		previous()
	}
	return id, true
}

// releaseTransferDestinationSubscription releases the matching dialog
// subscription. An id of zero releases whichever subscription the window owns.
func (fm *FileManager) releaseTransferDestinationSubscription(id uint64) {
	if fm == nil {
		return
	}
	fm.lifecycleMu.Lock()
	if id != 0 && id != fm.transferDestSubID {
		fm.lifecycleMu.Unlock()
		return
	}
	unsubscribe := fm.transferDestUnsub
	fm.transferDestUnsub = nil
	fm.lifecycleMu.Unlock()
	if unsubscribe != nil {
		unsubscribe()
	}
}

func (fm *FileManager) beginWindowClose() bool {
	fm.lifecycleMu.Lock()
	defer fm.lifecycleMu.Unlock()
	if fm.closed {
		return false
	}
	fm.closed = true
	fm.quitConfirmationOpen = false
	return true
}

func (fm *FileManager) isWindowClosed() bool {
	if fm == nil {
		return true
	}
	fm.lifecycleMu.Lock()
	defer fm.lifecycleMu.Unlock()
	return fm.closed
}

func (fm *FileManager) beginQuitConfirmation() bool {
	fm.lifecycleMu.Lock()
	defer fm.lifecycleMu.Unlock()
	if fm.closed || fm.quitConfirmationOpen {
		return false
	}
	fm.quitConfirmationOpen = true
	return true
}

func (fm *FileManager) endQuitConfirmation() {
	fm.lifecycleMu.Lock()
	fm.quitConfirmationOpen = false
	fm.lifecycleMu.Unlock()
}

func windowCloseNeedsConfirmation(openWindows int32) bool {
	return openWindows <= 1
}

// QuitApplication handles application quit logic with confirmation dialog.
func (fm *FileManager) QuitApplication() {
	currentCount := atomic.LoadInt32(&windowCount)
	debugPrint("WindowLifecycle: QuitApplication called, current window count: %d", currentCount)

	if !windowCloseNeedsConfirmation(currentCount) {
		// Multiple windows open, just close current window
		fm.closeWindow()
	} else {
		// Last window, show confirmation dialog
		fm.showQuitConfirmationDialog()
	}
}

// showQuitConfirmationDialog shows a confirmation dialog before quitting.
func (fm *FileManager) showQuitConfirmationDialog() {
	if !fm.beginQuitConfirmation() {
		return
	}

	activeJobs := activeJobCount(fm.jobManager().List())
	dialog := ui.NewQuitConfirmDialog(fm.keyManager, debugPrint, activeJobs)
	dialog.ShowDialog(fm.window, func(confirmed bool) {
		fm.endQuitConfirmation()
		if confirmed {
			debugPrint("WindowLifecycle: User confirmed quit")
			fm.closeWindow()
			return
		}

		debugPrint("WindowLifecycle: User cancelled quit")
		// Return focus to file list after dialog closes
		fm.FocusFileList()
	})
}

func activeJobCount(snaps []jobs.JobSnapshot) int {
	count := 0
	for _, snap := range snaps {
		switch snap.Status {
		case jobs.StatusPending, jobs.StatusRunning:
			count++
		}
	}
	return count
}
