package jobs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"nmf/internal/fileinfo"
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
	if j.Type == TypeDelete {
		return m.runDeleteJob(j)
	}
	if j.Type == TypeExtract {
		return m.runExtractJob(j)
	}
	destPath, err := resolveExecutionPath(j.DestDir)
	if err != nil {
		return wrapPath(j.DestDir, err)
	}
	execCtx := newExecutionContext()
	defer func() {
		if err := execCtx.close(); err != nil {
			dbg("job %d: execution context close error: %v", j.ID, err)
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
		j.Message = ""
		j.clearFileProgressLocked()
		j.mu.Unlock()
		dbg("job %d: process %s", j.ID, src)
		m.notify()
		srcPath, err := resolveExecutionPath(src)
		if err != nil {
			err = wrapPath(src, err)
		} else {
			err = copyOrMovePathResolved(j, execCtx, srcPath, destPath)
		}
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

// --- copying primitives ---

var errCanceled = errors.New("job canceled")
var errSkipped = errors.New("job item skipped")
var errUnsafeDeleteTarget = errors.New("unsafe delete target")

var trashPath = fileinfo.TrashPath

const progressNotifyInterval = 350 * time.Millisecond

type executionBackend int

const (
	backendLocal executionBackend = iota
	backendSMB
	backendArchive
)

type executionPath struct {
	raw            string
	path           string
	backend        executionBackend
	smb            fileinfo.SMBPathOps
	smbOpener      fileinfo.SMBSessionOpener
	smbDisplayRoot string
	archivePath    string
}

type executionContext struct {
	smbSessions map[string]fileinfo.SMBSession
	archiveVFSs map[string]*fileinfo.ArchiveVFS
}

type virtualFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

func (v virtualFileInfo) Name() string       { return v.name }
func (v virtualFileInfo) Size() int64        { return v.size }
func (v virtualFileInfo) Mode() os.FileMode  { return v.mode }
func (v virtualFileInfo) ModTime() time.Time { return v.modTime }
func (v virtualFileInfo) IsDir() bool        { return v.mode.IsDir() }
func (v virtualFileInfo) Sys() any           { return nil }

func deletePermanentPath(j *Job, execCtx *executionContext, src string) error {
	srcPath, err := resolveExecutionPath(src)
	if err != nil {
		return wrapPath(src, err)
	}
	if err := validateDeleteTarget(srcPath); err != nil {
		return wrapPath(srcPath.displayPath(), err)
	}
	return deletePermanentResolved(j, execCtx, srcPath)
}

func validateDeleteTarget(p executionPath) error {
	switch p.backend {
	case backendArchive:
		return fmt.Errorf("%w: archive paths are read-only", errUnsafeDeleteTarget)
	case backendSMB:
		clean := normalizeSMBExecutionPath(p.path)
		if clean == "/" || clean == "." || clean == "" {
			return fmt.Errorf("%w: refusing to delete SMB share root", errUnsafeDeleteTarget)
		}
	default:
		clean := filepath.Clean(p.path)
		if clean == "." || clean == "" {
			return fmt.Errorf("%w: refusing to delete empty or relative root path", errUnsafeDeleteTarget)
		}
		volume := filepath.VolumeName(clean)
		root := string(os.PathSeparator)
		if volume != "" {
			root = volume + string(os.PathSeparator)
		}
		if clean == root {
			return fmt.Errorf("%w: refusing to delete filesystem root", errUnsafeDeleteTarget)
		}
	}
	return nil
}

func deletePermanentResolved(j *Job, execCtx *executionContext, src executionPath) error {
	if canceled(j) {
		return errCanceled
	}

	fi, err := lstatPath(execCtx, src)
	if err != nil {
		return wrapPath(src.displayPath(), err)
	}

	if fi.IsDir() && !isLinkLikeForTraversal(execCtx, src, fi) {
		entries, err := readDir(execCtx, src)
		if err != nil {
			return wrapPath(src.displayPath(), err)
		}
		for _, e := range entries {
			if canceled(j) {
				return errCanceled
			}
			child := joinPath(src, e.Name())
			if err := deletePermanentResolved(j, execCtx, child); err != nil {
				return err
			}
		}
	}

	if canceled(j) {
		return errCanceled
	}
	dbg("job %d: permanent delete %s", j.ID, src.displayPath())
	if err := removePath(execCtx, src); err != nil {
		return wrapPath(src.displayPath(), err)
	}
	return nil
}

func newExecutionContext() *executionContext {
	return &executionContext{
		smbSessions: make(map[string]fileinfo.SMBSession),
		archiveVFSs: make(map[string]*fileinfo.ArchiveVFS),
	}
}

func (ctx *executionContext) close() error {
	if ctx == nil {
		return nil
	}
	var closeErr error
	for key, session := range ctx.smbSessions {
		if session == nil {
			continue
		}
		if err := session.Close(); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("%s: %w", key, err))
		}
	}
	ctx.smbSessions = make(map[string]fileinfo.SMBSession)
	for key, vfs := range ctx.archiveVFSs {
		if err := vfs.Close(); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("%s: %w", key, err))
		}
	}
	ctx.archiveVFSs = make(map[string]*fileinfo.ArchiveVFS)
	return closeErr
}

