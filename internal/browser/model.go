package browser

import (
	"cmp"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
)

// Model owns the mutable state of one file-manager browsing session. Widget
// state and rendering remain with the window controller; background callers
// interact with this model only through value-returning methods.
// Locking makes each method call internally consistent, but a sequence of
// separate calls is not one transaction; prefer the compound operations below
// when values must be observed or changed together.
type Model struct {
	mu sync.RWMutex

	path          string
	files         []fileinfo.FileInfo
	originalFiles []fileinfo.FileInfo // complete watcher baseline, including hidden entries
	sourceFiles   []fileinfo.FileInfo // baseline after the window hidden-file policy
	showHidden    bool
	selected      map[string]bool
	cursorPath    string
	cursorIndex   int
	storage       fileinfo.StorageInfo
	storageKnown  bool
	sort          config.SortConfig
	filter        *config.FilterEntry
}

type CursorChange struct {
	BeforeIndex int
	AfterIndex  int
	BeforePath  string
	AfterPath   string
}

// ListingStats is the allocation-free summary needed by list/status UI.
// Counts, marked file sizes, and storage data are captured under one model read lock.
type ListingStats struct {
	VisibleEntries int
	TotalEntries   int
	MarkedEntries  int
	MarkedSize     int64
	Storage        fileinfo.StorageInfo
	StorageKnown   bool
}

func New(path string, sortConfig config.SortConfig, showHidden bool) *Model {
	return &Model{
		path:        path,
		showHidden:  showHidden,
		selected:    make(map[string]bool),
		cursorIndex: -1,
		sort:        normalizeSortConfig(sortConfig),
	}
}

func (m *Model) Path() string {
	if m == nil {
		return ""
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.path
}

func (m *Model) SetPath(path string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.path = path
	m.mu.Unlock()
}

func (m *Model) Files() []fileinfo.FileInfo {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneFiles(m.files)
}

func (m *Model) FileCount() int {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.files)
}

