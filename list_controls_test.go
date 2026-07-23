package main

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
)

func TestUpdateFilesUsesActiveTemporarySort(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	fm := &FileManager{
		fileList: widget.NewList(
			func() int { return 0 },
			func() fyne.CanvasObject { return widget.NewLabel("") },
			func(widget.ListItemID, fyne.CanvasObject) {},
		),
		config: &config.Config{
			UI: config.UIConfig{
				Sort: config.SortConfig{SortBy: "name", SortOrder: "asc", DirectoriesFirst: true},
			},
		},
		activeSort:    config.SortConfig{SortBy: "modified", SortOrder: "desc", DirectoriesFirst: true},
		selectedFiles: map[string]bool{},
	}
	newer := fileinfo.FileInfo{Name: "a.txt", Path: "/tmp/a.txt", Modified: time.Unix(2, 0)}
	older := fileinfo.FileInfo{Name: "z.txt", Path: "/tmp/z.txt", Modified: time.Unix(1, 0)}

	fm.UpdateFiles([]fileinfo.FileInfo{older, newer})

	if len(fm.files) != 2 || fm.files[0].Name != "a.txt" || fm.files[1].Name != "z.txt" {
		t.Fatalf("files sorted by active sort = %+v, want newer first", fm.files)
	}
}

func TestApplyFilterUsesEffectivePatternBeforeComment(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	fm := &FileManager{
		fileList: widget.NewList(
			func() int { return 0 },
			func() fyne.CanvasObject { return widget.NewLabel("") },
			func(widget.ListItemID, fyne.CanvasObject) {},
		),
		config: &config.Config{
			UI: config.UIConfig{
				FileFilter: config.FileFilterConfig{},
				Sort:       config.SortConfig{SortBy: "name", SortOrder: "asc", DirectoriesFirst: true},
			},
		},
		state: &config.State{},
		files: []fileinfo.FileInfo{
			{Name: "main.go", Path: "/tmp/main.go"},
			{Name: "notes.md", Path: "/tmp/notes.md"},
			{Name: "docs", Path: "/tmp/docs", IsDir: true},
		},
		selectedFiles: map[string]bool{},
	}

	fm.ApplyFilter(&config.FilterEntry{Pattern: "*.go ;; 日本語"})

	if len(fm.files) != 2 {
		t.Fatalf("filtered files = %+v, want go file plus directory", fm.files)
	}
	if fm.files[0].Name != "docs" || fm.files[1].Name != "main.go" {
		t.Fatalf("filtered files = %+v, want directory and go file sorted by name", fm.files)
	}
}

func TestGetCurrentCursorIndexCacheHitAndSelfHeal(t *testing.T) {
	fm := &FileManager{
		files: []fileinfo.FileInfo{
			{Name: "apple.txt", Path: "/tmp/apple.txt", Size: 30},
			{Name: "banana.txt", Path: "/tmp/banana.txt", Size: 10},
			{Name: "cherry.txt", Path: "/tmp/cherry.txt", Size: 20},
		},
	}
	fm.sortFilesWithConfig(config.SortConfig{SortBy: "name", SortOrder: "asc"})
	fm.SetCursorByIndex(1)
	if fm.files[1].Name != "banana.txt" {
		t.Fatalf("setup: expected banana.txt at index 1, got %+v", fm.files)
	}
	if fm.cursorIndex != 1 {
		t.Fatalf("SetCursorByIndex should cache index 1, got %d", fm.cursorIndex)
	}

	// Re-sort by size: banana.txt (smallest) moves to index 0, so the cached
	// cursorIndex (still 1) no longer matches cursorPath ("banana.txt").
	fm.sortFilesWithConfig(config.SortConfig{SortBy: "size", SortOrder: "asc"})

	got := fm.GetCurrentCursorIndex()
	if got != 0 || fm.files[got].Name != "banana.txt" {
		t.Fatalf("GetCurrentCursorIndex self-heal = %d (%+v), want 0 (banana.txt)", got, fm.files)
	}
	if fm.cursorIndex != 0 {
		t.Fatalf("cursorIndex cache not updated after self-heal, got %d", fm.cursorIndex)
	}

	// Second call should now hit the cache directly and return the same value.
	if got2 := fm.GetCurrentCursorIndex(); got2 != 0 {
		t.Fatalf("GetCurrentCursorIndex cache hit = %d, want 0", got2)
	}
}

func TestRefreshCursorEmptyFileListDoesNotPanic(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	fm := &FileManager{
		fileList: widget.NewList(
			func() int { return 0 },
			func() fyne.CanvasObject { return widget.NewLabel("") },
			func(widget.ListItemID, fyne.CanvasObject) {},
		),
		cursorIndex: -1,
	}

	fm.RefreshCursor()
	if fm.cursorRefreshSeq != 1 {
		t.Fatalf("cursorRefreshSeq = %d, want 1", fm.cursorRefreshSeq)
	}
	if fm.cursorItemUpdateSeq != 0 {
		t.Fatalf("cursorItemUpdateSeq = %d, want 0 without a cursor row", fm.cursorItemUpdateSeq)
	}
}

