package ui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/widget"

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

func TestNavigationHistoryHorizontalScrollState(t *testing.T) {
	dialog := NewNavigationHistoryDialog(
		[]string{"/tmp/very/long/path"},
		map[string]time.Time{},
		nil,
		func(string, ...interface{}) {},
		search.NewPlainProvider(),
	)
	dialog.listScroller = newDialogListScroller(widget.NewLabel(""), 300, 100, 20)

	dialog.ScrollSelectedRight()
	if !dialog.scrollRight {
		t.Fatal("right scroll should enable follow mode")
	}

	dialog.ResetHorizontalScroll()
	if dialog.scrollRight {
		t.Fatal("left scroll should disable follow mode")
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
