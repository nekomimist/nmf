package keymanager

import (
	"testing"

	"fyne.io/fyne/v2"
)

func TestHistoryDialogHandlerCtrlFSwallowed(t *testing.T) {
	dialog := &fakeFilterSearchDialog{}
	handler := NewHistoryDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyF}, ModifierState{CtrlPressed: true}) {
		t.Fatal("Ctrl+F should be handled (swallowed)")
	}
}

func TestHistoryDialogHandlerCtrlRTogglesHistoryOrder(t *testing.T) {
	dialog := &fakeFilterSearchDialog{}
	handler := NewHistoryDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyR}, ModifierState{CtrlPressed: true}) {
		t.Fatal("Ctrl+R should be handled")
	}
	if dialog.order != 1 {
		t.Fatalf("ToggleHistoryOrder count = %d, want 1", dialog.order)
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

func TestHistoryDialogHandlerPasteShortcuts(t *testing.T) {
	dialog := &fakeFilterSearchDialog{}
	handler := NewHistoryDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyV}, ModifierState{CtrlPressed: true}) {
		t.Fatal("Ctrl+V should be handled")
	}
	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyInsert}, ModifierState{ShiftPressed: true}) {
		t.Fatal("Shift+Insert should be handled")
	}
	if dialog.paste != 2 {
		t.Fatalf("PasteFromClipboard count = %d, want 2", dialog.paste)
	}
}