func TestCursorRefreshDiagnosticsTrackCurrentItemUpdate(t *testing.T) {
	fm := &FileManager{
		files: []fileinfo.FileInfo{
			{Name: "alpha.txt", Path: "/tmp/alpha.txt"},
			{Name: "beta.txt", Path: "/tmp/beta.txt"},
		},
		cursorPath:  "/tmp/alpha.txt",
		cursorIndex: 0,
	}

	seq, cursorIdx := fm.beginCursorRefresh("test")
	if seq != 1 || cursorIdx != 0 {
		t.Fatalf("beginCursorRefresh = (seq=%d, index=%d), want (1, 0)", seq, cursorIdx)
	}

	// An UpdateItem call for another visible row must not acknowledge the
	// cursor refresh.
	fm.noteCursorItemUpdated(1)
	if fm.cursorItemUpdateSeq != 0 {
		t.Fatalf("cursorItemUpdateSeq = %d after non-cursor row, want 0", fm.cursorItemUpdateSeq)
	}

	fm.noteCursorItemUpdated(0)
	if fm.cursorItemUpdateSeq != seq {
		t.Fatalf("cursorItemUpdateSeq = %d, want %d", fm.cursorItemUpdateSeq, seq)
	}

	// Repeated updates for the same refresh retain the same acknowledgement.
	fm.noteCursorItemUpdated(0)
	if fm.cursorItemUpdateSeq != seq {
		t.Fatalf("cursorItemUpdateSeq = %d after duplicate update, want %d", fm.cursorItemUpdateSeq, seq)
	}
}

func newScrollMarginTestFileManager(count, margin int) *FileManager {
	files := make([]fileinfo.FileInfo, count)
	for i := range files {
		files[i] = fileinfo.FileInfo{
			Name: fmt.Sprintf("file-%02d", i),
			Path: fmt.Sprintf("/tmp/file-%02d", i),
		}
	}

	item := widget.NewLabel("row")
	fm := &FileManager{
		files:              files,
		fileListItemHeight: item.MinSize().Height,
		config: &config.Config{
			UI: config.UIConfig{ScrollMargin: margin},
		},
		cursorIndex: -1,
	}
	fm.fileList = widget.NewList(
		func() int { return len(fm.files) },
		func() fyne.CanvasObject { return widget.NewLabel("row") },
		func(widget.ListItemID, fyne.CanvasObject) {},
	)
	return fm
}

func TestCursorScrollTargetAppliesMarginAndClampsToList(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	fm := newScrollMarginTestFileManager(20, 3)
	padding := fm.fileList.Theme().Size(theme.SizeNamePadding)
	rowStride := fm.fileListItemHeight + padding
	fm.fileList.Resize(fyne.NewSize(300, fm.fileListItemHeight+6*rowStride))

	tests := []struct {
		name      string
		cursor    int
		direction int
		want      int
	}{
		{name: "down", cursor: 5, direction: 1, want: 8},
		{name: "up", cursor: 5, direction: -1, want: 2},
		{name: "top", cursor: 1, direction: -1, want: 0},
		{name: "bottom", cursor: 18, direction: 1, want: 19},
		{name: "neutral", cursor: 5, direction: 0, want: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fm.cursorScrollTarget(tt.cursor, tt.direction); got != tt.want {
				t.Fatalf("cursorScrollTarget(%d, %d) = %d, want %d", tt.cursor, tt.direction, got, tt.want)
			}
		})
	}

	fm.config.UI.ScrollMargin = 0
	if got := fm.cursorScrollTarget(5, 1); got != 5 {
		t.Fatalf("zero-margin target = %d, want cursor row 5", got)
	}
}

func TestCursorScrollTargetCapsMarginForShortViewport(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	fm := newScrollMarginTestFileManager(20, 100)
	padding := fm.fileList.Theme().Size(theme.SizeNamePadding)
	rowStride := fm.fileListItemHeight + padding
	// Three complete rows fit, so at most two row steps can be kept beside
	// the cursor without forcing the cursor itself out of view.
	fm.fileList.Resize(fyne.NewSize(300, fm.fileListItemHeight+2*rowStride))

	if got := fm.cursorScrollTarget(5, 1); got != 7 {
		t.Fatalf("short viewport down target = %d, want 7", got)
	}
	if got := fm.cursorScrollTarget(2, -1); got != 0 {
		t.Fatalf("short viewport up target = %d, want 0", got)
	}
}

