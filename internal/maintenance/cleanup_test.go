package maintenance

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
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

func TestPlanSkipsNetworkRemovableAndUnavailable(t *testing.T) {
	cfg := testState()
	cfg.CursorMemory.Entries["/network"] = "file.txt"
	cfg.CursorMemory.Entries["/removable"] = "file.txt"
	cfg.CursorMemory.Entries["/unavailable"] = "file.txt"
	cfg.CursorMemory.Entries["/local"] = "file.txt"

	result := Plan(cfg, DefaultOptions(), func(path string) (PathClass, error) {
		switch path {
		case "/network":
			return PathClass{Network: true}, nil
		case "/removable":
			return PathClass{Removable: true}, nil
		case "/unavailable":
			return PathClass{Unavailable: true}, nil
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
	if result.SkippedUnavailable != 1 {
		t.Fatalf("SkippedUnavailable = %d, want 1", result.SkippedUnavailable)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].Path != "/local" {
		t.Fatalf("candidates = %#v, want only /local", result.Candidates)
	}
}

func TestPlanCanIncludeUnavailableVolumes(t *testing.T) {
	cfg := testState()
	cfg.CursorMemory.Entries["X:\\missing"] = "file.txt"
	options := DefaultOptions()
	options.SkipUnavailableVolumes = false

	result := Plan(cfg, options, func(string) (PathClass, error) {
		return PathClass{Unavailable: true}, nil
	}, func(string) error {
		return fmt.Errorf("volume is unavailable")
	})

	if result.SkippedUnavailable != 0 {
		t.Fatalf("SkippedUnavailable = %d, want 0", result.SkippedUnavailable)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].Path != "X:\\missing" {
		t.Fatalf("candidates = %#v, want unavailable path", result.Candidates)
	}
}

func TestPlanSkipsArchiveStoredOnNetworkPath(t *testing.T) {
	cfg := testState()
	path := "smb://server/share/backup.zip!/inside"
	cfg.CursorMemory.Entries[path] = "file.txt"
	accessibleCalled := false

	result := Plan(cfg, DefaultOptions(), nil, func(string) error {
		accessibleCalled = true
		return fmt.Errorf("must not access a skipped network archive")
	})

	if accessibleCalled {
		t.Fatal("network archive was accessed despite SkipNetworkPaths")
	}
	if result.SkippedNetwork != 1 {
		t.Fatalf("SkippedNetwork = %d, want 1", result.SkippedNetwork)
	}
	if len(result.Candidates) != 0 {
		t.Fatalf("candidates = %#v, want none", result.Candidates)
	}
}

func TestDefaultAccessibleChecksOnlyArchiveBackingFile(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "encrypted.zip")
	if err := os.WriteFile(archivePath, []byte("not readable archive contents"), 0600); err != nil {
		t.Fatalf("write archive backing file: %v", err)
	}

	if err := DefaultAccessible(fileinfo.ArchiveRootPath(archivePath)); err != nil {
		t.Fatalf("existing archive backing file should be retained without opening it: %v", err)
	}
}

func TestDefaultAccessibleRejectsMissingArchiveBackingFile(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "missing.zip")

	if err := DefaultAccessible(fileinfo.ArchiveRootPath(archivePath)); err == nil {
		t.Fatal("missing archive backing file should be inaccessible")
	}
}

func TestPlanTargetsStopsAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	targets := Targets{CursorMemory: []string{"/first", "/second"}}
	firstStarted := make(chan struct{})

	done := make(chan error, 1)
	go func() {
		_, err := PlanTargets(ctx, targets, DefaultOptions(), classifyNone, func(ctx context.Context, _ string) error {
			close(firstStarted)
			<-ctx.Done()
			return ctx.Err()
		})
		done <- err
	}()

	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("first accessibility check did not start")
	}
	cancel()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("PlanTargets error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("PlanTargets did not stop after cancellation")
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
