package main

import (
	"fyne.io/fyne/v2"

	"nmf/internal/ui"
)

// JobsWindowController owns the single Jobs window shared across every
// FileManager window in the app. One instance is created at bootstrap and
// injected into each FileManager (including windows opened later via
// Ctrl-N / ReopenClosedWindow), mirroring how *watcher.WatchHub is shared.
//
// Show/Close are only ever invoked from Fyne UI callbacks, so this type is
// not safe for concurrent use and relies on everything running on the Fyne
// main goroutine, same as before.
type JobsWindowController struct {
	app        fyne.App
	debugPrint func(format string, args ...interface{})
	window     *ui.JobsWindow
}

// NewJobsWindowController creates a controller for the shared Jobs window.
func NewJobsWindowController(app fyne.App, debugPrint func(format string, args ...interface{})) *JobsWindowController {
	return &JobsWindowController{app: app, debugPrint: debugPrint}
}

// Show lazily creates (or recreates, if the previous one was closed) the
// shared Jobs window and brings it to the front.
func (c *JobsWindowController) Show() {
	if c.window == nil || c.window.Closed() {
		c.window = ui.NewJobsWindow(c.app, c.debugPrint)
		current := c.window
		c.window.SetOnClosed(func() {
			if c.window == current {
				c.window = nil
			}
		})
	}
	c.window.Show()
}

// Close closes the shared Jobs window, if one is open.
func (c *JobsWindowController) Close() {
	if c.window == nil {
		return
	}
	current := c.window
	c.window = nil
	current.Close()
}
