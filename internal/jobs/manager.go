package jobs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// debug hook, set from main; should print only when -d enabled
var debugf func(format string, args ...interface{})

// SetDebug installs a debug logger used when -d flag is on.
func SetDebug(fn func(format string, args ...interface{})) { debugf = fn }

func dbg(format string, args ...interface{}) {
	if debugf != nil {
		debugf("jobs: "+format, args...)
	}
}

// Manager coordinates queueing and background processing (single worker).
type Manager struct {
	mu          sync.Mutex
	cond        *sync.Cond
	queue       []*Job
	running     bool
	closed      bool
	nextID      int64
	nextSubID   int64
	subscribers map[int64]func()
	current     *Job
	history     []*Job
	historyMax  int
}

var (
	defaultManager *Manager
	once           sync.Once
)

// GetManager returns a singleton Manager.
func GetManager() *Manager {
	once.Do(func() { defaultManager = NewManager() })
	return defaultManager
}

// NewManager constructs and starts a Manager.
func NewManager() *Manager {
	m := &Manager{
		historyMax:  100,
		subscribers: make(map[int64]func()),
	}
	m.cond = sync.NewCond(&m.mu)
	go m.worker()
	dbg("manager created; worker started")
	return m
}

// Subscribe registers a callback called on state changes.
func (m *Manager) Subscribe(cb func()) func() {
	if cb == nil {
		return func() {}
	}

	m.mu.Lock()
	m.nextSubID++
	id := m.nextSubID
	if m.subscribers == nil {
		m.subscribers = make(map[int64]func())
	}
	m.subscribers[id] = cb
	n := len(m.subscribers)
	m.mu.Unlock()
	dbg("subscriber added (total=%d)", n)

	var once sync.Once
	return func() {
		once.Do(func() {
			m.mu.Lock()
			if m.subscribers != nil {
				delete(m.subscribers, id)
			}
			n := len(m.subscribers)
			m.mu.Unlock()
			dbg("subscriber removed (total=%d)", n)
		})
	}
}

func (m *Manager) notify() {
	// call without holding the lock to avoid re-entrancy
	m.mu.Lock()
	subs := make([]func(), 0, len(m.subscribers))
	for _, cb := range m.subscribers {
		subs = append(subs, cb)
	}
	m.mu.Unlock()
	dbg("notify subscribers: %d", len(subs))
	for _, cb := range subs {
		// best-effort; UI should marshal to main thread as needed
		if cb != nil {
			cb()
		}
	}
}

// EnqueueCopy enqueues a copy job.
func (m *Manager) EnqueueCopy(sources []string, destDir string) *Job {
	return m.EnqueueCopyWithResolver(sources, destDir, nil)
}

// EnqueueMove enqueues a move job.
func (m *Manager) EnqueueMove(sources []string, destDir string) *Job {
	return m.EnqueueMoveWithResolver(sources, destDir, nil)
}

// EnqueueExtract enqueues an archive extraction job.
func (m *Manager) EnqueueExtract(sources []string, destDir string) *Job {
	return m.EnqueueExtractWithResolver(sources, destDir, nil)
}

// EnqueueDelete enqueues a delete job.
func (m *Manager) EnqueueDelete(sources []string, mode DeleteMode) *Job {
	return m.enqueueDelete(sources, mode)
}

// EnqueueCopyWithResolver enqueues a copy job with an optional collision resolver.
func (m *Manager) EnqueueCopyWithResolver(sources []string, destDir string, resolver ConflictResolver) *Job {
	return m.EnqueueCopyWithOptions(sources, destDir, resolver, TransferOptions{})
}

// EnqueueCopyWithOptions enqueues a copy job with transfer options.
func (m *Manager) EnqueueCopyWithOptions(sources []string, destDir string, resolver ConflictResolver, options TransferOptions) *Job {
	return m.enqueue(TypeCopy, sources, destDir, resolver, options)
}

// EnqueueMoveWithResolver enqueues a move job with an optional collision resolver.
func (m *Manager) EnqueueMoveWithResolver(sources []string, destDir string, resolver ConflictResolver) *Job {
	return m.enqueue(TypeMove, sources, destDir, resolver, TransferOptions{PreserveTimestamps: true})
}

// EnqueueExtractWithResolver enqueues an archive extraction job with an optional collision resolver.
func (m *Manager) EnqueueExtractWithResolver(sources []string, destDir string, resolver ConflictResolver) *Job {
	return m.EnqueueExtractWithOptions(sources, destDir, resolver, TransferOptions{})
}

