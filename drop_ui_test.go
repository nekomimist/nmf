package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"

	"nmf/internal/jobs"
	"nmf/internal/ui"
)

func TestDroppedURIPathsAcceptsLocalFilesAndDeduplicates(t *testing.T) {
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "a.txt")
	if err := writeTestFile(filePath); err != nil {
		t.Fatal(err)
	}

	paths, err := droppedURIPaths([]fyne.URI{
		storage.NewFileURI(filePath),
		storage.NewFileURI(filePath),
	})
	if err != nil {
		t.Fatalf("droppedURIPaths returned error: %v", err)
	}
	if len(paths) != 1 || paths[0] != filePath {
		t.Fatalf("paths = %#v, want only %q", paths, filePath)
	}
}

func TestDroppedURIPathsRejectsNonFileURI(t *testing.T) {
	uri, err := storage.ParseURI("smb://server/share/file.txt")
	if err != nil {
		t.Fatal(err)
	}

	_, err = droppedURIPaths([]fyne.URI{uri})
	if err == nil || !strings.Contains(err.Error(), "Only local files") {
		t.Fatalf("error = %v, want local-file rejection", err)
	}
}

func TestDroppedURIPathsRejectsMissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.txt")

	_, err := droppedURIPaths([]fyne.URI{storage.NewFileURI(missing)})
	if err == nil || !strings.Contains(err.Error(), "Cannot access dropped file") {
		t.Fatalf("error = %v, want missing-file rejection", err)
	}
}

func TestDropDestinationAcceptsLocalDirectory(t *testing.T) {
	tmp := t.TempDir()

	dest, err := dropDestination(tmp)
	if err != nil {
		t.Fatalf("dropDestination returned error: %v", err)
	}
	if dest != tmp {
		t.Fatalf("dest = %q, want %q", dest, tmp)
	}
}

func TestDropDestinationRejectsArchiveAndSMB(t *testing.T) {
	cases := []struct {
		path    string
		wantErr string
	}{
		{path: "/tmp/archive.zip!/docs", wantErr: "archive views"},
		{path: "smb://server/share", wantErr: "direct SMB views"},
	}
	for _, tc := range cases {
		_, err := dropDestination(tc.path)
		if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
			t.Fatalf("dropDestination(%q) error = %v, want %q", tc.path, err, tc.wantErr)
		}
	}
}

func TestDroppedMoveSourcesSkipsSameDirectory(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "a.txt")
	other := filepath.Join(t.TempDir(), "b.txt")

	got := droppedMoveSources([]string{src, other}, tmp)
	if len(got) != 1 || got[0] != other {
		t.Fatalf("droppedMoveSources = %#v, want only %q", got, other)
	}
}

func TestEnqueueDroppedTransferSelectsCopyOrMove(t *testing.T) {
	resolver := jobs.ConflictResolver(func(context.Context, jobs.ConflictRequest) jobs.ConflictResolution {
		return jobs.ConflictResolution{}
	})
	sources := []string{"/tmp/a.txt", "/tmp/b.txt"}
	dest := "/tmp/dest"

	copyMgr := &fakeDropJobManager{}
	enqueueDroppedTransfer(copyMgr, ui.OpCopy, sources, dest, resolver)
	if copyMgr.copyCalls != 1 || copyMgr.moveCalls != 0 {
		t.Fatalf("copy calls = %d, move calls = %d; want copy only", copyMgr.copyCalls, copyMgr.moveCalls)
	}
	if strings.Join(copyMgr.sources, "|") != strings.Join(sources, "|") || copyMgr.dest != dest || copyMgr.resolver == nil {
		t.Fatalf("copy manager recorded sources=%#v dest=%q resolver nil=%t", copyMgr.sources, copyMgr.dest, copyMgr.resolver == nil)
	}

	moveMgr := &fakeDropJobManager{}
	enqueueDroppedTransfer(moveMgr, ui.OpMove, sources, dest, resolver)
	if moveMgr.moveCalls != 1 || moveMgr.copyCalls != 0 {
		t.Fatalf("move calls = %d, copy calls = %d; want move only", moveMgr.moveCalls, moveMgr.copyCalls)
	}
}

func writeTestFile(path string) error {
	return os.WriteFile(path, []byte("x"), 0o600)
}

type fakeDropJobManager struct {
	copyCalls int
	moveCalls int
	sources   []string
	dest      string
	resolver  jobs.ConflictResolver
}

func (f *fakeDropJobManager) EnqueueCopyWithResolver(sources []string, dest string, resolver jobs.ConflictResolver) *jobs.Job {
	f.copyCalls++
	f.sources = append([]string(nil), sources...)
	f.dest = dest
	f.resolver = resolver
	return &jobs.Job{Type: jobs.TypeCopy}
}

func (f *fakeDropJobManager) EnqueueMoveWithResolver(sources []string, dest string, resolver jobs.ConflictResolver) *jobs.Job {
	f.moveCalls++
	f.sources = append([]string(nil), sources...)
	f.dest = dest
	f.resolver = resolver
	return &jobs.Job{Type: jobs.TypeMove}
}
