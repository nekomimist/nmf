package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"nmf/internal/browser"
	"nmf/internal/config"
	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
)

func TestDisplayCachedDirectoryAppliesProvisionalListing(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	previous := filepath.Join(string(filepath.Separator), "previous")
	target := filepath.Join(string(filepath.Separator), "target")
	sortCfg := config.SortConfig{SortBy: "name", SortOrder: "asc", DirectoriesFirst: true}
	state := directoryLoadingTestState()
	state.CursorMemory.Entries[target] = "docs"
	fm := newParentFallbackTestFileManager(state)
	fm.browserModel().SetPath(previous)
	fm.directoryCache = browser.NewDirectoryCache(time.Minute, 2)
	fm.directoryCache.Put(browser.DirectorySnapshot{
		Path: target,
		Files: []fileinfo.FileInfo{
			{Name: "..", Path: fileinfo.ParentPath(target), IsDir: true},
			{Name: "docs", Path: fileinfo.JoinPath(target, "docs"), IsDir: true},
			{Name: "notes.txt", Path: fileinfo.JoinPath(target, "notes.txt")},
		},
		Sort: sortCfg,
	})

	if !fm.displayCachedDirectory(target, previous, sortCfg, nil) {
		t.Fatal("displayCachedDirectory returned a miss")
	}
	if got := fm.GetCurrentPath(); got != target {
		t.Fatalf("current path = %q, want cached target %q", got, target)
	}
	if fm.directoryListingState != directoryListingCachedRefreshing {
		t.Fatalf("listing state = %d, want cached-refreshing", fm.directoryListingState)
	}
	if got, want := namesOf(fm.GetFiles()), []string{"..", "docs", "notes.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("cached files = %v, want %v", got, want)
	}
	if got := fm.browserModel().CursorPath(); got != fileinfo.JoinPath(target, "docs") {
		t.Fatalf("cursor path = %q, want cached docs", got)
	}
	if got := state.NavigationHistory.Entries; len(got) != 1 || got[0] != previous {
		t.Fatalf("navigation history = %#v, want previous path", got)
	}
	if got := fm.directoryCacheStatusText(); got != "Cached listing — refreshing; navigation only" {
		t.Fatalf("cache status = %q, want refreshing navigation-only message", got)
	}
}

func TestCachedRevalidationPreservesMovedCursorAndRefreshesCache(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	dir := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := osWriteTestFile(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	state := directoryLoadingTestState()
	fm := newParentFallbackTestFileManager(state)
	fm.directoryCache = browser.NewDirectoryCache(time.Minute, 2)
	sortCfg := config.SortConfig{SortBy: "name", SortOrder: "asc", DirectoriesFirst: true}
	fm.browserModel().ReplaceDirectory(dir, []fileinfo.FileInfo{
		{Name: "..", Path: fileinfo.ParentPath(dir), IsDir: true},
		{Name: "a.txt", Path: filepath.Join(dir, "a.txt")},
		{Name: "b.txt", Path: filepath.Join(dir, "b.txt")},
	}, fileinfo.StorageInfo{}, false, sortCfg)
	fm.SetCursorByIndex(2)
	fm.directoryListingState = directoryListingCachedRefreshing

	handle := fm.directoryLoader.Begin()
	fm.loadDirectoryAsync(handle, dir, fileinfo.ParentPath(dir), sortCfg, false, nil, directoryLoadPresentation{cacheDisplayed: true})

	if fm.directoryListingState != directoryListingFresh {
		t.Fatalf("listing state = %d, want fresh", fm.directoryListingState)
	}
	if got := fm.browserModel().CursorPath(); got != filepath.Join(dir, "b.txt") {
		t.Fatalf("cursor path = %q, want moved cached cursor preserved", got)
	}
	snapshot, ok := fm.directoryCache.Get(dir)
	if !ok || len(snapshot.Files) != 3 {
		t.Fatalf("refreshed cache = ok %t files %d, want accepted real listing", ok, len(snapshot.Files))
	}
}

func TestCachedListingCommandPolicy(t *testing.T) {
	fm := &FileManager{directoryListingState: directoryListingCachedRefreshing}
	for _, commandID := range []string{
		keymanager.CommandCursorDown,
		keymanager.CommandOpen,
		keymanager.CommandHistoryBack,
		keymanager.CommandParentDirectory,
		keymanager.CommandHome,
		keymanager.CommandQuit,
	} {
		if !fm.mainScreenCommandAllowed(commandID) {
			t.Errorf("command %q should be allowed for cached navigation", commandID)
		}
	}
	for _, commandID := range []string{
		keymanager.CommandOpenDefaultApp,
		keymanager.CommandSelectToggle,
		keymanager.CommandSortShow,
		keymanager.CommandDeletePermanent,
		keymanager.CommandHistoryShow,
		keymanager.CommandJobsShow,
		"user.custom",
	} {
		if fm.mainScreenCommandAllowed(commandID) {
			t.Errorf("command %q should be blocked for cached navigation", commandID)
		}
	}
}

func TestCachedListingPointerCommandUsesSamePolicy(t *testing.T) {
	fm := &FileManager{
		browser:               newTestBrowser(testBrowserOptions{path: "/cached"}),
		directoryListingState: directoryListingCachedRefreshing,
	}
	runs := 0
	fm.mainScreenPointerCommand(keymanager.CommandJobsShow, func() { runs++ })()
	if runs != 0 {
		t.Fatalf("blocked pointer command ran %d times", runs)
	}
	fm.mainScreenPointerCommand(keymanager.CommandHome, func() { runs++ })()
	if runs != 1 {
		t.Fatalf("allowed pointer command run count = %d, want 1", runs)
	}
}

func directoryLoadingTestState() *config.State {
	return &config.State{
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
}

func osWriteTestFile(path string) error {
	return os.WriteFile(path, []byte("x"), 0o644)
}
