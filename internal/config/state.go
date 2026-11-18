package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

// CursorMemoryState holds the per-directory remembered cursor file (state.json).
// The maximum entry count is a config.json setting (CursorMemoryConfig.MaxEntries).
type CursorMemoryState struct {
	Entries  map[string]string    `json:"entries"`  // key: dirPath, value: fileName
	LastUsed map[string]time.Time `json:"lastUsed"` // LRU management
}

// NavigationHistoryState holds visited-path history (state.json). The maximum
// entry count is a config.json setting (NavigationHistoryConfig.MaxEntries).
type NavigationHistoryState struct {
	Entries  []string             `json:"entries"`  // Path history (frecency order)
	LastUsed map[string]time.Time `json:"lastUsed"` // Last usage timestamp
	UseCount map[string]int       `json:"useCount"` // Usage frequency counter
	Pinned   []string             `json:"pinned"`   // Saved paths exempt from history pruning
}

// FileFilterState holds file filter history and the currently applied filter
// (state.json). The maximum entry count is a config.json setting
// (FileFilterConfig.MaxEntries).
type FileFilterState struct {
	Entries []FilterEntry `json:"entries"` // Filter history (frecency order)
	Current *FilterEntry  `json:"current"` // Currently applied filter pattern
	Enabled bool          `json:"enabled"` // Current filter enabled state
}

// State represents runtime state persisted to state.json. It holds the parts
// of the former config.json UI state (cursor memory, navigation history, file
// filter history, and the last-applied sort) that mutate during normal use
// and therefore do not belong in the user-authored, version-controllable
// config.json.
type State struct {
	CursorMemory      CursorMemoryState      `json:"cursorMemory"`
	NavigationHistory NavigationHistoryState `json:"navigationHistory"`
	FileFilter        FileFilterState        `json:"fileFilter"`
	Sort              *SortConfig            `json:"sort,omitempty"` // Last-applied sort; nil means config.json's ui.sort is the effective default
}

// newDefaultState returns a State with empty, non-nil maps/slices and no
// recorded sort override.
func newDefaultState() *State {
	return &State{
		CursorMemory: CursorMemoryState{
			Entries:  make(map[string]string),
			LastUsed: make(map[string]time.Time),
		},
		NavigationHistory: NavigationHistoryState{
			Entries:  make([]string, 0),
			LastUsed: make(map[string]time.Time),
			UseCount: make(map[string]int),
			Pinned:   make([]string, 0),
		},
		FileFilter: FileFilterState{
			Entries: make([]FilterEntry, 0),
			Current: nil,
			Enabled: false,
		},
		Sort: nil,
	}
}

// cloneState returns a deep copy of s suitable for independent mutation, used
// to snapshot state before handing it to the async save worker.
func cloneState(s *State) *State {
	if s == nil {
		return nil
	}

	clone := *s
	clone.CursorMemory = cloneCursorMemoryState(s.CursorMemory)
	clone.NavigationHistory = cloneNavigationHistoryState(s.NavigationHistory)
	clone.FileFilter = cloneFileFilterState(s.FileFilter)
	if s.Sort != nil {
		sortCopy := *s.Sort
		clone.Sort = &sortCopy
	}
	return &clone
}

func cloneCursorMemoryState(src CursorMemoryState) CursorMemoryState {
	clone := src
	if src.Entries != nil {
		clone.Entries = make(map[string]string, len(src.Entries))
		for k, v := range src.Entries {
			clone.Entries[k] = v
		}
	}
	if src.LastUsed != nil {
		clone.LastUsed = make(map[string]time.Time, len(src.LastUsed))
		for k, v := range src.LastUsed {
			clone.LastUsed[k] = v
		}
	}
	return clone
}

func cloneNavigationHistoryState(src NavigationHistoryState) NavigationHistoryState {
	clone := src
	if src.Entries != nil {
		clone.Entries = make([]string, len(src.Entries))
		copy(clone.Entries, src.Entries)
	}
	if src.LastUsed != nil {
		clone.LastUsed = make(map[string]time.Time, len(src.LastUsed))
		for k, v := range src.LastUsed {
			clone.LastUsed[k] = v
		}
	}
	if src.UseCount != nil {
		clone.UseCount = make(map[string]int, len(src.UseCount))
		for k, v := range src.UseCount {
			clone.UseCount[k] = v
		}
	}
	if src.Pinned != nil {
		clone.Pinned = make([]string, len(src.Pinned))
		copy(clone.Pinned, src.Pinned)
	}
	return clone
}