func (m *Model) FileAt(index int) (fileinfo.FileInfo, bool) {
	if m == nil {
		return fileinfo.FileInfo{}, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if index < 0 || index >= len(m.files) {
		return fileinfo.FileInfo{}, false
	}
	return m.files[index], true
}

// CursorFile returns the cursor index and file from one model transaction.
// It takes the write lock because healing the cursor index updates its cache.
func (m *Model) CursorFile() (int, fileinfo.FileInfo, bool) {
	if m == nil {
		return -1, fileinfo.FileInfo{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	index := m.cursorIndexLocked()
	if index < 0 || index >= len(m.files) {
		return -1, fileinfo.FileInfo{}, false
	}
	return index, m.files[index], true
}

// CursorNeighborPaths returns viable refresh fallbacks in nearest-first order
// without cloning the complete listing.
func (m *Model) CursorNeighborPaths() []string {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	cursorIndex := m.cursorIndexLocked()
	if cursorIndex < 0 {
		return nil
	}

	neighbors := make([]string, 0, max(len(m.files)-1, 0))
	for distance := 1; distance < len(m.files); distance++ {
		for _, index := range []int{cursorIndex + distance, cursorIndex - distance} {
			if index < 0 || index >= len(m.files) {
				continue
			}
			file := m.files[index]
			if !fileinfo.IsFileOperationTarget(file) {
				continue
			}
			neighbors = append(neighbors, file.Path)
		}
	}
	return neighbors
}

// SourceFiles returns the entries eligible for display before applying a glob.
func (m *Model) SourceFiles() []fileinfo.FileInfo {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneFiles(m.sourceFiles)
}

// WatcherFiles includes hidden and glob-filtered entries so attribute changes
// can bring them back into view without a reload.
func (m *Model) WatcherFiles() []fileinfo.FileInfo {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneFiles(m.originalFiles)
}

func (m *Model) ListingStats() ListingStats {
	if m == nil {
		return ListingStats{}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := ListingStats{
		VisibleEntries: entryCount(m.files),
		TotalEntries:   entryCount(m.sourceFiles),
		MarkedEntries:  len(m.selected),
		Storage:        m.storage,
		StorageKnown:   m.storageKnown,
	}
	if stats.MarkedEntries > 0 {
		for _, file := range m.sourceFiles {
			if m.selected[file.Path] && !file.IsDir {
				stats.MarkedSize += file.Size
			}
		}
	}
	return stats
}

// ReplaceFiles replaces the listing used by watcher updates. The input becomes
// the unfiltered baseline, matching the previous FileManager update behavior.
// UI refreshes must happen after this method returns, never while the model lock
// is held.
func (m *Model) ReplaceFiles(files []fileinfo.FileInfo, resort bool) error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.replaceFilesLocked(files, resort)
}

func (m *Model) replaceFilesLocked(files []fileinfo.FileInfo, resort bool) error {
	return m.adoptFilesLocked(cloneFiles(files), resort)
}

// adoptFilesLocked takes internally owned data. Public inputs are copied by
// replaceFilesLocked; watcher merges already own the model's baseline.
func (m *Model) adoptFilesLocked(files []fileinfo.FileInfo, resort bool) error {
	// A shorter source list owns separate storage. Reuse it for same-size
	// watcher updates so one hidden entry doesn't force a full slice allocation.
	var visible []fileinfo.FileInfo
	if len(m.sourceFiles) < len(m.originalFiles) && len(files) == len(m.originalFiles) {
		visible = m.sourceFiles[:0]
	}
	m.originalFiles = files
	if resort {
		m.originalFiles = SortFiles(files, m.sort)
	}
	m.sourceFiles = m.originalFiles
	if !m.showHidden {
		first := slices.IndexFunc(m.originalFiles, fileinfo.FileInfo.IsHidden)
		if first >= 0 {
			if cap(visible) < len(files)-1 {
				visible = make([]fileinfo.FileInfo, 0, len(files)-1)
			}
			visible = append(visible, m.originalFiles[:first]...)
			for _, file := range m.originalFiles[first:] {
				if file.IsHidden() {
					delete(m.selected, file.Path)
					continue
				}
				visible = append(visible, file)
			}
			clear(visible[len(visible):cap(visible)])
			m.sourceFiles = visible
		}
	}
	m.files = m.sourceFiles

	var filterErr error
	if pattern := effectiveFilterPattern(m.filter); pattern != "" {
		filtered, err := fileinfo.FilterFiles(m.files, pattern)
		if err != nil {
			filterErr = err
		} else {
			m.files = filtered
		}
	}
	m.cursorIndex = -1
	return filterErr
}

// ApplyChanges merges watcher changes into the unfiltered baseline and then
// re-derives the visible listing. Basing the merge on m.files would discard
// entries hidden by an active filter when the next watcher snapshot arrives.
func (m *Model) ApplyChanges(added, deleted, modified []fileinfo.FileInfo) error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	files := m.originalFiles
	deletedPaths := make(map[string]bool, len(deleted))
	for _, file := range deleted {
		deletedPaths[file.Path] = true
	}
	updates := make(map[string]fileinfo.FileInfo, len(modified)+len(added))
	for _, file := range modified {
		updates[file.Path] = file
	}
	for _, file := range added {
		updates[file.Path] = file
	}
	typeFlipped := false
	hiddenCursorIndex := -1
	for i, file := range files {
		if deletedPaths[file.Path] {
			files[i].Status = fileinfo.StatusDeleted
			delete(m.selected, file.Path)
		}
		if updated, ok := updates[file.Path]; ok {
			if file.Path == m.cursorPath && !m.showHidden && updated.IsHidden() {
				hiddenCursorIndex = m.cursorIndexLocked()
			}
			typeFlipped = typeFlipped || file.IsDir != updated.IsDir
			files[i] = updated
			delete(updates, file.Path)
		}
	}
	for _, file := range added {
		if updated, ok := updates[file.Path]; ok {
			files = append(files, updated)
			delete(updates, file.Path)
		}
	}

	resort := len(added) > 0 || len(deleted) > 0 || typeFlipped
	if !resort {
		switch m.sort.SortBy {
		case "size", "modified":
			resort = true
		}
	}
	err := m.adoptFilesLocked(files, resort)
	if hiddenCursorIndex >= 0 {
		m.restoreCursorRowLocked(hiddenCursorIndex)
	}
	return err
}

func (m *Model) ReplaceDirectory(path string, files []fileinfo.FileInfo, storage fileinfo.StorageInfo, storageKnown bool, sortConfig config.SortConfig) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.path = path
	m.storage = storage
	m.storageKnown = storageKnown
	m.sort = normalizeSortConfig(sortConfig)
	m.selected = make(map[string]bool)
	// Keep an active filter across reloads and directory navigation. The loader
	// already sorted files with sortConfig, so no additional sort is needed.
	_ = m.replaceFilesLocked(files, false)
	m.mu.Unlock()
}

func (m *Model) Sort() config.SortConfig {
	if m == nil {
		return config.SortConfig{}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sort
}

func (m *Model) ApplySort(sortConfig config.SortConfig) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.sort = normalizeSortConfig(sortConfig)
	_ = m.adoptFilesLocked(m.originalFiles, true)
	m.mu.Unlock()
}

func (m *Model) Filter() *config.FilterEntry {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneFilter(m.filter)
}

func (m *Model) ApplyFilter(entry *config.FilterEntry) (matched, total int, err error) {
	if m == nil || entry == nil {
		return 0, 0, nil
	}
	pattern := config.EffectiveFilterPattern(entry.Pattern)
	if err := fileinfo.ValidatePattern(pattern); err != nil {
		return 0, 0, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	previous := m.filter
	m.filter = cloneFilter(entry)
	// An in-place rename can intentionally retain the old row position.
	// When filtering restores sort order, do so on the baseline as well so
	// the next metadata-only watcher update preserves that order.
	if err := m.adoptFilesLocked(m.originalFiles, true); err != nil {
		m.filter = previous
		_ = m.adoptFilesLocked(m.originalFiles, false)
		return 0, len(m.sourceFiles), err
	}
	return len(m.files), len(m.sourceFiles), nil
}

// ClearFilter removes the active filter and restores the unfiltered listing.
// It reports whether list content was replaced and therefore needs refreshing.
func (m *Model) ClearFilter() bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.filter = nil
	if len(m.originalFiles) == 0 {
		return false
	}
	_ = m.adoptFilesLocked(m.originalFiles, true)
	return true
}

func (m *Model) Upsert(created fileinfo.FileInfo) error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cursorPath = created.Path
	return m.adoptFilesLocked(upsertFileInfo(m.originalFiles, created), true)
}

