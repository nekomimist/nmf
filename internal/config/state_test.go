package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newTestStateManager returns a StateManager whose statePath points into a
// caller-provided path (typically under t.TempDir()) instead of the real OS
// state directory, mirroring how config_test.go repoints Manager.configPath.
func newTestStateManager(t *testing.T, statePath string) *StateManager {
	t.Helper()
	dummyDebugPrint := func(string, ...interface{}) {}
	sm := NewStateManager(dummyDebugPrint)
	sm.statePath = statePath
	t.Cleanup(func() {
		if err := sm.Close(); err != nil {
			t.Fatalf("StateManager.Close failed: %v", err)
		}
	})
	return sm
}

func TestNewDefaultState(t *testing.T) {
	state := newDefaultState()

	if state.CursorMemory.Entries == nil {
		t.Error("expected cursor memory entries to be initialized")
	}
	if state.CursorMemory.LastUsed == nil {
		t.Error("expected cursor memory lastUsed to be initialized")
	}
	if state.NavigationHistory.Entries == nil {
		t.Error("expected navigation history entries to be initialized")
	}
	if state.NavigationHistory.LastUsed == nil {
		t.Error("expected navigation history lastUsed to be initialized")
	}
	if state.NavigationHistory.UseCount == nil {
		t.Error("expected navigation history useCount to be initialized")
	}
	if state.NavigationHistory.Pinned == nil {
		t.Error("expected navigation history pinned to be initialized")
	}
	if state.FileFilter.Entries == nil {
		t.Error("expected file filter entries to be initialized")
	}
	if state.FileFilter.Current != nil {
		t.Error("expected file filter current to be nil")
	}
	if state.FileFilter.Enabled {
		t.Error("expected file filter enabled to be false")
	}
	if state.Sort != nil {
		t.Error("expected sort to be nil by default")
	}
}

func TestGetStatePath(t *testing.T) {
	path := getStatePath()
	if path == "" {
		t.Error("state path should not be empty")
	}
	if !strings.HasSuffix(path, "state.json") {
		t.Errorf("state path should end with 'state.json', got %q", path)
	}
}

func TestStateManagerLoadWithNoStateAndNoConfigWritesDefault(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "state.json")
	configPath := filepath.Join(tempDir, "config.json") // deliberately absent

	sm := newTestStateManager(t, statePath)

	state, err := sm.Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if state == nil {
		t.Fatal("expected non-nil default state")
	}
	if state.Sort != nil {
		t.Error("expected default state sort to be nil")
	}
	if len(state.NavigationHistory.Entries) != 0 {
		t.Errorf("expected empty navigation history, got %#v", state.NavigationHistory.Entries)
	}
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("expected state.json to be written, stat error: %v", err)
	}
}