func cloneFileFilterState(src FileFilterState) FileFilterState {
	clone := src
	if src.Entries != nil {
		clone.Entries = make([]FilterEntry, len(src.Entries))
		copy(clone.Entries, src.Entries)
	}
	if src.Current != nil {
		currentCopy := *src.Current
		clone.Current = &currentCopy
	}
	return clone
}

// AddToNavigationHistory adds a path to navigation history, bumping its usage
// count and pruning by frecency once entries exceed maxEntries. Pinned paths
// live in a separate list and are never pruned here.
func (s *State) AddToNavigationHistory(path string, maxEntries int) {
	s.AddNavigationHistoryPaths([]string{path}, maxEntries)
}

// AddNavigationHistoryPaths records the whole batch before ranking or pruning
// so a path repeatedly used within a job retains each visit.
func (s *State) AddNavigationHistoryPaths(paths []string, maxEntries int) {
	history := &s.NavigationHistory
	ensureNavigationHistoryStatsState(history)
	// Statistics for legacy files can include keys absent from Entries.
	var existing map[string]bool
	if len(paths) > 1 {
		existing = make(map[string]bool, len(history.Entries))
		for _, path := range history.Entries {
			existing[path] = true
		}
	}
	now := time.Now()
	for _, path := range paths {
		found := existing[path]
		if existing == nil {
			found = slices.Contains(history.Entries, path)
		} else {
			existing[path] = true
		}
		if !found {
			history.Entries = append(history.Entries, path)
		}
		now = time.Now()
		history.LastUsed[path] = now
		history.UseCount[path]++
	}
	sortNavigationHistoryState(history, now)
	pruneNavigationHistoryState(history, maxEntries)
}

// GetNavigationHistory returns the navigation history entries sorted by frecency.
func (s *State) GetNavigationHistory() []string {
	history := &s.NavigationHistory
	ensureNavigationHistoryStatsState(history)
	entries := history.Entries
	if len(entries) <= 1 {
		return entries
	}

	sorted := make([]string, len(entries))
	copy(sorted, entries)
	sortNavigationHistoryEntries(sorted, history.LastUsed, history.UseCount, time.Now())

	return sorted
}

// FilterNavigationHistory filters history entries by query (case-insensitive partial match).
func (s *State) FilterNavigationHistory(query string) []string {
	if query == "" {
		return s.NavigationHistory.Entries
	}

	query = strings.ToLower(query)
	var filtered []string

	for _, path := range s.NavigationHistory.Entries {
		if strings.Contains(strings.ToLower(path), query) {
			filtered = append(filtered, path)
		}
	}

	return filtered
}

// PinNavigationPath saves a path for History Jump without adding it to prunable history.
func (s *State) PinNavigationPath(path string) bool {
	if path == "" {
		return false
	}
	history := &s.NavigationHistory
	ensureNavigationHistoryPinnedState(history)
	for _, entry := range history.Pinned {
		if entry == path {
			return false
		}
	}
	history.Pinned = append(history.Pinned, path)
	return true
}

// UnpinNavigationPath removes a saved History Jump path.
func (s *State) UnpinNavigationPath(path string) bool {
	history := &s.NavigationHistory
	ensureNavigationHistoryPinnedState(history)
	for i, entry := range history.Pinned {
		if entry == path {
			history.Pinned = append(history.Pinned[:i], history.Pinned[i+1:]...)
			return true
		}
	}
	return false
}

// IsNavigationPathPinned reports whether a path is saved for History Jump.
func (s *State) IsNavigationPathPinned(path string) bool {
	history := &s.NavigationHistory
	ensureNavigationHistoryPinnedState(history)
	for _, entry := range history.Pinned {
		if entry == path {
			return true
		}
	}
	return false
}

func ensureNavigationHistoryStatsState(history *NavigationHistoryState) {
	if history.LastUsed == nil {
		history.LastUsed = make(map[string]time.Time)
	}
	if history.UseCount == nil {
		history.UseCount = make(map[string]int)
	}
	for _, path := range history.Entries {
		if _, ok := history.UseCount[path]; !ok {
			history.UseCount[path] = 1
		}
	}
}

func ensureNavigationHistoryPinnedState(history *NavigationHistoryState) {
	if history.Pinned == nil {
		history.Pinned = make([]string, 0)
	}
}

func sortNavigationHistoryState(history *NavigationHistoryState, now time.Time) {
	sortNavigationHistoryEntries(history.Entries, history.LastUsed, history.UseCount, now)
}