func TestRefreshCursorStartsScrollingAtConfiguredMargin(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	fm := newScrollMarginTestFileManager(40, 3)
	window := test.NewWindow(fm.fileList)
	defer window.Close()
	window.SetPadded(false)

	padding := fm.fileList.Theme().Size(theme.SizeNamePadding)
	rowStride := fm.fileListItemHeight + padding
	window.Resize(fyne.NewSize(300, fm.fileListItemHeight+9*rowStride))

	maxMargin, _, _ := fm.effectiveScrollMargin()
	if maxMargin < 3 {
		t.Fatalf("test viewport effective margin = %d, want at least 3", maxMargin)
	}

	fm.SetCursorByIndex(0)
	fm.RefreshCursor()
	lastFullyVisible := int((fm.fileList.Size().Height - fm.fileListItemHeight) / rowStride)
	lastWithoutScroll := lastFullyVisible - fm.config.UI.ScrollMargin
	for i := 1; i <= lastWithoutScroll; i++ {
		fm.SetCursorByIndex(i)
		fm.RefreshCursor()
	}
	if got := fm.fileList.GetScrollOffset(); got != 0 {
		t.Fatalf("offset before bottom margin = %v, want 0", got)
	}

	fm.SetCursorByIndex(lastWithoutScroll + 1)
	fm.RefreshCursor()
	if got := fm.fileList.GetScrollOffset(); got <= 0 {
		t.Fatalf("offset after entering bottom margin = %v, want positive", got)
	}

	startRow := 10
	fm.fileList.ScrollToOffset(float32(startRow) * rowStride)
	beforeUp := fm.fileList.GetScrollOffset()
	fm.cursorPath = fm.files[startRow+4].Path
	fm.cursorIndex = startRow + 4

	// Three complete rows remain above index startRow+3, so reaching it must
	// not scroll yet. The next upward move enters the margin and scrolls.
	fm.SetCursorByIndex(startRow + 3)
	fm.RefreshCursor()
	if got := fm.fileList.GetScrollOffset(); got != beforeUp {
		t.Fatalf("offset before top margin = %v, want %v", got, beforeUp)
	}
	fm.SetCursorByIndex(startRow + 2)
	fm.RefreshCursor()
	if got := fm.fileList.GetScrollOffset(); got >= beforeUp {
		t.Fatalf("offset after entering top margin = %v, want less than %v", got, beforeUp)
	}
}

func TestCursorScrollTargetCorrectsDirectionForOffscreenCursor(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	fm := newScrollMarginTestFileManager(40, 3)
	window := test.NewWindow(fm.fileList)
	defer window.Close()
	window.SetPadded(false)

	padding := fm.fileList.Theme().Size(theme.SizeNamePadding)
	rowStride := fm.fileListItemHeight + padding
	window.Resize(fyne.NewSize(300, fm.fileListItemHeight+9*rowStride))
	fm.fileList.ScrollToOffset(20 * rowStride)

	// The index increased relative to the old cursor, but the new cursor is
	// above the manually scrolled viewport. The viewport position must win so
	// ScrollTo uses an upward look-behind row and keeps the cursor visible.
	if got := fm.cursorScrollTarget(5, 1); got != 2 {
		t.Fatalf("offscreen cursor target = %d, want upward target 2", got)
	}
}

func TestRefreshCursorConsumesMoveDirection(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	fm := newScrollMarginTestFileManager(10, 3)
	fm.fileList.Resize(fyne.NewSize(300, 300))
	fm.SetCursorByIndex(0)
	fm.RefreshCursor()
	fm.SetCursorByIndex(1)
	if fm.cursorMoveDirection != 1 {
		t.Fatalf("pending move direction = %d, want down", fm.cursorMoveDirection)
	}
	fm.RefreshCursor()
	if fm.cursorMoveDirection != 0 {
		t.Fatalf("move direction after refresh = %d, want consumed", fm.cursorMoveDirection)
	}
	fm.refreshListAndCursor()
	if fm.cursorMoveDirection != 0 {
		t.Fatalf("move direction after structural refresh = %d, want none", fm.cursorMoveDirection)
	}
}

