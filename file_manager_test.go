package main

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/fileinfo"
)

func TestApplyChangesRefreshesVisibleListing(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	fm := &FileManager{browser: newTestBrowser(testBrowserOptions{files: []fileinfo.FileInfo{
		{Name: "file.txt", Path: "/tmp/file.txt", Size: 1},
	}})}
	var displayedSize int64
	fm.fileList = widget.NewList(fm.FileCount,
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, _ fyne.CanvasObject) { displayedSize = fm.GetFiles()[id].Size },
	)
	window := test.NewWindow(fm.fileList)
	defer window.Close()
	window.Resize(fyne.NewSize(400, 200))
	if displayedSize != 1 {
		t.Fatalf("initial displayed size = %d, want 1", displayedSize)
	}

	fm.ApplyChanges(nil, nil, []fileinfo.FileInfo{{Name: "file.txt", Path: "/tmp/file.txt", Size: 99}})

	if displayedSize != 99 {
		t.Fatalf("displayed size after changes = %d, want 99", displayedSize)
	}
}