func (ctx *executionContext) archiveVFSFor(p executionPath) (*fileinfo.ArchiveVFS, error) {
	if p.backend != backendArchive {
		return nil, fmt.Errorf("path is not archive-backed: %s", p.displayPath())
	}
	if ctx == nil {
		return fileinfo.NewArchiveVFS(p.archivePath)
	}
	if vfs, ok := ctx.archiveVFSs[p.archivePath]; ok && vfs != nil {
		return vfs, nil
	}
	vfs, err := fileinfo.NewArchiveVFS(p.archivePath)
	if err != nil {
		return nil, err
	}
	ctx.archiveVFSs[p.archivePath] = vfs
	return vfs, nil
}

func (ctx *executionContext) smbOpsFor(p executionPath) (fileinfo.SMBPathOps, error) {
	if p.backend != backendSMB {
		return nil, fmt.Errorf("path is not SMB-backed: %s", p.displayPath())
	}
	if p.smb == nil {
		return nil, fmt.Errorf("SMB backend is unavailable: %s", p.displayPath())
	}
	if p.smbOpener == nil || ctx == nil {
		return p.smb, nil
	}

	key := p.smbDisplayRoot
	if key == "" {
		key = "smb://"
	}
	if session, ok := ctx.smbSessions[key]; ok && session != nil {
		return session, nil
	}

	session, err := p.smbOpener.OpenSession()
	if err != nil {
		return nil, err
	}
	ctx.smbSessions[key] = session
	return session, nil
}

func (p executionPath) displayPath() string {
	switch p.backend {
	case backendArchive:
		return fileinfo.ArchiveDisplayPath(p.archivePath, p.path)
	case backendSMB:
		root := p.smbDisplayRoot
		if root == "" {
			root = "smb://"
		}
		rel := strings.TrimPrefix(strings.ReplaceAll(p.path, "\\", "/"), "/")
		if rel == "" {
			return root
		}
		return root + "/" + rel
	default:
		return p.path
	}
}

func canceled(j *Job) bool {
	select {
	case <-j.ctx.Done():
		return true
	default:
		return false
	}
}

func isLinkLikeForTraversal(execCtx *executionContext, p executionPath, fi os.FileInfo) bool {
	if fi.Mode()&os.ModeSymlink != 0 {
		return true
	}
	if !fileinfo.IsLinkModeCandidate(fi.Mode()) {
		return false
	}
	_, err := readlinkPath(execCtx, p)
	return err == nil
}

func linkTargetForCopy(execCtx *executionContext, p executionPath, fi os.FileInfo) (string, bool, error) {
	if fi.Mode()&os.ModeSymlink != 0 {
		target, err := readlinkPath(execCtx, p)
		return target, true, err
	}
	if !fileinfo.IsLinkModeCandidate(fi.Mode()) {
		return "", false, nil
	}
	target, err := readlinkPath(execCtx, p)
	if err != nil {
		return "", false, nil
	}
	return target, true, nil
}