func TestStateManagerMigratesLegacyConfigRuntimeState(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "state.json")
	configPath := filepath.Join(tempDir, "config.json")

	legacyJSON := []byte(`{
  "window": {"width": 800, "height": 600},
  "ui": {
    "cursorMemory": {
      "maxEntries": 100,
      "entries": {"/home/user/docs": "readme.txt"},
      "lastUsed": {"/home/user/docs": "2024-01-01T00:00:00Z"}
    },
    "navigationHistory": {
      "maxEntries": 10000,
      "entries": ["/home/user/docs", "/home/user/src"],
      "lastUsed": {"/home/user/docs": "2024-01-01T00:00:00Z", "/home/user/src": "2024-01-02T00:00:00Z"},
      "useCount": {"/home/user/docs": 3, "/home/user/src": 5},
      "pinned": ["/rare/project"]
    },
    "fileFilter": {
      "maxEntries": 30,
      "entries": [{"pattern": "*.go", "lastUsed": "2024-01-01T00:00:00Z", "useCount": 2}],
      "enabled": true,
      "current": {"pattern": "*.go", "lastUsed": "2024-01-01T00:00:00Z", "useCount": 2}
    }
  }
}`)
	if err := os.WriteFile(configPath, legacyJSON, 0644); err != nil {
		t.Fatalf("failed to write legacy config: %v", err)
	}
	originalBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read back legacy config: %v", err)
	}

	sm := newTestStateManager(t, statePath)

	state, err := sm.Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if got := state.CursorMemory.Entries["/home/user/docs"]; got != "readme.txt" {
		t.Errorf("cursorMemory entries = %#v, want migrated readme.txt", state.CursorMemory.Entries)
	}
	if len(state.NavigationHistory.Entries) != 2 {
		t.Fatalf("navigationHistory entries = %#v, want 2 entries", state.NavigationHistory.Entries)
	}
	if got := state.NavigationHistory.UseCount["/home/user/src"]; got != 5 {
		t.Errorf("navigationHistory useCount[/home/user/src] = %d, want 5", got)
	}
	if len(state.NavigationHistory.Pinned) != 1 || state.NavigationHistory.Pinned[0] != "/rare/project" {
		t.Errorf("navigationHistory pinned = %#v, want [/rare/project]", state.NavigationHistory.Pinned)
	}
	if !state.FileFilter.Enabled {
		t.Error("fileFilter enabled should be migrated true")
	}
	if state.FileFilter.Current == nil || state.FileFilter.Current.Pattern != "*.go" {
		t.Errorf("fileFilter current = %#v, want pattern *.go", state.FileFilter.Current)
	}
	if len(state.FileFilter.Entries) != 1 || state.FileFilter.Entries[0].Pattern != "*.go" {
		t.Errorf("fileFilter entries = %#v, want one *.go entry", state.FileFilter.Entries)
	}
	if state.Sort != nil {
		t.Error("expected sort to remain nil; migration must not seed ui.sort")
	}

	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("expected state.json to be written, stat error: %v", err)
	}

	afterBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to re-read config after migration: %v", err)
	}
	if string(afterBytes) != string(originalBytes) {
		t.Fatal("config.json bytes changed after migration; migration must never write configPath")
	}
}

func TestStateManagerMigrationFixesMissingUseCount(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "state.json")
	configPath := filepath.Join(tempDir, "config.json")

	legacyJSON := []byte(`{
  "ui": {
    "navigationHistory": {
      "entries": ["/tmp/one"],
      "lastUsed": {"/tmp/one": "2024-01-01T00:00:00Z"}
    }
  }
}`)
	if err := os.WriteFile(configPath, legacyJSON, 0644); err != nil {
		t.Fatalf("failed to write legacy config: %v", err)
	}

	sm := newTestStateManager(t, statePath)

	state, err := sm.Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if got := state.NavigationHistory.UseCount["/tmp/one"]; got != 1 {
		t.Fatalf("useCount = %d, want migrated 1", got)
	}
}

func TestStateManagerSkipsMigrationWhenStateFileExists(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "state.json")
	configPath := filepath.Join(tempDir, "config.json")

	legacyJSON := []byte(`{"ui": {"navigationHistory": {"entries": ["/should/not/be/migrated"]}}}`)
	if err := os.WriteFile(configPath, legacyJSON, 0644); err != nil {
		t.Fatalf("failed to write legacy config: %v", err)
	}

	existing := newDefaultState()
	existing.NavigationHistory.Entries = []string{"/already/there"}
	existingData, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal existing state: %v", err)
	}
	if err := os.WriteFile(statePath, existingData, 0644); err != nil {
		t.Fatalf("failed to write existing state.json: %v", err)
	}

	sm := newTestStateManager(t, statePath)

	state, err := sm.Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(state.NavigationHistory.Entries) != 1 || state.NavigationHistory.Entries[0] != "/already/there" {
		t.Errorf("navigationHistory entries = %#v, want existing state preserved, not migrated", state.NavigationHistory.Entries)
	}
}

func TestStateManagerLoadRejectsCorruptExistingStateFile(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "state.json")
	configPath := filepath.Join(tempDir, "config.json")

	if err := os.WriteFile(statePath, []byte("{not valid json"), 0644); err != nil {
		t.Fatalf("failed to write corrupt state.json: %v", err)
	}

	sm := newTestStateManager(t, statePath)

	if _, err := sm.Load(configPath); err == nil {
		t.Fatal("expected error loading corrupt state.json")
	}
}

func TestStateManagerSaveAsyncAndLoadRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "state.json")
	configPath := filepath.Join(tempDir, "config.json")

	sm := newTestStateManager(t, statePath)

	state := newDefaultState()
	state.NavigationHistory.Entries = []string{"/one", "/two"}
	state.NavigationHistory.LastUsed["/one"] = time.Now()
	state.NavigationHistory.UseCount["/one"] = 2
	state.Sort = &SortConfig{SortBy: "size", SortOrder: "desc", DirectoriesFirst: false}

	if err := sm.SaveAsync(state); err != nil {
		t.Fatalf("SaveAsync failed: %v", err)
	}
	// Give the worker goroutine a chance to drain the buffered save request
	// before Flush; otherwise Flush's send on the unbuffered flushRequests
	// channel can race the already-queued saveRequests value in the worker's
	// select and win, flushing nothing (mirrors a real caller doing other
	// work between SaveAsync and a later Flush/Close).
	time.Sleep(50 * time.Millisecond)
	if err := sm.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	fresh := newTestStateManager(t, statePath)
	loaded, err := fresh.Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(loaded.NavigationHistory.Entries) != 2 {
		t.Fatalf("entries = %#v, want 2", loaded.NavigationHistory.Entries)
	}
	if loaded.NavigationHistory.UseCount["/one"] != 2 {
		t.Fatalf("useCount[/one] = %d, want 2", loaded.NavigationHistory.UseCount["/one"])
	}
	if loaded.Sort == nil || loaded.Sort.SortBy != "size" || loaded.Sort.SortOrder != "desc" {
		t.Fatalf("sort = %#v, want sortBy size/desc", loaded.Sort)
	}
}

func TestStateManagerSaveIsAtomic(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "state.json")

	sm := newTestStateManager(t, statePath)

	state := newDefaultState()
	state.NavigationHistory.Entries = []string{"/one"}

	if err := sm.Save(state); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp") {
			t.Fatalf("found leftover temp file %q after Save", entry.Name())
		}
	}
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("expected state.json to exist: %v", err)
	}
}

func TestStateNavigationHistoryFrecencyOrdering(t *testing.T) {
	now := time.Now()
	state := newDefaultState()
	state.NavigationHistory.Entries = []string{"/recent", "/frequent", "/old"}
	state.NavigationHistory.LastUsed = map[string]time.Time{
		"/recent":   now.Add(-30 * time.Minute),
		"/frequent": now.Add(-2 * time.Hour),
		"/old":      now.Add(-8 * 24 * time.Hour),
	}
	state.NavigationHistory.UseCount = map[string]int{
		"/recent":   1,
		"/frequent": 4,
		"/old":      100,
	}

	got := state.GetNavigationHistory()
	want := []string{"/old", "/frequent", "/recent"}
	if len(got) != len(want) {
		t.Fatalf("history length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("history = %#v, want %#v", got, want)
		}
	}
}

func TestStateAddToNavigationHistoryIncrementsUseCountAndPrunesByScore(t *testing.T) {
	now := time.Now()
	state := newDefaultState()
	state.NavigationHistory.Entries = []string{"/keep", "/drop"}
	state.NavigationHistory.LastUsed = map[string]time.Time{
		"/keep": now.Add(-30 * time.Minute),
		"/drop": now.Add(-8 * 24 * time.Hour),
	}
	state.NavigationHistory.UseCount = map[string]int{
		"/keep": 1,
		"/drop": 1,
	}

	state.AddToNavigationHistory("/keep", 2)
	state.AddToNavigationHistory("/new", 2)

	if got := state.NavigationHistory.UseCount["/keep"]; got != 2 {
		t.Fatalf("useCount for /keep = %d, want 2", got)
	}
	if _, ok := state.NavigationHistory.LastUsed["/drop"]; ok {
		t.Fatal("/drop lastUsed should be pruned")
	}
	if _, ok := state.NavigationHistory.UseCount["/drop"]; ok {
		t.Fatal("/drop useCount should be pruned")
	}
	want := []string{"/keep", "/new"}
	if len(state.NavigationHistory.Entries) != len(want) {
		t.Fatalf("entries = %#v, want %#v", state.NavigationHistory.Entries, want)
	}
	for i := range want {
		if state.NavigationHistory.Entries[i] != want[i] {
			t.Fatalf("entries = %#v, want %#v", state.NavigationHistory.Entries, want)
		}
	}
}

