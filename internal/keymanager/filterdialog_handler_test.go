package keymanager

import (
	"testing"

	"fyne.io/fyne/v2"
)

func TestFilterDialogHandlerAcceptVariants(t *testing.T) {
	dialog := &fakeFilterSearchDialog{}
	handler := NewFilterDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if handler.GetName() != "FilterDialog" {
		t.Fatalf("GetName() = %q, want %q", handler.GetName(), "FilterDialog")
	}

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyReturn}, ModifierState{}) {
		t.Fatal("Return should be handled")
	}
	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyEscape}, ModifierState{}) {
		t.Fatal("Escape should be handled")
	}
}

func TestFilterDialogHandlerPlainDeleteOnlyClearsSearch(t *testing.T) {
	dialog := &fakeFilterSearchDialog{}
	handler := NewFilterDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyDelete}, ModifierState{}) {
		t.Fatal("plain Delete should be handled")
	}
	if handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyDelete}, ModifierState{ShiftPressed: true}) {
		t.Fatal("Shift+Delete should not be handled by FilterDialog")
	}
}

func TestFilterDialogHandlerMoveKeys(t *testing.T) {
	dialog := &fakeFilterSearchDialog{}
	handler := NewFilterDialogKeyHandler(dialog, func(string, ...interface{}) {})

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
}
