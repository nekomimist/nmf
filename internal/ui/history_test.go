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
		nil,
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
		nil,
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
		nil,
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

func TestNavigationHistoryFilterKeepsOpenPathMetadata(t *testing.T) {
	dialog := NewNavigationHistoryDialog(
		[]string{"/tmp/open", "/tmp/history"},
		map[string]bool{"/tmp/open": true},
		map[string]time.Time{},
		nil,
		func(string, ...interface{}) {},
		search.NewPlainProvider(),
	)

	dialog.updateFilteredPaths("open")

	if len(dialog.filteredPaths) != 1 || dialog.filteredPaths[0] != "/tmp/open" {
		t.Fatalf("filtered paths = %#v, want only /tmp/open", dialog.filteredPaths)
	}
	if !dialog.openPaths["/tmp/open"] {
		t.Fatal("open path metadata was not retained")
	}
}

func TestNavigationHistoryReportsSelectedPathChanges(t *testing.T) {
	dialog := NewNavigationHistoryDialog(
		[]string{"/tmp/one", "/tmp/two"},
		nil,
		map[string]time.Time{},
		nil,
		func(string, ...interface{}) {},
		search.NewPlainProvider(),
	)
	var got []string
	dialog.SetOnSelectedPathChanged(func(path string) {
		got = append(got, path)
	})

	dialog.MoveDown()
	dialog.updateFilteredPaths("missing")

	want := []string{"/tmp/one", "/tmp/two", ""}
	if len(got) != len(want) {
		t.Fatalf("selected path changes = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("selected path changes = %#v, want %#v", got, want)
		}
	}
}
