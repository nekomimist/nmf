package main

import (
	"time"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
)

func canonicalNavigationHistoryPath(p string) string {
	resolved, parsed, err := fileinfo.CanonicalDisplayPath(p)
	if err != nil {
		return p
	}
	if parsed.Scheme == fileinfo.SchemeArchive && parsed.Display != "" {
		return parsed.Display
	}
	return resolved
}

func normalizeNavigationHistory(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	entries := cfg.UI.NavigationHistory.Entries
	lastUsed := cfg.UI.NavigationHistory.LastUsed
	useCount := cfg.UI.NavigationHistory.UseCount
	normalized := make([]string, 0, len(entries))
	seen := make(map[string]bool, len(entries))
	normalizedLastUsed := make(map[string]timeValue, len(lastUsed))
	normalizedUseCount := make(map[string]int, len(useCount))
	changed := false

	for _, entry := range entries {
		canonical := canonicalNavigationHistoryPath(entry)
		if canonical != entry {
			changed = true
		}
		when := lastUsed[entry]
		if previous, ok := normalizedLastUsed[canonical]; !ok || when.After(previous.Time) {
			normalizedLastUsed[canonical] = timeValue{Time: when}
		}
		count := useCount[entry]
		if count <= 0 {
			count = 1
		}
		normalizedUseCount[canonical] += count
		if seen[canonical] {
			changed = true
			continue
		}
		seen[canonical] = true
		normalized = append(normalized, canonical)
	}

	if len(normalized) != len(entries) {
		changed = true
	}
	if !changed {
		return normalizePinnedNavigationHistory(cfg)
	}

	newLastUsed := make(map[string]time.Time, len(normalizedLastUsed))
	for path, when := range normalizedLastUsed {
		newLastUsed[path] = when.Time
	}
	cfg.UI.NavigationHistory.Entries = normalized
	cfg.UI.NavigationHistory.LastUsed = newLastUsed
	cfg.UI.NavigationHistory.UseCount = normalizedUseCount
	normalizePinnedNavigationHistory(cfg)
	return true
}

type timeValue struct {
	Time time.Time
}

func normalizePinnedNavigationHistory(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	pinned := cfg.UI.NavigationHistory.Pinned
	if pinned == nil {
		cfg.UI.NavigationHistory.Pinned = make([]string, 0)
		return true
	}
	normalized := make([]string, 0, len(pinned))
	seen := make(map[string]bool, len(pinned))
	changed := false
	for _, entry := range pinned {
		canonical := canonicalNavigationHistoryPath(entry)
		if canonical != entry {
			changed = true
		}
		if canonical == "" || seen[canonical] {
			changed = true
			continue
		}
		seen[canonical] = true
		normalized = append(normalized, canonical)
	}
	if len(normalized) != len(pinned) {
		changed = true
	}
	if changed {
		cfg.UI.NavigationHistory.Pinned = normalized
	}
	return changed
}
