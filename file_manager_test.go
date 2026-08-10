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
		browser: newTestBrowser(testBrowserOptions{files: files, sort: sortCfg}),
		fileList: widget.NewList(
			func() int { return 0 },
			func() fyne.CanvasObject { return widget.NewLabel("") },
			func(widget.ListItemID, fyne.CanvasObject) {},
		),
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
	gotFiles := fm.GetFiles()
	if got := namesOf(gotFiles); !reflect.DeepEqual(got, wantOrder) {
		t.Fatalf("modify-only ApplyChanges under name sort reordered: got %v, want unchanged order %v", got, wantOrder)
	}
	if gotFiles[1].Size != 999 {
		t.Fatalf("modified file content not applied: %+v", gotFiles[1])
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
	if got := namesOf(fm.GetFiles()); !reflect.DeepEqual(got, want) {
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
	if got := namesOf(fm.GetFiles()); !reflect.DeepEqual(got, want) {
		t.Fatalf("modify-only ApplyChanges under size sort did not resort: got %v, want %v", got, want)
	}
}

// TestApplyChangesModifyOnlyIsDirFlipResorts verifies that a modify-only
// merge still resorts when a modified entry's IsDir flips (e.g. "beta" is
// removed and replaced by a same-named directory between watcher polls),
// even under a "name" sort that would otherwise skip the resort. With
// DirectoriesFirst enabled, sortFileInfoSlice groups entries by IsDir before
// sorting each group by name, so skipping the resort would leave the flipped
// entry stranded in the file group instead of moving it into the directory
// group.
func TestApplyChangesModifyOnlyIsDirFlipResorts(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	files := []fileinfo.FileInfo{
		{Name: "alpha.txt", Path: "/tmp/alpha.txt", IsDir: false},
		{Name: "beta", Path: "/tmp/beta", IsDir: false},
	}
	fm := newApplyChangesTestFileManager(files, config.SortConfig{SortBy: "name", SortOrder: "asc", DirectoriesFirst: true})

	modified := fileinfo.FileInfo{Name: "beta", Path: "/tmp/beta", IsDir: true}
	fm.ApplyChanges(nil, nil, []fileinfo.FileInfo{modified})

	want := []string{"beta", "alpha.txt"}
	gotFiles := fm.GetFiles()
	if got := namesOf(gotFiles); !reflect.DeepEqual(got, want) {
		t.Fatalf("modify-only ApplyChanges with IsDir flip did not resort into directory group: got %v, want %v", got, want)
	}
	if !gotFiles[0].IsDir {
		t.Fatalf("expected flipped entry to be marked as a directory: %+v", gotFiles[0])
	}
}

// A file recreated under a name the list still shows as deleted arrives as an
// add, because the watcher baseline excludes deleted entries. Appending it
// would leave the same path in the list twice.
func TestApplyChangesReplacesRecreatedPathInsteadOfDuplicating(t *testing.T) {
	deleted := fileinfo.FileInfo{Path: "/tmp/a.txt", Name: "a.txt", Status: fileinfo.StatusDeleted}
	other := fileinfo.FileInfo{Path: "/tmp/b.txt", Name: "b.txt"}
	fm := newApplyChangesTestFileManager([]fileinfo.FileInfo{deleted, other},
		config.SortConfig{SortBy: "name", SortOrder: "asc"})

	recreated := fileinfo.FileInfo{Path: "/tmp/a.txt", Name: "a.txt", Status: fileinfo.StatusAdded}
	fm.ApplyChanges([]fileinfo.FileInfo{recreated}, nil, nil)

	got := fm.GetFiles()
	if want := []string{"a.txt", "b.txt"}; !reflect.DeepEqual(namesOf(got), want) {
		t.Fatalf("files = %v, want %v", namesOf(got), want)
	}
	if got[0].Status != fileinfo.StatusAdded {
		t.Fatalf("status = %v, want the recreated entry to replace the deleted one", got[0].Status)
	}
}

func TestApplyChangesPreservesUnfilteredListingWhileFilterActive(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	entry := &config.FilterEntry{Pattern: "*.png"}
	allFiles := []fileinfo.FileInfo{
		{Name: "image.png", Path: "/tmp/image.png"},
		{Name: "notes.txt", Path: "/tmp/notes.txt"},
	}
	fm := newApplyChangesTestFileManager(allFiles,
		config.SortConfig{SortBy: "name", SortOrder: "asc"})
	if _, _, err := fm.browserModel().ApplyFilter(entry); err != nil {
		t.Fatalf("ApplyFilter: %v", err)
	}
	fm.state = &config.State{
		FileFilter: config.FileFilterState{Current: entry, Enabled: true},
	}

	fm.ApplyChanges([]fileinfo.FileInfo{
		{Name: "cover.png", Path: "/tmp/cover.png", Status: fileinfo.StatusAdded},
		{Name: "readme.md", Path: "/tmp/readme.md", Status: fileinfo.StatusAdded},
	}, nil, nil)

	if got, want := namesOf(fm.GetFiles()), []string{"cover.png", "image.png"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("visible files = %v, want filtered view %v", got, want)
	}
	if got, want := namesOf(fm.browserModel().SourceFiles()), []string{"cover.png", "image.png", "notes.txt", "readme.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("complete files = %v, want all entries retained as %v", got, want)
	}

	fm.DisableFilter()
	if got, want := namesOf(fm.GetFiles()), []string{"cover.png", "image.png", "notes.txt", "readme.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("files after disabling filter = %v, want retained complete list %v", got, want)
	}
}