// copyOrMovePath copies or moves a path (file or directory).
func copyOrMovePath(j *Job, src string, destDir string) error {
	srcPath, err := resolveExecutionPath(src)
	if err != nil {
		return wrapPath(src, err)
	}
	destPath, err := resolveExecutionPath(destDir)
	if err != nil {
		return wrapPath(destDir, err)
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

	return copyOrMovePathResolved(j, execCtx, srcPath, destPath)
}

func validateDestinationDirectory(execCtx *executionContext, dest executionPath) error {
	if dest.backend == backendArchive {
		return wrapPath(dest.displayPath(), errors.New("archive destinations are read-only"))
	}
	info, err := statPath(execCtx, dest)
	if err != nil {
		return wrapPath(dest.displayPath(), err)
	}
	if !info.IsDir() {
		return wrapPath(dest.displayPath(), errors.New("destination is not a directory"))
	}
	return nil
}

func copyOrMovePathResolved(j *Job, execCtx *executionContext, src executionPath, destDir executionPath) error {
	if destDir.backend == backendArchive {
		return wrapPath(destDir.displayPath(), errors.New("archive destinations are read-only"))
	}
	if j.Type == TypeMove && src.backend == backendArchive {
		return wrapPath(src.displayPath(), errors.New("cannot move out of an archive; use copy instead"))
	}

	fi, err := lstatPath(execCtx, src)
	if err != nil {
		return wrapPath(src.displayPath(), err)
	}
	base := baseName(src)
	if err := validateArchiveSourceName(src, base); err != nil {
		return wrapPath(src.displayPath(), err)
	}
	dst := joinPath(destDir, base)
	dst, skipped, overwrite, err := resolveDestinationConflict(j, execCtx, src, dst, fi)
	if err != nil {
		return err
	}
	if skipped {
		return errSkipped
	}
	if sameExecutionPath(src, dst) {
		dbg("job %d: source and destination are identical; no-op %s", j.ID, src.displayPath())
		return nil
	}

	if j.Type == TypeMove && fi.IsDir() && isDescendantExecutionPath(dst, src) {
		return wrapPath(dst.displayPath(), errors.New("cannot move a directory into itself"))
	}

	if moved, err := tryFastMovePath(j, execCtx, src, dst, fi, overwrite); err != nil {
		return err
	} else if moved {
		return nil
	}

	if target, isLink, err := linkTargetForCopy(execCtx, src, fi); err != nil {
		return wrapPath(src.displayPath(), err)
	} else if isLink {
		if j.Type == TypeMove {
			if !overwrite {
				if err := renamePath(execCtx, src, dst); err == nil {
					dbg("job %d: rename link %s -> %s", j.ID, src.displayPath(), dst.displayPath())
					return nil
				} else {
					dbg("job %d: rename link fallback %s -> %s: %v", j.ID, src.displayPath(), dst.displayPath(), err)
				}
			} else if err := removePath(execCtx, dst); err != nil {
				return wrapPath(dst.displayPath(), err)
			} else if err := renamePath(execCtx, src, dst); err == nil {
				dbg("job %d: rename link %s -> %s", j.ID, src.displayPath(), dst.displayPath())
				return nil
			} else {
				dbg("job %d: rename link fallback %s -> %s: %v", j.ID, src.displayPath(), dst.displayPath(), err)
			}
		}
		dbg("job %d: symlink %s -> %s", j.ID, dst.displayPath(), target)
		if overwrite {
			if err := removePath(execCtx, dst); err != nil && !fileinfo.IsNotExist(err) {
				return wrapPath(dst.displayPath(), err)
			}
		}
		if err := symlinkPath(execCtx, target, dst); err != nil {
			return wrapPath(dst.displayPath(), err)
		}
		if j.Type == TypeMove {
			dbg("job %d: unlink %s", j.ID, src.displayPath())
			if err := removePath(execCtx, src); err != nil {
				return wrapPath(src.displayPath(), err)
			}
		}
		return nil
	}

	if fi.IsDir() {
		dbg("job %d: mkdir %s (mode=%v)", j.ID, dst.displayPath(), fi.Mode())
		if err := ensureDir(execCtx, dst, fi.Mode()); err != nil {
			return wrapPath(dst.displayPath(), err)
		}
		entries, err := readDir(execCtx, src)
		if err != nil {
			return wrapPath(src.displayPath(), err)
		}
		skippedChild := false
		for _, e := range entries {
			if canceled(j) {
				return errCanceled
			}
			if err := validateArchiveSourceName(src, e.Name()); err != nil {
				return wrapPath(src.displayPath(), err)
			}
			child := joinPath(src, e.Name())
			dbg("job %d: recurse %s -> %s", j.ID, child.displayPath(), dst.displayPath())
			if err := copyOrMovePathResolved(j, execCtx, child, dst); err != nil {
				if errors.Is(err, errSkipped) {
					skippedChild = true
					continue
				}
				return err
			}
		}
		if skippedChild {
			return errSkipped
		}
		if shouldPreserveTimestamps(j) {
			if err := chtimesPath(execCtx, dst, fi.ModTime(), fi.ModTime()); err != nil {
				return wrapPath(dst.displayPath(), err)
			}
		}
		if j.Type == TypeMove {
			if canceled(j) {
				return errCanceled
			}
			// remove empty dir after moving children
			dbg("job %d: rmdir %s", j.ID, src.displayPath())
			if err := removePath(execCtx, src); err != nil {
				return wrapPath(src.displayPath(), err)
			}
		}
		return nil
	}

	// regular file
	dbg("job %d: file %s -> %s", j.ID, src.displayPath(), dst.displayPath())
	if err := copyFileWithCancel(j, execCtx, src, dst, fi, overwrite); err != nil {
		return err
	}
	if j.Type == TypeMove {
		if canceled(j) {
			return errCanceled
		}
		dbg("job %d: remove %s", j.ID, src.displayPath())
		if err := removePath(execCtx, src); err != nil {
			return wrapPath(src.displayPath(), err)
		}
	}
	return nil
}

func validateArchiveSourceName(src executionPath, name string) error {
	if src.backend != backendArchive {
		return nil
	}
	return fileinfo.ValidateArchiveEntryBaseName(name)
}

func extractArchivePath(j *Job, execCtx *executionContext, src string, destDir executionPath) error {
	if fileinfo.IsArchivePath(src) {
		return wrapPath(src, errors.New("nested archive extraction is not supported"))
	}
	rootName := extractRootName(fileinfo.BaseName(src))
	if rootName == "" || rootName == "." {
		return wrapPath(src, errors.New("archive name is not usable as an extract directory"))
	}
	if err := fileinfo.ValidateArchiveEntryBaseName(rootName); err != nil {
		return wrapPath(src, err)
	}

	root := joinPath(destDir, rootName)
	rootInfo := virtualFileInfo{name: rootName, mode: os.ModeDir | 0755, modTime: time.Now()}
	root, skipped, _, err := resolveDestinationConflict(j, execCtx, extractSourcePath(src, "."), root, rootInfo)
	if err != nil {
		return err
	}
	if skipped {
		return errSkipped
	}
	if err := ensureDir(execCtx, root, rootInfo.Mode()); err != nil {
		return wrapPath(root.displayPath(), err)
	}

	err = fileinfo.ExtractArchive(j.ctx, src, func(ctx context.Context, entry fileinfo.ArchiveEntry) error {
		if canceled(j) {
			return errCanceled
		}
		if entry.Name == "." {
			return nil
		}
		if err := fileinfo.ValidateArchiveEntryPath(entry.Name, entry.Info.IsDir()); err != nil {
			return wrapPath(src, err)
		}
		if entry.LinkTarget != "" || fileinfo.IsLinkModeCandidate(entry.Info.Mode()) {
			return wrapPath(fileinfo.ArchiveDisplayPath(src, entry.Name), errors.New("archive links are not supported for extraction"))
		}

		dst, err := archiveEntryDestination(root, entry.Name)
		if err != nil {
			return wrapPath(fileinfo.ArchiveDisplayPath(src, entry.Name), err)
		}
		srcPath := extractSourcePath(src, entry.Name)
		if entry.Info.IsDir() {
			if err := ensureDir(execCtx, dst, entry.Info.Mode()); err != nil {
				return wrapPath(dst.displayPath(), err)
			}
			if shouldPreserveTimestamps(j) {
				if err := chtimesPath(execCtx, dst, entry.Info.ModTime(), entry.Info.ModTime()); err != nil {
					return wrapPath(dst.displayPath(), err)
				}
			}
			return nil
		}

		dst, skipped, overwrite, err := resolveDestinationConflict(j, execCtx, srcPath, dst, entry.Info)
		if err != nil {
			return err
		}
		if skipped {
			return nil
		}
		in, err := entry.Open()
		if err != nil {
			return wrapPath(srcPath.displayPath(), err)
		}
		defer in.Close()
		return copyReaderWithCancel(j, execCtx, in, srcPath.displayPath(), dst, entry.Info, overwrite)
	})
	if err != nil {
		return wrapPath(src, err)
	}
	return nil
}

func extractSourcePath(archivePath, inner string) executionPath {
	return executionPath{
		raw:         fileinfo.ArchiveDisplayPath(archivePath, inner),
		path:        inner,
		backend:     backendArchive,
		archivePath: archivePath,
	}
}

func archiveEntryDestination(root executionPath, entryName string) (executionPath, error) {
	dst := root
	for _, part := range strings.Split(entryName, "/") {
		if err := fileinfo.ValidateArchiveEntryBaseName(part); err != nil {
			return executionPath{}, err
		}
		dst = joinPath(dst, part)
	}
	return dst, nil
}

func extractRootName(name string) string {
	name = strings.TrimSpace(name)
	lower := strings.ToLower(name)
	for _, ext := range []string{
		".tar.gz", ".tar.bz2", ".tar.xz", ".tar.zst", ".tar.zstd",
		".tgz", ".tbz2", ".txz", ".tzst",
		".zip", ".7z", ".rar", ".tar",
	} {
		if strings.HasSuffix(lower, ext) && len(name) > len(ext) {
			return name[:len(name)-len(ext)]
		}
	}
	if ext := filepath.Ext(name); ext != "" && len(name) > len(ext) {
		return strings.TrimSuffix(name, ext)
	}
	return name
}

func shouldPreserveTimestamps(j *Job) bool {
	if j == nil {
		return false
	}
	return j.Type == TypeMove || j.Options.PreserveTimestamps
}

func tryFastMovePath(j *Job, execCtx *executionContext, src, dst executionPath, fi os.FileInfo, overwrite bool) (bool, error) {
	if j.Type != TypeMove {
		return false, nil
	}
	if canceled(j) {
		return false, errCanceled
	}
	if fi.IsDir() && !overwrite {
		exists, err := pathExists(execCtx, dst)
		if err != nil {
			return false, wrapPath(dst.displayPath(), err)
		}
		if exists {
			return false, nil
		}
	}
	if err := renamePath(execCtx, src, dst); err == nil {
		dbg("job %d: rename %s -> %s", j.ID, src.displayPath(), dst.displayPath())
		return true, nil
	} else {
		dbg("job %d: rename fallback %s -> %s: %v", j.ID, src.displayPath(), dst.displayPath(), err)
	}
	return false, nil
}

func resolveDestinationConflict(j *Job, execCtx *executionContext, src, dst executionPath, srcInfo os.FileInfo) (executionPath, bool, bool, error) {
	if sameExecutionPath(src, dst) {
		if j.Type == TypeMove {
			return dst, false, false, nil
		}
		return askDestinationConflict(j, execCtx, src, dst, srcInfo, srcInfo)
	}

	dstInfo, err := lstatPath(execCtx, dst)
	if err != nil {
		if fileinfo.IsNotExist(err) {
			return dst, false, false, nil
		}
		return dst, false, false, wrapPath(dst.displayPath(), err)
	}
	if srcInfo.IsDir() && dstInfo.IsDir() {
		return dst, false, false, nil
	}
	return askDestinationConflict(j, execCtx, src, dst, srcInfo, dstInfo)
}

func askDestinationConflict(j *Job, execCtx *executionContext, src, dst executionPath, srcInfo, dstInfo os.FileInfo) (executionPath, bool, bool, error) {
	for {
		suggested, err := nextAvailablePath(execCtx, dst)
		if err != nil {
			return dst, false, false, err
		}
		var resolution ConflictResolution
		switch j.conflictDefault {
		case ConflictSkip:
			resolution = ConflictResolution{Action: ConflictSkip}
		case ConflictAutoSuffix:
			resolution = ConflictResolution{Action: ConflictAutoSuffix}
		case ConflictOverwriteIfNewer:
			resolution = ConflictResolution{Action: ConflictOverwriteIfNewer}
		case ConflictOverwrite:
			resolution = ConflictResolution{Action: ConflictOverwrite}
		default:
			defaultAction := j.conflictDefault
			if defaultAction == "" {
				defaultAction = ConflictOverwriteIfNewer
			}
			resolution = resolveConflict(j, ConflictRequest{
				JobID:          j.ID,
				Type:           j.Type,
				SourcePath:     src.displayPath(),
				Destination:    dst.displayPath(),
				SourceModified: srcInfo.ModTime(),
				DestModified:   dstInfo.ModTime(),
				SuggestedName:  baseName(suggested),
				SuggestedPath:  suggested.displayPath(),
				IsDir:          srcInfo.IsDir(),
				DefaultAction:  defaultAction,
				CanApplyToRest: true,
			})
		}

		switch resolution.Action {
		case ConflictSkip:
			if resolution.ApplyToRest {
				j.conflictDefault = ConflictSkip
			}
			return dst, true, false, nil
		case ConflictCancelJob:
			return dst, false, false, errCanceled
		case ConflictOverwriteIfNewer:
			if resolution.ApplyToRest {
				j.conflictDefault = ConflictOverwriteIfNewer
			}
			if canOverwriteConflict(srcInfo, dstInfo) && sourceClearlyNewer(srcInfo, dstInfo) {
				return dst, false, true, nil
			}
			return dst, true, false, nil
		case ConflictOverwrite:
			if resolution.ApplyToRest {
				j.conflictDefault = ConflictOverwrite
			}
			if canOverwriteConflict(srcInfo, dstInfo) {
				return dst, false, true, nil
			}
			return dst, true, false, nil
		case ConflictRename:
			if resolution.ApplyToRest {
				j.conflictDefault = ConflictRename
			}
			name, err := fileinfo.ValidateRenameName(resolution.NewName)
			if err != nil {
				if j.Resolver == nil {
					return dst, false, false, wrapPath(dst.displayPath(), err)
				}
				j.conflictDefault = ConflictRename
				continue
			}
			renamed := joinPath(dirPath(dst), name)
			if exists, err := pathExists(execCtx, renamed); err != nil {
				return renamed, false, false, wrapPath(renamed.displayPath(), err)
			} else if exists {
				dst = renamed
				j.conflictDefault = ConflictRename
				continue
			}
			return renamed, false, false, nil
		case ConflictAutoSuffix, "":
			if resolution.ApplyToRest {
				j.conflictDefault = ConflictAutoSuffix
			}
			return suggested, false, false, nil
		default:
			return dst, false, false, fmt.Errorf("unknown conflict action: %s", resolution.Action)
		}
	}
}

func canOverwriteConflict(srcInfo, dstInfo os.FileInfo) bool {
	if srcInfo == nil || dstInfo == nil {
		return false
	}
	if srcInfo.IsDir() || dstInfo.IsDir() {
		return false
	}
	return srcInfo.Mode().Type() == dstInfo.Mode().Type()
}

func sourceClearlyNewer(srcInfo, dstInfo os.FileInfo) bool {
	const fatTimestampResolution = 2 * time.Second
	return srcInfo.ModTime().After(dstInfo.ModTime().Add(fatTimestampResolution))
}

func resolveConflict(j *Job, req ConflictRequest) ConflictResolution {
	if j.Resolver == nil {
		return ConflictResolution{Action: ConflictAutoSuffix}
	}
	return j.Resolver(j.ctx, req)
}

func sameExecutionPath(a, b executionPath) bool {
	if a.backend != b.backend {
		return false
	}
	if a.backend == backendArchive {
		return a.archivePath == b.archivePath && normalizeSMBExecutionPath(a.path) == normalizeSMBExecutionPath(b.path)
	}
	if a.backend == backendSMB {
		return normalizeSMBRoot(a.smbDisplayRoot) == normalizeSMBRoot(b.smbDisplayRoot) &&
			normalizeSMBExecutionPath(a.path) == normalizeSMBExecutionPath(b.path)
	}

	ap := filepath.Clean(a.path)
	bp := filepath.Clean(b.path)
	if runtime.GOOS == "windows" {
		ap = strings.ToLower(ap)
		bp = strings.ToLower(bp)
	}
	return ap == bp
}

func isDescendantExecutionPath(child, parent executionPath) bool {
	if child.backend != parent.backend {
		return false
	}
	switch child.backend {
	case backendArchive:
		if child.archivePath != parent.archivePath {
			return false
		}
		return isDescendantSlashPath(child.path, parent.path)
	case backendSMB:
		if normalizeSMBRoot(child.smbDisplayRoot) != normalizeSMBRoot(parent.smbDisplayRoot) {
			return false
		}
		return isDescendantSlashPath(child.path, parent.path)
	default:
		childPath := filepath.Clean(child.path)
		parentPath := filepath.Clean(parent.path)
		if runtime.GOOS == "windows" {
			childPath = strings.ToLower(childPath)
			parentPath = strings.ToLower(parentPath)
		}
		if childPath == parentPath {
			return false
		}
		rel, err := filepath.Rel(parentPath, childPath)
		if err != nil {
			return false
		}
		return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
	}
}

func isDescendantSlashPath(child, parent string) bool {
	childPath := normalizeSMBExecutionPath(child)
	parentPath := normalizeSMBExecutionPath(parent)
	if childPath == parentPath {
		return false
	}
	if parentPath == "/" {
		return strings.HasPrefix(childPath, "/") && childPath != "/"
	}
	return strings.HasPrefix(childPath, strings.TrimRight(parentPath, "/")+"/")
}

func pathExists(execCtx *executionContext, p executionPath) (bool, error) {
	if _, err := lstatPath(execCtx, p); err == nil {
		return true, nil
	} else if fileinfo.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

func nextAvailablePath(execCtx *executionContext, dst executionPath) (executionPath, error) {
	dir := dirPath(dst)
	stem, ext := splitCopyName(baseName(dst))
	for i := 1; ; i++ {
		candidate := joinPath(dir, fmt.Sprintf("%s (%d)%s", stem, i, ext))
		exists, err := pathExists(execCtx, candidate)
		if err != nil {
			return candidate, wrapPath(candidate.displayPath(), err)
		}
		if !exists {
			return candidate, nil
		}
	}
}

func splitCopyName(name string) (string, string) {
	dot := strings.LastIndex(name, ".")
	if dot <= 0 {
		return name, ""
	}
	return name[:dot], name[dot:]
}

func normalizeSMBRoot(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "smb://"
	}
	root = strings.ReplaceAll(root, "\\", "/")
	root = strings.TrimRight(root, "/")
	return strings.ToLower(root)
}

func normalizeSMBExecutionPath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	p = pathpkg.Clean("/" + strings.TrimPrefix(p, "/"))
	if p == "." {
		return "/"
	}
	return p
}

