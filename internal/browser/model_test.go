package browser

import (
	"reflect"
	"testing"
	"time"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
)

func TestModelReturnsDefensiveCopies(t *testing.T) {
	files := []fileinfo.FileInfo{{Name: "note.txt", Path: "/tmp/note.txt"}}
	filter := &config.FilterEntry{Pattern: "*.txt", UseCount: 3}
	model := New("/tmp", nameSort())
	model.ReplaceDirectory("/tmp", files, fileinfo.StorageInfo{Free: 10}, true, nameSort())
	model.SetSelected(files[0].Path, true)
	if _, _, err := model.ApplyFilter(filter); err != nil {
		t.Fatalf("ApplyFilter returned error: %v", err)
	}

	files[0].Name = "changed-input.txt"
	visible := model.Files()
	visible[0].Name = "changed-result.txt"
	original := model.OriginalFiles()
	original[0].Name = "changed-original-result.txt"
	selected := model.Selection()
	selected[files[0].Path] = false
	activeFilter := model.Filter()
	activeFilter.Pattern = "*.log"
	snapshot := model.Snapshot()
	snapshot.Files[0].Name = "changed-snapshot.txt"
	snapshot.Selected[files[0].Path] = false
	snapshot.Filter.Pattern = "*.md"

	if got := model.Files()[0].Name; got != "note.txt" {
		t.Fatalf("visible name = %q, want defensive copy to retain note.txt", got)
	}
	if got := model.OriginalFiles()[0].Name; got != "note.txt" {
		t.Fatalf("original name = %q, want defensive copy to retain note.txt", got)
	}
	if !model.IsSelected(files[0].Path) {
		t.Fatal("selection changed through a returned map")
	}
	if got := model.Filter().Pattern; got != "*.txt" {
		t.Fatalf("filter pattern = %q, want defensive copy to retain *.txt", got)
	}
}

func TestModelFilterLifecycleRestoresUnfilteredListing(t *testing.T) {
	files := []fileinfo.FileInfo{
		{Name: "..", Path: "/", IsDir: true},
		{Name: "docs", Path: "/tmp/docs", IsDir: true},
		{Name: "main.go", Path: "/tmp/main.go"},
		{Name: "notes.md", Path: "/tmp/notes.md"},
	}
	model := New("/tmp", nameSort())
	model.ReplaceDirectory("/tmp", files, fileinfo.StorageInfo{}, false, nameSort())

	matched, total, err := model.ApplyFilter(&config.FilterEntry{Pattern: "*.go ;; source files"})
	if err != nil {
		t.Fatalf("ApplyFilter returned error: %v", err)
	}
	if matched != 3 || total != 4 {
		t.Fatalf("ApplyFilter counts = %d/%d, want 3/4", matched, total)
	}
	if got, want := fileNames(model.Files()), []string{"..", "docs", "main.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("filtered names = %v, want %v", got, want)
	}

	if !model.ClearFilter() {
		t.Fatal("ClearFilter reported no listing replacement")
	}
	if model.Filter() != nil {
		t.Fatal("ClearFilter left an active filter")
	}
	if got, want := fileNames(model.Files()), []string{"..", "docs", "main.go", "notes.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("restored names = %v, want %v", got, want)
	}
}

func TestModelCursorFollowsPathAcrossSort(t *testing.T) {
	model := New("/tmp", nameSort())
	model.ReplaceDirectory("/tmp", []fileinfo.FileInfo{
		{Name: "alpha.txt", Path: "/tmp/alpha.txt", Size: 30},
		{Name: "banana.txt", Path: "/tmp/banana.txt", Size: 10},
		{Name: "cherry.txt", Path: "/tmp/cherry.txt", Size: 20},
	}, fileinfo.StorageInfo{}, false, nameSort())

	model.SetCursorIndex(1)
	if model.cursorIndex != 1 {
		t.Fatalf("cursor cache = %d, want 1", model.cursorIndex)
	}
	model.ApplySort(config.SortConfig{SortBy: "size", SortOrder: "asc"})
	if model.cursorIndex != -1 {
		t.Fatalf("cursor cache after sort = %d, want invalidated", model.cursorIndex)
	}
	if got := model.CursorIndex(); got != 0 {
		t.Fatalf("cursor index after sort = %d, want banana at 0", got)
	}
	if model.cursorIndex != 0 || model.CursorPath() != "/tmp/banana.txt" {
		t.Fatalf("healed cursor = (%d, %q), want (0, banana path)", model.cursorIndex, model.CursorPath())
	}
}

