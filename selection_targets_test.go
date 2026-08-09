package main

import (
	"reflect"
	"testing"

	"fyne.io/fyne/v2/test"

	"nmf/internal/fileinfo"
)

func TestGetAllSelectedFilesUsesAllOpenWindowsInOrder(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	runtime := newFileManagerWindowTestRuntime(t)

	left := &FileManager{
		runtime:       runtime,
		window:        app.NewWindow("left"),
		files:         []fileinfo.FileInfo{{Name: "a.txt", Path: "/left/a.txt"}, {Name: "skip.txt", Path: "/left/skip.txt"}},
		selectedFiles: map[string]bool{"/left/a.txt": true, "/left/skip.txt": false},
	}
	right := &FileManager{
		runtime: runtime,
		window:  app.NewWindow("right"),
		files: []fileinfo.FileInfo{
			{Name: "deleted.txt", Path: "/right/deleted.txt", Status: fileinfo.StatusDeleted},
			{Name: "b.txt", Path: "/right/b.txt"},
		},
		selectedFiles: map[string]bool{"/right/deleted.txt": true, "/right/b.txt": true},
	}

	registerFileManagerWindow(left)
	registerFileManagerWindow(right)

	gotFiles := left.GetAllSelectedFiles()
	got := make([]string, len(gotFiles))
	for i, fi := range gotFiles {
		got[i] = fi.Path
	}
	want := []string{"/left/a.txt", "/right/b.txt"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetAllSelectedFiles() = %#v, want %#v", got, want)
	}
}

func newFileManagerWindowTestRuntime(t *testing.T) *ApplicationRuntime {
	t.Helper()
	return &ApplicationRuntime{
		windows:                 newFileManagerWindowRegistry(),
		navigationHistoryEvents: newNavigationHistoryEventHub(),
	}
}