// resolveExecutionPath maps display paths to backend-specific execution paths.
func resolveExecutionPath(p string) (executionPath, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return executionPath{}, errors.New("path is empty")
	}

	vfs, parsed, err := fileinfo.ResolveRead(p)
	if err != nil {
		return executionPath{}, err
	}

	native := parsed.Native
	if native == "" {
		native = p
	}

	if parsed.Scheme == fileinfo.SchemeArchive {
		_ = fileinfo.CloseVFS(vfs)
		if native == "" {
			native = "."
		}
		return executionPath{
			raw:         p,
			path:        native,
			backend:     backendArchive,
			archivePath: parsed.Archive,
		}, nil
	}

	if parsed.Scheme == fileinfo.SchemeSMB && parsed.Provider != "local" {
		smb, ok := vfs.(fileinfo.SMBPathOps)
		if !ok {
			_ = fileinfo.CloseVFS(vfs)
			return executionPath{}, fmt.Errorf("direct SMB provider is unavailable on this platform: %s", p)
		}
		opener, _ := vfs.(fileinfo.SMBSessionOpener)
		root := "smb://"
		if parsed.Host != "" && parsed.Share != "" {
			root = "smb://" + pathpkg.Join(parsed.Host, parsed.Share)
		}
		if native == "" {
			native = "/"
		}
		return executionPath{
			raw:            p,
			path:           native,
			backend:        backendSMB,
			smb:            smb,
			smbOpener:      opener,
			smbDisplayRoot: root,
		}, nil
	}

	_ = fileinfo.CloseVFS(vfs)
	return executionPath{
		raw:     p,
		path:    native,
		backend: backendLocal,
	}, nil
}

