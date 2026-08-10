package main

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
)

func TestIsParentDirectoryNavigationDistinguishesReload(t *testing.T) {
	for _, path := range []string{"/", `C:\`, `D:\`} {
		t.Run(path, func(t *testing.T) {
			if isParentDirectoryNavigation(path, path) {
				t.Fatalf("same path %q was classified as parent navigation", path)
			}
		})
	}

	parent := t.TempDir()
	child := filepath.Join(parent, "child")
	if !isParentDirectoryNavigation(child, parent) {
		t.Fatalf("child-to-parent navigation %q -> %q was not recognized", child, parent)
	}
}

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

	entries, opened, usedFallback, err := readDirectoryWithParentFallback(
		context.Background(), requested, true, fileinfo.ReadDirPortableContext,
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
	entries, opened, usedFallback, err := readDirectoryWithParentFallback(
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

// newParentFallbackTestFileManager builds a minimally-wired FileManager
// suitable for driving loadDirectoryAsync directly. fileListView and window
// are deliberately left nil: with fileListView nil, focusFileList takes its
// "skipped" branch (see directory_loading.go), so no fyne.Window is needed at
// all. fyne.Do runs synchronously against the fyne/v2/test driver (see
// test/driver.go DoFromGoroutine), so calling loadDirectoryAsync directly
// (not via `go`) applies its UI-thread callback inline and deterministically.
func newParentFallbackTestFileManager(state *config.State) *FileManager {
	return &FileManager{
		state: state,
		config: &config.Config{UI: config.UIConfig{
			NavigationHistory: config.NavigationHistoryConfig{MaxEntries: 20},
			CursorMemory:      config.CursorMemoryConfig{MaxEntries: 20},
		}},
		fileList: widget.NewList(
			func() int { return 0 },
			func() fyne.CanvasObject { return widget.NewLabel("") },
			func(widget.ListItemID, fyne.CanvasObject) {},
		),
		selectedFiles: map[string]bool{},
	}
}

// TestLoadDirectoryAsyncFallbackSkipsHistoryWhenReopeningSameDirectory pins
// down that the post-fallback "did we actually change directory" check in
// loadDirectoryAsync (directory_loading.go: `if previousPath != "" &&
// previousPath != path`) compares previousPath against the opened path, not
// the originally-requested missing one. requested never exists, so a version
// of the guard that compared against requestedPath instead would consider
// this "a move" (previousPath != requestedPath) and wrongly record history,
// even though the fallback lands back on the exact directory the user was
// already in.
func TestLoadDirectoryAsyncFallbackSkipsHistoryWhenReopeningSameDirectory(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	opened := t.TempDir()
	requested := filepath.Join(opened, "missing", "child")

	state := &config.State{
		CursorMemory: config.CursorMemoryState{
			Entries:  map[string]string{},
			LastUsed: map[string]time.Time{},
		},
		NavigationHistory: config.NavigationHistoryState{
			Entries:  []string{},
			LastUsed: map[string]time.Time{},
			UseCount: map[string]int{},
			Pinned:   []string{},
		},
	}
	fm := newParentFallbackTestFileManager(state)

	ctx, loadID := fm.beginDirectoryLoad()
	fm.loadDirectoryAsync(ctx, loadID, requested, opened, config.SortConfig{SortBy: "name", SortOrder: "asc"}, true)

	if fm.currentPath != opened {
		t.Fatalf("currentPath = %q, want fallback to have opened %q", fm.currentPath, opened)
	}
	if got := state.NavigationHistory.Entries; len(got) != 0 {
		t.Fatalf("NavigationHistory.Entries = %#v, want empty: reopening the same directory via fallback is not a navigation", got)
	}
}

// TestLoadDirectoryAsyncFallbackRestoresCursorForOpenedPathNotRequestedPath
// pins down that restoreCursorPosition is called with the opened path (see
// directory_loading.go's `fm.restoreCursorPosition(path)`, where path was
// reassigned to loadedPath after the fallback succeeded), not the originally
// requested missing path. Cursor memory is seeded only under the opened
// path's key; if the code looked up the requested path instead, the memory
// entry below would never be reached and the cursor would fall back to
// index 0.
func TestLoadDirectoryAsyncFallbackRestoresCursorForOpenedPathNotRequestedPath(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	opened := t.TempDir()
	if err := os.WriteFile(filepath.Join(opened, "aaa_first.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(opened, "zzz_target.txt"), []byte("z"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	requested := filepath.Join(opened, "missing", "child")
	// previousPath is unrelated to opened, so this exercises a genuine
	// directory change alongside the cursor-restoration assertion below.
	previousPath := t.TempDir()

	state := &config.State{
		CursorMemory: config.CursorMemoryState{
			// Keyed by the opened (fallback) path, never by requested: if the
			// code restored by requestedPath, this entry would be unreachable.
			Entries:  map[string]string{opened: "zzz_target.txt"},
			LastUsed: map[string]time.Time{},
		},
		NavigationHistory: config.NavigationHistoryState{
			Entries:  []string{},
			LastUsed: map[string]time.Time{},
			UseCount: map[string]int{},
			Pinned:   []string{},
		},
	}
	fm := newParentFallbackTestFileManager(state)

	ctx, loadID := fm.beginDirectoryLoad()
	fm.loadDirectoryAsync(ctx, loadID, requested, previousPath, config.SortConfig{SortBy: "name", SortOrder: "asc"}, true)

	if fm.currentPath != opened {
		t.Fatalf("currentPath = %q, want fallback to have opened %q", fm.currentPath, opened)
	}
	wantCursor := filepath.Join(opened, "zzz_target.txt")
	if fm.cursorPath != wantCursor {
		t.Fatalf("cursorPath = %q, want %q (restored from cursor memory keyed by the opened path)", fm.cursorPath, wantCursor)
	}
	if _, ok := state.CursorMemory.LastUsed[opened]; !ok {
		t.Fatal("restoreCursorPosition should have refreshed LastUsed for the opened path's cursor-memory entry")
	}
	if got := state.NavigationHistory.Entries; len(got) != 1 || got[0] != previousPath {
		t.Fatalf("NavigationHistory.Entries = %#v, want only %q recorded", got, previousPath)
	}
}

func TestLoadDirectoryAsyncRefreshRestoresNearestSurvivingCursorNeighbor(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	dir := t.TempDir()
	first := filepath.Join(dir, "aaa_first.txt")
	deleted := filepath.Join(dir, "bbb_deleted.txt")
	next := filepath.Join(dir, "ccc_next.txt")
	for _, path := range []string{first, next} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("WriteFile(%q): %v", path, err)
		}
	}

	state := &config.State{
		CursorMemory: config.CursorMemoryState{
			Entries:  map[string]string{},
			LastUsed: map[string]time.Time{},
		},
		NavigationHistory: config.NavigationHistoryState{
			Entries:  []string{},
			LastUsed: map[string]time.Time{},
			UseCount: map[string]int{},
			Pinned:   []string{},
		},
	}
	fm := newParentFallbackTestFileManager(state)
	fm.currentPath = dir
	fm.files = []fileinfo.FileInfo{
		{Name: "..", Path: fileinfo.ParentPath(dir), IsDir: true},
		{Name: "aaa_first.txt", Path: first},
		{Name: "bbb_deleted.txt", Path: deleted, Status: fileinfo.StatusDeleted},
		{Name: "ccc_next.txt", Path: next},
	}
	fm.SetCursorByIndex(2)

	neighbors := fm.cursorNeighborPaths()
	if want := []string{next, first}; !reflect.DeepEqual(neighbors, want) {
		t.Fatalf("cursorNeighborPaths() = %#v, want %#v", neighbors, want)
	}

	ctx, loadID := fm.beginDirectoryLoad()
	fm.loadDirectoryAsync(ctx, loadID, dir, dir, config.SortConfig{SortBy: "name", SortOrder: "asc"}, false, neighbors)

	if fm.cursorPath != next {
		t.Fatalf("cursorPath = %q, want following pre-refresh neighbor %q", fm.cursorPath, next)
	}
}

func TestLoadDirectoryAsyncKeepsActiveFilterOnReloadAndNavigation(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	parent := t.TempDir()
	dir := filepath.Join(parent, "child")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatalf("Mkdir assets: %v", err)
	}
	for name, contents := range map[string]string{
		"image.png": "png",
		"notes.txt": "text",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
			t.Fatalf("WriteFile(%q): %v", name, err)
		}
	}

	for _, tt := range []struct {
		name         string
		previousPath string
	}{
		{name: "same-directory reload", previousPath: dir},
		{name: "subdirectory navigation", previousPath: parent},
	} {
		t.Run(tt.name, func(t *testing.T) {
			entry := &config.FilterEntry{Pattern: "*.png"}
			state := &config.State{
				CursorMemory: config.CursorMemoryState{
					Entries:  map[string]string{},
					LastUsed: map[string]time.Time{},
				},
				NavigationHistory: config.NavigationHistoryState{
					Entries:  []string{},
					LastUsed: map[string]time.Time{},
					UseCount: map[string]int{},
					Pinned:   []string{},
				},
				FileFilter: config.FileFilterState{Current: entry, Enabled: true},
			}
			fm := newParentFallbackTestFileManager(state)
			fm.currentPath = tt.previousPath
			fm.currentFilter = entry

			ctx, loadID := fm.beginDirectoryLoad()
			fm.loadDirectoryAsync(ctx, loadID, dir, tt.previousPath,
				config.SortConfig{SortBy: "name", SortOrder: "asc", DirectoriesFirst: true}, false)

			if got, want := namesOf(fm.files), []string{"..", "assets", "image.png"}; !reflect.DeepEqual(got, want) {
				t.Fatalf("visible files = %v, want active filter preserved as %v", got, want)
			}
			if got, want := namesOf(fm.originalFiles), []string{"..", "assets", "image.png", "notes.txt"}; !reflect.DeepEqual(got, want) {
				t.Fatalf("complete files = %v, want unfiltered source %v", got, want)
			}
			if fm.currentFilter != entry || !fm.state.FileFilter.Enabled {
				t.Fatal("active filter state changed during directory load")
			}
		})
	}
}