// EnqueueExtractWithOptions enqueues an archive extraction job with transfer options.
func (m *Manager) EnqueueExtractWithOptions(sources []string, destDir string, resolver ConflictResolver, options TransferOptions) *Job {
	return m.enqueue(TypeExtract, sources, destDir, resolver, options)
}

func (m *Manager) enqueue(t Type, sources []string, destDir string, resolver ConflictResolver, options TransferOptions) *Job {
	j := &Job{ID: atomic.AddInt64(&m.nextID, 1), Type: t, Sources: append([]string(nil), sources...), DestDir: destDir, Resolver: resolver, Options: options, Status: StatusPending, EnqueuedAt: time.Now()}
	j.ctx, j.cancel = contextWithCancel()
	j.TotalFiles = len(sources)

	m.mu.Lock()
	m.queue = append(m.queue, j)
	m.mu.Unlock()
	dbg("enqueue id=%d type=%s n=%d preserve_timestamps=%t -> %s", j.ID, string(t), len(sources), options.PreserveTimestamps, destDir)
	m.notify()
	m.cond.Signal()
	return j
}

func (m *Manager) enqueueDelete(sources []string, mode DeleteMode) *Job {
	if mode == "" {
		mode = DeleteModeTrash
	}
	j := &Job{
		ID:         atomic.AddInt64(&m.nextID, 1),
		Type:       TypeDelete,
		Sources:    append([]string(nil), sources...),
		DeleteMode: mode,
		Status:     StatusPending,
		EnqueuedAt: time.Now(),
	}
	j.ctx, j.cancel = contextWithCancel()
	j.TotalFiles = len(sources)

	m.mu.Lock()
	m.queue = append(m.queue, j)
	m.mu.Unlock()
	dbg("enqueue id=%d type=%s mode=%s n=%d", j.ID, string(TypeDelete), string(mode), len(sources))
	m.notify()
	m.cond.Signal()
	return j
}

// Cancel cancels a job by ID.
func (m *Manager) Cancel(id int64) bool {
	m.mu.Lock()
	// pending in queue
	for i, j := range m.queue {
		if j.ID == id {
			j.mu.Lock()
			j.Status = StatusCanceled
			j.CompletedAt = time.Now()
			callbacks, snapshot := j.takeFinishedCallbacksLocked()
			j.mu.Unlock()
			m.queue = append(m.queue[:i], m.queue[i+1:]...)
			dbg("cancel pending id=%d", id)
			m.addHistoryLocked(j)
			m.mu.Unlock()
			m.notify()
			invokeFinishedCallbacks(callbacks, snapshot)
			return true
		}
	}
	// currently running
	if m.current != nil && m.current.ID == id {
		m.current.Cancel()
		dbg("cancel running id=%d", id)
		m.mu.Unlock()
		m.notify()
		return true
	}
	m.mu.Unlock()
	return false
}

// Summary contains only the state needed by per-window job indicators.
type Summary struct {
	Pending                  int
	Running                  int
	HasUnacknowledgedFailure bool
}

// Summary avoids copying sources, results, and failures on every progress update.
func (m *Manager) Summary() Summary {
	m.mu.Lock()
	defer m.mu.Unlock()
	var summary Summary
	add := func(j *Job) {
		j.mu.RLock()
		defer j.mu.RUnlock()
		switch j.Status {
		case StatusPending:
			summary.Pending++
		case StatusRunning:
			summary.Running++
		case StatusFailed:
			summary.HasUnacknowledgedFailure = summary.HasUnacknowledgedFailure || !j.FailureAcknowledged
		}
	}
	if m.current != nil {
		add(m.current)
	}
	for _, j := range m.queue {
		add(j)
	}
	for _, j := range m.history {
		add(j)
	}
	return summary
}

// List returns detailed snapshots of running, pending, and historical jobs.
func (m *Manager) List() []JobSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]JobSnapshot, 0, len(m.queue)+1+len(m.history))
	if m.current != nil {
		out = append(out, m.current.Snapshot())
	}
	for _, j := range m.queue {
		out = append(out, j.Snapshot())
	}
	for i := len(m.history) - 1; i >= 0; i-- {
		out = append(out, m.history[i].Snapshot())
	}
	return out
}

