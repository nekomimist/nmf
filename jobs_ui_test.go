package main

import (
	"slices"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/config"
	"nmf/internal/ui"
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

func TestBuildDestinationCandidatesIncludesHistoryTimestampsForOpenPaths(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	now := time.Now()
	state := &config.State{NavigationHistory: config.NavigationHistoryState{
		Entries: []string{"/current", "/other", "/frequent", "/recent"},
		LastUsed: map[string]time.Time{
			"/current":  now.Add(-time.Hour),
			"/other":    now.Add(-2 * time.Hour),
			"/frequent": now.Add(-24 * time.Hour),
			"/recent":   now,
		},
		UseCount: map[string]int{"/current": 1, "/other": 1, "/frequent": 100, "/recent": 1},
	}}
	fm := &FileManager{browser: newTestBrowser(testBrowserOptions{path: "/current"}), state: state, window: app.NewWindow("current")}
	other := &FileManager{browser: newTestBrowser(testBrowserOptions{path: "/other"}), window: app.NewWindow("other")}
	fm.runtime = &ApplicationRuntime{windows: &fileManagerWindowRegistry{windows: []*FileManager{fm, other}}}

	candidates := fm.buildDestinationCandidates()
	wantPaths := []string{"/other", "/current", "/frequent", "/recent"}
	gotPaths := make([]string, len(candidates))
	for i, candidate := range candidates {
		gotPaths[i] = candidate.Path
		if want := state.NavigationHistory.LastUsed[candidate.Path]; !candidate.LastUsed.Equal(want) {
			t.Fatalf("timestamp for %s = %v, want %v", candidate.Path, candidate.LastUsed, want)
		}
	}
	if !slices.Equal(gotPaths, wantPaths) {
		t.Fatalf("candidate paths = %v, want %v", gotPaths, wantPaths)
	}
	if !candidates[0].OpenInWindow || !candidates[1].OpenInWindow {
		t.Fatal("open paths should retain their metadata when deduplicated against history")
	}
	if candidates[2] != (ui.DestinationCandidate{Path: "/frequent", LastUsed: state.NavigationHistory.LastUsed["/frequent"]}) {
		t.Fatalf("history candidate = %#v, want path and timestamp only", candidates[2])
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
