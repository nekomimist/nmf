package jobs

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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

func TestAcknowledgeFailureMarksOnlyFailedJobs(t *testing.T) {
	failed := &Job{ID: 1, Status: StatusFailed}
	completed := &Job{ID: 2, Status: StatusCompleted}
	m := &Manager{history: []*Job{failed, completed}}

	if !m.AcknowledgeFailure(failed.ID) {
		t.Fatal("AcknowledgeFailure should report a failed job state change")
	}
	if !failed.Snapshot().FailureAcknowledged {
		t.Fatal("failed job should be acknowledged")
	}
	if m.AcknowledgeFailure(failed.ID) {
		t.Fatal("AcknowledgeFailure should not report a second state change")
	}
	if m.AcknowledgeFailure(completed.ID) {
		t.Fatal("AcknowledgeFailure should ignore completed jobs")
	}
	if completed.Snapshot().FailureAcknowledged {
		t.Fatal("completed job should not be acknowledged")
	}
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

type trackingSMBOps struct {
	fakeSMBOps
	renameCalls int
	renameErr   error
}

func (o *trackingSMBOps) Rename(oldpath, newpath string) error {
	o.renameCalls++
	return o.renameErr
}

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

func TestCopyToSameDirectoryUsesAutoSuffix(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "file.txt")
	want := []byte("keep me")
	if err := os.WriteFile(src, want, 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	job := &Job{Type: TypeCopy, ctx: context.Background()}
	if err := copyOrMovePath(job, src, tmpDir); err != nil {
		t.Fatalf("copy same directory should auto suffix: %v", err)
	}

	got, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("source should remain readable: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("source content changed: got %q want %q", got, want)
	}
	copied, err := os.ReadFile(filepath.Join(tmpDir, "file (1).txt"))
	if err != nil {
		t.Fatalf("auto-suffixed copy missing: %v", err)
	}
	if string(copied) != string(want) {
		t.Fatalf("copy content changed: got %q want %q", copied, want)
	}
}

func TestDeleteTrashJobUsesTrashBackend(t *testing.T) {
	oldTrashPath := trashPath
	defer func() { trashPath = oldTrashPath }()

	var got []string
	trashPath = func(_ context.Context, p string) error {
		got = append(got, p)
		return nil
	}

	j := &Job{
		Type:       TypeDelete,
		Sources:    []string{"one.txt", "two.txt"},
		DeleteMode: DeleteModeTrash,
		ctx:        context.Background(),
		TotalFiles: 2,
	}

	if err := (&Manager{}).runDeleteJob(j); err != nil {
		t.Fatalf("runDeleteJob returned error: %v", err)
	}
	if strings.Join(got, ",") != "one.txt,two.txt" {
		t.Fatalf("trash paths = %#v, want both sources", got)
	}
	if j.DoneFiles != 2 {
		t.Fatalf("DoneFiles = %d, want 2", j.DoneFiles)
	}
}

func TestPermanentDeleteRemovesDirectoryRecursively(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "delete-me")
	if err := os.MkdirAll(filepath.Join(root, "child"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "child", "file.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	j := &Job{Type: TypeDelete, DeleteMode: DeleteModePermanent, ctx: context.Background()}
	if err := deletePermanentPath(j, newExecutionContext(), root); err != nil {
		t.Fatalf("deletePermanentPath returned error: %v", err)
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatalf("deleted directory still exists or stat failed unexpectedly: %v", err)
	}
}

func TestPermanentDeleteDoesNotFollowDirectorySymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating directory symlinks on Windows often requires privileges")
	}
	tmp := t.TempDir()
	targetDir := filepath.Join(tmp, "target")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}
	targetFile := filepath.Join(targetDir, "keep.txt")
	if err := os.WriteFile(targetFile, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tmp, "link")
	if err := os.Symlink(targetDir, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	j := &Job{Type: TypeDelete, DeleteMode: DeleteModePermanent, ctx: context.Background()}
	if err := deletePermanentPath(j, newExecutionContext(), link); err != nil {
		t.Fatalf("deletePermanentPath returned error: %v", err)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("symlink still exists or stat failed unexpectedly: %v", err)
	}
	if _, err := os.Lstat(targetFile); err != nil {
		t.Fatalf("symlink target should remain, got stat error: %v", err)
	}
}

