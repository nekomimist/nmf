package main

import (
	"testing"
	"time"

	"nmf/internal/config"
)

func TestCanonicalNavigationHistoryPathNormalizesWSLAliases(t *testing.T) {
	got := canonicalNavigationHistoryPath(`\\wsl$\Ubuntu\home\neko`)
	want := "smb://wsl.localhost/Ubuntu/home/neko"
	if got != want {
		t.Fatalf("canonicalNavigationHistoryPath = %q, want %q", got, want)
	}
}

func TestNormalizeNavigationHistoryDeduplicatesCanonicalPaths(t *testing.T) {
	older := time.Now().Add(-time.Hour)
	newer := time.Now()
	cfg := &config.Config{
		UI: config.UIConfig{
			NavigationHistory: config.NavigationHistoryConfig{
				Entries: []string{
					`\\wsl$\Ubuntu\home\neko`,
					"smb://wsl.localhost/Ubuntu/home/neko",
				},
				LastUsed: map[string]time.Time{
					`\\wsl$\Ubuntu\home\neko`:              older,
					"smb://wsl.localhost/Ubuntu/home/neko": newer,
				},
			},
		},
	}

	if !normalizeNavigationHistory(cfg) {
		t.Fatal("normalizeNavigationHistory should report changes")
	}
	if len(cfg.UI.NavigationHistory.Entries) != 1 {
		t.Fatalf("entries = %#v, want one deduplicated entry", cfg.UI.NavigationHistory.Entries)
	}
	path := cfg.UI.NavigationHistory.Entries[0]
	if path != "smb://wsl.localhost/Ubuntu/home/neko" {
		t.Fatalf("entry = %q, want canonical WSL path", path)
	}
	if !cfg.UI.NavigationHistory.LastUsed[path].Equal(newer) {
		t.Fatalf("lastUsed = %v, want newest %v", cfg.UI.NavigationHistory.LastUsed[path], newer)
	}
}