func (m *Model) Rename(oldPath, newName, newPath string) bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	previousIndex := m.cursorIndexLocked()
	for i := range m.originalFiles {
		file := &m.originalFiles[i]
		if file.Path != oldPath {
			continue
		}
		file.Name, file.Path, file.Status = newName, newPath, fileinfo.StatusNormal
		if file.FileType == fileinfo.FileTypeRegular || file.FileType == fileinfo.FileTypeHidden {
			file.FileType = fileinfo.FileTypeRegular
			if file.IsHidden() {
				file.FileType = fileinfo.FileTypeHidden
			}
		}
		if m.selected[oldPath] {
			delete(m.selected, oldPath)
			m.selected[newPath] = true
		}
		cursorRenamed := m.cursorPath == oldPath
		if cursorRenamed {
			m.cursorPath = newPath
		}
		_ = m.adoptFilesLocked(m.originalFiles, false)
		if cursorRenamed && m.cursorIndexLocked() < 0 {
			m.restoreCursorRowLocked(previousIndex)
		}
		return true
	}
	return false
}

// Keep the cursor near its former visible row when a local rename or a hidden
// attribute change removes its path from the listing.
func (m *Model) restoreCursorRowLocked(index int) {
	m.cursorPath = ""
	m.cursorIndex = -1
	if index >= 0 && len(m.files) > 0 {
		m.cursorIndex = min(index, len(m.files)-1)
		m.cursorPath = m.files[m.cursorIndex].Path
	}
}

