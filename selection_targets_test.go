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
	resetFileManagerWindowTestRegistry(t)

	left := &FileManager{
		window:        app.NewWindow("left"),
		files:         []fileinfo.FileInfo{{Name: "a.txt", Path: "/left/a.txt"}, {Name: "skip.txt", Path: "/left/skip.txt"}},
		selectedFiles: map[string]bool{"/left/a.txt": true, "/left/skip.txt": false},
	}
	right := &FileManager{
		window: app.NewWindow("right"),
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

func resetFileManagerWindowTestRegistry(t *testing.T) {
	t.Helper()

	windowOrderMu.Lock()
	windowOrder = nil
	reopenPaths = nil
	windowOrderMu.Unlock()
	windowRegistry.Range(func(key, _ any) bool {
		windowRegistry.Delete(key)
		return true
	})
	t.Cleanup(func() {
		windowOrderMu.Lock()
		windowOrder = nil
		reopenPaths = nil
		windowOrderMu.Unlock()
		windowRegistry.Range(func(key, _ any) bool {
			windowRegistry.Delete(key)
			return true
		})
	})
}
