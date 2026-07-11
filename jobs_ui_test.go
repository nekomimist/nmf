package main

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestJobsButtonText(t *testing.T) {
	tests := []struct {
		name          string
		remainingJobs int
		want          string
	}{
		{name: "none", remainingJobs: 0, want: "Jobs"},
		{name: "negative", remainingJobs: -1, want: "Jobs"},
		{name: "one", remainingJobs: 1, want: "Jobs (1)"},
		{name: "multiple", remainingJobs: 3, want: "Jobs (3)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := jobsButtonText(tt.remainingJobs); got != tt.want {
				t.Fatalf("jobsButtonText(%d) = %q, want %q", tt.remainingJobs, got, tt.want)
			}
		})
	}
}

func TestJobsBlinkDropsTicksAfterWindowClose(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	fm := &FileManager{
		window:     app.NewWindow("closed"),
		jobsButton: widget.NewButton("Jobs", nil),
		closed:     true,
	}
	fm.jobsButton.Importance = widget.MediumImportance

	fm.startJobsBlink()
	time.Sleep(650 * time.Millisecond)
	fm.stopJobsBlink()

	if fm.jobsButton.Importance != widget.MediumImportance {
		t.Fatalf("closed window jobs importance = %v, want unchanged", fm.jobsButton.Importance)
	}
}
