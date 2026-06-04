package main

import (
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