func baseName(p executionPath) string {
	if p.backend == backendArchive {
		if p.path == "." {
			return fileinfo.BaseName(p.archivePath)
		}
		return pathpkg.Base(p.path)
	}
	if p.backend == backendSMB {
		return p.smb.Base(p.path)
	}
	return filepath.Base(p.path)
}

func joinPath(base executionPath, name string) executionPath {
	out := base
	if base.backend == backendArchive {
		if base.path == "." {
			out.path = pathpkg.Clean(name)
		} else {
			out.path = pathpkg.Join(base.path, name)
		}
	} else if base.backend == backendSMB {
		out.path = base.smb.Join(base.path, name)
	} else {
		out.path = filepath.Join(base.path, name)
	}
	out.raw = out.path
	return out
}

func dirPath(p executionPath) executionPath {
	out := p
	if p.backend == backendArchive {
		parent := pathpkg.Dir(strings.TrimPrefix(p.path, "/"))
		if parent == "." || parent == "/" {
			parent = "."
		}
		out.path = parent
		out.raw = out.path
		return out
	}
	if p.backend == backendSMB {
		clean := strings.ReplaceAll(p.path, "\\", "/")
		parent := pathpkg.Dir(clean)
		if parent == "." {
			parent = ""
		}
		out.path = parent
		out.raw = out.path
		return out
	}
	out.path = filepath.Dir(p.path)
	out.raw = out.path
	return out
}