func TestStatePinnedNavigationHistoryDoesNotCountAgainstHistoryLimit(t *testing.T) {
	state := newDefaultState()
	state.NavigationHistory.Pinned = []string{"/rare"}

	state.AddToNavigationHistory("/one", 1)
	state.AddToNavigationHistory("/two", 1)

	if len(state.NavigationHistory.Entries) != 1 {
		t.Fatalf("history entries = %#v, want one pruned entry", state.NavigationHistory.Entries)
	}
	if len(state.NavigationHistory.Pinned) != 1 || state.NavigationHistory.Pinned[0] != "/rare" {
		t.Fatalf("pinned = %#v, want /rare retained", state.NavigationHistory.Pinned)
	}
}

func TestStatePinAndUnpinNavigationPath(t *testing.T) {
	state := newDefaultState()

	if !state.PinNavigationPath("/rare") {
		t.Fatal("first pin should add path")
	}
	if state.PinNavigationPath("/rare") {
		t.Fatal("second pin should not duplicate path")
	}
	if !state.IsNavigationPathPinned("/rare") {
		t.Fatal("/rare should be pinned")
	}
	if !state.UnpinNavigationPath("/rare") {
		t.Fatal("unpin should remove existing path")
	}
	if state.IsNavigationPathPinned("/rare") {
		t.Fatal("/rare should no longer be pinned")
	}
	if state.UnpinNavigationPath("/rare") {
		t.Fatal("unpin should report false for missing path")
	}
}

func TestStateFileFilterHistoryUsesFrecencyOrdering(t *testing.T) {
	now := time.Now()
	state := newDefaultState()
	state.FileFilter.Entries = []FilterEntry{
		{Pattern: "*.go", LastUsed: now.Add(-30 * time.Minute), UseCount: 1},
		{Pattern: "*.md", LastUsed: now.Add(-2 * time.Hour), UseCount: 4},
		{Pattern: "*.log", LastUsed: now.Add(-8 * 24 * time.Hour), UseCount: 100},
	}

	got := state.GetFileFilterEntries()
	want := []string{"*.log", "*.md", "*.go"}
	if len(got) != len(want) {
		t.Fatalf("filter history length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Pattern != want[i] {
			t.Fatalf("filter history = %#v, want patterns %#v", got, want)
		}
	}
}

func TestStateAddToFileFilterHistoryIncrementsUseCountAndPrunesByScore(t *testing.T) {
	now := time.Now()
	state := newDefaultState()
	state.FileFilter.Entries = []FilterEntry{
		{Pattern: "*.keep", LastUsed: now.Add(-2 * time.Hour), UseCount: 4},
		{Pattern: "*.drop", LastUsed: now.Add(-8 * 24 * time.Hour), UseCount: 1},
	}

	state.AddToFileFilterHistory(&FilterEntry{Pattern: "*.new ;; 日本語"}, 2)
	state.AddToFileFilterHistory(&FilterEntry{Pattern: "*.keep"}, 2)

	if len(state.FileFilter.Entries) != 2 {
		t.Fatalf("entries = %#v, want two entries", state.FileFilter.Entries)
	}
	if state.FileFilter.Entries[0].Pattern != "*.keep" || state.FileFilter.Entries[0].UseCount != 5 {
		t.Fatalf("first entry = %#v, want incremented *.keep", state.FileFilter.Entries[0])
	}
	if state.FileFilter.Entries[1].Pattern != "*.new ;; 日本語" {
		t.Fatalf("entries = %#v, want new entry retained", state.FileFilter.Entries)
	}
	for _, entry := range state.FileFilter.Entries {
		if entry.Pattern == "*.drop" {
			t.Fatalf("entries = %#v, drop should be pruned", state.FileFilter.Entries)
		}
	}
}

