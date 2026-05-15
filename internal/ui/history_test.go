package ui

import (
	"testing"
	"time"

	"nmf/internal/search"
)

func TestNavigationHistoryBackspaceRemovesUTF8Rune(t *testing.T) {
	dialog := NewNavigationHistoryDialog(
		[]string{"/tmp"},
		map[string]time.Time{},
		nil,
		func(string, ...interface{}) {},
		search.NewPlainProvider(),
	)
	dialog.searchEntry.SetText("日本語")

	dialog.BackspaceSearch()

	if got := dialog.GetSearchText(); got != "日本" {
		t.Fatalf("search text got %q, want %q", got, "日本")
	}
}

func TestNavigationHistoryFilterUsesMigemoMatcher(t *testing.T) {
	dialog := NewNavigationHistoryDialog(
		[]string{"/tmp/日本語", "/tmp/alpha"},
		map[string]time.Time{},
		nil,
		func(string, ...interface{}) {},
		search.NewProvider(func(string, ...interface{}) {}),
	)

	dialog.updateFilteredPaths("nihongo")

	if len(dialog.filteredPaths) != 1 || dialog.filteredPaths[0] != "/tmp/日本語" {
		t.Fatalf("filtered paths = %#v, want only Japanese path", dialog.filteredPaths)
	}
}
