package main

import (
	"testing"

	"nmf/internal/jobs"
)

func TestActiveJobCountCountsOnlyPendingAndRunning(t *testing.T) {
	snaps := []jobs.JobSnapshot{
		{Status: jobs.StatusPending},
		{Status: jobs.StatusRunning},
		{Status: jobs.StatusCompleted},
		{Status: jobs.StatusFailed},
		{Status: jobs.StatusCanceled},
	}

	if got := activeJobCount(snaps); got != 2 {
		t.Fatalf("activeJobCount = %d, want 2", got)
	}
}
