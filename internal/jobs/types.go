package jobs

import (
	"context"
	"sync"
	"time"
)

// Type represents job type.
type Type string

const (
	TypeCopy Type = "copy"
	TypeMove Type = "move"
)

// Status represents job status.
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCanceled  Status = "canceled"
)

// Job holds a single copy/move job.
type Job struct {
	// immutable fields
	ID      int64
	Type    Type
	Sources []string // absolute/native source paths
	DestDir string   // absolute/native destination directory

	// state
	mu            sync.RWMutex
	Status        Status
	TotalFiles    int
	DoneFiles     int
	CurrentSource string
	Message       string
	Error         string
	Failures      []JobFailure
	EnqueuedAt    time.Time
	StartedAt     time.Time
	CompletedAt   time.Time

	// cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// Snapshot returns a copy of important fields for UI.
func (j *Job) Snapshot() JobSnapshot {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return JobSnapshot{
		ID:            j.ID,
		Type:          j.Type,
		Status:        j.Status,
		TotalFiles:    j.TotalFiles,
		DoneFiles:     j.DoneFiles,
		CurrentSource: j.CurrentSource,
		Message:       j.Message,
		Error:         j.Error,
		DestDir:       j.DestDir,
		EnqueuedAt:    j.EnqueuedAt,
		StartedAt:     j.StartedAt,
		CompletedAt:   j.CompletedAt,
		Sources:       append([]string(nil), j.Sources...),
	}
}

// JobSnapshot is a read-only view for UI.
type JobSnapshot struct {
	ID            int64
	Type          Type
	Status        Status
	Sources       []string
	DestDir       string
	TotalFiles    int
	DoneFiles     int
	CurrentSource string
	Message       string
	Error         string
	Failures      []JobFailure
	EnqueuedAt    time.Time
	StartedAt     time.Time
	CompletedAt   time.Time
}

// JobFailure records a single failing path and error message.
type JobFailure struct {
	TopSource string // top-level source item being processed when failure occurred
	Path      string // specific path that failed (may be a child inside a directory)
	Error     string
}
