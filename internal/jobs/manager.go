package jobs

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
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
	subscribers []func()
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
	m := &Manager{historyMax: 100}
	m.cond = sync.NewCond(&m.mu)
	go m.worker()
	dbg("manager created; worker started")
	return m
}

// Subscribe registers a callback called on state changes.
func (m *Manager) Subscribe(cb func()) {
	m.mu.Lock()
	m.subscribers = append(m.subscribers, cb)
	n := len(m.subscribers)
	m.mu.Unlock()
	dbg("subscriber added (total=%d)", n)
}

func (m *Manager) notify() {
	// call without holding the lock to avoid re-entrancy
	m.mu.Lock()
	subs := append([]func(){}, m.subscribers...)
	m.mu.Unlock()
	dbg("notify subscribers: %d", len(subs))
	for _, cb := range subs {
		// best-effort; UI should marshal to main thread as needed
		cb()
	}
}

// EnqueueCopy enqueues a copy job.
func (m *Manager) EnqueueCopy(sources []string, destDir string) *Job {
	return m.enqueue(TypeCopy, sources, destDir)
}

// EnqueueMove enqueues a move job.
func (m *Manager) EnqueueMove(sources []string, destDir string) *Job {
	return m.enqueue(TypeMove, sources, destDir)
}

func (m *Manager) enqueue(t Type, sources []string, destDir string) *Job {
	j := &Job{ID: atomic.AddInt64(&m.nextID, 1), Type: t, Sources: append([]string(nil), sources...), DestDir: destDir, Status: StatusPending, EnqueuedAt: time.Now()}
	j.ctx, j.cancel = contextWithCancel()
	j.TotalFiles = len(sources)

	m.mu.Lock()
	m.queue = append(m.queue, j)
	m.mu.Unlock()
	dbg("enqueue id=%d type=%s n=%d -> %s", j.ID, string(t), len(sources), destDir)
	m.notify()
	m.cond.Signal()
	return j
}

// Cancel cancels a job by ID.
func (m *Manager) Cancel(id int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	// pending in queue
	for i, j := range m.queue {
		if j.ID == id {
			j.mu.Lock()
			j.Status = StatusCanceled
			j.CompletedAt = time.Now()
			j.mu.Unlock()
			m.queue = append(m.queue[:i], m.queue[i+1:]...)
			dbg("cancel pending id=%d", id)
			m.addHistoryLocked(j)
			go m.notify()
			return true
		}
	}
	// currently running
	if m.current != nil && m.current.ID == id {
		m.current.Cancel()
		dbg("cancel running id=%d", id)
		go m.notify()
		return true
	}
	return false
}

// List returns snapshots of pending + possibly running job (head is running when active).
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
		j.mu.Unlock()
		dbg("start job id=%d", j.ID)
		m.notify()
		err := m.runJob(j)
		j.mu.Lock()
		if err != nil {
			if errors.Is(err, errCanceled) {
				j.Status = StatusCanceled
				dbg("job canceled id=%d after %d/%d", j.ID, j.DoneFiles, j.TotalFiles)
			} else {
				j.Status = StatusFailed
				j.Error = err.Error()
				dbg("job failed id=%d err=%v", j.ID, err)
			}
		} else {
			j.Status = StatusCompleted
			dbg("job completed id=%d done=%d", j.ID, j.DoneFiles)
		}
		j.CompletedAt = time.Now()
		j.mu.Unlock()
		m.notify()
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
	for i, src := range j.Sources {
		if canceled(j) {
			return errCanceled
		}
		j.mu.Lock()
		j.CurrentSource = src
		j.Message = ""
		j.mu.Unlock()
		dbg("job %d: process %s", j.ID, src)
		m.notify()
		if err := copyOrMovePath(j, src, j.DestDir); err != nil {
			// record failure detail
			fp := failingPath(err)
			j.mu.Lock()
			j.Failures = append(j.Failures, JobFailure{TopSource: src, Path: fp, Error: err.Error()})
			j.mu.Unlock()
			return err
		}
		j.mu.Lock()
		j.DoneFiles = i + 1
		j.mu.Unlock()
		dbg("job %d: done %d/%d", j.ID, j.DoneFiles, j.TotalFiles)
		m.notify()
	}
	return nil
}

