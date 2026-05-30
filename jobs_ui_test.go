package main

import "testing"

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
