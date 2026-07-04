package main

import (
	"reflect"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
)

func TestUpdateFilesUsesActiveTemporarySort(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	fm := &FileManager{
		fileBinding: binding.NewUntypedList(),
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
		fileBinding: binding.NewUntypedList(),
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

	fm := &FileManager{}
	for _, tt := range tests {
		t.Run(tt.sortBy+"_"+tt.sortOrder, func(t *testing.T) {
			files := newFiles()
			fm.sortSlice(files, config.SortConfig{SortBy: tt.sortBy, SortOrder: tt.sortOrder})
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