// --- copying primitives ---

var errCanceled = errors.New("job canceled")

func canceled(j *Job) bool {
	select {
	case <-j.ctx.Done():
		return true
	default:
		return false
	}
}

// copyOrMovePath copies or moves a path (file or directory).
func copyOrMovePath(j *Job, src string, destDir string) error {
	fi, err := os.Lstat(src)
	if err != nil {
		return wrapPath(src, err)
	}
	base := filepath.Base(src)
	dst := filepath.Join(destDir, base)

	if fi.IsDir() {
		dbg("job %d: mkdir %s (mode=%v)", j.ID, dst, fi.Mode())
		if err := ensureDir(dst, fi.Mode()); err != nil {
			return wrapPath(dst, err)
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return wrapPath(src, err)
		}
		for _, e := range entries {
			if canceled(j) {
				return errCanceled
			}
			child := filepath.Join(src, e.Name())
			dbg("job %d: recurse %s -> %s", j.ID, child, dst)
			if err := copyOrMovePath(j, child, dst); err != nil {
				return err
			}
		}
		if j.Type == TypeMove {
			if canceled(j) {
				return errCanceled
			}
			// remove empty dir after moving children
			dbg("job %d: rmdir %s", j.ID, src)
			if err := os.Remove(src); err != nil {
				return wrapPath(src, err)
			}
		}
		return nil
	}

	// handle symlink as symlink
	if fi.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return wrapPath(src, err)
		}
		// remove existing destination symlink/file if exists
		_ = os.Remove(dst)
		dbg("job %d: symlink %s -> %s", j.ID, dst, target)
		if err := os.Symlink(target, dst); err != nil {
			return wrapPath(dst, err)
		}
		if j.Type == TypeMove {
			dbg("job %d: unlink %s", j.ID, src)
			if err := os.Remove(src); err != nil {
				return wrapPath(src, err)
			}
		}
		return nil
	}

	// regular file
	dbg("job %d: file %s -> %s", j.ID, src, dst)
	if err := copyFileWithCancel(j, src, dst, fi.Mode()); err != nil {
		return err
	}
	if j.Type == TypeMove {
		if canceled(j) {
			return errCanceled
		}
		dbg("job %d: remove %s", j.ID, src)
		if err := os.Remove(src); err != nil {
			return wrapPath(src, err)
		}
	}
	return nil
}

func ensureDir(path string, mode os.FileMode) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}
	// best-effort to set mode
	_ = os.Chmod(path, mode.Perm())
	return nil
}

func copyFileWithCancel(j *Job, src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return wrapPath(dst, err)
	}
	in, err := os.Open(src)
	if err != nil {
		return wrapPath(src, err)
	}
	defer in.Close()
	// create temp file then rename for atomic-ish replace
	tmp := dst + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return wrapPath(tmp, err)
	}
	buf := make([]byte, 1<<20) // 1 MiB
	for {
		if canceled(j) {
			out.Close()
			os.Remove(tmp)
			return errCanceled
		}
		n, rerr := in.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				out.Close()
				os.Remove(tmp)
				return wrapPath(tmp, werr)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			out.Close()
			os.Remove(tmp)
			return wrapPath(src, rerr)
		}
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return wrapPath(tmp, err)
	}
	if err := os.Chmod(tmp, mode.Perm()); err != nil {
		os.Remove(tmp)
		return wrapPath(tmp, err)
	}
	// Replace destination
	dbg("job %d: rename %s -> %s", j.ID, tmp, dst)
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return wrapPath(dst, err)
	}
	return nil
}

// --- error wrapping helpers ---

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
