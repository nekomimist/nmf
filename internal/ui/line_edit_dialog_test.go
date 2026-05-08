package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

func TestLineEditEntryReadlineCursorMovement(t *testing.T) {
	entry := NewLineEditEntry(nil)
	entry.SetText("abc")
	entry.MoveCursorEnd()

	entry.MoveCursorLeft()
	if entry.CursorColumn != 2 {
		t.Fatalf("cursor after left = %d, want 2", entry.CursorColumn)
	}
	entry.MoveCursorStart()
	if entry.CursorColumn != 0 {
		t.Fatalf("cursor after start = %d, want 0", entry.CursorColumn)
	}
	entry.MoveCursorRight()
	if entry.CursorColumn != 1 {
		t.Fatalf("cursor after right = %d, want 1", entry.CursorColumn)
	}
	entry.MoveCursorEnd()
	if entry.CursorColumn != 3 {
		t.Fatalf("cursor after end = %d, want 3", entry.CursorColumn)
	}
}

func TestLineEditEntryDeleteBeforeCursor(t *testing.T) {
	entry := NewLineEditEntry(nil)
	entry.SetText("abcd")
	entry.setCursor(2)

	entry.DeleteBeforeCursor()

	if entry.Text != "acd" {
		t.Fatalf("text = %q, want %q", entry.Text, "acd")
	}
	if entry.CursorColumn != 1 {
		t.Fatalf("cursor = %d, want 1", entry.CursorColumn)
	}
}

func TestLineEditEntryDeleteAtCursor(t *testing.T) {
	entry := NewLineEditEntry(nil)
	entry.SetText("abcd")
	entry.setCursor(1)

	entry.DeleteAtCursor()

	if entry.Text != "acd" {
		t.Fatalf("text = %q, want %q", entry.Text, "acd")
	}
	if entry.CursorColumn != 1 {
		t.Fatalf("cursor = %d, want 1", entry.CursorColumn)
	}
}

func TestLineEditEntryDeleteBeforeCursorToStart(t *testing.T) {
	entry := NewLineEditEntry(nil)
	entry.SetText("abcd")
	entry.setCursor(3)

	entry.DeleteBeforeCursorToStart()

	if entry.Text != "d" {
		t.Fatalf("text = %q, want %q", entry.Text, "d")
	}
	if entry.CursorColumn != 0 {
		t.Fatalf("cursor = %d, want 0", entry.CursorColumn)
	}
}

func TestLineEditEntryDeleteAfterCursorToEnd(t *testing.T) {
	entry := NewLineEditEntry(nil)
	entry.SetText("abcd")
	entry.setCursor(1)

	entry.DeleteAfterCursorToEnd()

	if entry.Text != "a" {
		t.Fatalf("text = %q, want %q", entry.Text, "a")
	}
	if entry.CursorColumn != 1 {
		t.Fatalf("cursor = %d, want 1", entry.CursorColumn)
	}
}

func TestLineEditEntryHandlesRunes(t *testing.T) {
	entry := NewLineEditEntry(nil)
	entry.SetText("あいう")
	entry.setCursor(2)

	entry.DeleteBeforeCursor()

	if entry.Text != "あう" {
		t.Fatalf("text = %q, want %q", entry.Text, "あう")
	}
	if entry.CursorColumn != 1 {
		t.Fatalf("cursor = %d, want 1", entry.CursorColumn)
	}
}

func TestLineEditEntryInsertTextAtCursor(t *testing.T) {
	entry := NewLineEditEntry(nil)
	entry.SetText("acd")
	entry.setCursor(1)

	entry.InsertText("b")

	if entry.Text != "abcd" {
		t.Fatalf("text = %q, want %q", entry.Text, "abcd")
	}
	if entry.CursorColumn != 2 {
		t.Fatalf("cursor = %d, want 2", entry.CursorColumn)
	}
}

func TestLineEditEntryKeyDownHandlesReadlineKeys(t *testing.T) {
	entry := NewLineEditEntry(nil)
	entry.SetText("abcd")
	entry.MoveCursorEnd()

	entry.KeyDown(&fyne.KeyEvent{Name: desktop.KeyControlLeft})
	entry.KeyDown(&fyne.KeyEvent{Name: fyne.KeyA})

	if entry.CursorColumn != 0 {
		t.Fatalf("cursor after ctrl-a = %d, want 0", entry.CursorColumn)
	}

	entry.KeyDown(&fyne.KeyEvent{Name: fyne.KeyE})
	if entry.CursorColumn != 4 {
		t.Fatalf("cursor after ctrl-e = %d, want 4", entry.CursorColumn)
	}

	entry.KeyDown(&fyne.KeyEvent{Name: fyne.KeyH})
	if entry.Text != "abc" {
		t.Fatalf("text after ctrl-h = %q, want %q", entry.Text, "abc")
	}

	entry.KeyUp(&fyne.KeyEvent{Name: desktop.KeyControlLeft})
}

func TestLineEditEntrySelectAllShortcutMovesToStart(t *testing.T) {
	entry := NewLineEditEntry(nil)
	entry.SetText("abcd")
	entry.MoveCursorEnd()

	entry.TypedShortcut(&fyne.ShortcutSelectAll{})

	if entry.CursorColumn != 0 {
		t.Fatalf("cursor after shortcut select-all = %d, want 0", entry.CursorColumn)
	}
}

func TestLineEditEntryFocusLostClearsCtrlState(t *testing.T) {
	entry := NewLineEditEntry(nil)
	entry.SetText("abcd")
	entry.MoveCursorEnd()
	entry.KeyDown(&fyne.KeyEvent{Name: desktop.KeyControlLeft})

	entry.FocusLost()
	entry.KeyDown(&fyne.KeyEvent{Name: fyne.KeyA})

	if entry.CursorColumn != 4 {
		t.Fatalf("cursor after plain a keydown = %d, want 4", entry.CursorColumn)
	}
}
