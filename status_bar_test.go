package main

import (
	"strings"
	"testing"

	"nmf/internal/fileinfo"
)

func TestCountEntriesExcludingParent(t *testing.T) {
	files := []fileinfo.FileInfo{
		{Name: ".."},
		{Name: "alpha.txt"},
		{Name: "docs", IsDir: true},
	}

	if got := countEntriesExcludingParent(files); got != 2 {
		t.Fatalf("countEntriesExcludingParent got %d, want 2", got)
	}
}

func TestCountMarkedFilesCountsOnlyTrueValues(t *testing.T) {
	selected := map[string]bool{
		"/tmp/a": true,
		"/tmp/b": false,
		"/tmp/c": true,
	}

	if got := countMarkedFiles(selected); got != 2 {
		t.Fatalf("countMarkedFiles got %d, want 2", got)
	}
}

func TestStatusBarTextShowsVisibleAndTotalEntries(t *testing.T) {
	fm := &FileManager{
		files: []fileinfo.FileInfo{
			{Name: ".."},
			{Name: "visible.txt"},
		},
		originalFiles: []fileinfo.FileInfo{
			{Name: ".."},
			{Name: "visible.txt"},
			{Name: "filtered.log"},
		},
		selectedFiles: map[string]bool{
			"/tmp/visible.txt": true,
		},
		storageInfo: fileinfo.StorageInfo{
			Free:  1024,
			Used:  2048,
			Total: 3072,
		},
		storageKnown: true,
	}

	text := fm.statusBarText()
	for _, want := range []string{
		"Mark: 1",
		"Entry: 1/2",
		"Free: 1.0 KB",
		"Used: 2.0 KB",
		"Total: 3.0 KB",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("statusBarText %q does not contain %q", text, want)
		}
	}
}

func TestStatusBarTextUsesDashForUnknownStorage(t *testing.T) {
	fm := &FileManager{
		files:         []fileinfo.FileInfo{{Name: "a.txt"}},
		originalFiles: []fileinfo.FileInfo{{Name: "a.txt"}},
		selectedFiles: map[string]bool{},
	}

	text := fm.statusBarText()
	if !strings.Contains(text, "Free: - | Used: - | Total: -") {
		t.Fatalf("statusBarText %q should use dashes for unknown storage", text)
	}
}
