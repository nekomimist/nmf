package main

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/config"
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

func TestJobsBlinkQueuedUpdateHonorsLifecycle(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	for _, state := range []string{"active", "closed", "stopped"} {
		t.Run(state, func(t *testing.T) {
			fm := &FileManager{jobsButton: widget.NewButton("Jobs", nil)}
			fm.jobsButton.Importance = widget.HighImportance
			stop := make(chan struct{})
			queued := fm.jobsBlinkUpdate(stop, false)
			want := widget.HighImportance
			switch state {
			case "active":
				want = widget.MediumImportance
			case "closed":
				fm.closed = true
			case "stopped":
				close(stop)
			}
			queued()
			if got := fm.jobsButton.Importance; got != want {
				t.Fatalf("queued blink importance = %v, want %v", got, want)
			}
		})
	}
}

func TestBuildDestinationCandidatesMarksCurrentWindowPathOpen(t *testing.T) {
	fm := &FileManager{
		browser: newTestBrowser(testBrowserOptions{path: "/current"}),
		state:   &config.State{},
	}

	candidates := fm.buildDestinationCandidates()

	if len(candidates) != 1 {
		t.Fatalf("destination candidates = %#v, want current path only", candidates)
	}
	if candidates[0].Path != "/current" || !candidates[0].OpenInWindow {
		t.Fatalf("current destination = %#v, want open current path", candidates[0])
	}
}
