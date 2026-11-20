package main

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"

	"nmf/internal/fileinfo"
	"nmf/internal/jobs"
)

func TestRenameInitialSelection(t *testing.T) {
	tests := []struct {
		name    string
		target  fileinfo.FileInfo
		wantEnd int
		wantNil bool
	}{
		{name: "simple extension", target: fileinfo.FileInfo{Name: "note.txt"}, wantEnd: 4},
		{name: "last extension", target: fileinfo.FileInfo{Name: "archive.tar.gz"}, wantEnd: 11},
		{name: "multibyte", target: fileinfo.FileInfo{Name: "日本語.txt"}, wantEnd: 3},
		{name: "no extension", target: fileinfo.FileInfo{Name: "README"}, wantEnd: 6},
		{name: "hidden file", target: fileinfo.FileInfo{Name: ".gitignore"}, wantEnd: 10},
		{name: "trailing dot", target: fileinfo.FileInfo{Name: "file."}, wantEnd: 5},
		{name: "directory", target: fileinfo.FileInfo{Name: "dir.name", IsDir: true}, wantEnd: 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renameInitialSelection(tt.target)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("renameInitialSelection() = %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("renameInitialSelection() = nil, want selection")
			}
			if got.Start != 0 || got.End != tt.wantEnd {
				t.Fatalf("renameInitialSelection() = %+v, want start=0 end=%d", got, tt.wantEnd)
			}
		})
	}
}

func TestRenameCurrentFileRecordsCompletedHistory(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "note.txt")
	newPath := filepath.Join(dir, "Note.txt")
	if err := os.WriteFile(oldPath, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := &jobs.Manager{}
	fm := &FileManager{window: app.NewWindow("Rename"), runtime: &ApplicationRuntime{jobManager: m}}
	target := fileinfo.FileInfo{Name: "note.txt", Path: oldPath}
	if !fm.renameCurrentFile(target, target.Name) || len(m.ListRows()) != 0 {
		t.Fatal("same-name rename should succeed without creating history")
	}
	if fm.renameCurrentFile(target, "../invalid") || len(m.ListRows()) != 0 {
		t.Fatal("failed rename should not create history")
	}
	if !fm.renameCurrentFile(target, "Note.txt") {
		t.Fatal("case-only rename did not succeed")
	}
	rows := m.ListRows()
	if len(rows) != 1 || rows[0].Type != jobs.TypeRename || rows[0].Status != jobs.StatusCompleted || rows[0].CompletedAt.IsZero() {
		t.Fatalf("rename rows = %+v", rows)
	}
	detail, ok := m.DisplayDetails(rows[0].ID, 10)
	if !ok || len(detail.Sources) != 1 || detail.Sources[0] != oldPath || detail.RenameTo != newPath {
		t.Fatalf("rename details = %+v, want %q → %q", detail, oldPath, newPath)
	}
	if m.Summary() != (jobs.Summary{}) || m.Cancel(rows[0].ID) {
		t.Fatal("completed rename should not count as active or be cancelable")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "Note.txt" {
		t.Fatalf("directory entries after rename = %v, error = %v", entries, err)
	}
}
