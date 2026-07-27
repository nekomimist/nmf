package main

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

	entries, opened, usedFallback, err := readDirectoryWithParentFallback(
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

	entries, opened, usedFallback, err := readDirectoryWithParentFallback(
		context.Background(), requested, true, fileinfo.ReadDirPortableContext,
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
			_, opened, usedFallback, err := readDirectoryWithParentFallback(
				context.Background(), requested, tt.allowFallback,
				func(_ context.Context, path string) ([]os.DirEntry, error) {
					calls = append(calls, path)
					return nil, tt.read(path)
				},
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
	_, opened, usedFallback, err := readDirectoryWithParentFallback(
		context.Background(), shareRoot, true,
		func(_ context.Context, path string) ([]os.DirEntry, error) {
			calls = append(calls, path)
			return nil, fs.ErrNotExist
		},
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

func TestReadDirectoryWithParentFallbackHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	_, _, _, err := readDirectoryWithParentFallback(
		ctx, filepath.Join(t.TempDir(), "missing"), true,
		func(context.Context, string) ([]os.DirEntry, error) {
			called = true
			return nil, nil
		},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if called {
		t.Fatal("reader was called after cancellation")
	}
}

func TestBeginDirectoryLoadCancelsPreviousLoad(t *testing.T) {
	fm := &FileManager{currentPath: t.TempDir()}

	firstCtx, firstID := fm.beginDirectoryLoad()
	secondCtx, secondID := fm.beginDirectoryLoad()

	if firstID == secondID {
		t.Fatal("load IDs should be unique")
	}
	if !errors.Is(firstCtx.Err(), context.Canceled) {
		t.Fatalf("first context error = %v, want context.Canceled", firstCtx.Err())
	}
	if !fm.directoryLoadActive(secondID) {
		t.Fatal("second load should be active")
	}

	fm.cancelDirectoryLoad(firstID)
	if !fm.directoryLoadActive(secondID) {
		t.Fatal("stale cancel should not cancel the active load")
	}

	fm.cancelDirectoryLoad(secondID)
	if !errors.Is(secondCtx.Err(), context.Canceled) {
		t.Fatalf("second context error = %v, want context.Canceled", secondCtx.Err())
	}
	if fm.directoryLoadActive(secondID) {
		t.Fatal("active load should be cleared after cancel")
	}
}

func TestFinishDirectoryLoadRejectsStaleLoad(t *testing.T) {
	fm := &FileManager{}

	_, firstID := fm.beginDirectoryLoad()
	_, secondID := fm.beginDirectoryLoad()

	if fm.finishDirectoryLoad(firstID) {
		t.Fatal("stale load should not finish")
	}
	if !fm.finishDirectoryLoad(secondID) {
		t.Fatal("active load should finish")
	}
	if fm.directoryLoadActive(secondID) {
		t.Fatal("active load should be cleared after finish")
	}
}

func TestInvalidateActiveDirectoryLoadCancelsWithoutRestart(t *testing.T) {
	fm := &FileManager{}
	ctx, loadID := fm.beginDirectoryLoad()

	fm.invalidateActiveDirectoryLoad()

	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("load context error = %v, want context.Canceled", ctx.Err())
	}
	if fm.directoryLoadActive(loadID) {
		t.Fatal("invalidated load should no longer be active")
	}
	if fm.finishDirectoryLoad(loadID) {
		t.Fatal("invalidated load should not apply a queued UI callback")
	}
}
