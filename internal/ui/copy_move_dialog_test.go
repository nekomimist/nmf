package ui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/widget"
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

func TestCopyMoveHorizontalScrollState(t *testing.T) {
	dialog := NewCopyMoveDialog(
		OpCopy,
		[]string{"file.txt"},
		[]DestinationCandidate{{Path: "/tmp/very/long/path"}},
		map[string]time.Time{},
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