func pruneNavigationHistoryState(history *NavigationHistoryState, maxEntries int) {
	if maxEntries <= 0 || len(history.Entries) <= maxEntries {
		return
	}
	for _, path := range history.Entries[maxEntries:] {
		delete(history.LastUsed, path)
		delete(history.UseCount, path)
	}
	history.Entries = history.Entries[:maxEntries]
}

// GetFileFilterEntries returns filter history sorted by frecency.
func (s *State) GetFileFilterEntries() []FilterEntry {
	entries := s.FileFilter.Entries
	if len(entries) <= 1 {
		return entries
	}
	sorted := make([]FilterEntry, len(entries))
	copy(sorted, entries)
	sortFileFilterEntries(sorted, time.Now())
	return sorted
}

// AddToFileFilterHistory records a filter pattern use and prunes by frecency
// once entries exceed maxEntries.
func (s *State) AddToFileFilterHistory(entry *FilterEntry, maxEntries int) {
	if entry == nil || strings.TrimSpace(entry.Pattern) == "" || EffectiveFilterPattern(entry.Pattern) == "" {
		return
	}
	filter := &s.FileFilter
	now := time.Now()
	for i := range filter.Entries {
		if filter.Entries[i].Pattern == entry.Pattern {
			filter.Entries[i].LastUsed = now
			filter.Entries[i].UseCount++
			sortFileFilterEntries(filter.Entries, now)
			pruneFileFilterEntriesState(filter, maxEntries, now)
			return
		}
	}

	newEntry := *entry
	newEntry.LastUsed = now
	if newEntry.UseCount <= 0 {
		newEntry.UseCount = 1
	}
	filter.Entries = append(filter.Entries, newEntry)
	sortFileFilterEntries(filter.Entries, now)
	pruneFileFilterEntriesState(filter, maxEntries, now)
}

// RemoveFileFilterEntry removes an exact saved filter pattern from history.
func (s *State) RemoveFileFilterEntry(pattern string) bool {
	entries := s.FileFilter.Entries
	for i := range entries {
		if entries[i].Pattern == pattern {
			s.FileFilter.Entries = append(entries[:i], entries[i+1:]...)
			return true
		}
	}
	return false
}

func pruneFileFilterEntriesState(filter *FileFilterState, maxEntries int, now time.Time) {
	if maxEntries <= 0 || len(filter.Entries) <= maxEntries {
		return
	}
	sortFileFilterEntries(filter.Entries, now)
	filter.Entries = filter.Entries[:maxEntries]
}

// sortNavigationHistoryEntries sorts path entries by frecency (highest score
// first), breaking ties by most-recent use and then lexically for stability.
func sortNavigationHistoryEntries(entries []string, lastUsed map[string]time.Time, useCount map[string]int, now time.Time) {
	sort.SliceStable(entries, func(i, j int) bool {
		scoreI := frecencyScore(useCount[entries[i]], lastUsed[entries[i]], now)
		scoreJ := frecencyScore(useCount[entries[j]], lastUsed[entries[j]], now)
		if scoreI != scoreJ {
			return scoreI > scoreJ
		}
		timeI := lastUsed[entries[i]]
		timeJ := lastUsed[entries[j]]
		if !timeI.Equal(timeJ) {
			return timeI.After(timeJ)
		}
		return entries[i] < entries[j]
	})
}

// frecencyScore favors entries used recently and/or often, decaying the
// weight of a use-count as its age bucket grows.
func frecencyScore(useCount int, lastUsed time.Time, now time.Time) float64 {
	if useCount <= 0 {
		useCount = 1
	}
	age := now.Sub(lastUsed)
	switch {
	case lastUsed.IsZero():
		return float64(useCount) * 0.25
	case age <= time.Hour:
		return float64(useCount) * 4
	case age <= 24*time.Hour:
		return float64(useCount) * 2
	case age <= 7*24*time.Hour:
		return float64(useCount) * 0.5
	default:
		return float64(useCount) * 0.25
	}
}

// EffectiveFilterPattern returns the glob portion of a saved filter entry.
// Text after ";;" is treated as a searchable user comment.
func EffectiveFilterPattern(pattern string) string {
	if idx := strings.Index(pattern, ";;"); idx >= 0 {
		pattern = pattern[:idx]
	}
	return strings.TrimSpace(pattern)
}

