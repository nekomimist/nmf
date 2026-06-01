package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/jobs"
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

func TestJobsWindowCompletedDetailsSummarizeTargets(t *testing.T) {
	sources := make([]string, 12)
	for i := range sources {
		sources[i] = fmt.Sprintf("/tmp/source-%02d.txt", i+1)
	}
	jw := &JobsWindow{
		details:     widget.NewLabel(""),
		items:       []jobs.JobSnapshot{{ID: 7, Type: jobs.TypeCopy, Status: jobs.StatusCompleted, Sources: sources, DestDir: "/tmp/dst", DoneFiles: 12, TotalFiles: 12}},
		selectedIdx: 0,
	}

	jw.updateDetails()
	got := jw.details.Text

	for i := 1; i <= 10; i++ {
		want := fmt.Sprintf("source-%02d.txt", i)
		if !strings.Contains(got, want) {
			t.Fatalf("completed details missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "source-11.txt") {
		t.Fatalf("completed details should omit target beyond limit:\n%s", got)
	}
	if !strings.Contains(got, "... and 2 more") {
		t.Fatalf("completed details missing remaining count:\n%s", got)
	}
}

func TestJobsWindowCompletedDetailsDoesNotAddMoreLineAtLimit(t *testing.T) {
	sources := []string{
		"/tmp/one.txt",
		"/tmp/two.txt",
	}
	jw := &JobsWindow{
		details:     widget.NewLabel(""),
		items:       []jobs.JobSnapshot{{ID: 8, Type: jobs.TypeDelete, Status: jobs.StatusCompleted, Sources: sources, DeleteMode: jobs.DeleteModeTrash, DoneFiles: 2, TotalFiles: 2}},
		selectedIdx: 0,
	}

	jw.updateDetails()
	got := jw.details.Text

	if !strings.Contains(got, "Targets:") || !strings.Contains(got, "one.txt") || !strings.Contains(got, "two.txt") {
		t.Fatalf("completed details missing target names:\n%s", got)
	}
	if strings.Contains(got, "... and") {
		t.Fatalf("completed details should not show a remaining count:\n%s", got)
	}
}

func TestJobsWindowRunningDetailsShowsCurrentFileProgress(t *testing.T) {
	started := time.Now().Add(-2 * time.Second)
	updated := started.Add(2 * time.Second)
	jw := &JobsWindow{
		details: widget.NewLabel(""),
		items: []jobs.JobSnapshot{{
			ID:                9,
			Type:              jobs.TypeCopy,
			Status:            jobs.StatusRunning,
			DestDir:           "/tmp/dst",
			DoneFiles:         0,
			TotalFiles:        1,
			CurrentFile:       "/tmp/source/big.bin",
			CurrentBytes:      1024,
			CurrentTotalBytes: 2048,
			CurrentStartedAt:  started,
			CurrentUpdatedAt:  updated,
		}},
		selectedIdx: 0,
	}

	jw.updateDetails()
	got := jw.details.Text

	for _, want := range []string{"Current: big.bin", "1.0 KiB / 2.0 KiB (50.0%)", "512 B/s", "ETA 00:02"} {
		if !strings.Contains(got, want) {
			t.Fatalf("running details missing %q in:\n%s", want, got)
		}
	}
}

func TestRunningProgressSummaryIncludesPercentAndETA(t *testing.T) {
	started := time.Now().Add(-4 * time.Second)
	it := jobs.JobSnapshot{
		Status:            jobs.StatusRunning,
		CurrentFile:       "/tmp/source/big.bin",
		CurrentBytes:      2048,
		CurrentTotalBytes: 4096,
		CurrentStartedAt:  started,
		CurrentUpdatedAt:  started.Add(4 * time.Second),
	}

	got := runningProgressSummary(it)
	if !strings.Contains(got, "50.0%") || !strings.Contains(got, "ETA 00:04") {
		t.Fatalf("runningProgressSummary = %q, want percent and ETA", got)
	}
}

func TestRunningProgressSummaryFallsBackToBytesWithoutTotal(t *testing.T) {
	it := jobs.JobSnapshot{
		Status:       jobs.StatusRunning,
		CurrentFile:  "/tmp/source/stream.bin",
		CurrentBytes: 1536,
	}

	if got := runningProgressSummary(it); got != "1.5 KiB" {
		t.Fatalf("runningProgressSummary = %q, want byte fallback", got)
	}
}
