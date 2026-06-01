package jobs

import (
	"context"
	"sync"
	"time"
)

// Type represents job type.
type Type string

const (
	TypeCopy   Type = "copy"
	TypeMove   Type = "move"
	TypeDelete Type = "delete"
)

// DeleteMode controls whether a delete job uses OS trash or permanent removal.
type DeleteMode string

const (
	DeleteModeTrash     DeleteMode = "trash"
	DeleteModePermanent DeleteMode = "permanent"
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

// Job holds a single copy/move/delete job.
type Job struct {
	// immutable fields
	ID              int64
	Type            Type
	Sources         []string // absolute/native source paths
	DestDir         string   // absolute/native destination directory
	DeleteMode      DeleteMode
	Resolver        ConflictResolver
	Options         TransferOptions
	conflictDefault ConflictAction

	// state
	mu                  sync.RWMutex
	Status              Status
	TotalFiles          int
	DoneFiles           int
	CurrentSource       string
	Message             string
	Error               string
	Failures            []JobFailure
	FailureAcknowledged bool
	EnqueuedAt          time.Time
	StartedAt           time.Time
	CompletedAt         time.Time
	CurrentFile         string
	CurrentBytes        int64
	CurrentTotalBytes   int64
	CurrentStartedAt    time.Time
	CurrentUpdatedAt    time.Time
	lastProgressNotify  time.Time
	progressNotify      func()

	// cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// TransferOptions controls copy/move execution details.
type TransferOptions struct {
	PreserveTimestamps bool
}

// ConflictAction is the user's choice when a destination path already exists.
type ConflictAction string

const (
	ConflictSkip             ConflictAction = "skip"
	ConflictRename           ConflictAction = "rename"
	ConflictAutoSuffix       ConflictAction = "auto_suffix"
	ConflictOverwriteIfNewer ConflictAction = "overwrite_if_newer"
	ConflictOverwrite        ConflictAction = "overwrite"
	ConflictCancelJob        ConflictAction = "cancel_job"
)

// ConflictResolver is called by the worker when a destination name collision is
// detected at execution time.
type ConflictResolver func(context.Context, ConflictRequest) ConflictResolution

// ConflictRequest describes a destination name collision.
type ConflictRequest struct {
	JobID          int64
	Type           Type
	SourcePath     string
	Destination    string
	SourceModified time.Time
	DestModified   time.Time
	SuggestedName  string
	SuggestedPath  string
	IsDir          bool
	DefaultAction  ConflictAction
	CanApplyToRest bool
}

// ConflictResolution contains the selected collision behavior.
type ConflictResolution struct {
	Action      ConflictAction
	NewName     string
	ApplyToRest bool
}

// Snapshot returns a copy of important fields for UI.
func (j *Job) Snapshot() JobSnapshot {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return JobSnapshot{
		ID:                  j.ID,
		Type:                j.Type,
		Status:              j.Status,
		TotalFiles:          j.TotalFiles,
		DoneFiles:           j.DoneFiles,
		CurrentSource:       j.CurrentSource,
		Message:             j.Message,
		Error:               j.Error,
		DestDir:             j.DestDir,
		DeleteMode:          j.DeleteMode,
		FailureAcknowledged: j.FailureAcknowledged,
		EnqueuedAt:          j.EnqueuedAt,
		StartedAt:           j.StartedAt,
		CompletedAt:         j.CompletedAt,
		CurrentFile:         j.CurrentFile,
		CurrentBytes:        j.CurrentBytes,
		CurrentTotalBytes:   j.CurrentTotalBytes,
		CurrentStartedAt:    j.CurrentStartedAt,
		CurrentUpdatedAt:    j.CurrentUpdatedAt,
		Sources:             append([]string(nil), j.Sources...),
		Failures:            append([]JobFailure(nil), j.Failures...),
	}
}

func (j *Job) beginFileProgress(path string, totalBytes int64) {
	now := time.Now()
	j.mu.Lock()
	j.CurrentFile = path
	j.CurrentBytes = 0
	j.CurrentTotalBytes = totalBytes
	j.CurrentStartedAt = now
	j.CurrentUpdatedAt = now
	j.lastProgressNotify = now
	notify := j.progressNotify
	j.mu.Unlock()

	if notify != nil {
		notify()
	}
}

func (j *Job) addFileProgress(bytes int64, force bool) {
	if bytes <= 0 && !force {
		return
	}
	now := time.Now()
	var notify func()
	j.mu.Lock()
	if bytes > 0 {
		j.CurrentBytes += bytes
		if j.CurrentTotalBytes > 0 && j.CurrentBytes > j.CurrentTotalBytes {
			j.CurrentBytes = j.CurrentTotalBytes
		}
	}
	j.CurrentUpdatedAt = now
	if force || j.lastProgressNotify.IsZero() || now.Sub(j.lastProgressNotify) >= progressNotifyInterval {
		j.lastProgressNotify = now
		notify = j.progressNotify
	}
	j.mu.Unlock()

	if notify != nil {
		notify()
	}
}

func (j *Job) completeFileProgress() {
	j.mu.Lock()
	total := j.CurrentTotalBytes
	current := j.CurrentBytes
	j.mu.Unlock()

	if total > 0 && current < total {
		j.addFileProgress(total-current, true)
		return
	}
	j.addFileProgress(0, true)
}

func (j *Job) clearFileProgressLocked() {
	j.CurrentFile = ""
	j.CurrentBytes = 0
	j.CurrentTotalBytes = 0
	j.CurrentStartedAt = time.Time{}
	j.CurrentUpdatedAt = time.Time{}
	j.lastProgressNotify = time.Time{}
}

// JobSnapshot is a read-only view for UI.
type JobSnapshot struct {
	ID                  int64
	Type                Type
	Status              Status
	Sources             []string
	DestDir             string
	DeleteMode          DeleteMode
	TotalFiles          int
	DoneFiles           int
	CurrentSource       string
	Message             string
	Error               string
	Failures            []JobFailure
	FailureAcknowledged bool
	EnqueuedAt          time.Time
	StartedAt           time.Time
	CompletedAt         time.Time
	CurrentFile         string
	CurrentBytes        int64
	CurrentTotalBytes   int64
	CurrentStartedAt    time.Time
	CurrentUpdatedAt    time.Time
}

// JobFailure records a single failing path and error message.
type JobFailure struct {
	TopSource string // top-level source item being processed when failure occurred
	Path      string // specific path that failed (may be a child inside a directory)
	Error     string
}
