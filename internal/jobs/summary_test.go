package jobs

import "testing"

func TestSummaryTracksActiveJobsAndFailureAcknowledgment(t *testing.T) {
	m := &Manager{
		current: &Job{ID: 1, Status: StatusRunning},
		queue:   []*Job{{ID: 2, Status: StatusPending}, {ID: 3, Status: StatusPending}},
		history: []*Job{
			{ID: 4, Status: StatusCompleted},
			{ID: 5, Status: StatusCanceled},
			{ID: 6, Status: StatusFailed, FailureAcknowledged: true},
			{ID: 7, Status: StatusFailed},
		},
	}
	if got, want := m.Summary(), (Summary{Pending: 2, Running: 1, HasUnacknowledgedFailure: true}); got != want {
		t.Fatalf("summary = %+v, want %+v", got, want)
	}
	if !m.AcknowledgeFailure(7) {
		t.Fatal("failure was not acknowledged")
	}
	if got, want := m.Summary(), (Summary{Pending: 2, Running: 1}); got != want {
		t.Fatalf("acknowledged summary = %+v, want %+v", got, want)
	}
	if got := (&Manager{}).Summary(); got != (Summary{}) {
		t.Fatalf("empty summary = %+v", got)
	}
}

func TestJobsWindowRowsAndSelectedDetails(t *testing.T) {
	done := &Job{ID: 2, Status: StatusCompleted, Sources: []string{"a", "b", "c"}, TotalFiles: 3, results: []Result{{Source: "a"}}}
	m := &Manager{current: &Job{ID: 1, Status: StatusRunning, Sources: done.Sources}, history: []*Job{done}}
	rows := m.ListRows()
	if len(rows) != 2 || rows[0].ID != 1 || rows[1].ID != 2 {
		t.Fatalf("rows = %#v", rows)
	}
	for _, row := range rows {
		if len(row.Sources) != 0 || len(row.Results) != 0 {
			t.Fatal("row copied job details")
		}
	}
	detail, ok := m.DisplayDetails(2, 1)
	if !ok || len(detail.Sources) != 1 || detail.Sources[0] != "a" || detail.TotalFiles != 3 || len(detail.Results) != 0 {
		t.Fatalf("details = %#v", detail)
	}
	detail.Sources[0] = "changed"
	if done.Sources[0] != "a" {
		t.Fatal("details modified job sources")
	}
}