func lstatPath(execCtx *executionContext, p executionPath) (os.FileInfo, error) {
	if p.backend == backendArchive {
		vfs, err := execCtx.archiveVFSFor(p)
		if err != nil {
			return nil, err
		}
		return vfs.Stat(p.path)
	}
	if p.backend == backendSMB {
		ops, err := execCtx.smbOpsFor(p)
		if err != nil {
			return nil, err
		}
		return ops.Lstat(p.path)
	}
	return os.Lstat(p.path)
}

func statPath(execCtx *executionContext, p executionPath) (os.FileInfo, error) {
	if p.backend == backendArchive {
		vfs, err := execCtx.archiveVFSFor(p)
		if err != nil {
			return nil, err
		}
		return vfs.Stat(p.path)
	}
	if p.backend == backendSMB {
		ops, err := execCtx.smbOpsFor(p)
		if err != nil {
			return nil, err
		}
		return ops.Stat(p.path)
	}
	return os.Stat(p.path)
}

func readDir(execCtx *executionContext, p executionPath) ([]os.DirEntry, error) {
	if p.backend == backendArchive {
		vfs, err := execCtx.archiveVFSFor(p)
		if err != nil {
			return nil, err
		}
		return vfs.ReadDir(p.path)
	}
	if p.backend == backendSMB {
		ops, err := execCtx.smbOpsFor(p)
		if err != nil {
			return nil, err
		}
		return ops.ReadDir(p.path)
	}
	return os.ReadDir(p.path)
}

