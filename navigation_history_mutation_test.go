package main

import (
	"path/filepath"
	"testing"
	"time"

	"nmf/internal/config"
	"nmf/internal/jobs"
)

func TestRemoveNavigationHistoryTreeRemovesDescendantsAndPinnedPaths(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	child := filepath.Join(root, "child")
	sibling := root + "-old"
	state := navigationHistoryTestState(root, child, sibling)
	state.NavigationHistory.Pinned = []string{root, child, sibling}

	if !removeNavigationHistoryTree(state, root) {
		t.Fatal("removeNavigationHistoryTree returned false")
	}
	if got := state.NavigationHistory.Entries; len(got) != 1 || got[0] != sibling {
		t.Fatalf("entries = %#v, want only %q", got, sibling)
	}
	if got := state.NavigationHistory.Pinned; len(got) != 1 || got[0] != sibling {
		t.Fatalf("pinned = %#v, want only %q", got, sibling)
	}
	if _, ok := state.NavigationHistory.LastUsed[root]; ok {
		t.Fatal("lastUsed retained removed root")
	}
	if _, ok := state.NavigationHistory.UseCount[child]; ok {
		t.Fatal("useCount retained removed child")
	}
}

func TestRewriteNavigationHistoryTreeRebasesDescendantsAndPinnedPaths(t *testing.T) {
	parent := t.TempDir()
	oldRoot := filepath.Join(parent, "old")
	oldChild := filepath.Join(oldRoot, "child")
	newRoot := filepath.Join(parent, "new")
	newChild := filepath.Join(newRoot, "child")
	unrelated := filepath.Join(parent, "unrelated")
	state := navigationHistoryTestState(oldRoot, oldChild, unrelated)
	state.NavigationHistory.Pinned = []string{oldRoot, oldChild, unrelated}

	if !rewriteNavigationHistoryTree(state, oldRoot, newRoot, 20) {
		t.Fatal("rewriteNavigationHistoryTree returned false")
	}
	if containsNavigationPath(state.NavigationHistory.Entries, oldRoot) || containsNavigationPath(state.NavigationHistory.Entries, oldChild) {
		t.Fatalf("old paths retained after rewrite: %#v", state.NavigationHistory.Entries)
	}
	for _, want := range []string{newRoot, newChild, unrelated} {
		if !containsNavigationPath(state.NavigationHistory.Entries, want) {
			t.Fatalf("entries = %#v, missing %q", state.NavigationHistory.Entries, want)
		}
	}
	if got := state.NavigationHistory.Pinned; !containsNavigationPath(got, newRoot) || !containsNavigationPath(got, newChild) || containsNavigationPath(got, oldRoot) {
		t.Fatalf("pinned = %#v, want rebased paths", got)
	}
	if got := state.NavigationHistory.UseCount[newRoot]; got != 2 {
		t.Fatalf("useCount[%q] = %d, want 2 (old use plus rename use)", newRoot, got)
	}
}

func TestApplyJobNavigationHistoryUsesOnlyTopLevelDirectoryResults(t *testing.T) {
	parent := t.TempDir()
	copyRoot := filepath.Join(parent, "copied")
	extractRoot := filepath.Join(parent, "extracted")
	existingExtractRoot := filepath.Join(parent, "existing")
	fm := &FileManager{
		config: &config.Config{UI: config.UIConfig{NavigationHistory: config.NavigationHistoryConfig{MaxEntries: 20}}},
		state:  navigationHistoryTestState(),
	}

	fm.applyJobNavigationHistory(jobs.JobSnapshot{
		Type: jobs.TypeCopy,
		Results: []jobs.Result{
			{SourceIsDir: true, Destination: copyRoot},
			{SourceIsDir: false, Destination: filepath.Join(parent, "file.txt")},
		},
	})
	fm.applyJobNavigationHistory(jobs.JobSnapshot{
		Type: jobs.TypeExtract,
		Results: []jobs.Result{
			{Destination: extractRoot, DestinationCreated: true},
			{Destination: existingExtractRoot, DestinationCreated: false},
		},
	})

	entries := fm.state.NavigationHistory.Entries
	if !containsNavigationPath(entries, copyRoot) || !containsNavigationPath(entries, extractRoot) {
		t.Fatalf("entries = %#v, missing copied or extracted root", entries)
	}
	if containsNavigationPath(entries, filepath.Join(parent, "file.txt")) || containsNavigationPath(entries, existingExtractRoot) {
		t.Fatalf("entries = %#v, included a non-directory or existing extract root", entries)
	}
}

func navigationHistoryTestState(paths ...string) *config.State {
	lastUsed := make(map[string]time.Time, len(paths))
	useCount := make(map[string]int, len(paths))
	for _, path := range paths {
		lastUsed[path] = time.Unix(1, 0)
		useCount[path] = 1
	}
	return &config.State{NavigationHistory: config.NavigationHistoryState{
		Entries:  append([]string(nil), paths...),
		LastUsed: lastUsed,
		UseCount: useCount,
		Pinned:   make([]string, 0),
	}}
}

func containsNavigationPath(paths []string, want string) bool {
	for _, path := range paths {
		if path == want {
			return true
		}
	}
	return false
}