func TestSortSliceEquivalence(t *testing.T) {
	t0 := time.Unix(1000, 0)
	t1 := time.Unix(2000, 0)
	t2 := time.Unix(3000, 0)
	t3 := time.Unix(4000, 0)
	t4 := time.Unix(5000, 0)

	newFiles := func() []fileinfo.FileInfo {
		return []fileinfo.FileInfo{
			{Name: "Banana.txt", Size: 100, Modified: t3},
			{Name: "apple.TXT", Size: 50, Modified: t1},
			{Name: "README", Size: 10, Modified: t4},
			{Name: "notes", Size: 20, Modified: t0},
			{Name: "zeta.md", Size: 5, Modified: t2},
		}
	}

	tests := []struct {
		sortBy    string
		sortOrder string
		want      []string
	}{
		{"name", "asc", []string{"apple.TXT", "Banana.txt", "notes", "README", "zeta.md"}},
		{"name", "desc", []string{"zeta.md", "README", "notes", "Banana.txt", "apple.TXT"}},
		{"size", "asc", []string{"zeta.md", "README", "notes", "apple.TXT", "Banana.txt"}},
		{"size", "desc", []string{"Banana.txt", "apple.TXT", "notes", "README", "zeta.md"}},
		{"modified", "asc", []string{"notes", "apple.TXT", "zeta.md", "Banana.txt", "README"}},
		{"modified", "desc", []string{"README", "Banana.txt", "zeta.md", "apple.TXT", "notes"}},
		{"extension", "asc", []string{"notes", "README", "zeta.md", "apple.TXT", "Banana.txt"}},
		{"extension", "desc", []string{"Banana.txt", "apple.TXT", "zeta.md", "README", "notes"}},
	}

	for _, tt := range tests {
		t.Run(tt.sortBy+"_"+tt.sortOrder, func(t *testing.T) {
			files := newFiles()
			sortSlice(files, config.SortConfig{SortBy: tt.sortBy, SortOrder: tt.sortOrder})
			got := make([]string, len(files))
			for i, f := range files {
				got[i] = f.Name
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("sortSlice(SortBy=%s,SortOrder=%s) = %v, want %v", tt.sortBy, tt.sortOrder, got, tt.want)
			}
		})
	}
}

// TestSortFileInfoSlicePure exercises sortFileInfoSlice as a pure function
// (no *FileManager involved), verifying it neither mutates the input slice
// header's backing semantics unexpectedly nor touches any FileManager state,
// and that it keeps the ".."-pinning invariant under both DirectoriesFirst
// settings.
func TestSortFileInfoSlicePure(t *testing.T) {
	input := func() []fileinfo.FileInfo {
		return []fileinfo.FileInfo{
			{Name: "..", IsDir: true},
			{Name: "zeta", IsDir: true},
			{Name: "banana.txt"},
			{Name: "apple", IsDir: true},
			{Name: "cherry.txt"},
		}
	}

	t.Run("DirectoriesFirst", func(t *testing.T) {
		got := sortFileInfoSlice(input(), config.SortConfig{SortBy: "name", SortOrder: "asc", DirectoriesFirst: true})
		names := make([]string, len(got))
		for i, f := range got {
			names[i] = f.Name
		}
		want := []string{"..", "apple", "zeta", "banana.txt", "cherry.txt"}
		if !reflect.DeepEqual(names, want) {
			t.Fatalf("sortFileInfoSlice(DirectoriesFirst=true) = %v, want %v", names, want)
		}
		if !got[0].IsDir || got[0].Name != ".." {
			t.Fatalf("expected \"..\" pinned at index 0, got %+v", got[0])
		}
	})

	t.Run("FlatSort", func(t *testing.T) {
		got := sortFileInfoSlice(input(), config.SortConfig{SortBy: "name", SortOrder: "asc", DirectoriesFirst: false})
		names := make([]string, len(got))
		for i, f := range got {
			names[i] = f.Name
		}
		want := []string{"..", "apple", "banana.txt", "cherry.txt", "zeta"}
		if !reflect.DeepEqual(names, want) {
			t.Fatalf("sortFileInfoSlice(DirectoriesFirst=false) = %v, want %v", names, want)
		}
		if got[0].Name != ".." {
			t.Fatalf("expected \"..\" pinned at index 0, got %+v", got[0])
		}
	})

	t.Run("NoParentEntry", func(t *testing.T) {
		files := []fileinfo.FileInfo{
			{Name: "b", IsDir: true},
			{Name: "a.txt"},
		}
		got := sortFileInfoSlice(files, config.SortConfig{SortBy: "name", SortOrder: "asc", DirectoriesFirst: true})
		names := make([]string, len(got))
		for i, f := range got {
			names[i] = f.Name
		}
		want := []string{"b", "a.txt"}
		if !reflect.DeepEqual(names, want) {
			t.Fatalf("sortFileInfoSlice(no parent) = %v, want %v", names, want)
		}
	})

	t.Run("ShortCircuitsBelowTwoEntries", func(t *testing.T) {
		if got := sortFileInfoSlice(nil, config.SortConfig{SortBy: "name"}); got != nil {
			t.Fatalf("sortFileInfoSlice(nil) = %v, want nil", got)
		}
		single := []fileinfo.FileInfo{{Name: "only"}}
		if got := sortFileInfoSlice(single, config.SortConfig{SortBy: "name"}); len(got) != 1 || got[0].Name != "only" {
			t.Fatalf("sortFileInfoSlice(single) = %v, want unchanged single-element slice", got)
		}
	})
}
