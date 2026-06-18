package main

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/fileinfo"
)

func newMouseListTestFileManager(t *testing.T) *FileManager {
	t.Helper()

	files := []fileinfo.FileInfo{
		{Name: "..", Path: "/tmp", IsDir: true},
		{Name: "a.txt", Path: "/tmp/a.txt"},
		{Name: "b.txt", Path: "/tmp/b.txt"},
		{Name: "gone.txt", Path: "/tmp/gone.txt", Status: fileinfo.StatusDeleted},
		{Name: "docs", Path: "/tmp/docs", IsDir: true},
	}
	return &FileManager{
		files:         files,
		originalFiles: files,
		fileList: widget.NewList(
			func() int { return len(files) },
			func() fyne.CanvasObject { return widget.NewLabel("") },
			func(widget.ListItemID, fyne.CanvasObject) {},
		),
		selectedFiles: map[string]bool{},
	}
}

func TestHandleFileNameClickTogglesMarkAndMovesCursor(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	fm := newMouseListTestFileManager(t)

	fm.handleFileNameClick(1, fm.files[1], 0)

	if got := fm.GetCurrentCursorIndex(); got != 1 {
		t.Fatalf("cursor index = %d, want 1", got)
	}
	if !fm.selectedFiles["/tmp/a.txt"] {
		t.Fatalf("selectedFiles = %+v, want a.txt marked", fm.selectedFiles)
	}

	fm.handleFileNameClick(1, fm.files[1], 0)

	if fm.selectedFiles["/tmp/a.txt"] {
		t.Fatalf("selectedFiles = %+v, want a.txt unmarked", fm.selectedFiles)
	}
}

func TestHandleFileNameClickShiftMarksRangeFromPreviousCursor(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	fm := newMouseListTestFileManager(t)
	fm.SetCursorByIndex(1)

	fm.handleFileNameClick(4, fm.files[4], fyne.KeyModifierShift)

	if got := fm.GetCurrentCursorIndex(); got != 4 {
		t.Fatalf("cursor index = %d, want 4", got)
	}
	for _, path := range []string{"/tmp/a.txt", "/tmp/b.txt", "/tmp/docs"} {
		if !fm.selectedFiles[path] {
			t.Fatalf("selectedFiles = %+v, want %s marked", fm.selectedFiles, path)
		}
	}
	if fm.selectedFiles["/tmp/gone.txt"] {
		t.Fatalf("selectedFiles = %+v, deleted item should not be marked", fm.selectedFiles)
	}
}

func TestHandleFileNameClickSkipsParentAndDeletedEntries(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	fm := newMouseListTestFileManager(t)

	fm.handleFileNameClick(0, fm.files[0], 0)
	fm.handleFileNameClick(3, fm.files[3], 0)

	if got := fm.GetCurrentCursorIndex(); got != 3 {
		t.Fatalf("cursor index = %d, want 3", got)
	}
	if len(fm.selectedFiles) != 0 {
		t.Fatalf("selectedFiles = %+v, want no marks", fm.selectedFiles)
	}
}
