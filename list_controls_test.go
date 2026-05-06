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
