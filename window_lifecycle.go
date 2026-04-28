package main

import (
	"sync/atomic"

	"fyne.io/fyne/v2"

	"nmf/internal/ui"
)

// closeWindow handles window closing logic.
func (fm *FileManager) closeWindow() {
	// Remove from registry
	windowRegistry.Delete(fm.window)
	remaining := atomic.AddInt32(&windowCount, -1)

	debugPrint("WindowLifecycle: Window closed, remaining windows: %d", remaining)

	// Stop directory watcher
	if fm.dirWatcher != nil {
		fm.dirWatcher.Stop()
	}

	// Stop blinking indicator if active
	fm.stopJobsBlink()

	// Unsubscribe from jobs updates for this window.
	if fm.jobsUnsub != nil {
		fm.jobsUnsub()
		fm.jobsUnsub = nil
	}

	// Close the window
	fm.window.Close()

	// If this was the last window, quit the application
	if remaining == 0 {
		debugPrint("WindowLifecycle: Last window closed, quitting application")
		closeJobsWindow()
		fyne.CurrentApp().Quit()
	}
}

// QuitApplication handles application quit logic with confirmation dialog.
func (fm *FileManager) QuitApplication() {
	currentCount := atomic.LoadInt32(&windowCount)
	debugPrint("WindowLifecycle: QuitApplication called, current window count: %d", currentCount)

	if currentCount > 1 {
		// Multiple windows open, just close current window
		fm.closeWindow()
	} else {
		// Last window, show confirmation dialog
		fm.showQuitConfirmationDialog()
	}
}

// showQuitConfirmationDialog shows a confirmation dialog before quitting.
func (fm *FileManager) showQuitConfirmationDialog() {
	dialog := ui.NewQuitConfirmDialog(fm.keyManager, debugPrint)
	dialog.ShowDialog(fm.window, func(confirmed bool) {
		if confirmed {
			debugPrint("WindowLifecycle: User confirmed quit")
			fm.closeWindow()
		} else {
			debugPrint("WindowLifecycle: User cancelled quit")
		}
		// Return focus to file list after dialog closes
		fm.FocusFileList()
	})
}
