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
		nil,
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
		nil,
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
		nil,
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

func TestNavigationHistoryFilterMatchesAllQueryTokens(t *testing.T) {
	dialog := NewNavigationHistoryDialog(
		[]string{"/tmp/project/archive", "/tmp/project/docs", "/tmp/archive/logs"},
		nil,
		nil,
		nil,
		map[string]time.Time{},
		nil,
		func(string, ...interface{}) {},
		search.NewPlainProvider(),
	)

	dialog.updateFilteredPaths("archive project")

	if len(dialog.filteredPaths) != 1 || dialog.filteredPaths[0] != "/tmp/project/archive" {
		t.Fatalf("filtered paths = %#v, want only project archive path", dialog.filteredPaths)
	}
}

func TestNavigationHistoryFilterKeepsOpenPathMetadata(t *testing.T) {
	dialog := NewNavigationHistoryDialog(
		[]string{"/tmp/open", "/tmp/history"},
		map[string]bool{"/tmp/open": true},
		nil,
		nil,
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
		nil,
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

func TestNavigationHistoryDisplaysPinnedMarkerWithoutChangingSelection(t *testing.T) {
	dialog := NewNavigationHistoryDialog(
		[]string{"/tmp/pinned"},
		nil,
		map[string]bool{"/tmp/pinned": true},
		nil,
		map[string]time.Time{},
		nil,
		func(string, ...interface{}) {},
		search.NewPlainProvider(),
	)

	if got := dialog.displayPath("/tmp/pinned"); got != "* /tmp/pinned" {
		t.Fatalf("display path = %q, want pinned marker", got)
	}
	if dialog.selectedPath != "/tmp/pinned" {
		t.Fatalf("selected path = %q, want raw path", dialog.selectedPath)
	}
}

func TestNavigationHistoryUnpinSelectedPathRemovesPinnedOnlyPath(t *testing.T) {
	dialog := NewNavigationHistoryDialog(
		[]string{"/tmp/pinned", "/tmp/history"},
		nil,
		map[string]bool{"/tmp/pinned": true},
		map[string]bool{"/tmp/pinned": true},
		map[string]time.Time{},
		nil,
		func(string, ...interface{}) {},
		search.NewPlainProvider(),
	)
	var unpinned string
	dialog.unpinCallback = func(path string) bool {
		unpinned = path
		return true
	}

	dialog.UnpinSelectedPath()

	if unpinned != "/tmp/pinned" {
		t.Fatalf("unpinned path = %q, want /tmp/pinned", unpinned)
	}
	if dialog.pinnedPaths["/tmp/pinned"] {
		t.Fatal("path should no longer be marked pinned")
	}
	if len(dialog.allPaths) != 1 || dialog.allPaths[0] != "/tmp/history" {
		t.Fatalf("all paths = %#v, want only history path", dialog.allPaths)
	}
}

func TestNavigationHistoryUnpinSelectedPathKeepsHistoryBackedPath(t *testing.T) {
	dialog := NewNavigationHistoryDialog(
		[]string{"/tmp/pinned"},
		nil,
		map[string]bool{"/tmp/pinned": true},
		nil,
		map[string]time.Time{},
		nil,
		func(string, ...interface{}) {},
		search.NewPlainProvider(),
	)
	dialog.unpinCallback = func(path string) bool { return true }

	dialog.UnpinSelectedPath()

	if len(dialog.allPaths) != 1 || dialog.allPaths[0] != "/tmp/pinned" {
		t.Fatalf("all paths = %#v, want history-backed path retained", dialog.allPaths)
	}
	if dialog.displayPath("/tmp/pinned") != "/tmp/pinned" {
		t.Fatalf("display path = %q, want marker removed", dialog.displayPath("/tmp/pinned"))
	}
}