func TestCopyDirectorySymlinkCopiesLinkOnly(t *testing.T) {
	tmp := t.TempDir()
	targetDir, targetFile := makeSymlinkTargetTree(t, tmp)
	link := filepath.Join(tmp, "link")
	createDirectorySymlink(t, targetDir, link)
	dstRoot := filepath.Join(tmp, "dst")
	if err := os.Mkdir(dstRoot, 0755); err != nil {
		t.Fatal(err)
	}

	j := &Job{Type: TypeCopy, ctx: context.Background()}
	if err := copyOrMovePath(j, link, dstRoot); err != nil {
		t.Fatalf("copyOrMovePath returned error: %v", err)
	}

	dstLink := filepath.Join(dstRoot, "link")
	assertSymlink(t, dstLink)
	if _, err := os.Lstat(targetFile); err != nil {
		t.Fatalf("symlink target should remain, got stat error: %v", err)
	}
}

func TestMoveDirectorySymlinkMovesLinkOnly(t *testing.T) {
	tmp := t.TempDir()
	targetDir, targetFile := makeSymlinkTargetTree(t, tmp)
	link := filepath.Join(tmp, "link")
	createDirectorySymlink(t, targetDir, link)
	dstRoot := filepath.Join(tmp, "dst")
	if err := os.Mkdir(dstRoot, 0755); err != nil {
		t.Fatal(err)
	}

	j := &Job{Type: TypeMove, ctx: context.Background()}
	if err := copyOrMovePath(j, link, dstRoot); err != nil {
		t.Fatalf("copyOrMovePath returned error: %v", err)
	}

	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("source symlink still exists or stat failed unexpectedly: %v", err)
	}
	assertSymlink(t, filepath.Join(dstRoot, "link"))
	if _, err := os.Lstat(targetFile); err != nil {
		t.Fatalf("symlink target should remain, got stat error: %v", err)
	}
}

func makeSymlinkTargetTree(t *testing.T, root string) (string, string) {
	t.Helper()
	targetDir := filepath.Join(root, "target")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}
	targetFile := filepath.Join(targetDir, "keep.txt")
	if err := os.WriteFile(targetFile, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	return targetDir, targetFile
}

func createDirectorySymlink(t *testing.T, target string, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
}

func assertSymlink(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("Lstat(%q) returned error: %v", path, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%q mode = %v, want symlink", path, info.Mode())
	}
}

func TestValidateDeleteTargetRejectsFilesystemRoot(t *testing.T) {
	root := string(os.PathSeparator)
	if runtime.GOOS == "windows" {
		root = filepath.VolumeName(os.TempDir()) + string(os.PathSeparator)
	}

	err := validateDeleteTarget(executionPath{path: root, backend: backendLocal})
	if !errors.Is(err, errUnsafeDeleteTarget) {
		t.Fatalf("validateDeleteTarget(%q) error = %v, want errUnsafeDeleteTarget", root, err)
	}
}

func TestValidateDeleteTargetRejectsSMBShareRoot(t *testing.T) {
	err := validateDeleteTarget(executionPath{path: "/", backend: backendSMB})
	if !errors.Is(err, errUnsafeDeleteTarget) {
		t.Fatalf("validateDeleteTarget(SMB root) error = %v, want errUnsafeDeleteTarget", err)
	}
}

func TestMoveFileToSameDirectoryIsNoop(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "file.txt")
	want := []byte("do not delete")
	if err := os.WriteFile(src, want, 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	job := &Job{Type: TypeMove, ctx: context.Background()}
	if err := copyOrMovePath(job, src, tmpDir); err != nil {
		t.Fatalf("move same directory should be no-op: %v", err)
	}

	got, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("source should remain readable: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("source content changed: got %q want %q", got, want)
	}
}

