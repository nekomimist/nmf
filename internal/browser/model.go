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
// interact with this model only through snapshots and value-returning methods.
type Model struct {
	mu sync.RWMutex

	path          string
	files         []fileinfo.FileInfo
	originalFiles []fileinfo.FileInfo
	selected      map[string]bool
	cursorPath    string
	cursorIndex   int
	storage       fileinfo.StorageInfo
	storageKnown  bool
	sort          config.SortConfig
	filter        *config.FilterEntry
}

type Snapshot struct {
	Path          string
	Files         []fileinfo.FileInfo
	OriginalFiles []fileinfo.FileInfo
	Selected      map[string]bool
	CursorPath    string
	CursorIndex   int
	Storage       fileinfo.StorageInfo
	StorageKnown  bool
	Sort          config.SortConfig
	Filter        *config.FilterEntry
}

type CursorChange struct {
	BeforeIndex int
	AfterIndex  int
	BeforePath  string
	AfterPath   string
}

func New(path string, sortConfig config.SortConfig) *Model {
	return &Model{
		path:        path,
		selected:    make(map[string]bool),
		cursorIndex: -1,
		sort:        sortConfig,
	}
}

func (m *Model) Snapshot() Snapshot {
	if m == nil {
		return Snapshot{CursorIndex: -1}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return Snapshot{
		Path:          m.path,
		Files:         cloneFiles(m.files),
		OriginalFiles: cloneFiles(m.originalFiles),
		Selected:      cloneSelection(m.selected),
		CursorPath:    m.cursorPath,
		CursorIndex:   m.cursorIndexLocked(),
		Storage:       m.storage,
		StorageKnown:  m.storageKnown,
		Sort:          m.sort,
		Filter:        cloneFilter(m.filter),
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

func (m *Model) OriginalFiles() []fileinfo.FileInfo {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneFiles(m.originalFiles)
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
	m.originalFiles = cloneFiles(files)
	if resort {
		m.originalFiles = SortFiles(m.originalFiles, m.sort)
	}
	m.files = cloneFiles(m.originalFiles)

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

// ApplyChanges merges watcher changes into the visible listing and reports a
// filter error if the configured pattern can no longer be applied.
func (m *Model) ApplyChanges(added, deleted, modified []fileinfo.FileInfo) error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	files := cloneFiles(m.files)
	for _, deletedFile := range deleted {
		for i, file := range files {
			if file.Path == deletedFile.Path {
				files[i].Status = fileinfo.StatusDeleted
				delete(m.selected, deletedFile.Path)
				break
			}
		}
	}

	typeFlipped := false
	for _, modifiedFile := range modified {
		for i, file := range files {
			if file.Path == modifiedFile.Path {
				if file.IsDir != modifiedFile.IsDir {
					typeFlipped = true
				}
				files[i] = modifiedFile
				break
			}
		}
	}

	for _, addedFile := range added {
		if i := indexOfPath(files, addedFile.Path); i >= 0 {
			files[i] = addedFile
			continue
		}
		files = append(files, addedFile)
	}

	resort := len(added) > 0 || len(deleted) > 0 || typeFlipped
	if !resort {
		switch m.sort.SortBy {
		case "size", "modified":
			resort = true
		}
	}
	return m.replaceFilesLocked(files, resort)
}

func (m *Model) ReplaceDirectory(path string, files []fileinfo.FileInfo, storage fileinfo.StorageInfo, storageKnown bool, sortConfig config.SortConfig) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.path = path
	m.storage = storage
	m.storageKnown = storageKnown
	m.sort = sortConfig
	m.selected = make(map[string]bool)
	// Keep an active filter across reloads and directory navigation. The loader
	// already sorted files with sortConfig, so no additional sort is needed.
	_ = m.replaceFilesLocked(files, false)
	m.mu.Unlock()
}

func (m *Model) Storage() (fileinfo.StorageInfo, bool) {
	if m == nil {
		return fileinfo.StorageInfo{}, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.storage, m.storageKnown
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
	m.sort = sortConfig
	if m.originalFiles == nil {
		m.files = SortFiles(m.files, sortConfig)
		m.cursorIndex = -1
		m.mu.Unlock()
		return
	}
	m.originalFiles = SortFiles(m.originalFiles, sortConfig)
	m.files = cloneFiles(m.originalFiles)
	if pattern := effectiveFilterPattern(m.filter); pattern != "" {
		if filtered, err := fileinfo.FilterFiles(m.files, pattern); err == nil {
			m.files = filtered
		}
	}
	m.cursorIndex = -1
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
	base := m.originalFiles
	if len(base) == 0 {
		base = m.files
		m.originalFiles = cloneFiles(m.files)
	}
	filtered, err := fileinfo.FilterFiles(base, pattern)
	if err != nil {
		return 0, len(base), err
	}
	m.filter = cloneFilter(entry)
	m.files = SortFiles(filtered, m.sort)
	m.cursorIndex = -1
	return len(m.files), len(base), nil
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
	m.files = SortFiles(cloneFiles(m.originalFiles), m.sort)
	m.cursorIndex = -1
	return true
}

func (m *Model) Upsert(created fileinfo.FileInfo) error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.originalFiles = upsertFileInfo(m.originalFiles, created)
	m.files = cloneFiles(m.originalFiles)
	var filterErr error
	if pattern := effectiveFilterPattern(m.filter); pattern != "" {
		filtered, err := fileinfo.FilterFiles(m.originalFiles, pattern)
		if err != nil {
			filterErr = err
		} else {
			m.files = filtered
		}
	}
	m.files = SortFiles(m.files, m.sort)
	m.cursorPath = created.Path
	m.cursorIndex = -1
	return filterErr
}

func (m *Model) Rename(oldPath, newName, newPath string) bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	update := func(files []fileinfo.FileInfo) bool {
		updated := false
		for i := range files {
			if files[i].Path == oldPath {
				files[i].Name = newName
				files[i].Path = newPath
				files[i].Status = fileinfo.StatusNormal
				updated = true
			}
		}
		return updated
	}
	updated := update(m.files)
	update(m.originalFiles)
	if m.cursorPath == oldPath {
		m.cursorPath = newPath
	}
	if m.selected[oldPath] {
		delete(m.selected, oldPath)
		m.selected[newPath] = true
	}
	m.cursorIndex = -1
	return updated
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
	m.selected[path] = selected
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
	m.selected[path] = !m.selected[path]
	return m.selected[path]
}

func (m *Model) ClearSelection() {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.selected = make(map[string]bool)
	m.mu.Unlock()
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
		if !isTargetFileInfo(file) || m.selected[file.Path] {
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
	files := make([]fileinfo.FileInfo, 0, len(m.selected))
	for _, file := range m.files {
		if m.selected[file.Path] && isTargetFileInfo(file) {
			files = append(files, file)
		}
	}
	return files
}

func SortFiles(files []fileinfo.FileInfo, sortConfig config.SortConfig) []fileinfo.FileInfo {
	if len(files) <= 1 {
		return files
	}
	if sortConfig.DirectoriesFirst {
		var dirs []fileinfo.FileInfo
		var regularFiles []fileinfo.FileInfo
		for _, file := range files {
			if file.Name == ".." {
				continue
			}
			if file.IsDir {
				dirs = append(dirs, file)
			} else {
				regularFiles = append(regularFiles, file)
			}
		}
		SortSlice(dirs, sortConfig)
		SortSlice(regularFiles, sortConfig)
		result := make([]fileinfo.FileInfo, 0, len(files))
		for _, file := range files {
			if file.Name == ".." {
				result = append(result, file)
				break
			}
		}
		result = append(result, dirs...)
		return append(result, regularFiles...)
	}

	var parent *fileinfo.FileInfo
	var regularFiles []fileinfo.FileInfo
	for _, file := range files {
		if file.Name == ".." {
			entry := file
			parent = &entry
		} else {
			regularFiles = append(regularFiles, file)
		}
	}
	SortSlice(regularFiles, sortConfig)
	result := make([]fileinfo.FileInfo, 0, len(files))
	if parent != nil {
		result = append(result, *parent)
	}
	return append(result, regularFiles...)
}

type sortKey struct {
	file      fileinfo.FileInfo
	lowerName string
	lowerExt  string
}

func SortSlice(files []fileinfo.FileInfo, sortConfig config.SortConfig) {
	if len(files) <= 1 {
		return
	}
	keys := make([]sortKey, len(files))
	for i, file := range files {
		key := sortKey{file: file, lowerName: strings.ToLower(file.Name)}
		if sortConfig.SortBy == "extension" {
			key.lowerExt = strings.ToLower(filepath.Ext(file.Name))
		}
		keys[i] = key
	}
	desc := sortConfig.SortOrder == "desc"
	slices.SortFunc(keys, func(a, b sortKey) int {
		var result int
		switch sortConfig.SortBy {
		case "size":
			result = cmp.Compare(a.file.Size, b.file.Size)
		case "modified":
			result = a.file.Modified.Compare(b.file.Modified)
		case "extension":
			switch {
			case a.lowerExt == "" && b.lowerExt != "":
				result = -1
			case a.lowerExt != "" && b.lowerExt == "":
				result = 1
			default:
				result = cmp.Compare(a.lowerExt, b.lowerExt)
				if result == 0 {
					result = cmp.Compare(a.lowerName, b.lowerName)
				}
			}
		default:
			result = cmp.Compare(a.lowerName, b.lowerName)
		}
		if desc {
			return -result
		}
		return result
	})
	for i, key := range keys {
		files[i] = key.file
	}
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

func indexOfPath(files []fileinfo.FileInfo, path string) int {
	for i, file := range files {
		if file.Path == path {
			return i
		}
	}
	return -1
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

func isTargetFileInfo(file fileinfo.FileInfo) bool {
	return file.Name != ".." && file.Status != fileinfo.StatusDeleted
}