func ensureDir(execCtx *executionContext, p executionPath, mode os.FileMode) error {
	if p.backend == backendArchive {
		return errors.New("archive paths are read-only")
	}
	perm := mode.Perm()
	if perm == 0 {
		perm = 0755
	}
	if p.backend == backendSMB {
		ops, err := execCtx.smbOpsFor(p)
		if err != nil {
			return err
		}
		return ops.MkdirAll(p.path, perm)
	}
	if err := os.MkdirAll(p.path, perm); err != nil {
		return err
	}
	// best-effort to set mode
	_ = os.Chmod(p.path, perm)
	return nil
}

func chtimesPath(execCtx *executionContext, p executionPath, atime, mtime time.Time) error {
	if p.backend == backendArchive {
		return errors.New("archive paths are read-only")
	}
	if p.backend == backendSMB {
		ops, err := execCtx.smbOpsFor(p)
		if err != nil {
			return err
		}
		return ops.Chtimes(p.path, atime, mtime)
	}
	return os.Chtimes(p.path, atime, mtime)
}

func removePath(execCtx *executionContext, p executionPath) error {
	if p.backend == backendArchive {
		return errors.New("archive paths are read-only")
	}
	if p.backend == backendSMB {
		ops, err := execCtx.smbOpsFor(p)
		if err != nil {
			return err
		}
		return ops.Remove(p.path)
	}
	return os.Remove(p.path)
}

func readlinkPath(execCtx *executionContext, p executionPath) (string, error) {
	if p.backend == backendArchive {
		return "", errors.New("archive symlink targets are not supported")
	}
	if p.backend == backendSMB {
		ops, err := execCtx.smbOpsFor(p)
		if err != nil {
			return "", err
		}
		return ops.Readlink(p.path)
	}
	return os.Readlink(p.path)
}

func symlinkPath(execCtx *executionContext, target string, link executionPath) error {
	if link.backend == backendArchive {
		return errors.New("archive paths are read-only")
	}
	if link.backend == backendSMB {
		ops, err := execCtx.smbOpsFor(link)
		if err != nil {
			return err
		}
		return ops.Symlink(target, link.path)
	}
	return os.Symlink(target, link.path)
}

