package maintenance

import (
	"fmt"
	"testing"
	"time"

	"nmf/internal/config"
)

func TestPlanFindsInaccessibleEntries(t *testing.T) {
	cfg := testState()
	cfg.CursorMemory.Entries["/missing-cursor"] = "file.txt"
	cfg.CursorMemory.LastUsed["/missing-cursor"] = time.Now()
	cfg.CursorMemory.Entries["/ok-cursor"] = "file.txt"
	cfg.CursorMemory.LastUsed["/ok-cursor"] = time.Now()
	cfg.NavigationHistory.Entries = []string{"/ok-history", "/missing-history"}
	cfg.NavigationHistory.LastUsed["/ok-history"] = time.Now()
	cfg.NavigationHistory.LastUsed["/missing-history"] = time.Now()
	cfg.NavigationHistory.UseCount["/ok-history"] = 1
	cfg.NavigationHistory.UseCount["/missing-history"] = 1

	result := Plan(cfg, DefaultOptions(), classifyNone, func(path string) error {
		if path == "/missing-cursor" || path == "/missing-history" {
			return fmt.Errorf("not found")
		}
		return nil
	})

	if len(result.Candidates) != 2 {
		t.Fatalf("candidates = %d, want 2: %#v", len(result.Candidates), result.Candidates)
	}
	if result.ScannedCursorMemory != 2 {
		t.Fatalf("ScannedCursorMemory = %d, want 2", result.ScannedCursorMemory)
	}
	if result.ScannedNavigationHistory != 2 {
		t.Fatalf("ScannedNavigationHistory = %d, want 2", result.ScannedNavigationHistory)
	}
}

func TestPlanRespectsTaskSelection(t *testing.T) {
	cfg := testState()
	cfg.CursorMemory.Entries["/missing-cursor"] = "file.txt"
	cfg.NavigationHistory.Entries = []string{"/missing-history"}

	options := DefaultOptions()
	options.CleanCursorMemory = false
	result := Plan(cfg, options, classifyNone, func(path string) error {
		return fmt.Errorf("not found")
	})

	if len(result.Candidates) != 1 {
		t.Fatalf("candidates = %d, want 1", len(result.Candidates))
	}
	if result.Candidates[0].Task != TaskNavigationHistory {
		t.Fatalf("candidate task = %s, want %s", result.Candidates[0].Task, TaskNavigationHistory)
	}
}

func TestPlanSkipsNetworkAndRemovable(t *testing.T) {
	cfg := testState()
	cfg.CursorMemory.Entries["/network"] = "file.txt"
	cfg.CursorMemory.Entries["/removable"] = "file.txt"
	cfg.CursorMemory.Entries["/local"] = "file.txt"

	result := Plan(cfg, DefaultOptions(), func(path string) (PathClass, error) {
		switch path {
		case "/network":
			return PathClass{Network: true}, nil
		case "/removable":
			return PathClass{Removable: true}, nil
		default:
			return PathClass{}, nil
		}
	}, func(path string) error {
		return fmt.Errorf("not found")
	})

	if result.SkippedNetwork != 1 {
		t.Fatalf("SkippedNetwork = %d, want 1", result.SkippedNetwork)
	}
	if result.SkippedRemovable != 1 {
		t.Fatalf("SkippedRemovable = %d, want 1", result.SkippedRemovable)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].Path != "/local" {
		t.Fatalf("candidates = %#v, want only /local", result.Candidates)
	}
}

func TestApplyRemovesOnlyLatestCandidates(t *testing.T) {
	cfg := testState()
	cfg.CursorMemory.Entries["/remove-cursor"] = "file.txt"
	cfg.CursorMemory.LastUsed["/remove-cursor"] = time.Now()
	cfg.CursorMemory.Entries["/keep-cursor"] = "file.txt"
	cfg.CursorMemory.LastUsed["/keep-cursor"] = time.Now()
	cfg.NavigationHistory.Entries = []string{"/remove-history", "/keep-history"}
	cfg.NavigationHistory.LastUsed["/remove-history"] = time.Now()
	cfg.NavigationHistory.LastUsed["/keep-history"] = time.Now()
	cfg.NavigationHistory.UseCount["/remove-history"] = 1
	cfg.NavigationHistory.UseCount["/keep-history"] = 1

	removed := Apply(cfg, Result{Candidates: []Candidate{
		{Task: TaskCursorMemory, Path: "/remove-cursor"},
		{Task: TaskNavigationHistory, Path: "/remove-history"},
	}})

	if removed != 2 {
		t.Fatalf("removed = %d, want 2", removed)
	}
	if _, exists := cfg.CursorMemory.Entries["/remove-cursor"]; exists {
		t.Fatal("cursor memory entry was not removed")
	}
	if _, exists := cfg.CursorMemory.LastUsed["/remove-cursor"]; exists {
		t.Fatal("cursor memory lastUsed was not removed")
	}
	if _, exists := cfg.CursorMemory.Entries["/keep-cursor"]; !exists {
		t.Fatal("cursor memory keep entry was removed")
	}
	if got := cfg.NavigationHistory.Entries; len(got) != 1 || got[0] != "/keep-history" {
		t.Fatalf("history entries = %#v, want only /keep-history", got)
	}
	if _, exists := cfg.NavigationHistory.LastUsed["/remove-history"]; exists {
		t.Fatal("history lastUsed was not removed")
	}
	if _, exists := cfg.NavigationHistory.UseCount["/remove-history"]; exists {
		t.Fatal("history useCount was not removed")
	}
}

func classifyNone(path string) (PathClass, error) {
	return PathClass{}, nil
}

func testState() *config.State {
	return &config.State{
		CursorMemory: config.CursorMemoryState{
			Entries:  make(map[string]string),
			LastUsed: make(map[string]time.Time),
		},
		NavigationHistory: config.NavigationHistoryState{
			Entries:  make([]string, 0),
			LastUsed: make(map[string]time.Time),
			UseCount: make(map[string]int),
		},
	}
}
