package main

import (
	"testing"

	"nmf/internal/config"
)

func TestCreateDirectoryAddsNewPathToNavigationHistory(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{}
	cfg.UI.NavigationHistory = config.NavigationHistoryConfig{
		MaxEntries: 100,
		Entries:    []string{},
	}
	fm := &FileManager{
		currentPath: tmpDir,
		config:      cfg,
	}

	if !fm.CreateDirectory("created") {
		t.Fatal("CreateDirectory returned false")
	}

	history := cfg.GetNavigationHistory()
	if len(history) == 0 {
		t.Fatal("navigation history is empty")
	}
	if got, want := history[0], tmpDir+"/created"; got != want {
		t.Fatalf("history[0] = %q, want %q", got, want)
	}
}