func TestModelRangeSelectionSkipsNonTargets(t *testing.T) {
	model := New("/tmp", nameSort())
	model.ReplaceDirectory("/tmp", []fileinfo.FileInfo{
		{Name: "..", Path: "/", IsDir: true},
		{Name: "a.txt", Path: "/tmp/a.txt"},
		{Name: "gone.txt", Path: "/tmp/gone.txt", Status: fileinfo.StatusDeleted},
		{Name: "docs", Path: "/tmp/docs", IsDir: true},
	}, fileinfo.StorageInfo{}, false, nameSort())

	if !model.MarkRange(0, 3) {
		t.Fatal("first MarkRange reported no change")
	}
	if model.MarkRange(0, 3) {
		t.Fatal("repeated MarkRange reported a change")
	}
	if got, want := fileNames(model.SelectedFiles()), []string{"a.txt", "docs"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("selected files = %v, want %v", got, want)
	}
	if model.IsSelected("/") || model.IsSelected("/tmp/gone.txt") {
		t.Fatalf("selection = %#v, parent and deleted entries must be skipped", model.Selection())
	}
}

func TestModelApplyChangesUpdatesSelectionAndSortOrder(t *testing.T) {
	model := New("/tmp", config.SortConfig{SortBy: "modified", SortOrder: "asc"})
	model.ReplaceDirectory("/tmp", []fileinfo.FileInfo{
		{Name: "a.txt", Path: "/tmp/a.txt", Modified: time.Unix(1, 0)},
		{Name: "b.txt", Path: "/tmp/b.txt", Modified: time.Unix(2, 0)},
	}, fileinfo.StorageInfo{}, false, config.SortConfig{SortBy: "modified", SortOrder: "asc"})
	model.SetSelected("/tmp/a.txt", true)

	if err := model.ApplyChanges(
		[]fileinfo.FileInfo{{Name: "c.txt", Path: "/tmp/c.txt", Modified: time.Unix(0, 0)}},
		[]fileinfo.FileInfo{{Path: "/tmp/a.txt"}},
		[]fileinfo.FileInfo{{Name: "b.txt", Path: "/tmp/b.txt", Modified: time.Unix(3, 0)}},
	); err != nil {
		t.Fatalf("ApplyChanges returned error: %v", err)
	}

	if got, want := fileNames(model.Files()), []string{"c.txt", "a.txt", "b.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("names after changes = %v, want %v", got, want)
	}
	if model.IsSelected("/tmp/a.txt") {
		t.Fatal("deleted entry remained selected")
	}
	files := model.Files()
	if files[1].Status != fileinfo.StatusDeleted || !files[2].Modified.Equal(time.Unix(3, 0)) {
		t.Fatalf("changed files = %#v, want deleted a and modified b", files)
	}
}

func TestModelRenamePreservesRowOrderAndUpdatesReferences(t *testing.T) {
	model := New("/tmp", nameSort())
	model.ReplaceDirectory("/tmp", []fileinfo.FileInfo{
		{Name: "alpha.txt", Path: "/tmp/alpha.txt"},
		{Name: "beta.txt", Path: "/tmp/beta.txt"},
	}, fileinfo.StorageInfo{}, false, nameSort())
	model.SetCursorIndex(0)
	model.SetSelected("/tmp/alpha.txt", true)

	if !model.Rename("/tmp/alpha.txt", "zeta.txt", "/tmp/zeta.txt") {
		t.Fatal("Rename reported no visible update")
	}
	if got, want := fileNames(model.Files()), []string{"zeta.txt", "beta.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("renamed row order = %v, want existing row order %v", got, want)
	}
	if model.CursorPath() != "/tmp/zeta.txt" || !model.IsSelected("/tmp/zeta.txt") || model.IsSelected("/tmp/alpha.txt") {
		t.Fatalf("rename references = cursor %q selection %#v", model.CursorPath(), model.Selection())
	}
	if got := model.OriginalFiles()[0].Path; got != "/tmp/zeta.txt" {
		t.Fatalf("original listing path = %q, want renamed path", got)
	}
}

func nameSort() config.SortConfig {
	return config.SortConfig{SortBy: "name", SortOrder: "asc", DirectoriesFirst: true}
}

func fileNames(files []fileinfo.FileInfo) []string {
	names := make([]string, len(files))
	for i, file := range files {
		names[i] = file.Name
	}
	return names
}