func (m *Model) CursorPath() string {
	if m == nil {
		return ""
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cursorPath
}

func (m *Model) SetCursorPath(path string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.cursorPath = path
	m.cursorIndex = -1
	m.mu.Unlock()
}

func (m *Model) CursorIndex() int {
	if m == nil {
		return -1
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cursorIndexLocked()
}

func (m *Model) cursorIndexLocked() int {
	if m.cursorPath == "" {
		m.cursorIndex = -1
		return -1
	}
	if m.cursorIndex >= 0 && m.cursorIndex < len(m.files) && m.files[m.cursorIndex].Path == m.cursorPath {
		return m.cursorIndex
	}
	for i, file := range m.files {
		if file.Path == m.cursorPath {
			m.cursorIndex = i
			return i
		}
	}
	m.cursorIndex = -1
	return -1
}

func (m *Model) SetCursorIndex(index int) CursorChange {
	change := CursorChange{BeforeIndex: -1, AfterIndex: -1}
	if m == nil {
		return change
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	change.BeforeIndex = m.cursorIndexLocked()
	change.BeforePath = m.cursorPath
	if index >= 0 && index < len(m.files) {
		m.cursorPath = m.files[index].Path
		m.cursorIndex = index
	} else {
		m.cursorPath = ""
		m.cursorIndex = -1
	}
	change.AfterIndex = m.cursorIndex
	change.AfterPath = m.cursorPath
	return change
}

// SetCursorByName moves the cursor to the first matching visible name in one
// transaction. A missing name leaves the cursor unchanged.
func (m *Model) SetCursorByName(name string) (CursorChange, bool) {
	change := CursorChange{BeforeIndex: -1, AfterIndex: -1}
	if m == nil {
		return change, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	change.BeforeIndex = m.cursorIndexLocked()
	change.BeforePath = m.cursorPath
	for index, file := range m.files {
		if file.Name != name {
			continue
		}
		m.cursorPath = file.Path
		m.cursorIndex = index
		change.AfterIndex = index
		change.AfterPath = file.Path
		return change, true
	}
	change.AfterIndex = change.BeforeIndex
	change.AfterPath = change.BeforePath
	return change, false
}

func (m *Model) Selection() map[string]bool {
	if m == nil {
		return map[string]bool{}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneSelection(m.selected)
}

func (m *Model) IsSelected(path string) bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.selected[path]
}

func (m *Model) SetSelected(path string, selected bool) {
	if m == nil {
		return
	}
	m.mu.Lock()
	if m.selected == nil {
		m.selected = make(map[string]bool)
	}
	if selected {
		m.selected[path] = true
	} else {
		delete(m.selected, path)
	}
	m.mu.Unlock()
}

func (m *Model) RemoveSelected(path string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	delete(m.selected, path)
	m.mu.Unlock()
}

func (m *Model) ToggleSelected(path string) bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.selected == nil {
		m.selected = make(map[string]bool)
	}
	if m.selected[path] {
		delete(m.selected, path)
		return false
	}
	m.selected[path] = true
	return true
}

// SelectAll marks every visible ordinary entry in one model transaction. It
// reports whether the listing contained at least one selectable entry.
func (m *Model) SelectAll() bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.selected == nil {
		m.selected = make(map[string]bool)
	}
	found := false
	for _, file := range m.files {
		if !fileinfo.IsFileOperationTarget(file) {
			continue
		}
		m.selected[file.Path] = true
		found = true
	}
	return found
}

// InvertSelection flips visible ordinary entries in one model transaction.
// When directories are excluded, any existing directory mark is cleared.
func (m *Model) InvertSelection(includeDirectories bool) bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.selected == nil {
		m.selected = make(map[string]bool)
	}
	changed := false
	for _, file := range m.files {
		if !fileinfo.IsFileOperationTarget(file) {
			continue
		}
		if file.IsDir && !includeDirectories {
			if m.selected[file.Path] {
				delete(m.selected, file.Path)
				changed = true
			}
			continue
		}
		if m.selected[file.Path] {
			delete(m.selected, file.Path)
		} else {
			m.selected[file.Path] = true
		}
		changed = true
	}
	return changed
}

func (m *Model) ReplaceSelection(files []fileinfo.FileInfo) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.selected = make(map[string]bool, len(files))
	for _, file := range files {
		m.selected[file.Path] = true
	}
	m.mu.Unlock()
}

func (m *Model) MarkRange(anchor, target int) bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.files) == 0 || anchor < 0 || anchor >= len(m.files) || target < 0 || target >= len(m.files) {
		return false
	}
	if m.selected == nil {
		m.selected = make(map[string]bool)
	}
	start, end := anchor, target
	if start > end {
		start, end = end, start
	}
	changed := false
	for i := start; i <= end; i++ {
		file := m.files[i]
		if !fileinfo.IsFileOperationTarget(file) || m.selected[file.Path] {
			continue
		}
		m.selected[file.Path] = true
		changed = true
	}
	return changed
}

func (m *Model) SelectedFiles() []fileinfo.FileInfo {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.selectedFilesLocked()
}

