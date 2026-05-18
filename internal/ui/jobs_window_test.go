package ui

import (
	"fmt"
	"strings"
	"testing"

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
