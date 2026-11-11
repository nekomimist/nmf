package main

import (
	"path/filepath"
	"reflect"
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

func TestRewriteNavigationHistoryMergesDestinationStatistics(t *testing.T) {
	parent := t.TempDir()
	oldRoot, newRoot := filepath.Join(parent, "old"), filepath.Join(parent, "new")
	oldChild, newChild := filepath.Join(oldRoot, "child"), filepath.Join(newRoot, "child")
	older, newer := time.Unix(100, 0), time.Unix(200, 0)
	for _, destinationFirst := range []bool{false, true} {
		name := "source first"
		entries := []string{oldRoot, oldChild, newRoot, newChild}
		if destinationFirst {
			name = "destination first"
			entries = []string{newRoot, newChild, oldRoot, oldChild}
		}
		t.Run(name, func(t *testing.T) {
			state := navigationHistoryTestState(entries...)
			state.NavigationHistory.UseCount = map[string]int{oldRoot: 3, newRoot: 4, oldChild: 2, newChild: 5}
			state.NavigationHistory.LastUsed = map[string]time.Time{oldRoot: older, newRoot: newer, oldChild: older, newChild: newer}
			state.NavigationHistory.Pinned = []string{oldChild, newChild, oldRoot, newRoot}
			if !rewriteNavigationHistoryTree(state, oldRoot, newRoot, 20) {
				t.Fatal("history was not rewritten")
			}
			history := state.NavigationHistory
			if len(history.Entries) != 2 || !containsNavigationPath(history.Entries, newRoot) || !containsNavigationPath(history.Entries, newChild) {
				t.Fatalf("merged entries = %v", history.Entries)
			}
			if want := map[string]int{newRoot: 8, newChild: 7}; !reflect.DeepEqual(history.UseCount, want) {
				t.Fatalf("merged counts = %v, want %v", history.UseCount, want)
			}
			if len(history.LastUsed) != 2 || !history.LastUsed[newChild].Equal(newer) || !history.LastUsed[newRoot].After(newer) {
				t.Fatalf("merged timestamps = %v", history.LastUsed)
			}
			if want := []string{newChild, newRoot}; !reflect.DeepEqual(history.Pinned, want) {
				t.Fatalf("merged pins = %v, want %v", history.Pinned, want)
			}
		})
	}
}

func TestRewriteNavigationHistoryPreservesLegacyStatistics(t *testing.T) {
	parent := t.TempDir()
	oldRoot, newRoot := filepath.Join(parent, "old"), filepath.Join(parent, "new")
	oldChild, newChild := filepath.Join(oldRoot, "child"), filepath.Join(newRoot, "child")
	for _, tt := range []struct {
		name                           string
		oldCount, canonicalCount, want int
	}{
		{"canonical fallback", 0, 5, 5},
		{"missing statistics", 0, 0, 1},
		{"negative count", -2, 5, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			state := navigationHistoryTestState(oldChild)
			state.NavigationHistory.UseCount = map[string]int{oldChild: tt.oldCount, newChild: tt.canonicalCount}
			newer := time.Unix(200, 0)
			state.NavigationHistory.LastUsed[newChild] = newer
			rewriteNavigationHistoryTree(state, oldRoot, newRoot, 20)
			if got := state.NavigationHistory.UseCount[newChild]; got != tt.want {
				t.Fatalf("rebased count = %d, want %d", got, tt.want)
			}
			if got := state.NavigationHistory.LastUsed[newChild]; !got.Equal(newer) {
				t.Fatalf("rebased timestamp = %v, want %v", got, newer)
			}
		})
	}
}

func TestApplyJobNavigationHistoryNotifiesOncePerBatch(t *testing.T) {
	parent := t.TempDir()
	first, second := filepath.Join(parent, "first"), filepath.Join(parent, "second")
	target := filepath.Join(parent, "target")
	for _, jobType := range []jobs.Type{jobs.TypeCopy, jobs.TypeExtract, jobs.TypeMove, jobs.TypeDelete} {
		t.Run(string(jobType), func(t *testing.T) {
			hub := newNavigationHistoryEventHub()
			fm := &FileManager{
				runtime: &ApplicationRuntime{navigationHistoryEvents: hub},
				state:   navigationHistoryTestState(first, second),
			}
			var notifications []string
			hub.subscribe(func(path string) { notifications = append(notifications, path) })
			fm.applyJobNavigationHistory(jobs.JobSnapshot{Type: jobType, Results: []jobs.Result{
				{Source: first, Destination: target, SourceIsDir: true, DestinationCreated: true},
				{Source: second, Destination: target + "2", SourceIsDir: true, DestinationCreated: true},
				{Source: "ignored", Destination: "ignored"},
			}})
			preferred := canonicalNavigationHistoryPath(target + "2")
			if jobType == jobs.TypeDelete {
				preferred = ""
			}
			if want := []string{preferred}; !reflect.DeepEqual(notifications, want) {
				t.Fatalf("notifications = %v, want %v", notifications, want)
			}
			notifications = nil
			fm.applyJobNavigationHistory(jobs.JobSnapshot{Type: jobType, Results: []jobs.Result{{Source: "ignored"}}})
			if len(notifications) != 0 {
				t.Fatalf("ignored results notified listeners: %v", notifications)
			}
		})
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