// sortFileFilterEntries sorts filter entries by frecency (highest score
// first), breaking ties by most-recent use and then lexically for stability.
func sortFileFilterEntries(entries []FilterEntry, now time.Time) {
	sort.SliceStable(entries, func(i, j int) bool {
		scoreI := frecencyScore(entries[i].UseCount, entries[i].LastUsed, now)
		scoreJ := frecencyScore(entries[j].UseCount, entries[j].LastUsed, now)
		if scoreI != scoreJ {
			return scoreI > scoreJ
		}
		if !entries[i].LastUsed.Equal(entries[j].LastUsed) {
			return entries[i].LastUsed.After(entries[j].LastUsed)
		}
		return entries[i].Pattern < entries[j].Pattern
	})
}

// EffectiveSort returns the state's recorded sort override, or configDefault
// (typically cfg.UI.Sort) when no sort has been applied yet.
func (s *State) EffectiveSort(configDefault SortConfig) SortConfig {
	if s != nil && s.Sort != nil {
		return *s.Sort
	}
	return configDefault
}

// StateManager coalesces runtime-state changes and serializes atomic writes.
type StateManager struct {
	statePath  string
	debugPrint func(format string, args ...interface{})

	mu               sync.Mutex
	saveDelay        time.Duration
	timer            *time.Timer
	revision         uint64
	pending          *State
	owned            bool   // pending may only be read on the state owner's goroutine.
	snapshot         *State // Immutable until a successful write or a newer capture.
	snapshotRevision uint64
	writerActive     bool
	closed           bool

	writeMu sync.Mutex
	written uint64
}

// ErrStateManagerClosed is returned when operations are attempted after Close has been called.
var ErrStateManagerClosed = errors.New("state manager closed")

// NewStateManager creates a manager for the default runtime-state file.
func NewStateManager(debugPrint func(format string, args ...interface{})) *StateManager {
	return NewStateManagerWithPath(getStatePath(), debugPrint)
}

// NewStateManagerWithPath creates a runtime-state manager rooted at an
// explicit state.json path instead of the OS-default location. Used by the
// -profile/-state-dir CLI overrides.
func NewStateManagerWithPath(statePath string, debugPrint func(format string, args ...interface{})) *StateManager {
	if debugPrint == nil {
		debugPrint = func(string, ...interface{}) {}
	}
	return &StateManager{statePath: statePath, debugPrint: debugPrint, saveDelay: 500 * time.Millisecond}
}

// StatePath returns the full state.json path used by this manager.
func (m *StateManager) StatePath() string {
	return m.statePath
}

// Load returns runtime state from state.json. If state.json does not exist,
// it performs a one-time, best-effort migration of legacy runtime keys out of
// configPath (config.json's ui.cursorMemory/navigationHistory/fileFilter data
// fields), then synchronously writes state.json so its existence becomes the
// migration marker for future launches. configPath is never written to: if it
// is missing, unreadable, or fails to parse, migration simply yields a
// default state instead of failing Load.
func (m *StateManager) Load(configPath string) (*State, error) {
	data, err := os.ReadFile(m.statePath)
	if err == nil {
		var state State
		if err := json.Unmarshal(data, &state); err != nil {
			return nil, fmt.Errorf("error parsing state file: %w", err)
		}
		normalizeState(&state)
		m.debugPrint("State: loaded path=%s", m.statePath)
		return &state, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("error reading state file: %w", err)
	}

	state := migrateLegacyRuntimeState(configPath, m.debugPrint)
	if err := m.saveState(state); err != nil {
		return nil, fmt.Errorf("error writing migrated state file: %w", err)
	}
	m.debugPrint("State: migrated and wrote path=%s", m.statePath)
	return state, nil
}

// normalizeState fixes up a state loaded from disk so its maps/slices are
// non-nil and its derived fields (useCount defaults, pinned list) are
// consistent, mirroring the legacy ensureNavigationHistoryStats/Pinned
// migration behavior that used to run on every config.json load.
func normalizeState(state *State) {
	if state.CursorMemory.Entries == nil {
		state.CursorMemory.Entries = make(map[string]string)
	}
	if state.CursorMemory.LastUsed == nil {
		state.CursorMemory.LastUsed = make(map[string]time.Time)
	}
	if state.NavigationHistory.Entries == nil {
		state.NavigationHistory.Entries = make([]string, 0)
	}
	ensureNavigationHistoryStatsState(&state.NavigationHistory)
	ensureNavigationHistoryPinnedState(&state.NavigationHistory)
	if state.FileFilter.Entries == nil {
		state.FileFilter.Entries = make([]FilterEntry, 0)
	}
}

