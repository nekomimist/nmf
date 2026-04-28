package ui

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestJobsWindowCloseIsIdempotent(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	jw := NewJobsWindow(app, func(string, ...interface{}) {})
	closedCount := 0
	jw.SetOnClosed(func() { closedCount++ })

	jw.Show()
	jw.Close()
	jw.Close()

	if !jw.Closed() {
		t.Fatal("JobsWindow should report closed after Close")
	}
	if closedCount != 1 {
		t.Fatalf("onClosed count = %d, want 1", closedCount)
	}
}

func TestJobsWindowUsesJobsTitle(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	jw := NewJobsWindow(app, func(string, ...interface{}) {})

	if got := jw.Window().Title(); got != "Jobs" {
		t.Fatalf("window title = %q, want Jobs", got)
	}
}
