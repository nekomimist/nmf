package browser

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"nmf/internal/fileinfo"
)

func TestReadDirectoryWithParentFallbackFindsNearestAccessibleParent(t *testing.T) {
	parent := t.TempDir()
	requested := filepath.Join(parent, "missing", "child")
	missingParent := filepath.Dir(requested)
	var calls []string

	entries, opened, usedFallback, err := ReadDirectoryWithParentFallback(
		context.Background(), requested, true,
		func(_ context.Context, path string) ([]os.DirEntry, error) {
			calls = append(calls, path)
			switch path {
			case requested, missingParent:
				return nil, fs.ErrNotExist
			case parent:
				return []os.DirEntry{}, nil
			default:
				t.Fatalf("unexpected read path %q", path)
				return nil, nil
			}
		},
		nil,
	)
	if err != nil {
		t.Fatalf("readDirectoryWithParentFallback returned error: %v", err)
	}
	if !usedFallback {
		t.Fatal("usedFallback = false, want true")
	}
	if opened != parent {
		t.Fatalf("opened = %q, want %q", opened, parent)
	}
	if entries == nil {
		t.Fatal("entries = nil, want successful empty directory listing")
	}
	if want := []string{requested, missingParent, parent}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("read paths = %#v, want %#v", calls, want)
	}
}

func TestReadDirectoryWithParentFallbackRecognizesPortableMissingPath(t *testing.T) {
	parent := t.TempDir()
	requested := filepath.Join(parent, "missing", "child")

	entries, opened, usedFallback, err := ReadDirectoryWithParentFallback(
		context.Background(), requested, true, fileinfo.ReadDirPortableContext,
		nil,
	)
	if err != nil {
		t.Fatalf("readDirectoryWithParentFallback returned error: %v", err)
	}
	if !usedFallback {
		t.Fatal("usedFallback = false, want true")
	}
	if opened != parent {
		t.Fatalf("opened = %q, want %q", opened, parent)
	}
	if entries == nil {
		t.Fatal("entries = nil, want successful empty directory listing")
	}
}

func TestReadDirectoryWithParentFallbackDoesNotMaskOtherFailures(t *testing.T) {
	requested := filepath.Join(t.TempDir(), "missing")
	parent := filepath.Dir(requested)
	tests := []struct {
		name          string
		allowFallback bool
		read          func(path string) error
		wantCalls     []string
		wantErr       error
	}{
		{
			name:          "disabled",
			allowFallback: false,
			read: func(string) error {
				return fs.ErrNotExist
			},
			wantCalls: []string{requested},
			wantErr:   fs.ErrNotExist,
		},
		{
			name:          "permission denied",
			allowFallback: true,
			read: func(string) error {
				return fs.ErrPermission
			},
			wantCalls: []string{requested},
			wantErr:   fs.ErrPermission,
		},
		{
			name:          "parent becomes inaccessible",
			allowFallback: true,
			read: func(path string) error {
				if path == requested {
					return fs.ErrNotExist
				}
				if path == parent {
					return fs.ErrPermission
				}
				t.Fatalf("unexpected read path %q", path)
				return nil
			},
			wantCalls: []string{requested, parent},
			wantErr:   fs.ErrPermission,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			_, opened, usedFallback, err := ReadDirectoryWithParentFallback(
				context.Background(), requested, tt.allowFallback,
				func(_ context.Context, path string) ([]os.DirEntry, error) {
					calls = append(calls, path)
					return nil, tt.read(path)
				},
				nil,
			)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if opened != "" {
				t.Fatalf("opened = %q, want empty on failure", opened)
			}
			if usedFallback {
				t.Fatal("usedFallback = true, want false on failure")
			}
			if !reflect.DeepEqual(calls, tt.wantCalls) {
				t.Fatalf("read paths = %#v, want %#v", calls, tt.wantCalls)
			}
		})
	}
}

func TestReadDirectoryWithParentFallbackStopsAtSMBShareRoot(t *testing.T) {
	const shareRoot = "smb://server/share"
	var calls []string
	_, opened, usedFallback, err := ReadDirectoryWithParentFallback(
		context.Background(), shareRoot, true,
		func(_ context.Context, path string) ([]os.DirEntry, error) {
			calls = append(calls, path)
			return nil, fs.ErrNotExist
		},
		nil,
	)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("error = %v, want fs.ErrNotExist", err)
	}
	if opened != "" || usedFallback {
		t.Fatalf("opened = %q, usedFallback = %t; want no fallback", opened, usedFallback)
	}
	if want := []string{shareRoot}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("read paths = %#v, want %#v", calls, want)
	}
}

// TestReadDirectoryWithParentFallbackStopsOnENOTDIR locks in the ENOTDIR vs
// ENOENT boundary: fileinfo.IsNotExist (see internal/fileinfo/not_exist.go)
// only matches errors.Is(err, fs.ErrNotExist) plus provider-native
// not-exist errors, and ENOTDIR is neither, so a request whose failure is
// "not a directory" (an intermediate path component exists but is a regular
// file) must stop and surface the error instead of walking up to a parent.
func TestReadDirectoryWithParentFallbackStopsOnENOTDIR(t *testing.T) {
	parent := t.TempDir()
	blocker := filepath.Join(parent, "blocker.txt")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	requested := filepath.Join(blocker, "child")

	entries, opened, usedFallback, err := ReadDirectoryWithParentFallback(
		context.Background(), requested, true, fileinfo.ReadDirPortableContext,
		nil,
	)
	if err == nil {
		t.Fatal("readDirectoryWithParentFallback succeeded, want ENOTDIR surfaced")
	}
	if fileinfo.IsNotExist(err) {
		t.Fatalf("IsNotExist(%v) = true, want false: ENOTDIR must not be classified as missing", err)
	}
	if opened != "" {
		t.Fatalf("opened = %q, want empty on stop", opened)
	}
	if usedFallback {
		t.Fatal("usedFallback = true, want false: ENOTDIR must not trigger the parent walk")
	}
	if entries != nil {
		t.Fatalf("entries = %v, want nil on stop", entries)
	}
}

