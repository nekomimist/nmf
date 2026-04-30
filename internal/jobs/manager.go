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

// EnqueueDelete enqueues a delete job.
func (m *Manager) EnqueueDelete(sources []string, mode DeleteMode) *Job {
	return m.enqueueDelete(sources, mode)
}

// EnqueueCopyWithResolver enqueues a copy job with an optional collision resolver.
func (m *Manager) EnqueueCopyWithResolver(sources []string, destDir string, resolver ConflictResolver) *Job {
	return m.enqueue(TypeCopy, sources, destDir, resolver)
}

// EnqueueMoveWithResolver enqueues a move job with an optional collision resolver.
func (m *Manager) EnqueueMoveWithResolver(sources []string, destDir string, resolver ConflictResolver) *Job {
	return m.enqueue(TypeMove, sources, destDir, resolver)
}

func (m *Manager) enqueue(t Type, sources []string, destDir string, resolver ConflictResolver) *Job {
	j := &Job{ID: atomic.AddInt64(&m.nextID, 1), Type: t, Sources: append([]string(nil), sources...), DestDir: destDir, Resolver: resolver, Status: StatusPending, EnqueuedAt: time.Now()}
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
	if j.Type == TypeDelete {
		return m.runDeleteJob(j)
	}
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
			if errors.Is(err, errSkipped) {
				dbg("job %d: skipped %s", j.ID, src)
				j.mu.Lock()
				j.DoneFiles = i + 1
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

// --- copying primitives ---

var errCanceled = errors.New("job canceled")
var errSkipped = errors.New("job item skipped")
var errUnsafeDeleteTarget = errors.New("unsafe delete target")

var trashPath = fileinfo.TrashPath

type executionBackend int

const (
	backendLocal executionBackend = iota
	backendSMB
)

type executionPath struct {
	raw            string
	path           string
	backend        executionBackend
	smb            fileinfo.SMBPathOps
	smbOpener      fileinfo.SMBSessionOpener
	smbDisplayRoot string
}

type executionContext struct {
	smbSessions map[string]fileinfo.SMBSession
}

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

	if fi.IsDir() && fi.Mode()&os.ModeSymlink == 0 {
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
	return closeErr
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

	return copyOrMovePathResolved(j, execCtx, srcPath, destPath)
}

func copyOrMovePathResolved(j *Job, execCtx *executionContext, src executionPath, destDir executionPath) error {
	fi, err := lstatPath(execCtx, src)
	if err != nil {
		return wrapPath(src.displayPath(), err)
	}
	base := baseName(src)
	dst := joinPath(destDir, base)
	dst, skipped, err := resolveDestinationConflict(j, execCtx, src, dst, fi)
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

	// handle symlink as symlink
	if fi.Mode()&os.ModeSymlink != 0 {
		target, err := readlinkPath(execCtx, src)
		if err != nil {
			return wrapPath(src.displayPath(), err)
		}
		// remove existing destination symlink/file if exists
		_ = removePath(execCtx, dst)
		dbg("job %d: symlink %s -> %s", j.ID, dst.displayPath(), target)
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

	// regular file
	dbg("job %d: file %s -> %s", j.ID, src.displayPath(), dst.displayPath())
	if err := copyFileWithCancel(j, execCtx, src, dst, fi.Mode()); err != nil {
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

func resolveDestinationConflict(j *Job, execCtx *executionContext, src, dst executionPath, srcInfo os.FileInfo) (executionPath, bool, error) {
	if sameExecutionPath(src, dst) {
		if j.Type == TypeMove {
			return dst, false, nil
		}
		return askDestinationConflict(j, execCtx, src, dst, srcInfo)
	}

	dstInfo, err := lstatPath(execCtx, dst)
	if err != nil {
		if os.IsNotExist(err) {
			return dst, false, nil
		}
		return dst, false, wrapPath(dst.displayPath(), err)
	}
	if srcInfo.IsDir() && dstInfo.IsDir() {
		return dst, false, nil
	}
	return askDestinationConflict(j, execCtx, src, dst, srcInfo)
}

func askDestinationConflict(j *Job, execCtx *executionContext, src, dst executionPath, srcInfo os.FileInfo) (executionPath, bool, error) {
	for {
		suggested, err := nextAvailablePath(execCtx, dst)
		if err != nil {
			return dst, false, err
		}
		var resolution ConflictResolution
		switch j.conflictDefault {
		case ConflictSkip:
			resolution = ConflictResolution{Action: ConflictSkip}
		case ConflictAutoSuffix:
			resolution = ConflictResolution{Action: ConflictAutoSuffix}
		default:
			defaultAction := j.conflictDefault
			if defaultAction == "" {
				defaultAction = ConflictAutoSuffix
			}
			resolution = resolveConflict(j, ConflictRequest{
				JobID:          j.ID,
				Type:           j.Type,
				SourcePath:     src.displayPath(),
				Destination:    dst.displayPath(),
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
			return dst, true, nil
		case ConflictCancelJob:
			return dst, false, errCanceled
		case ConflictRename:
			if resolution.ApplyToRest {
				j.conflictDefault = ConflictRename
			}
			name, err := fileinfo.ValidateRenameName(resolution.NewName)
			if err != nil {
				if j.Resolver == nil {
					return dst, false, wrapPath(dst.displayPath(), err)
				}
				j.conflictDefault = ConflictRename
				continue
			}
			renamed := joinPath(dirPath(dst), name)
			if exists, err := pathExists(execCtx, renamed); err != nil {
				return renamed, false, wrapPath(renamed.displayPath(), err)
			} else if exists {
				dst = renamed
				j.conflictDefault = ConflictRename
				continue
			}
			return renamed, false, nil
		case ConflictAutoSuffix, "":
			if resolution.ApplyToRest {
				j.conflictDefault = ConflictAutoSuffix
			}
			return suggested, false, nil
		default:
			return dst, false, fmt.Errorf("unknown conflict action: %s", resolution.Action)
		}
	}
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

func pathExists(execCtx *executionContext, p executionPath) (bool, error) {
	if _, err := lstatPath(execCtx, p); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
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

	if parsed.Scheme == fileinfo.SchemeSMB && parsed.Provider != "local" {
		smb, ok := vfs.(fileinfo.SMBPathOps)
		if !ok {
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

	return executionPath{
		raw:     p,
		path:    native,
		backend: backendLocal,
	}, nil
}

func baseName(p executionPath) string {
	if p.backend == backendSMB {
		return p.smb.Base(p.path)
	}
	return filepath.Base(p.path)
}

func joinPath(base executionPath, name string) executionPath {
	out := base
	if base.backend == backendSMB {
		out.path = base.smb.Join(base.path, name)
	} else {
		out.path = filepath.Join(base.path, name)
	}
	out.raw = out.path
	return out
}

func dirPath(p executionPath) executionPath {
	out := p
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
	if p.backend == backendSMB {
		ops, err := execCtx.smbOpsFor(p)
		if err != nil {
			return nil, err
		}
		return ops.Lstat(p.path)
	}
	return os.Lstat(p.path)
}

func readDir(execCtx *executionContext, p executionPath) ([]os.DirEntry, error) {
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
	if p.backend == backendSMB {
		ops, err := execCtx.smbOpsFor(p)
		if err != nil {
			return err
		}
		return ops.MkdirAll(p.path, 0755)
	}
	if err := os.MkdirAll(p.path, 0755); err != nil {
		return err
	}
	// best-effort to set mode
	_ = os.Chmod(p.path, mode.Perm())
	return nil
}

func removePath(execCtx *executionContext, p executionPath) error {
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
	if link.backend == backendSMB {
		ops, err := execCtx.smbOpsFor(link)
		if err != nil {
			return err
		}
		return ops.Symlink(target, link.path)
	}
	return os.Symlink(target, link.path)
}

func openReadPath(execCtx *executionContext, p executionPath) (io.ReadCloser, error) {
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
	if p.backend == backendSMB {
		// SMB create attributes can map mode bits differently than local fs.
		// Use a writable default for temp output, then rely on replace semantics.
		ops, err := execCtx.smbOpsFor(p)
		if err != nil {
			return nil, err
		}
		return ops.OpenFile(p.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	}
	return os.OpenFile(p.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
}

func replacePath(execCtx *executionContext, tmp executionPath, dst executionPath) error {
	if dst.backend == backendSMB {
		ops, err := execCtx.smbOpsFor(dst)
		if err != nil {
			return err
		}
		if err := ops.Rename(tmp.path, dst.path); err == nil {
			return nil
		} else {
			_ = ops.Remove(dst.path)
			return ops.Rename(tmp.path, dst.path)
		}
	}
	return os.Rename(tmp.path, dst.path)
}

func copyFileWithCancel(j *Job, execCtx *executionContext, src, dst executionPath, mode os.FileMode) error {
	tmp := dst
	tmp.path = dst.path + ".part"
	tmp.raw = tmp.path

	if err := ensureDir(execCtx, dirPath(tmp), 0755); err != nil {
		return wrapPath(dst.displayPath(), err)
	}

	in, err := openReadPath(execCtx, src)
	if err != nil {
		return wrapPath(src.displayPath(), err)
	}
	defer in.Close()

	out, err := openWritePath(execCtx, tmp, mode)
	if err != nil {
		return wrapPath(tmp.displayPath(), err)
	}

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
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			out.Close()
			_ = removePath(execCtx, tmp)
			return wrapPath(src.displayPath(), rerr)
		}
	}
	if err := out.Close(); err != nil {
		_ = removePath(execCtx, tmp)
		return wrapPath(tmp.displayPath(), err)
	}

	if dst.backend == backendLocal {
		if err := os.Chmod(tmp.path, mode.Perm()); err != nil {
			_ = removePath(execCtx, tmp)
			return wrapPath(tmp.displayPath(), err)
		}
	}

	dbg("job %d: rename %s -> %s", j.ID, tmp.displayPath(), dst.displayPath())
	if err := replacePath(execCtx, tmp, dst); err != nil {
		_ = removePath(execCtx, tmp)
		return wrapPath(dst.displayPath(), err)
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
