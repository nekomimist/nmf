package jobs

import (
	"errors"
	"io"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"nmf/internal/fileinfo"
)

func TestSubscribeUnsubscribe(t *testing.T) {
	m := &Manager{}
	var aCount int32
	var bCount int32

	unsubA := m.Subscribe(func() { atomic.AddInt32(&aCount, 1) })
	unsubB := m.Subscribe(func() { atomic.AddInt32(&bCount, 1) })

	m.notify()
	if got := atomic.LoadInt32(&aCount); got != 1 {
		t.Fatalf("subscriber A should be called once, got %d", got)
	}
	if got := atomic.LoadInt32(&bCount); got != 1 {
		t.Fatalf("subscriber B should be called once, got %d", got)
	}

	unsubA()
	m.notify()
	if got := atomic.LoadInt32(&aCount); got != 1 {
		t.Fatalf("subscriber A should not be called after unsubscribe, got %d", got)
	}
	if got := atomic.LoadInt32(&bCount); got != 2 {
		t.Fatalf("subscriber B should still be called, got %d", got)
	}

	// Unsubscribe must be idempotent.
	unsubA()
	m.notify()
	if got := atomic.LoadInt32(&aCount); got != 1 {
		t.Fatalf("subscriber A should remain unsubscribed, got %d", got)
	}
	if got := atomic.LoadInt32(&bCount); got != 3 {
		t.Fatalf("subscriber B should still receive notifications, got %d", got)
	}

	unsubB()
	m.notify()
	if got := atomic.LoadInt32(&aCount); got != 1 {
		t.Fatalf("subscriber A should remain unsubscribed, got %d", got)
	}
	if got := atomic.LoadInt32(&bCount); got != 3 {
		t.Fatalf("subscriber B should stop after unsubscribe, got %d", got)
	}
}

func TestSubscribeNilCallbackReturnsNoopUnsubscribe(t *testing.T) {
	m := &Manager{}
	unsub := m.Subscribe(nil)
	unsub()
	// notify must remain safe even with no subscribers.
	m.notify()
}

func TestResolveExecutionPath_Empty(t *testing.T) {
	if _, err := resolveExecutionPath("   "); err == nil {
		t.Fatalf("expected error for empty path")
	}
}

func TestResolveExecutionPath_Local(t *testing.T) {
	got, err := resolveExecutionPath(".")
	if err != nil {
		t.Fatalf("expected local path to resolve, got error: %v", err)
	}
	if got.path == "" {
		t.Fatalf("expected non-empty resolved path")
	}
	if got.backend != backendLocal {
		t.Fatalf("expected local backend, got %v", got.backend)
	}
}

func TestResolveExecutionPath_SMBProviderBehavior(t *testing.T) {
	// Pick a very unlikely host/share so this normally resolves to direct SMB provider.
	// If the environment happens to have a matching mount/provider-local mapping,
	// the expected behavior changes accordingly.
	input := "smb://__codex_unlikely_host__/__codex_unlikely_share__"

	_, parsed, rerr := fileinfo.ResolveRead(input)
	got, err := resolveExecutionPath(input)

	if rerr != nil {
		if err == nil {
			t.Fatalf("expected resolveExecutionPath to return error when resolver fails")
		}
		return
	}

	if parsed.Scheme == fileinfo.SchemeSMB && parsed.Provider == "smb" {
		if runtime.GOOS == "linux" {
			if err != nil {
				t.Fatalf("expected direct SMB provider to resolve on linux, got error: %v", err)
			}
			if got.backend != backendSMB {
				t.Fatalf("expected SMB backend, got %v", got.backend)
			}
		} else {
			if err == nil {
				t.Fatalf("expected direct SMB provider to be unsupported on non-linux")
			}
		}
		return
	}

	if err != nil {
		t.Fatalf("expected local-provider SMB to resolve, got error: %v", err)
	}
	if got.path == "" {
		t.Fatalf("expected non-empty resolved path for local-provider SMB")
	}
	if got.backend != backendLocal {
		t.Fatalf("expected local backend for local-provider SMB, got %v", got.backend)
	}
}

type fakeSMBOps struct{}

