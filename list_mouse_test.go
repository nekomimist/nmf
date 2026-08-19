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
		browser: newTestBrowser(testBrowserOptions{files: files}),
		fileList: widget.NewList(
			func() int { return len(files) },
			func() fyne.CanvasObject { return widget.NewLabel("") },
			func(widget.ListItemID, fyne.CanvasObject) {},
		),
	}
}

func TestHandleFileNameClickTogglesMarkAndMovesCursor(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	fm := newMouseListTestFileManager(t)

	file, _ := fm.FileAt(1)
	fm.handleFileNameClick(1, file, 0)

	if got := fm.GetCurrentCursorIndex(); got != 1 {
		t.Fatalf("cursor index = %d, want 1", got)
	}
	selected := fm.GetSelectedFiles()
	if !selected["/tmp/a.txt"] {
		t.Fatalf("selectedFiles = %+v, want a.txt marked", selected)
	}

	fm.handleFileNameClick(1, file, 0)

	selected = fm.GetSelectedFiles()
	if selected["/tmp/a.txt"] {
		t.Fatalf("selectedFiles = %+v, want a.txt unmarked", selected)
	}
}

func TestHandleFileNameClickShiftMarksRangeFromPreviousCursor(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	fm := newMouseListTestFileManager(t)
	fm.SetCursorByIndex(1)

	file, _ := fm.FileAt(4)
	fm.handleFileNameClick(4, file, fyne.KeyModifierShift)

	if got := fm.GetCurrentCursorIndex(); got != 4 {
		t.Fatalf("cursor index = %d, want 4", got)
	}
	selected := fm.GetSelectedFiles()
	for _, path := range []string{"/tmp/a.txt", "/tmp/b.txt", "/tmp/docs"} {
		if !selected[path] {
			t.Fatalf("selectedFiles = %+v, want %s marked", selected, path)
		}
	}
	if selected["/tmp/gone.txt"] {
		t.Fatalf("selectedFiles = %+v, deleted item should not be marked", selected)
	}
}

func TestHandleFileNameClickSkipsParentAndDeletedEntries(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	fm := newMouseListTestFileManager(t)

	parent, _ := fm.FileAt(0)
	deleted, _ := fm.FileAt(3)
	fm.handleFileNameClick(0, parent, 0)
	fm.handleFileNameClick(3, deleted, 0)

	if got := fm.GetCurrentCursorIndex(); got != 3 {
		t.Fatalf("cursor index = %d, want 3", got)
	}
	selected := fm.GetSelectedFiles()
	if len(selected) != 0 {
		t.Fatalf("selectedFiles = %+v, want no marks", selected)
	}
}

func TestHandleFileNameClickMovesCursorWithoutMarkingCachedListing(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	fm := newMouseListTestFileManager(t)
	fm.directoryListingState = directoryListingCachedRefreshing
	file, _ := fm.FileAt(1)
	fm.handleFileNameClick(1, file, fyne.KeyModifierShift)

	if got := fm.GetCurrentCursorIndex(); got != 1 {
		t.Fatalf("cursor index = %d, want 1", got)
	}
	if selected := fm.GetSelectedFiles(); len(selected) != 0 {
		t.Fatalf("selected files = %+v, want cached click to leave marks empty", selected)
	}
}