func TestStateRemoveFileFilterEntryRemovesExactPattern(t *testing.T) {
	state := newDefaultState()
	state.FileFilter.Entries = []FilterEntry{
		{Pattern: "*.go ;; Go"},
		{Pattern: "*.go ;; Golang"},
	}

	if !state.RemoveFileFilterEntry("*.go ;; Go") {
		t.Fatal("RemoveFileFilterEntry should report removal")
	}
	if len(state.FileFilter.Entries) != 1 || state.FileFilter.Entries[0].Pattern != "*.go ;; Golang" {
		t.Fatalf("entries = %#v, want only exact non-deleted pattern", state.FileFilter.Entries)
	}
}

func TestCloneStateDeepCopiesPinnedNavigationHistory(t *testing.T) {
	state := newDefaultState()
	state.NavigationHistory.Pinned = []string{"/rare"}

	clone := cloneState(state)
	clone.NavigationHistory.Pinned[0] = "/changed"

	if state.NavigationHistory.Pinned[0] != "/rare" {
		t.Errorf("expected original pinned history path to remain unchanged, got %q", state.NavigationHistory.Pinned[0])
	}
}

func TestCloneStateDeepCopiesCursorMemory(t *testing.T) {
	state := newDefaultState()
	state.CursorMemory.Entries["/dir"] = "file.txt"
	state.CursorMemory.LastUsed["/dir"] = time.Now()

	clone := cloneState(state)
	clone.CursorMemory.Entries["/dir"] = "changed.txt"
	clone.CursorMemory.LastUsed["/dir"] = time.Time{}

	if state.CursorMemory.Entries["/dir"] != "file.txt" {
		t.Errorf("expected original cursor memory entry to remain unchanged, got %q", state.CursorMemory.Entries["/dir"])
	}
	if state.CursorMemory.LastUsed["/dir"].IsZero() {
		t.Error("expected original cursor memory lastUsed to remain unchanged")
	}
}

func TestCloneStateDeepCopiesFileFilterEntries(t *testing.T) {
	state := newDefaultState()
	state.FileFilter.Entries = []FilterEntry{{Pattern: "*.go"}}
	state.FileFilter.Current = &FilterEntry{Pattern: "*.go"}

	clone := cloneState(state)
	clone.FileFilter.Entries[0].Pattern = "*.changed"
	clone.FileFilter.Current.Pattern = "*.changed"

	if state.FileFilter.Entries[0].Pattern != "*.go" {
		t.Errorf("expected original file filter entry to remain unchanged, got %q", state.FileFilter.Entries[0].Pattern)
	}
	if state.FileFilter.Current.Pattern != "*.go" {
		t.Errorf("expected original file filter current to remain unchanged, got %q", state.FileFilter.Current.Pattern)
	}
}

func TestEffectiveSortReturnsOverrideWhenPresent(t *testing.T) {
	state := newDefaultState()
	state.Sort = &SortConfig{SortBy: "size", SortOrder: "desc", DirectoriesFirst: false}
	configDefault := SortConfig{SortBy: "name", SortOrder: "asc", DirectoriesFirst: true}

	got := state.EffectiveSort(configDefault)
	if got.SortBy != "size" || got.SortOrder != "desc" || got.DirectoriesFirst {
		t.Fatalf("EffectiveSort = %+v, want override", got)
	}
}

func TestEffectiveSortFallsBackToConfigDefault(t *testing.T) {
	state := newDefaultState()
	configDefault := SortConfig{SortBy: "name", SortOrder: "asc", DirectoriesFirst: true}

	got := state.EffectiveSort(configDefault)
	if got != configDefault {
		t.Fatalf("EffectiveSort = %+v, want configDefault %+v", got, configDefault)
	}
}