// TestReadDirectoryWithParentFallbackEscapesArchiveBoundaryToFilesystemParent
// locks in archiveParentPath's escape hatch (internal/fileinfo/archive_path.go):
// once the walk reaches the archive root ("archive.ext!/", inner == "."),
// ParentPath delegates to ParentPath(archiveFile), stepping out of the
// archive scheme entirely and onto the archive file's own filesystem parent.
// This matters when the archive file itself no longer opens (e.g. it was
// deleted): identifyArchiveFormat's os.Open on the missing archive produces
// a plain *PathError satisfying fileinfo.IsNotExist, so the walk treats a
// vanished archive exactly like a vanished directory and keeps climbing
// plain filesystem parents afterward.
//
// The archive read itself (opening an actual .zip via archives.ArchiveFS) is
// not exercised here: readDirectoryWithParentFallback only depends on the
// injected read function plus fileinfo.ParentPath/archiveParentPath, both
// pure path functions, so a fake reader fully exercises the real boundary
// logic without needing a real archive file on disk.
func TestReadDirectoryWithParentFallbackEscapesArchiveBoundaryToFilesystemParent(t *testing.T) {
	tmpRoot := t.TempDir()
	// subDir deliberately does not exist on disk: the fake reader simulates
	// its absence too, so the walk must continue past it after escaping the
	// archive scheme, all the way to tmpRoot.
	subDir := filepath.Join(tmpRoot, "sub")
	archiveFile := filepath.Join(subDir, "archive.zip")
	requested := fileinfo.ArchiveDisplayPath(archiveFile, "deep")
	archiveRoot := fileinfo.ArchiveRootPath(archiveFile)

	var calls []string
	entries, opened, usedFallback, err := ReadDirectoryWithParentFallback(
		context.Background(), requested, true,
		func(_ context.Context, path string) ([]os.DirEntry, error) {
			calls = append(calls, path)
			switch path {
			case requested, archiveRoot, subDir:
				return nil, fs.ErrNotExist
			case tmpRoot:
				return []os.DirEntry{}, nil
			default:
				t.Fatalf("unexpected read path %q", path)
				return nil, nil
			}
		},
		nil,
	)
	if err != nil {
		t.Fatalf("readDirectoryWithParentFallback returned error: %v", err)
	}
	if !usedFallback {
		t.Fatal("usedFallback = false, want true")
	}
	if opened != tmpRoot {
		t.Fatalf("opened = %q, want %q", opened, tmpRoot)
	}
	if entries == nil {
		t.Fatal("entries = nil, want successful empty directory listing")
	}
	if want := []string{requested, archiveRoot, subDir, tmpRoot}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("read paths = %#v, want %#v (requested -> archive root -> archive file's fs parent -> grandparent)", calls, want)
	}
}

func TestReadDirectoryWithParentFallbackHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	_, _, _, err := ReadDirectoryWithParentFallback(
		ctx, filepath.Join(t.TempDir(), "missing"), true,
		func(context.Context, string) ([]os.DirEntry, error) {
			called = true
			return nil, nil
		},
		nil,
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if called {
		t.Fatal("reader was called after cancellation")
	}
}

func TestDirectoryLoaderBeginCancelsPreviousLoad(t *testing.T) {
	loader := NewDirectoryLoader()

	first := loader.Begin()
	second := loader.Begin()

	if first.ID == second.ID {
		t.Fatal("load IDs should be unique")
	}
	if !errors.Is(first.Context.Err(), context.Canceled) {
		t.Fatalf("first context error = %v, want context.Canceled", first.Context.Err())
	}
	if !loader.Active(second.ID) {
		t.Fatal("second load should be active")
	}

	loader.Cancel(first.ID)
	if !loader.Active(second.ID) {
		t.Fatal("stale cancel should not cancel the active load")
	}

	loader.Cancel(second.ID)
	if !errors.Is(second.Context.Err(), context.Canceled) {
		t.Fatalf("second context error = %v, want context.Canceled", second.Context.Err())
	}
	if loader.Active(second.ID) {
		t.Fatal("active load should be cleared after cancel")
	}
}

func TestDirectoryLoaderFinishRejectsStaleLoad(t *testing.T) {
	loader := NewDirectoryLoader()

	first := loader.Begin()
	second := loader.Begin()

	if loader.Finish(first.ID) {
		t.Fatal("stale load should not finish")
	}
	if !loader.Finish(second.ID) {
		t.Fatal("active load should finish")
	}
	if loader.Active(second.ID) {
		t.Fatal("active load should be cleared after finish")
	}
}

func TestDirectoryLoaderCancelActiveInvalidatesResult(t *testing.T) {
	loader := NewDirectoryLoader()
	handle := loader.Begin()

	loader.CancelActive()

	if !errors.Is(handle.Context.Err(), context.Canceled) {
		t.Fatalf("load context error = %v, want context.Canceled", handle.Context.Err())
	}
	if loader.Active(handle.ID) {
		t.Fatal("invalidated load should no longer be active")
	}
	if loader.Finish(handle.ID) {
		t.Fatal("invalidated load should not apply a queued UI callback")
	}
}
