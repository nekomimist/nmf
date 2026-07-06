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

func normalizeNavigationHistory(state *config.State) bool {
	if state == nil {
		return false
	}
	entries := state.NavigationHistory.Entries
	lastUsed := state.NavigationHistory.LastUsed
	useCount := state.NavigationHistory.UseCount
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
		return normalizePinnedNavigationHistory(state)
	}

	newLastUsed := make(map[string]time.Time, len(normalizedLastUsed))
	for path, when := range normalizedLastUsed {
		newLastUsed[path] = when.Time
	}
	state.NavigationHistory.Entries = normalized
	state.NavigationHistory.LastUsed = newLastUsed
	state.NavigationHistory.UseCount = normalizedUseCount
	normalizePinnedNavigationHistory(state)
	return true
}

type timeValue struct {
	Time time.Time
}

func normalizePinnedNavigationHistory(state *config.State) bool {
	if state == nil {
		return false
	}
	pinned := state.NavigationHistory.Pinned
	if pinned == nil {
		state.NavigationHistory.Pinned = make([]string, 0)
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
		state.NavigationHistory.Pinned = normalized
	}
	return changed
}
