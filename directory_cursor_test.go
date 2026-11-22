package main

import (
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
)

func TestDirectoryCursorPriorityAndMemoryUsage(t *testing.T) {
	path := t.TempDir()
	child := filepath.Join(path, "child")
	preferred := filepath.Join(path, "preferred")
	neighbor := filepath.Join(path, "neighbor")
	missing := filepath.Join(path, "missing")
	files := []fileinfo.FileInfo{
		{Name: "..", Path: fileinfo.ParentPath(path)},
		{Name: "child", Path: child},
		{Name: "saved", Path: filepath.Join(path, "saved")},
		{Name: "preferred", Path: preferred},
		{Name: "neighbor", Path: neighbor},
	}
	for _, tt := range []struct {
		name          string
		previous      string
		preferred     string
		saved         string
		neighbors     []string
		empty         bool
		want          int
		memoryTouched bool
	}{
		{name: "preferred before parent", previous: child, preferred: preferred, saved: "saved", want: 3},
		{name: "preferred before saved", previous: path, preferred: preferred, saved: "saved", want: 3},
		{name: "parent before saved and neighbors", previous: child, saved: "saved", neighbors: []string{neighbor}, want: 1},
		{name: "missing preferred falls back to parent", previous: child, preferred: missing, saved: "saved", want: 1},
		{name: "missing parent uses first row", previous: missing, saved: "saved", neighbors: []string{neighbor}, want: 0},
		{name: "saved before neighbors", previous: path, saved: "saved", neighbors: []string{neighbor}, want: 2, memoryTouched: true},
		{name: "missing preferred falls back to saved", previous: path, preferred: missing, saved: "saved", want: 2, memoryTouched: true},
		{name: "neighbors retain candidate order", previous: path, saved: "deleted", neighbors: []string{missing, neighbor, preferred}, want: 4, memoryTouched: true},
		{name: "missing saved and neighbors use first row", previous: path, saved: "deleted", neighbors: []string{missing}, want: 0, memoryTouched: true},
		{name: "no saved entry uses neighbor", previous: path, neighbors: []string{neighbor}, want: 4},
		{name: "empty listing ignores memory", previous: path, saved: "saved", preferred: preferred, empty: true, want: -1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			old := time.Unix(1, 0)
			state := &config.State{CursorMemory: config.CursorMemoryState{
				Entries:  map[string]string{},
				LastUsed: map[string]time.Time{path: old},
			}}
			if tt.saved != "" {
				state.CursorMemory.Entries[path] = tt.saved
			}
			fm := &FileManager{state: state}
			listing := files
			if tt.empty {
				listing = nil
			}
			if got := fm.directoryCursorIndex(listing, path, tt.previous, tt.neighbors, tt.preferred); got != tt.want {
				t.Fatalf("cursor index = %d, want %d", got, tt.want)
			}
			if touched := !state.CursorMemory.LastUsed[path].Equal(old); touched != tt.memoryTouched {
				t.Fatalf("cursor memory touched = %t, want %t", touched, tt.memoryTouched)
			}
		})
	}
}

func TestSavingExistingCursorAtCapacityAndLoweringLimit(t *testing.T) {
	// Consecutive saves have the same timestamp until the bubble's clock advances.
	synctest.Test(t, testSavingCursorsAtCapacity)
}

func testSavingCursorsAtCapacity(t *testing.T) {
	state := &config.State{CursorMemory: config.CursorMemoryState{
		Entries:  map[string]string{"/a": "1", "/b": "2", "/c": "3"},
		LastUsed: map[string]time.Time{"/b": time.Unix(2, 0), "/c": time.Unix(3, 0)},
	}}
	fm := &FileManager{state: state, config: config.Default(), browser: newTestBrowser(testBrowserOptions{
		files: []fileinfo.FileInfo{{Name: "new", Path: "/c/new"}}, cursorPath: "/c/new",
	})}
	fm.config.UI.CursorMemory.MaxEntries = 3
	fm.SaveCursorPosition("/c")
	if len(state.CursorMemory.Entries) != 3 || state.CursorMemory.Entries["/a"] != "1" {
		t.Fatalf("evicted on update: %v", state.CursorMemory.Entries)
	}
	fm.config.UI.CursorMemory.MaxEntries = 1
	fm.SaveCursorPosition("/d")
	if len(state.CursorMemory.Entries) != 1 || state.CursorMemory.Entries["/d"] != "new" {
		t.Fatalf("failed to enforce reduced limit: %v", state.CursorMemory.Entries)
	}
	fm.SaveCursorPosition("/e")
	if len(state.CursorMemory.Entries) != 1 || state.CursorMemory.Entries["/e"] != "new" {
		t.Fatalf("new entry evicted at fixed limit: %v", state.CursorMemory.Entries)
	}
}