// CommandTargets captures eligible marked rows, or the eligible cursor row
// when there are no visible marks, from one version of the listing.
func (m *Model) CommandTargets() []fileinfo.FileInfo {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if selected := m.selectedFilesLocked(); len(selected) > 0 {
		return selected
	}
	if idx := m.cursorIndexLocked(); idx >= 0 && fileinfo.IsFileOperationTarget(m.files[idx]) {
		return []fileinfo.FileInfo{m.files[idx]}
	}
	return nil
}

func (m *Model) selectedFilesLocked() []fileinfo.FileInfo {
	if len(m.selected) == 0 {
		return nil
	}
	files := make([]fileinfo.FileInfo, 0, len(m.selected))
	for _, file := range m.files {
		if m.selected[file.Path] && fileinfo.IsFileOperationTarget(file) {
			files = append(files, file)
		}
	}
	return files
}

type sortKey struct {
	index     int
	lowerName string
	lowerExt  string
}

// SortFiles sorts without modifying input, pinning the parent and optionally
// grouping directories. Zero/one-entry inputs need no copy.
func SortFiles(files []fileinfo.FileInfo, sortConfig config.SortConfig) []fileinfo.FileInfo {
	if len(files) <= 1 {
		return files
	}
	sortConfig = normalizeSortConfig(sortConfig)
	keys := make([]sortKey, len(files))
	for i, file := range files {
		key := sortKey{index: i}
		if sortConfig.SortBy != "size" && sortConfig.SortBy != "modified" {
			key.lowerName = strings.ToLower(file.Name)
		}
		if sortConfig.SortBy == "extension" {
			key.lowerExt = strings.ToLower(filepath.Ext(file.Name))
		}
		keys[i] = key
	}
	desc := sortConfig.SortOrder == "desc"
	slices.SortFunc(keys, func(a, b sortKey) int {
		af, bf := files[a.index], files[b.index]
		if af.Name == ".." && bf.Name != ".." {
			return -1
		}
		if bf.Name == ".." && af.Name != ".." {
			return 1
		}
		if sortConfig.DirectoriesFirst && af.IsDir != bf.IsDir {
			if af.IsDir {
				return -1
			}
			return 1
		}
		var result int
		switch sortConfig.SortBy {
		case "size":
			result = cmp.Compare(af.Size, bf.Size)
		case "modified":
			result = af.Modified.Compare(bf.Modified)
		case "extension":
			result = cmp.Compare(a.lowerExt, b.lowerExt)
			if result == 0 {
				result = cmp.Compare(a.lowerName, b.lowerName)
			}
		default:
			result = cmp.Compare(a.lowerName, b.lowerName)
		}
		if desc {
			return -result
		}
		return result
	})
	result := make([]fileinfo.FileInfo, len(files))
	for i, key := range keys {
		result[i] = files[key.index]
	}
	return result
}

func cloneFiles(files []fileinfo.FileInfo) []fileinfo.FileInfo {
	if files == nil {
		return nil
	}
	result := make([]fileinfo.FileInfo, len(files))
	copy(result, files)
	return result
}

func cloneSelection(selected map[string]bool) map[string]bool {
	result := make(map[string]bool, len(selected))
	for path, marked := range selected {
		result[path] = marked
	}
	return result
}

func cloneFilter(filter *config.FilterEntry) *config.FilterEntry {
	if filter == nil {
		return nil
	}
	result := *filter
	return &result
}

func effectiveFilterPattern(filter *config.FilterEntry) string {
	if filter == nil {
		return ""
	}
	return config.EffectiveFilterPattern(filter.Pattern)
}

func upsertFileInfo(files []fileinfo.FileInfo, created fileinfo.FileInfo) []fileinfo.FileInfo {
	for i := range files {
		if files[i].Path == created.Path {
			files[i] = created
			return files
		}
	}
	return append(files, created)
}

func entryCount(files []fileinfo.FileInfo) int {
	count := len(files)
	if count > 0 && files[0].Name == ".." {
		count--
	}
	return count
}

func normalizeSortConfig(sortConfig config.SortConfig) config.SortConfig {
	zeroValue := sortConfig.SortBy == "" && sortConfig.SortOrder == ""
	if sortConfig.SortBy == "" {
		sortConfig.SortBy = "name"
	}
	if sortConfig.SortOrder == "" {
		sortConfig.SortOrder = "asc"
	}
	if zeroValue {
		sortConfig.DirectoriesFirst = true
	}
	return sortConfig
}
