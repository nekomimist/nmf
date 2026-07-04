package main

import (
	"reflect"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
)

func newApplyChangesTestFileManager(files []fileinfo.FileInfo, sortCfg config.SortConfig) *FileManager {
	return &FileManager{
		files: files,
		fileList: widget.NewList(
			func() int { return 0 },
			func() fyne.CanvasObject { return widget.NewLabel("") },
			func(widget.ListItemID, fyne.CanvasObject) {},
		),
		activeSort:    sortCfg,
		selectedFiles: map[string]bool{},
	}
}

func namesOf(files []fileinfo.FileInfo) []string {
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.Name
	}
	return names
}

// TestApplyChangesModifyOnlyUnderNameSortSkipsResort verifies the item 2.3
// optimization: a modify-only merge (no added/deleted files) under a
// name/extension sort must not re-sort, since a plain modification never
// changes the file's name. The fixture starts deliberately out of
// alphabetical order; if the code incorrectly re-sorted, the assertion on
// order below would fail.
func TestApplyChangesModifyOnlyUnderNameSortSkipsResort(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	files := []fileinfo.FileInfo{
		{Name: "gamma.txt", Path: "/tmp/gamma.txt", Size: 30},
		{Name: "alpha.txt", Path: "/tmp/alpha.txt", Size: 10},
		{Name: "beta.txt", Path: "/tmp/beta.txt", Size: 20},
	}
	fm := newApplyChangesTestFileManager(files, config.SortConfig{SortBy: "name", SortOrder: "asc"})

	modified := fileinfo.FileInfo{Name: "alpha.txt", Path: "/tmp/alpha.txt", Size: 999}
	fm.ApplyChanges(nil, nil, []fileinfo.FileInfo{modified})

	wantOrder := []string{"gamma.txt", "alpha.txt", "beta.txt"}
	if got := namesOf(fm.files); !reflect.DeepEqual(got, wantOrder) {
		t.Fatalf("modify-only ApplyChanges under name sort reordered: got %v, want unchanged order %v", got, wantOrder)
	}
	if fm.files[1].Size != 999 {
		t.Fatalf("modified file content not applied: %+v", fm.files[1])
	}
}

// TestApplyChangesAddedUnderNameSortResorts verifies that adding files still
// triggers a full re-sort (sortAffected is true whenever len(added) > 0),
// using the same deliberately-unsorted fixture as the modify-only test above.
func TestApplyChangesAddedUnderNameSortResorts(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	files := []fileinfo.FileInfo{
		{Name: "gamma.txt", Path: "/tmp/gamma.txt"},
		{Name: "alpha.txt", Path: "/tmp/alpha.txt"},
		{Name: "beta.txt", Path: "/tmp/beta.txt"},
	}
	fm := newApplyChangesTestFileManager(files, config.SortConfig{SortBy: "name", SortOrder: "asc"})

	added := fileinfo.FileInfo{Name: "delta.txt", Path: "/tmp/delta.txt"}
	fm.ApplyChanges([]fileinfo.FileInfo{added}, nil, nil)

	want := []string{"alpha.txt", "beta.txt", "delta.txt", "gamma.txt"}
	if got := namesOf(fm.files); !reflect.DeepEqual(got, want) {
		t.Fatalf("added-file ApplyChanges did not resort: got %v, want %v", got, want)
	}
}

// TestApplyChangesModifyOnlyUnderSizeSortResorts verifies that a modify-only
// merge still resorts under "size" (and, symmetrically, "modified"), since a
// content modification can change either value.
func TestApplyChangesModifyOnlyUnderSizeSortResorts(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	files := []fileinfo.FileInfo{
		{Name: "small.txt", Path: "/tmp/small.txt", Size: 10},
		{Name: "big.txt", Path: "/tmp/big.txt", Size: 20},
	}
	fm := newApplyChangesTestFileManager(files, config.SortConfig{SortBy: "size", SortOrder: "asc"})

	modified := fileinfo.FileInfo{Name: "big.txt", Path: "/tmp/big.txt", Size: 1}
	fm.ApplyChanges(nil, nil, []fileinfo.FileInfo{modified})

	want := []string{"big.txt", "small.txt"}
	if got := namesOf(fm.files); !reflect.DeepEqual(got, want) {
		t.Fatalf("modify-only ApplyChanges under size sort did not resort: got %v, want %v", got, want)
	}
}
