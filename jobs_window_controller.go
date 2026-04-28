package main

import (
	"fyne.io/fyne/v2"

	"nmf/internal/ui"
)

var jobsWindow *ui.JobsWindow

func showJobsWindow() {
	if jobsWindow == nil || jobsWindow.Closed() {
		jobsWindow = ui.NewJobsWindow(fyne.CurrentApp(), debugPrint)
		current := jobsWindow
		jobsWindow.SetOnClosed(func() {
			if jobsWindow == current {
				jobsWindow = nil
			}
		})
	}
	jobsWindow.Show()
}

func closeJobsWindow() {
	if jobsWindow == nil {
		return
	}
	current := jobsWindow
	jobsWindow = nil
	current.Close()
}