// AcknowledgeFailure marks a failed job as seen by the user.
func (m *Manager) AcknowledgeFailure(id int64) bool {
	if id == 0 {
		return false
	}

	var changed bool
	m.mu.Lock()
	for _, j := range m.allJobsLocked() {
		if j.ID != id {
			continue
		}
		j.mu.Lock()
		if j.Status == StatusFailed && !j.FailureAcknowledged {
			j.FailureAcknowledged = true
			changed = true
		}
		j.mu.Unlock()
		break
	}
	m.mu.Unlock()

	if changed {
		m.notify()
	}
	return changed
}

func (m *Manager) allJobsLocked() []*Job {
	out := make([]*Job, 0, len(m.queue)+len(m.history)+1)
	if m.current != nil {
		out = append(out, m.current)
	}
	out = append(out, m.queue...)
	out = append(out, m.history...)
	return out
}

func (m *Manager) worker() {
	for {
		m.mu.Lock()
		for len(m.queue) == 0 && !m.closed {
			dbg("worker waiting (queue=0, closed=%t)", m.closed)
			m.cond.Wait()
		}
		if m.closed {
			m.mu.Unlock()
			return
		}
		// pop head
		j := m.queue[0]
		m.queue = m.queue[1:]
		m.current = j
		dbg("worker popped id=%d type=%s (remaining=%d)", j.ID, string(j.Type), len(m.queue))
		m.mu.Unlock()

		// run job serially
		j.mu.Lock()
		j.Status = StatusRunning
		j.StartedAt = time.Now()
		j.progressNotify = m.notify
		j.mu.Unlock()
		dbg("start job id=%d", j.ID)
		m.notify()
		err := m.runJob(j)
		j.mu.Lock()
		j.progressNotify = nil
		if err != nil {
			if errors.Is(err, errCanceled) {
				j.Status = StatusCanceled
				dbg("job canceled id=%d after %d/%d", j.ID, j.DoneFiles, j.TotalFiles)
			} else {
				j.Status = StatusFailed
				j.Error = err.Error()
				dbg("%s", formatJobFailureDebug(j, err))
			}
		} else {
			j.Status = StatusCompleted
			dbg("job completed id=%d done=%d", j.ID, j.DoneFiles)
		}
		j.CompletedAt = time.Now()
		callbacks, snapshot := j.takeFinishedCallbacksLocked()
		j.mu.Unlock()
		m.notify()
		invokeFinishedCallbacks(callbacks, snapshot)
		m.mu.Lock()
		m.current = nil
		m.addHistoryLocked(j)
		m.mu.Unlock()
	}
}

// addHistoryLocked appends a finished job to history and trims oldest; caller must hold m.mu
func (m *Manager) addHistoryLocked(j *Job) {
	m.history = append(m.history, j)
	if m.historyMax > 0 && len(m.history) > m.historyMax {
		drop := len(m.history) - m.historyMax
		if drop > 0 {
			m.history = append([]*Job{}, m.history[drop:]...)
		}
	}
}

// runJob processes one job.
func (m *Manager) runJob(j *Job) error {
	dbg("runJob id=%d total=%d dest=%s", j.ID, len(j.Sources), j.DestDir)
	if j.Type == TypeDelete {
		return m.runDeleteJob(j)
	}
	if j.Type == TypeExtract {
		return m.runExtractJob(j)
	}
	destPath, execCtx, err := openTransferDestination(j.DestDir)
	if err != nil {
		return err
	}
	defer func() {
		if err := execCtx.close(); err != nil {
			dbg("job %d: execution context close error: %v", j.ID, err)
		}
	}()
	for i, src := range j.Sources {
		if canceled(j) {
			return errCanceled
		}
		j.mu.Lock()
		j.CurrentSource = src
		j.Message = ""
		j.clearFileProgressLocked()
		j.mu.Unlock()
		dbg("job %d: process %s", j.ID, src)
		m.notify()
		err := transferSource(j, execCtx, src, destPath)
		if err != nil {
			if errors.Is(err, errSkipped) {
				dbg("job %d: skipped %s", j.ID, src)
				j.mu.Lock()
				j.DoneFiles = i + 1
				j.clearFileProgressLocked()
				j.mu.Unlock()
				m.notify()
				continue
			}
			// record failure detail
			fp := failingPath(err)
			j.mu.Lock()
			j.Failures = append(j.Failures, JobFailure{TopSource: src, Path: fp, Error: err.Error()})
			j.mu.Unlock()
			return err
		}
		j.mu.Lock()
		j.DoneFiles = i + 1
		j.clearFileProgressLocked()
		j.mu.Unlock()
		dbg("job %d: done %d/%d", j.ID, j.DoneFiles, j.TotalFiles)
		m.notify()
	}
	return nil
}

