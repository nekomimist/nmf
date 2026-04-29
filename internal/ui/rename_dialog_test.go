package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
)

func TestMiddleEllipsizeFileNameLeavesShortNameUnchanged(t *testing.T) {
	name := "short.txt"

	got := middleEllipsizeFileName(name, 20)

	if got != name {
		t.Fatalf("middleEllipsizeFileName() = %q, want %q", got, name)
	}
}

func TestMiddleEllipsizeFileNamePreservesSuffix(t *testing.T) {
	name := "very-long-file-name-with-important-ending.tar.gz"

	got := middleEllipsizeFileName(name, 24)

	if !strings.Contains(got, "...") {
		t.Fatalf("middleEllipsizeFileName() = %q, want middle ellipsis", got)
	}
	if !strings.HasSuffix(got, "ending.tar.gz") {
		t.Fatalf("middleEllipsizeFileName() = %q, want suffix preserved", got)
	}
	if len([]rune(got)) > 24 {
		t.Fatalf("middleEllipsizeFileName() length = %d, want <= 24", len([]rune(got)))
	}
}

func TestMiddleEllipsizeFileNamePreservesMoreSuffixThanPrefix(t *testing.T) {
	name := "abcdefghijklmnopqrstuvwxyz0123456789.txt"

	got := middleEllipsizeFileName(name, 18)
	parts := strings.Split(got, "...")
	if len(parts) != 2 {
		t.Fatalf("middleEllipsizeFileName() = %q, want one middle ellipsis", got)
	}
	if len([]rune(parts[1])) <= len([]rune(parts[0])) {
		t.Fatalf("suffix length = %d, prefix length = %d; want suffix longer", len([]rune(parts[1])), len([]rune(parts[0])))
	}
	if !strings.HasSuffix(got, "456789.txt") {
		t.Fatalf("middleEllipsizeFileName() = %q, want terminal text preserved", got)
	}
}

func TestMiddleEllipsizeFileNameHandlesTinyLimit(t *testing.T) {
	got := middleEllipsizeFileName("abcdef", 2)

	if got != "ab" {
		t.Fatalf("middleEllipsizeFileName() = %q, want %q", got, "ab")
	}
}

func TestRenameEntryEscapeCancels(t *testing.T) {
	cancelled := false
	entry := newRenameEntry(func() {
		cancelled = true
	})

	entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})

	if !cancelled {
		t.Fatal("Escape should cancel rename entry")
	}
}