func TestMoveDirectoryToSameParentIsNoop(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "dir")
	child := filepath.Join(srcDir, "child.txt")
	if err := os.Mkdir(srcDir, 0755); err != nil {
		t.Fatalf("make source dir: %v", err)
	}
	if err := os.WriteFile(child, []byte("child"), 0644); err != nil {
		t.Fatalf("write child: %v", err)
	}

	job := &Job{Type: TypeMove, ctx: context.Background()}
	if err := copyOrMovePath(job, srcDir, tmpDir); err != nil {
		t.Fatalf("move directory to same parent should be no-op: %v", err)
	}

	if _, err := os.Stat(child); err != nil {
		t.Fatalf("source directory contents should remain: %v", err)
	}
}

func TestMoveFileUsesRenameFastPath(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	dstDir := filepath.Join(tmpDir, "dst")
	if err := os.Mkdir(srcDir, 0755); err != nil {
		t.Fatalf("make source dir: %v", err)
	}
	if err := os.Mkdir(dstDir, 0755); err != nil {
		t.Fatalf("make destination dir: %v", err)
	}
	src := filepath.Join(srcDir, "file.txt")
	if err := os.WriteFile(src, []byte("source"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	job := &Job{Type: TypeMove, ctx: context.Background()}
	if err := copyOrMovePath(job, src, dstDir); err != nil {
		t.Fatalf("move should use rename fast path: %v", err)
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("source should be moved away, got %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(dstDir, "file.txt")); err != nil || string(got) != "source" {
		t.Fatalf("destination content wrong: got %q err=%v", got, err)
	}
	part := filepath.Join(dstDir, "file.txt.part")
	if _, err := os.Stat(part); !os.IsNotExist(err) {
		t.Fatalf("copy temp file should not be created on rename fast path, got %v", err)
	}
}

func TestMoveDirectoryIntoItselfIsRejected(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "dir")
	destDir := filepath.Join(srcDir, "child")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("make nested dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("source"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	job := &Job{Type: TypeMove, ctx: context.Background()}
	if err := copyOrMovePath(job, srcDir, destDir); err == nil || !strings.Contains(err.Error(), "cannot move a directory into itself") {
		t.Fatalf("move into descendant error = %v, want self-move rejection", err)
	}
	if _, err := os.Stat(filepath.Join(srcDir, "file.txt")); err != nil {
		t.Fatalf("source should remain after rejected move: %v", err)
	}
}

func TestMoveDirectoryMergeSkipsRenameFastPath(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src", "dir")
	dstParent := filepath.Join(tmpDir, "dst")
	dstDir := filepath.Join(dstParent, "dir")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("make source dir: %v", err)
	}
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		t.Fatalf("make destination dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "from-source.txt"), []byte("source"), 0644); err != nil {
		t.Fatalf("write source child: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dstDir, "existing.txt"), []byte("existing"), 0644); err != nil {
		t.Fatalf("write destination child: %v", err)
	}

	job := &Job{Type: TypeMove, ctx: context.Background()}
	if err := copyOrMovePath(job, srcDir, dstParent); err != nil {
		t.Fatalf("move directory merge failed: %v", err)
	}
	if _, err := os.Stat(srcDir); !os.IsNotExist(err) {
		t.Fatalf("source directory should be removed after merge, got %v", err)
	}
	for _, name := range []string{"from-source.txt", "existing.txt"} {
		if _, err := os.Stat(filepath.Join(dstDir, name)); err != nil {
			t.Fatalf("merged destination missing %s: %v", name, err)
		}
	}
}

func TestMoveSMBUsesRenameFastPathWithinShare(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(src, []byte("source"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	fi, err := os.Lstat(src)
	if err != nil {
		t.Fatalf("stat source: %v", err)
	}

	ops := &trackingSMBOps{}
	root := "smb://host/share"
	job := &Job{ID: 42, Type: TypeMove, ctx: context.Background()}
	execCtx := newExecutionContext()
	srcPath := executionPath{backend: backendSMB, path: "/from/file.txt", smb: ops, smbDisplayRoot: root}
	dstPath := executionPath{backend: backendSMB, path: "/to/file.txt", smb: ops, smbDisplayRoot: root}

	moved, err := tryFastMovePath(job, execCtx, srcPath, dstPath, fi, false)
	if err != nil {
		t.Fatalf("tryFastMovePath returned error: %v", err)
	}
	if !moved {
		t.Fatal("tryFastMovePath should report moved")
	}
	if ops.renameCalls != 1 {
		t.Fatalf("SMB Rename calls = %d, want 1", ops.renameCalls)
	}
}

func TestCopyCollisionSkipDoesNotOverwrite(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	src := filepath.Join(srcDir, "file.txt")
	dst := filepath.Join(dstDir, "file.txt")
	if err := os.WriteFile(src, []byte("source"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := os.WriteFile(dst, []byte("existing"), 0644); err != nil {
		t.Fatalf("write destination: %v", err)
	}

	job := &Job{
		Type: TypeCopy,
		ctx:  context.Background(),
		Resolver: func(context.Context, ConflictRequest) ConflictResolution {
			return ConflictResolution{Action: ConflictSkip}
		},
	}
	if err := copyOrMovePath(job, src, dstDir); !errors.Is(err, errSkipped) {
		t.Fatalf("expected skipped copy, got %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(got) != "existing" {
		t.Fatalf("destination overwritten: %q", got)
	}
}

func TestMoveCollisionRenameDoesNotOverwrite(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	src := filepath.Join(srcDir, "file.txt")
	existing := filepath.Join(dstDir, "file.txt")
	renamed := filepath.Join(dstDir, "moved.txt")
	if err := os.WriteFile(src, []byte("source"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := os.WriteFile(existing, []byte("existing"), 0644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	job := &Job{
		Type: TypeMove,
		ctx:  context.Background(),
		Resolver: func(context.Context, ConflictRequest) ConflictResolution {
			return ConflictResolution{Action: ConflictRename, NewName: "moved.txt"}
		},
	}
	if err := copyOrMovePath(job, src, dstDir); err != nil {
		t.Fatalf("move with rename failed: %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("source should be removed after move, got %v", err)
	}
	if got, err := os.ReadFile(existing); err != nil || string(got) != "existing" {
		t.Fatalf("existing destination changed: got %q err=%v", got, err)
	}
	if got, err := os.ReadFile(renamed); err != nil || string(got) != "source" {
		t.Fatalf("renamed destination wrong: got %q err=%v", got, err)
	}
}

func TestCopyCollisionOverwriteIfNewerReplacesClearlyOlderDestination(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	src := filepath.Join(srcDir, "file.txt")
	dst := filepath.Join(dstDir, "file.txt")
	if err := os.WriteFile(src, []byte("source"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := os.WriteFile(dst, []byte("existing"), 0644); err != nil {
		t.Fatalf("write destination: %v", err)
	}
	base := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(dst, base, base); err != nil {
		t.Fatalf("set destination time: %v", err)
	}
	if err := os.Chtimes(src, base.Add(3*time.Second), base.Add(3*time.Second)); err != nil {
		t.Fatalf("set source time: %v", err)
	}

	job := &Job{
		Type: TypeCopy,
		ctx:  context.Background(),
		Resolver: func(context.Context, ConflictRequest) ConflictResolution {
			return ConflictResolution{Action: ConflictOverwriteIfNewer}
		},
	}
	if err := copyOrMovePath(job, src, dstDir); err != nil {
		t.Fatalf("copy with overwrite-if-newer failed: %v", err)
	}
	if got, err := os.ReadFile(dst); err != nil || string(got) != "source" {
		t.Fatalf("destination not overwritten: got %q err=%v", got, err)
	}
}

func TestCopyCollisionOverwriteIfNewerSkipsWithinFATTolerance(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	src := filepath.Join(srcDir, "file.txt")
	dst := filepath.Join(dstDir, "file.txt")
	if err := os.WriteFile(src, []byte("source"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := os.WriteFile(dst, []byte("existing"), 0644); err != nil {
		t.Fatalf("write destination: %v", err)
	}
	base := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(dst, base, base); err != nil {
		t.Fatalf("set destination time: %v", err)
	}
	if err := os.Chtimes(src, base.Add(2*time.Second), base.Add(2*time.Second)); err != nil {
		t.Fatalf("set source time: %v", err)
	}

	job := &Job{
		Type: TypeCopy,
		ctx:  context.Background(),
		Resolver: func(context.Context, ConflictRequest) ConflictResolution {
			return ConflictResolution{Action: ConflictOverwriteIfNewer}
		},
	}
	if err := copyOrMovePath(job, src, dstDir); !errors.Is(err, errSkipped) {
		t.Fatalf("expected skipped copy, got %v", err)
	}
	if got, err := os.ReadFile(dst); err != nil || string(got) != "existing" {
		t.Fatalf("destination changed: got %q err=%v", got, err)
	}
}

func TestMoveCollisionOverwriteReplacesDestinationAndRemovesSource(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	src := filepath.Join(srcDir, "file.txt")
	dst := filepath.Join(dstDir, "file.txt")
	if err := os.WriteFile(src, []byte("source"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := os.WriteFile(dst, []byte("existing"), 0644); err != nil {
		t.Fatalf("write destination: %v", err)
	}

	job := &Job{
		Type: TypeMove,
		ctx:  context.Background(),
		Resolver: func(context.Context, ConflictRequest) ConflictResolution {
			return ConflictResolution{Action: ConflictOverwrite}
		},
	}
	if err := copyOrMovePath(job, src, dstDir); err != nil {
		t.Fatalf("move with overwrite failed: %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("source should be removed after move, got %v", err)
	}
	if got, err := os.ReadFile(dst); err != nil || string(got) != "source" {
		t.Fatalf("destination not overwritten: got %q err=%v", got, err)
	}
}

func TestOverwriteIfNewerApplyToRemainingCollisions(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	base := time.Unix(1_700_000_000, 0)
	for _, name := range []string{"a.txt", "b.txt"} {
		src := filepath.Join(srcDir, name)
		dst := filepath.Join(dstDir, name)
		if err := os.WriteFile(src, []byte("source-"+name), 0644); err != nil {
			t.Fatalf("write source %s: %v", name, err)
		}
		if err := os.WriteFile(dst, []byte("existing"), 0644); err != nil {
			t.Fatalf("write destination %s: %v", name, err)
		}
		if err := os.Chtimes(dst, base, base); err != nil {
			t.Fatalf("set destination time %s: %v", name, err)
		}
		if err := os.Chtimes(src, base.Add(3*time.Second), base.Add(3*time.Second)); err != nil {
			t.Fatalf("set source time %s: %v", name, err)
		}
	}

	var calls int
	job := &Job{
		Type: TypeCopy,
		ctx:  context.Background(),
		Resolver: func(context.Context, ConflictRequest) ConflictResolution {
			calls++
			return ConflictResolution{Action: ConflictOverwriteIfNewer, ApplyToRest: true}
		},
	}
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := copyOrMovePath(job, filepath.Join(srcDir, name), dstDir); err != nil {
			t.Fatalf("copy %s failed: %v", name, err)
		}
	}
	if calls != 1 {
		t.Fatalf("resolver calls: got %d want 1", calls)
	}
	for _, name := range []string{"a.txt", "b.txt"} {
		if got, err := os.ReadFile(filepath.Join(dstDir, name)); err != nil || string(got) != "source-"+name {
			t.Fatalf("destination %s not overwritten: got %q err=%v", name, got, err)
		}
	}
}

func TestOverwriteSkipsTypeMismatch(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	src := filepath.Join(srcDir, "item")
	dst := filepath.Join(dstDir, "item")
	if err := os.WriteFile(src, []byte("source"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := os.Mkdir(dst, 0755); err != nil {
		t.Fatalf("make destination directory: %v", err)
	}

	job := &Job{
		Type: TypeCopy,
		ctx:  context.Background(),
		Resolver: func(context.Context, ConflictRequest) ConflictResolution {
			return ConflictResolution{Action: ConflictOverwrite}
		},
	}
	if err := copyOrMovePath(job, src, dstDir); !errors.Is(err, errSkipped) {
		t.Fatalf("expected skipped copy, got %v", err)
	}
	if info, err := os.Stat(dst); err != nil || !info.IsDir() {
		t.Fatalf("destination directory changed: info=%v err=%v", info, err)
	}
}

func TestApplyAutoSuffixToRemainingCollisions(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(srcDir, name), []byte(name), 0644); err != nil {
			t.Fatalf("write source %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dstDir, name), []byte("existing"), 0644); err != nil {
			t.Fatalf("write destination %s: %v", name, err)
		}
	}

	var calls int
	job := &Job{
		Type: TypeCopy,
		ctx:  context.Background(),
		Resolver: func(context.Context, ConflictRequest) ConflictResolution {
			calls++
			return ConflictResolution{Action: ConflictAutoSuffix, ApplyToRest: true}
		},
	}
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := copyOrMovePath(job, filepath.Join(srcDir, name), dstDir); err != nil {
			t.Fatalf("copy %s failed: %v", name, err)
		}
	}
	if calls != 1 {
		t.Fatalf("resolver calls: got %d want 1", calls)
	}
	for _, name := range []string{"a (1).txt", "b (1).txt"} {
		if _, err := os.Stat(filepath.Join(dstDir, name)); err != nil {
			t.Fatalf("expected auto-suffixed file %s: %v", name, err)
		}
	}
}

func TestDirectoryCollisionMergesContents(t *testing.T) {
	srcRoot := t.TempDir()
	dstRoot := t.TempDir()
	srcDir := filepath.Join(srcRoot, "dir")
	dstDir := filepath.Join(dstRoot, "dir")
	if err := os.Mkdir(srcDir, 0755); err != nil {
		t.Fatalf("make source dir: %v", err)
	}
	if err := os.Mkdir(dstDir, 0755); err != nil {
		t.Fatalf("make destination dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "child.txt"), []byte("child"), 0644); err != nil {
		t.Fatalf("write source child: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dstDir, "other.txt"), []byte("other"), 0644); err != nil {
		t.Fatalf("write destination child: %v", err)
	}

	job := &Job{Type: TypeCopy, ctx: context.Background()}
	if err := copyOrMovePath(job, srcDir, dstRoot); err != nil {
		t.Fatalf("copy directory merge failed: %v", err)
	}
	for _, name := range []string{"child.txt", "other.txt"} {
		if _, err := os.Stat(filepath.Join(dstDir, name)); err != nil {
			t.Fatalf("merged directory missing %s: %v", name, err)
		}
	}
}

func TestMoveDirectorySkippedChildKeepsSourceDirectory(t *testing.T) {
	srcRoot := t.TempDir()
	dstRoot := t.TempDir()
	srcDir := filepath.Join(srcRoot, "dir")
	dstDir := filepath.Join(dstRoot, "dir")
	if err := os.Mkdir(srcDir, 0755); err != nil {
		t.Fatalf("make source dir: %v", err)
	}
	if err := os.Mkdir(dstDir, 0755); err != nil {
		t.Fatalf("make destination dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "child.txt"), []byte("source"), 0644); err != nil {
		t.Fatalf("write source child: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dstDir, "child.txt"), []byte("existing"), 0644); err != nil {
		t.Fatalf("write destination child: %v", err)
	}

	job := &Job{
		Type: TypeMove,
		ctx:  context.Background(),
		Resolver: func(context.Context, ConflictRequest) ConflictResolution {
			return ConflictResolution{Action: ConflictSkip}
		},
	}
	if err := copyOrMovePath(job, srcDir, dstRoot); !errors.Is(err, errSkipped) {
		t.Fatalf("expected skipped move, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(srcDir, "child.txt")); err != nil {
		t.Fatalf("skipped child should remain in source: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(dstDir, "child.txt")); err != nil || string(got) != "existing" {
		t.Fatalf("destination child changed: got %q err=%v", got, err)
	}
}

func TestConflictCancelStopsJob(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	src := filepath.Join(srcDir, "file.txt")
	if err := os.WriteFile(src, []byte("source"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dstDir, "file.txt"), []byte("existing"), 0644); err != nil {
		t.Fatalf("write destination: %v", err)
	}

	job := &Job{
		Type: TypeCopy,
		ctx:  context.Background(),
		Resolver: func(context.Context, ConflictRequest) ConflictResolution {
			return ConflictResolution{Action: ConflictCancelJob}
		},
	}
	if err := copyOrMovePath(job, src, dstDir); !errors.Is(err, errCanceled) {
		t.Fatalf("expected canceled error, got %v", err)
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
