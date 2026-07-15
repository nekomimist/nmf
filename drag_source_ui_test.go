package main

import (
	"path/filepath"
	"strings"
	"testing"

	"nmf/internal/fileinfo"
)

func TestCollectDragSourcePathsUsesSelectionBeforeDraggedItem(t *testing.T) {
	tmp := t.TempDir()
	selectedPath := filepath.Join(tmp, "selected.txt")
	draggedPath := filepath.Join(tmp, "dragged.txt")
	if err := writeTestFile(selectedPath); err != nil {
		t.Fatal(err)
	}
	if err := writeTestFile(draggedPath); err != nil {
		t.Fatal(err)
	}

	fm := &FileManager{
		files: []fileinfo.FileInfo{
			{Name: "selected.txt", Path: selectedPath},
			{Name: "dragged.txt", Path: draggedPath},
		},
		selectedFiles: map[string]bool{selectedPath: true},
	}

	paths, err := fm.collectDragSourcePaths(fileinfo.FileInfo{Name: "dragged.txt", Path: draggedPath})
	if err != nil {
		t.Fatalf("collectDragSourcePaths returned error: %v", err)
	}
	if len(paths) != 1 || paths[0] != selectedPath {
		t.Fatalf("paths = %#v, want selected path %q", paths, selectedPath)
	}
}

func TestCollectDragSourcePathsFallsBackToDraggedItem(t *testing.T) {
	tmp := t.TempDir()
	draggedPath := filepath.Join(tmp, "dragged.txt")
	if err := writeTestFile(draggedPath); err != nil {
		t.Fatal(err)
	}

	fm := &FileManager{selectedFiles: map[string]bool{}}
	paths, err := fm.collectDragSourcePaths(fileinfo.FileInfo{Name: "dragged.txt", Path: draggedPath})
	if err != nil {
		t.Fatalf("collectDragSourcePaths returned error: %v", err)
	}
	if len(paths) != 1 || paths[0] != draggedPath {
		t.Fatalf("paths = %#v, want dragged path %q", paths, draggedPath)
	}
}

func TestDragSourceNativePathRejectsInvalidSources(t *testing.T) {
	cases := []struct {
		name string
		fi   fileinfo.FileInfo
		want string
	}{
		{
			name: "parent",
			fi:   fileinfo.FileInfo{Name: "..", Path: "/tmp"},
			want: "invalid drag source",
		},
		{
			name: "deleted",
			fi:   fileinfo.FileInfo{Name: "gone.txt", Path: "/tmp/gone.txt", Status: fileinfo.StatusDeleted},
			want: "deleted item",
		},
		{
			name: "archive",
			fi:   fileinfo.FileInfo{Name: "readme.txt", Path: "/tmp/archive.zip!/readme.txt"},
			want: "archive item",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := dragSourceNativePath(tc.fi)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}
