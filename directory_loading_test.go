package main

import (
	"context"
	"errors"
	"image/color"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/browser"
	"nmf/internal/config"
	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
	"nmf/internal/ui"
)

type directoryLoadingTheme struct{}

func (directoryLoadingTheme) GetCustomColor(string) color.RGBA {
	return color.RGBA{A: 255}
}

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

func TestBusyEscapeCancelsLatestDirectoryLoad(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	loader := browser.NewDirectoryLoader()
	fm := &FileManager{
		browser:         browser.New("/tmp", config.SortConfig{SortBy: "name", SortOrder: "asc"}),
		directoryLoader: loader,
		keyManager:      km,
	}
	fm.busy = ui.NewBusyController(nil, km, directoryLoadingTheme{}, time.Hour, nil)

	first := loader.Begin()
	fm.busy.Begin("Loading first...", fm.cancelActiveDirectoryLoad)
	second := loader.Begin()
	fm.busy.Begin("Loading second...", fm.cancelActiveDirectoryLoad)

	if !errors.Is(first.Context.Err(), context.Canceled) {
		t.Fatalf("first load context error = %v, want context.Canceled", first.Context.Err())
	}
	handler := km.GetCurrentHandler()
	if handler == nil || !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyEscape}, keymanager.ModifierState{}) {
		t.Fatal("busy handler did not handle Escape")
	}
	if !errors.Is(second.Context.Err(), context.Canceled) {
		t.Fatalf("latest load context error = %v, want context.Canceled", second.Context.Err())
	}
	if loader.Active(second.ID) {
		t.Fatal("latest load remained active after Escape")
	}
	if fm.busy.Active() || km.GetStackSize() != 0 {
		t.Fatalf("busy state after Escape = active %t stack %d, want inactive/empty", fm.busy.Active(), km.GetStackSize())
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
		browser:         newTestBrowser(testBrowserOptions{}),
		directoryLoader: browser.NewDirectoryLoader(),
		state:           state,
		config: &config.Config{UI: config.UIConfig{
			NavigationHistory: config.NavigationHistoryConfig{MaxEntries: 20},
			CursorMemory:      config.CursorMemoryConfig{MaxEntries: 20},
		}},
		fileList: widget.NewList(
			func() int { return 0 },
			func() fyne.CanvasObject { return widget.NewLabel("") },
			func(widget.ListItemID, fyne.CanvasObject) {},
		),
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

	handle := fm.directoryLoader.Begin()
	fm.loadDirectoryAsync(handle, requested, opened, config.SortConfig{SortBy: "name", SortOrder: "asc"}, true, nil, directoryLoadPresentation{})

	if got := fm.GetCurrentPath(); got != opened {
		t.Fatalf("currentPath = %q, want fallback to have opened %q", got, opened)
	}
	if got := state.NavigationHistory.Entries; len(got) != 0 {
		t.Fatalf("NavigationHistory.Entries = %#v, want empty: reopening the same directory via fallback is not a navigation", got)
	}
	if len(fm.navigationBackStack) != 0 {
		t.Fatalf("navigationBackStack = %#v, want empty after reopening the same directory", fm.navigationBackStack)
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

	handle := fm.directoryLoader.Begin()
	fm.loadDirectoryAsync(handle, requested, previousPath, config.SortConfig{SortBy: "name", SortOrder: "asc"}, true, nil, directoryLoadPresentation{})

	if got := fm.GetCurrentPath(); got != opened {
		t.Fatalf("currentPath = %q, want fallback to have opened %q", got, opened)
	}
	wantCursor := filepath.Join(opened, "zzz_target.txt")
	if got := fm.browserModel().CursorPath(); got != wantCursor {
		t.Fatalf("cursorPath = %q, want %q (restored from cursor memory keyed by the opened path)", got, wantCursor)
	}
	if _, ok := state.CursorMemory.LastUsed[opened]; !ok {
		t.Fatal("restoreCursorPosition should have refreshed LastUsed for the opened path's cursor-memory entry")
	}
	if got := state.NavigationHistory.Entries; len(got) != 1 || got[0] != previousPath {
		t.Fatalf("NavigationHistory.Entries = %#v, want only %q recorded", got, previousPath)
	}
	if got, want := fm.navigationBackStack, []string{previousPath}; !reflect.DeepEqual(got, want) {
		t.Fatalf("navigationBackStack = %#v, want %#v", got, want)
	}
}

func TestLoadDirectoryAsyncHistoryBackPopsWithoutPushingDeparture(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	older := t.TempDir()
	target := t.TempDir()
	departure := t.TempDir()
	state := directoryLoadingTestState()
	fm := newParentFallbackTestFileManager(state)
	fm.navigationBackStack = []string{older, target}

	handle := fm.directoryLoader.Begin()
	fm.loadDirectoryAsync(
		handle,
		target,
		departure,
		config.SortConfig{SortBy: "name", SortOrder: "asc"},
		true,
		nil,
		directoryLoadPresentation{navigation: directoryNavigation{kind: directoryNavigationBack, target: target}},
	)

	if got := fm.GetCurrentPath(); got != target {
		t.Fatalf("currentPath = %q, want history target %q", got, target)
	}
	if got, want := fm.navigationBackStack, []string{older}; !reflect.DeepEqual(got, want) {
		t.Fatalf("navigationBackStack = %#v, want %#v", got, want)
	}
	if got := state.NavigationHistory.Entries; len(got) != 1 || got[0] != departure {
		t.Fatalf("NavigationHistory.Entries = %#v, want departed path %q recorded", got, departure)
	}
}

func TestCanceledHistoryBackKeepsTarget(t *testing.T) {
	target := t.TempDir()
	fm := newParentFallbackTestFileManager(directoryLoadingTestState())
	fm.navigationBackStack = []string{target}
	handle := fm.directoryLoader.Begin()
	fm.directoryLoader.Cancel(handle.ID)

	fm.loadDirectoryAsync(
		handle,
		target,
		t.TempDir(),
		config.SortConfig{SortBy: "name", SortOrder: "asc"},
		true,
		nil,
		directoryLoadPresentation{navigation: directoryNavigation{kind: directoryNavigationBack, target: target}},
	)

	if got, want := fm.navigationBackStack, []string{target}; !reflect.DeepEqual(got, want) {
		t.Fatalf("navigationBackStack = %#v, want canceled target retained as %#v", got, want)
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
	files := []fileinfo.FileInfo{
		{Name: "..", Path: fileinfo.ParentPath(dir), IsDir: true},
		{Name: "aaa_first.txt", Path: first},
		{Name: "bbb_deleted.txt", Path: deleted, Status: fileinfo.StatusDeleted},
		{Name: "ccc_next.txt", Path: next},
	}
	fm.browserModel().ReplaceDirectory(dir, files, fileinfo.StorageInfo{}, false, config.SortConfig{SortBy: "name", SortOrder: "asc"})
	fm.SetCursorByIndex(2)

	neighbors := fm.cursorNeighborPaths()
	if want := []string{next, first}; !reflect.DeepEqual(neighbors, want) {
		t.Fatalf("cursorNeighborPaths() = %#v, want %#v", neighbors, want)
	}

	handle := fm.directoryLoader.Begin()
	fm.loadDirectoryAsync(handle, dir, dir, config.SortConfig{SortBy: "name", SortOrder: "asc"}, false, neighbors, directoryLoadPresentation{})

	if got := fm.browserModel().CursorPath(); got != next {
		t.Fatalf("cursorPath = %q, want following pre-refresh neighbor %q", got, next)
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
			fm.browserModel().SetPath(tt.previousPath)
			if _, _, err := fm.browserModel().ApplyFilter(entry); err != nil {
				t.Fatalf("ApplyFilter: %v", err)
			}

			handle := fm.directoryLoader.Begin()
			fm.loadDirectoryAsync(handle, dir, tt.previousPath,
				config.SortConfig{SortBy: "name", SortOrder: "asc", DirectoriesFirst: true}, false, nil, directoryLoadPresentation{})

			if got, want := namesOf(fm.GetFiles()), []string{"..", "assets", "image.png"}; !reflect.DeepEqual(got, want) {
				t.Fatalf("visible files = %v, want active filter preserved as %v", got, want)
			}
			if got, want := namesOf(fm.browserModel().SourceFiles()), []string{"..", "assets", "image.png", "notes.txt"}; !reflect.DeepEqual(got, want) {
				t.Fatalf("complete files = %v, want unfiltered source %v", got, want)
			}
			active := fm.browserModel().Filter()
			if active == nil || active.Pattern != entry.Pattern || fm.state.FileFilter.Current != entry || !fm.state.FileFilter.Enabled {
				t.Fatal("active filter state changed during directory load")
			}
		})
	}
}