func (m *Manager) runDeleteJob(j *Job) error {
	execCtx := newExecutionContext()
	defer func() {
		if err := execCtx.close(); err != nil {
			dbg("job %d: SMB session close error: %v", j.ID, err)
		}
	}()

	for i, src := range j.Sources {
		if canceled(j) {
			return errCanceled
		}
		j.mu.Lock()
		j.CurrentSource = src
		j.Message = string(j.DeleteMode)
		j.mu.Unlock()
		m.notify()

		result, isDirectory := topLevelDirectoryResult(execCtx, src)
		var err error
		if j.DeleteMode == DeleteModePermanent {
			err = deletePermanentPath(j, execCtx, src)
		} else {
			err = trashPath(j.ctx, src)
		}
		if err != nil {
			fp := failingPath(err)
			j.mu.Lock()
			j.Failures = append(j.Failures, JobFailure{TopSource: src, Path: fp, Error: err.Error()})
			j.mu.Unlock()
			return err
		}
		if isDirectory {
			j.addResult(result)
		}
		j.mu.Lock()
		j.DoneFiles = i + 1
		j.mu.Unlock()
		m.notify()
	}
	return nil
}

func (m *Manager) runExtractJob(j *Job) error {
	destPath, err := resolveExecutionPath(j.DestDir)
	if err != nil {
		return wrapPath(j.DestDir, err)
	}
	if destPath.backend == backendArchive {
		return wrapPath(destPath.displayPath(), errors.New("archive destinations are read-only"))
	}

	execCtx := newExecutionContext()
	defer func() {
		if err := execCtx.close(); err != nil {
			dbg("job %d: SMB session close error: %v", j.ID, err)
		}
	}()
	if err := validateDestinationDirectory(execCtx, destPath); err != nil {
		return err
	}

	for i, src := range j.Sources {
		if canceled(j) {
			return errCanceled
		}
		j.mu.Lock()
		j.CurrentSource = src
		j.Message = "extract"
		j.clearFileProgressLocked()
		j.mu.Unlock()
		m.notify()

		if err := extractArchivePath(j, execCtx, src, destPath); err != nil {
			if errors.Is(err, errSkipped) {
				dbg("job %d: skipped extract %s", j.ID, src)
				j.mu.Lock()
				j.DoneFiles = i + 1
				j.clearFileProgressLocked()
				j.mu.Unlock()
				m.notify()
				continue
			}
			fp := failingPath(err)
			j.mu.Lock()
			j.Failures = append(j.Failures, JobFailure{TopSource: src, Path: fp, Error: err.Error()})
			j.mu.Unlock()
			return err
		}

		j.mu.Lock()
		j.DoneFiles = i + 1
		j.clearFileProgressLocked()
		j.mu.Unlock()
		m.notify()
	}
	return nil
}

var errCanceled = errors.New("job canceled")

var errSkipped = errors.New("job item skipped")

func canceled(j *Job) bool {
	select {
	case <-j.ctx.Done():
		return true
	default:
		return false
	}
}

type opError struct {
	Path string
	Err  error
}

func (e opError) Error() string { return e.Path + ": " + e.Err.Error() }

func (e opError) Unwrap() error { return e.Err }

func wrapPath(p string, err error) error {
	if err == nil {
		return nil
	}
	return opError{Path: p, Err: err}
}

func failingPath(err error) string {
	var oe opError
	if errors.As(err, &oe) {
		return oe.Path
	}
	return ""
}

func formatJobFailureDebug(j *Job, err error) string {
	message := fmt.Sprintf(
		"job failed id=%d type=%s done=%d/%d source=%q dest=%q delete_mode=%q path=%q",
		j.ID,
		j.Type,
		j.DoneFiles,
		j.TotalFiles,
		j.CurrentSource,
		j.DestDir,
		j.DeleteMode,
		failingPath(err),
	)
	var errno syscall.Errno
	if errors.As(err, &errno) {
		message += fmt.Sprintf(" errno=%d", uint64(errno))
	}
	return message + fmt.Sprintf(" error=%q", err.Error())
}

// exported cancel for running job (owner keeps pointer)
func (j *Job) Cancel() {
	if j.cancel != nil {
		j.cancel()
	}
}

// context helper separated for testability
func contextWithCancel() (ctx context.Context, cancel func()) {
	return context.WithCancel(context.Background())
}
