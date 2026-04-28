package main

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestShowJobsWindowReusesSingleton(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	t.Cleanup(func() {
		closeJobsWindow()
		jobsWindow = nil
	})

	showJobsWindow()
	first := jobsWindow
	if first == nil {
		t.Fatal("showJobsWindow should create a Jobs window")
	}

	showJobsWindow()
	if jobsWindow != first {
		t.Fatal("showJobsWindow should reuse the existing Jobs window")
	}
}

func TestCloseJobsWindowClearsSingleton(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	t.Cleanup(func() {
		closeJobsWindow()
		jobsWindow = nil
	})

	showJobsWindow()
	closeJobsWindow()

	if jobsWindow != nil {
		t.Fatal("closeJobsWindow should clear the Jobs window singleton")
	}
}