// legacyRuntimeStateDoc mirrors only the runtime-state keys that used to live
// in config.json, for one-time migration into state.json. It deliberately
// does not mirror ui.sort: per the migration design, State.Sort stays nil
// after migration so config.json's ui.sort remains the effective default
// until the user applies a sort again.
type legacyRuntimeStateDoc struct {
	UI struct {
		CursorMemory struct {
			Entries  map[string]string    `json:"entries"`
			LastUsed map[string]time.Time `json:"lastUsed"`
		} `json:"cursorMemory"`
		NavigationHistory struct {
			Entries  []string             `json:"entries"`
			LastUsed map[string]time.Time `json:"lastUsed"`
			UseCount map[string]int       `json:"useCount"`
			Pinned   []string             `json:"pinned"`
		} `json:"navigationHistory"`
		FileFilter struct {
			Entries []FilterEntry `json:"entries"`
			Current *FilterEntry  `json:"current"`
			Enabled bool          `json:"enabled"`
		} `json:"fileFilter"`
	} `json:"ui"`
}

// migrateLegacyRuntimeState builds a State from legacy runtime keys found in
// configPath. Any read or parse failure is treated as "nothing to migrate"
// rather than an error: migration is a best-effort convenience, not a
// required step, and configPath is never modified by this function.
func migrateLegacyRuntimeState(configPath string, debugPrint func(format string, args ...interface{})) *State {
	state := newDefaultState()

	data, err := os.ReadFile(configPath)
	if err != nil {
		debugPrint("State: no legacy config to migrate from path=%s err=%v", configPath, err)
		return state
	}

	var legacy legacyRuntimeStateDoc
	if err := json.Unmarshal(data, &legacy); err != nil {
		debugPrint("State: legacy config unreadable for migration path=%s err=%v", configPath, err)
		return state
	}

	if legacy.UI.CursorMemory.Entries != nil {
		state.CursorMemory.Entries = legacy.UI.CursorMemory.Entries
	}
	if legacy.UI.CursorMemory.LastUsed != nil {
		state.CursorMemory.LastUsed = legacy.UI.CursorMemory.LastUsed
	}

	if legacy.UI.NavigationHistory.Entries != nil {
		state.NavigationHistory.Entries = legacy.UI.NavigationHistory.Entries
	}
	if legacy.UI.NavigationHistory.LastUsed != nil {
		state.NavigationHistory.LastUsed = legacy.UI.NavigationHistory.LastUsed
	}
	if legacy.UI.NavigationHistory.UseCount != nil {
		state.NavigationHistory.UseCount = legacy.UI.NavigationHistory.UseCount
	}
	if legacy.UI.NavigationHistory.Pinned != nil {
		state.NavigationHistory.Pinned = legacy.UI.NavigationHistory.Pinned
	}
	ensureNavigationHistoryStatsState(&state.NavigationHistory)
	ensureNavigationHistoryPinnedState(&state.NavigationHistory)

	if legacy.UI.FileFilter.Entries != nil {
		state.FileFilter.Entries = legacy.UI.FileFilter.Entries
	}
	state.FileFilter.Enabled = legacy.UI.FileFilter.Enabled
	if legacy.UI.FileFilter.Current != nil {
		currentCopy := *legacy.UI.FileFilter.Current
		state.FileFilter.Current = &currentCopy
	}

	debugPrint("State: migrated legacy runtime state from path=%s", configPath)
	return state
}

// Save persists state synchronously.
func (m *StateManager) Save(state *State) error {
	if err := m.SaveAsync(state); err != nil {
		return err
	}
	return m.Flush()
}

// SaveAsync takes an independent snapshot before returning. Callers may
// immediately mutate the supplied state.
func (m *StateManager) SaveAsync(state *State) error {
	return m.schedule(cloneState(state), nil)
}

// ScheduleSave coalesces mutations to a state owned by a single goroutine.
// dispatch queues snapshot capture on that owner. Flush and Close must also
// run on the owner, or after all state mutations have stopped.
func (m *StateManager) ScheduleSave(state *State, dispatch func(func())) error {
	if dispatch == nil {
		return m.SaveAsync(state)
	}
	return m.schedule(state, dispatch)
}

func (m *StateManager) schedule(state *State, dispatch func(func())) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrStateManagerClosed
	}
	m.revision++
	rev := m.revision
	m.pending, m.owned = state, dispatch != nil
	if m.timer != nil {
		m.timer.Stop()
	}
	m.timer = time.AfterFunc(m.saveDelay, func() {
		if dispatch == nil {
			m.saveDue(rev)
		} else {
			dispatch(func() { m.saveDue(rev) })
		}
	})
	return nil
}