func (fakeSMBOps) ReadDir(path string) ([]os.DirEntry, error) { return nil, nil }
func (fakeSMBOps) Stat(path string) (os.FileInfo, error)      { return nil, os.ErrNotExist }
func (fakeSMBOps) Lstat(path string) (os.FileInfo, error)     { return nil, os.ErrNotExist }
func (fakeSMBOps) Open(path string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (fakeSMBOps) OpenFile(path string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	return nopReadWriteCloser{}, nil
}
func (fakeSMBOps) MkdirAll(path string, perm os.FileMode) error { return nil }
func (fakeSMBOps) Remove(path string) error                     { return nil }
func (fakeSMBOps) Rename(oldpath, newpath string) error         { return nil }
func (fakeSMBOps) Readlink(path string) (string, error)         { return "", nil }
func (fakeSMBOps) Symlink(target, linkpath string) error        { return nil }
func (fakeSMBOps) Base(p string) string                         { return pathBase(p) }
func (fakeSMBOps) Join(elem ...string) string                   { return strings.Join(elem, "/") }

type nopReadWriteCloser struct{}

func (nopReadWriteCloser) Read([]byte) (int, error)    { return 0, io.EOF }
func (nopReadWriteCloser) Write(p []byte) (int, error) { return len(p), nil }
func (nopReadWriteCloser) Close() error                { return nil }

type fakeSMBSession struct {
	fakeSMBOps
	closeCalls int
}

func (s *fakeSMBSession) Close() error {
	s.closeCalls++
	return nil
}

type fakeSMBOpener struct {
	openCalls int
	err       error
	new       func() fileinfo.SMBSession
}

func (o *fakeSMBOpener) OpenSession() (fileinfo.SMBSession, error) {
	o.openCalls++
	if o.err != nil {
		return nil, o.err
	}
	if o.new != nil {
		return o.new(), nil
	}
	return &fakeSMBSession{}, nil
}

func TestExecutionContextSMBSessionReusePerRoot(t *testing.T) {
	ctx := newExecutionContext()
	session := &fakeSMBSession{}
	opener := &fakeSMBOpener{
		new: func() fileinfo.SMBSession { return session },
	}

	p := executionPath{
		backend:        backendSMB,
		path:           "/file.txt",
		smb:            fakeSMBOps{},
		smbOpener:      opener,
		smbDisplayRoot: "smb://host/share",
	}

	ops1, err := ctx.smbOpsFor(p)
	if err != nil {
		t.Fatalf("first smbOpsFor failed: %v", err)
	}
	ops2, err := ctx.smbOpsFor(p)
	if err != nil {
		t.Fatalf("second smbOpsFor failed: %v", err)
	}
	if opener.openCalls != 1 {
		t.Fatalf("expected one OpenSession call, got %d", opener.openCalls)
	}
	if ops1 != ops2 {
		t.Fatalf("expected cached session ops to be reused")
	}
	if err := ctx.close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	if session.closeCalls != 1 {
		t.Fatalf("expected session Close once, got %d", session.closeCalls)
	}
}

func TestExecutionContextSMBSessionSeparatePerRoot(t *testing.T) {
	ctx := newExecutionContext()
	var sessions []*fakeSMBSession
	opener := &fakeSMBOpener{
		new: func() fileinfo.SMBSession {
			s := &fakeSMBSession{}
			sessions = append(sessions, s)
			return s
		},
	}

	p1 := executionPath{
		backend:        backendSMB,
		path:           "/a",
		smb:            fakeSMBOps{},
		smbOpener:      opener,
		smbDisplayRoot: "smb://host/share1",
	}
	p2 := executionPath{
		backend:        backendSMB,
		path:           "/b",
		smb:            fakeSMBOps{},
		smbOpener:      opener,
		smbDisplayRoot: "smb://host/share2",
	}

	if _, err := ctx.smbOpsFor(p1); err != nil {
		t.Fatalf("smbOpsFor p1 failed: %v", err)
	}
	if _, err := ctx.smbOpsFor(p2); err != nil {
		t.Fatalf("smbOpsFor p2 failed: %v", err)
	}
	if _, err := ctx.smbOpsFor(p1); err != nil {
		t.Fatalf("smbOpsFor p1 (cached) failed: %v", err)
	}

	if opener.openCalls != 2 {
		t.Fatalf("expected two OpenSession calls for distinct roots, got %d", opener.openCalls)
	}

	if err := ctx.close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	for i, s := range sessions {
		if s.closeCalls != 1 {
			t.Fatalf("session %d close count: got %d want 1", i, s.closeCalls)
		}
	}
}

func TestExecutionContextSMBSessionFallbackWithoutOpener(t *testing.T) {
	ctx := newExecutionContext()
	baseOps := &fakeSMBOps{}
	p := executionPath{
		backend:        backendSMB,
		path:           "/x",
		smb:            baseOps,
		smbDisplayRoot: "smb://host/share",
	}

	ops, err := ctx.smbOpsFor(p)
	if err != nil {
		t.Fatalf("smbOpsFor failed: %v", err)
	}
	if ops != baseOps {
		t.Fatalf("expected fallback to base SMB ops without opener")
	}
	if got := len(ctx.smbSessions); got != 0 {
		t.Fatalf("expected no cached sessions, got %d", got)
	}
}

func TestExecutionContextSMBSessionOpenError(t *testing.T) {
	ctx := newExecutionContext()
	wantErr := errors.New("open session failed")
	opener := &fakeSMBOpener{err: wantErr}
	p := executionPath{
		backend:        backendSMB,
		path:           "/x",
		smb:            fakeSMBOps{},
		smbOpener:      opener,
		smbDisplayRoot: "smb://host/share",
	}

	if _, err := ctx.smbOpsFor(p); !errors.Is(err, wantErr) {
		t.Fatalf("expected open error %v, got %v", wantErr, err)
	}
	if opener.openCalls != 1 {
		t.Fatalf("expected one OpenSession attempt, got %d", opener.openCalls)
	}
}

func pathBase(p string) string {
	p = strings.TrimSuffix(p, "/")
	idx := strings.LastIndex(p, "/")
	if idx < 0 {
		return p
	}
	return p[idx+1:]
}
