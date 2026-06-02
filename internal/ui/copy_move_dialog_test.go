package ui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/widget"

	"nmf/internal/search"
)

func TestCopyMoveFilterKeepsOpenDestinationMetadata(t *testing.T) {
	dialog := NewCopyMoveDialog(
		OpCopy,
		[]string{"file.txt"},
		[]DestinationCandidate{
			{Path: "/tmp/open", OpenInOtherWindow: true},
			{Path: "/tmp/history"},
		},
		map[string]time.Time{},
		false,
		nil,
		func(string, ...interface{}) {},
	)

	dialog.updateFiltered("open")

	if len(dialog.filteredDest) != 1 || dialog.filteredDest[0].Path != "/tmp/open" {
		t.Fatalf("filtered destinations = %#v, want only /tmp/open", dialog.filteredDest)
	}
	if !dialog.openDest["/tmp/open"] {
		t.Fatal("open destination metadata was not retained")
	}

	dialog.updateFiltered("")
	if len(dialog.allDest) != 2 {
		t.Fatalf("all destinations length = %d, want 2", len(dialog.allDest))
	}
}

func TestCopyMoveFilterMatchesAllQueryTokens(t *testing.T) {
	dialog := NewCopyMoveDialog(
		OpCopy,
		[]string{"file.txt"},
		[]DestinationCandidate{
			{Path: "/tmp/project/archive"},
			{Path: "/tmp/project/docs"},
			{Path: "/tmp/archive/logs"},
		},
		map[string]time.Time{},
		false,
		nil,
		func(string, ...interface{}) {},
		search.NewPlainProvider(),
	)

	dialog.updateFiltered("archive project")

	if len(dialog.filteredDest) != 1 || dialog.filteredDest[0].Path != "/tmp/project/archive" {
		t.Fatalf("filtered destinations = %#v, want only project archive path", dialog.filteredDest)
	}
}

func TestCopyMoveFilterUsesMigemoMatcher(t *testing.T) {
	dialog := NewCopyMoveDialog(
		OpCopy,
		[]string{"file.txt"},
		[]DestinationCandidate{
			{Path: "/tmp/日本語"},
			{Path: "/tmp/alpha"},
		},
		map[string]time.Time{},
		false,
		nil,
		func(string, ...interface{}) {},
		search.NewProvider(func(string, ...interface{}) {}),
	)

	dialog.updateFiltered("nihongo tmp")

	if len(dialog.filteredDest) != 1 || dialog.filteredDest[0].Path != "/tmp/日本語" {
		t.Fatalf("filtered destinations = %#v, want only Japanese destination", dialog.filteredDest)
	}
}

func TestCopyMoveHorizontalScrollState(t *testing.T) {
	dialog := NewCopyMoveDialog(
		OpCopy,
		[]string{"file.txt"},
		[]DestinationCandidate{{Path: "/tmp/very/long/path"}},
		map[string]time.Time{},
		false,
		nil,
		func(string, ...interface{}) {},
	)
	dialog.destScroll = newDialogListScroller(widget.NewLabel(""), 300, 100, 20)

	dialog.ScrollSelectedRight()
	if !dialog.scrollRight {
		t.Fatal("right scroll should enable follow mode")
	}

	dialog.ResetHorizontalScroll()
	if dialog.scrollRight {
		t.Fatal("left scroll should disable follow mode")
	}
}

func TestCopyMoveReportsSelectedPathChanges(t *testing.T) {
	dialog := NewCopyMoveDialog(
		OpCopy,
		[]string{"file.txt"},
		[]DestinationCandidate{{Path: "/tmp/one"}, {Path: "/tmp/two"}},
		map[string]time.Time{},
		false,
		nil,
		func(string, ...interface{}) {},
	)
	var got []string
	dialog.SetOnSelectedPathChanged(func(path string) {
		got = append(got, path)
	})

	dialog.MoveDown()
	dialog.updateFiltered("missing")

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

func TestCopyMoveSetDestinationsKeepsSearchAndSelectsPreferredPath(t *testing.T) {
	dialog := NewCopyMoveDialog(
		OpCopy,
		[]string{"file.txt"},
		[]DestinationCandidate{{Path: "/tmp/source"}},
		map[string]time.Time{},
		false,
		nil,
		func(string, ...interface{}) {},
	)
	dialog.AppendToSearch("project")

	dialog.SetDestinations([]DestinationCandidate{
		{Path: "/tmp/project/base"},
		{Path: "/tmp/project/new"},
		{Path: "/tmp/other"},
	}, "/tmp/project/new")

	if got := dialog.GetSearchText(); got != "project" {
		t.Fatalf("search text = %q, want project", got)
	}
	if got := dialog.selectedPath; got != "/tmp/project/new" {
		t.Fatalf("selected path = %q, want /tmp/project/new", got)
	}
	if len(dialog.filteredDest) != 2 {
		t.Fatalf("filtered destinations = %#v, want two project paths", dialog.filteredDest)
	}
}

func TestCopyMoveOpenDestinationUsesSelectedPath(t *testing.T) {
	dialog := NewCopyMoveDialog(
		OpCopy,
		[]string{"file.txt"},
		[]DestinationCandidate{{Path: "/tmp/one"}},
		map[string]time.Time{},
		false,
		nil,
		func(string, ...interface{}) {},
	)
	var got string
	dialog.SetOnOpenDestination(func(path string) { got = path })

	dialog.OpenDestination()

	if got != "/tmp/one" {
		t.Fatalf("open destination path = %q, want /tmp/one", got)
	}
}

func TestCopyDialogPreserveTimestampsDefault(t *testing.T) {
	dialog := NewCopyMoveDialog(
		OpCopy,
		[]string{"file.txt"},
		[]DestinationCandidate{{Path: "/tmp/one"}},
		map[string]time.Time{},
		true,
		nil,
		func(string, ...interface{}) {},
	)

	if !dialog.PreserveTimestamps() {
		t.Fatal("copy dialog should use configured preserve timestamps default")
	}
}

func TestMoveDialogDoesNotExposePreserveTimestamps(t *testing.T) {
	dialog := NewCopyMoveDialog(
		OpMove,
		[]string{"file.txt"},
		[]DestinationCandidate{{Path: "/tmp/one"}},
		map[string]time.Time{},
		true,
		nil,
		func(string, ...interface{}) {},
	)

	if dialog.PreserveTimestamps() {
		t.Fatal("move dialog should not expose copy preserve timestamps option")
	}
}
