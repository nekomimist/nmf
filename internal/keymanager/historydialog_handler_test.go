package keymanager

import (
	"testing"

	"fyne.io/fyne/v2"
)

func TestHistoryDialogHandlerAcceptAndCancel(t *testing.T) {
	dialog := &fakeFilterSearchDialog{}
	handler := NewHistoryDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if handler.GetName() != "HistoryDialog" {
		t.Fatalf("GetName() = %q, want %q", handler.GetName(), "HistoryDialog")
	}

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyEscape}, ModifierState{}) {
		t.Fatal("Escape should be handled")
	}
}

func TestHistoryDialogHandlerMoveKeys(t *testing.T) {
	dialog := &fakeFilterSearchDialog{}
	handler := NewHistoryDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyUp}, ModifierState{}) {
		t.Fatal("Up should be handled")
	}
	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyUp}, ModifierState{ShiftPressed: true}) {
		t.Fatal("Shift+Up should be handled")
	}
	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyDown}, ModifierState{}) {
		t.Fatal("Down should be handled")
	}
	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyDown}, ModifierState{ShiftPressed: true}) {
		t.Fatal("Shift+Down should be handled")
	}

	// Ctrl+Up no longer falls back to plain MoveUp under exact-modifier
	// matching (loose-match normalization); it must be unhandled.
	if handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyUp}, ModifierState{CtrlPressed: true}) {
		t.Fatal("Ctrl+Up should not be handled")
	}
}

func TestHistoryDialogHandlerCtrlFSwallowed(t *testing.T) {
	dialog := &fakeFilterSearchDialog{}
	handler := NewHistoryDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyF}, ModifierState{CtrlPressed: true}) {
		t.Fatal("Ctrl+F should be handled (swallowed)")
	}
}

func TestHistoryDialogHandlerPlainDeleteOnlyClearsSearch(t *testing.T) {
	dialog := &fakeFilterSearchDialog{}
	handler := NewHistoryDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyDelete}, ModifierState{}) {
		t.Fatal("plain Delete should be handled")
	}
	// Shift+Delete arrives as a folded Cut shortcut; it must not match the
	// search-clear binding (which requires no modifiers).
	if handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyDelete}, ModifierState{ShiftPressed: true}) {
		t.Fatal("Shift+Delete should not be handled by HistoryDialog")
	}
}