func renamePath(execCtx *executionContext, src executionPath, dst executionPath) error {
	if src.backend != dst.backend {
		return errors.New("cannot rename across backends")
	}
	if dst.backend == backendArchive {
		return errors.New("archive paths are read-only")
	}
	if dst.backend == backendSMB {
		if normalizeSMBRoot(src.smbDisplayRoot) != normalizeSMBRoot(dst.smbDisplayRoot) {
			return errors.New("cannot rename across SMB shares")
		}
		ops, err := execCtx.smbOpsFor(src)
		if err != nil {
			return err
		}
		return ops.Rename(src.path, dst.path)
	}
	return os.Rename(src.path, dst.path)
}

func openReadPath(execCtx *executionContext, p executionPath) (io.ReadCloser, error) {
	if p.backend == backendArchive {
		vfs, err := execCtx.archiveVFSFor(p)
		if err != nil {
			return nil, err
		}
		return vfs.Open(p.path)
	}
	if p.backend == backendSMB {
		ops, err := execCtx.smbOpsFor(p)
		if err != nil {
			return nil, err
		}
		return ops.Open(p.path)
	}
	return os.Open(p.path)
}

func openWritePath(execCtx *executionContext, p executionPath, mode os.FileMode) (io.ReadWriteCloser, error) {
	if p.backend == backendArchive {
		return nil, errors.New("archive paths are read-only")
	}
	perm := mode.Perm()
	if perm == 0 {
		perm = 0666
	}
	if p.backend == backendSMB {
		// SMB create attributes can map mode bits differently than local fs.
		// Use a writable default for temp output, then rely on replace semantics.
		ops, err := execCtx.smbOpsFor(p)
		if err != nil {
			return nil, err
		}
		return ops.OpenFile(p.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	}
	return os.OpenFile(p.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
}

func replacePath(execCtx *executionContext, tmp executionPath, dst executionPath, overwrite bool) error {
	if dst.backend == backendArchive {
		return errors.New("archive paths are read-only")
	}
	if dst.backend == backendSMB {
		ops, err := execCtx.smbOpsFor(dst)
		if err != nil {
			return err
		}
		if err := ops.Rename(tmp.path, dst.path); err == nil {
			return nil
		} else if overwrite {
			_ = ops.Remove(dst.path)
			return ops.Rename(tmp.path, dst.path)
		} else {
			return err
		}
	}
	if err := os.Rename(tmp.path, dst.path); err == nil {
		return nil
	} else if overwrite {
		_ = os.Remove(dst.path)
		return os.Rename(tmp.path, dst.path)
	} else {
		return err
	}
}

func copyFileWithCancel(j *Job, execCtx *executionContext, src, dst executionPath, fi os.FileInfo, overwrite bool) error {
	in, err := openReadPath(execCtx, src)
	if err != nil {
		return wrapPath(src.displayPath(), err)
	}
	defer in.Close()
	return copyReaderWithCancel(j, execCtx, in, src.displayPath(), dst, fi, overwrite)
}

func copyReaderWithCancel(j *Job, execCtx *executionContext, in io.Reader, srcDisplay string, dst executionPath, fi os.FileInfo, overwrite bool) error {
	tmp := dst
	tmp.path = dst.path + ".part"
	tmp.raw = tmp.path

	if err := ensureDir(execCtx, dirPath(tmp), 0755); err != nil {
		return wrapPath(dst.displayPath(), err)
	}

	out, err := openWritePath(execCtx, tmp, fi.Mode())
	if err != nil {
		return wrapPath(tmp.displayPath(), err)
	}

	totalBytes := fi.Size()
	if totalBytes < 0 {
		totalBytes = 0
	}
	j.beginFileProgress(srcDisplay, totalBytes)

	buf := make([]byte, 1<<20) // 1 MiB
	for {
		if canceled(j) {
			out.Close()
			_ = removePath(execCtx, tmp)
			return errCanceled
		}
		n, rerr := in.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				out.Close()
				_ = removePath(execCtx, tmp)
				return wrapPath(tmp.displayPath(), werr)
			}
			j.addFileProgress(int64(n), false)
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			out.Close()
			_ = removePath(execCtx, tmp)
			return wrapPath(srcDisplay, rerr)
		}
	}
	j.completeFileProgress()
	if err := out.Close(); err != nil {
		_ = removePath(execCtx, tmp)
		return wrapPath(tmp.displayPath(), err)
	}

	if dst.backend == backendLocal {
		if err := os.Chmod(tmp.path, fi.Mode().Perm()); err != nil {
			_ = removePath(execCtx, tmp)
			return wrapPath(tmp.displayPath(), err)
		}
	}

	dbg("job %d: rename %s -> %s", j.ID, tmp.displayPath(), dst.displayPath())
	if err := replacePath(execCtx, tmp, dst, overwrite); err != nil {
		_ = removePath(execCtx, tmp)
		return wrapPath(dst.displayPath(), err)
	}
	if shouldPreserveTimestamps(j) {
		if err := chtimesPath(execCtx, dst, fi.ModTime(), fi.ModTime()); err != nil {
			return wrapPath(dst.displayPath(), err)
		}
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
