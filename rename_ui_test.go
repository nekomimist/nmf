package main

import (
	"testing"

	"nmf/internal/fileinfo"
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
		{name: "no extension", target: fileinfo.FileInfo{Name: "README"}, wantNil: true},
		{name: "hidden file", target: fileinfo.FileInfo{Name: ".gitignore"}, wantNil: true},
		{name: "trailing dot", target: fileinfo.FileInfo{Name: "file."}, wantNil: true},
		{name: "directory", target: fileinfo.FileInfo{Name: "dir.name", IsDir: true}, wantNil: true},
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