func (m *StateManager) saveDue(rev uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.revision != rev || m.pending == nil {
		return
	}
	m.captureLocked()
	if !m.writerActive {
		m.writerActive = true
		go m.writeSnapshots()
	}
}

// A slow filesystem can fall behind several captures. Keep only the newest
// waiting snapshot and let a single writer finish the one already in progress.
func (m *StateManager) writeSnapshots() {
	m.mu.Lock()
	for m.snapshot != nil {
		rev, snapshot := m.snapshotRevision, m.snapshot
		m.mu.Unlock()
		err := m.write(rev, snapshot)
		if err != nil {
			m.debugPrint("State: Error saving state: %v", err)
		}
		m.mu.Lock()
		if err != nil && m.snapshotRevision == rev {
			break // Flush, Close, or a newer capture retries the pending state.
		}
	}
	m.writerActive = false
	m.mu.Unlock()
}

// captureLocked snapshots a dirty state, or returns the snapshot from an
// in-flight or failed write. This lets Flush retry or finish it synchronously.
func (m *StateManager) captureLocked() *State {
	if m.timer != nil {
		m.timer.Stop()
		m.timer = nil
	}
	if m.pending != nil {
		m.snapshot = m.pending
		m.snapshotRevision = m.revision
		if m.owned {
			m.snapshot = cloneState(m.pending)
		}
		m.pending = nil
	}
	return m.snapshot
}

func (m *StateManager) write(rev uint64, snapshot *State) error {
	if snapshot == nil {
		return nil
	}
	m.writeMu.Lock()
	defer m.writeMu.Unlock()
	if rev <= m.written {
		return nil
	}
	if err := m.saveState(snapshot); err != nil {
		return err
	}
	m.written = rev
	m.mu.Lock()
	if m.snapshotRevision == rev {
		m.snapshot = nil
	}
	m.mu.Unlock()
	return nil
}

// Flush synchronously writes the latest scheduled state.
func (m *StateManager) Flush() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return ErrStateManagerClosed
	}
	snapshot := m.captureLocked()
	rev := m.snapshotRevision
	m.mu.Unlock()
	return m.write(rev, snapshot)
}

// Close captures pending mutations and waits for the latest revision to reach disk.
func (m *StateManager) Close() error {
	m.mu.Lock()
	m.closed = true
	snapshot := m.captureLocked()
	rev := m.snapshotRevision
	m.mu.Unlock()
	return m.write(rev, snapshot)
}

// saveState writes state to statePath atomically: marshal into a temp file
// created in the same directory, then rename over the destination so a crash
// mid-write cannot leave a corrupt state.json behind. The temp file is
// removed on any failure path.
func (m *StateManager) saveState(state *State) error {
	stateDir := filepath.Dir(m.statePath)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("error creating state directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling state: %w", err)
	}

	tmp, err := os.CreateTemp(stateDir, "state-*.json.tmp")
	if err != nil {
		return fmt.Errorf("error creating temp state file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("error writing temp state file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("error closing temp state file: %w", err)
	}

	if err := os.Rename(tmpPath, m.statePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("error renaming temp state file: %w", err)
	}

	return nil
}

// getStatePath returns the runtime state.json path following OS conventions,
// mirroring getConfigPath's directory layout and fallback style.
func getStatePath() string {
	var stateDir string

	switch runtime.GOOS {
	case "windows":
		// Windows: %LOCALAPPDATA%\nekomimist\nmf\state.json
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "state.json"
			}
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		stateDir = filepath.Join(localAppData, "nekomimist", "nmf")

	case "darwin":
		// macOS: ~/Library/Application Support/nekomimist/nmf/state.json
		home, err := os.UserHomeDir()
		if err != nil {
			return "state.json"
		}
		stateDir = filepath.Join(home, "Library", "Application Support", "nekomimist", "nmf")

	default:
		// Linux/Unix: $XDG_STATE_HOME/nekomimist/nmf/state.json or ~/.local/state/nekomimist/nmf/state.json
		xdgStateHome := os.Getenv("XDG_STATE_HOME")
		if xdgStateHome == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "state.json"
			}
			xdgStateHome = filepath.Join(home, ".local", "state")
		}
		stateDir = filepath.Join(xdgStateHome, "nekomimist", "nmf")
	}

	return filepath.Join(stateDir, "state.json")
}
