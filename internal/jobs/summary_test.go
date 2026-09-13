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
