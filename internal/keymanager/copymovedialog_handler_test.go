package keymanager

import (
	"testing"

	"fyne.io/fyne/v2"
)

func TestCopyMoveDialogHandlerAcceptVariants(t *testing.T) {
	dialog := &fakeFilterSearchDialog{}
	handler := NewCopyMoveDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if handler.GetName() != "CopyMoveDialog" {
		t.Fatalf("GetName() = %q, want %q", handler.GetName(), "CopyMoveDialog")
	}

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyReturn}, ModifierState{}) {
		t.Fatal("Return should be handled")
	}

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyReturn}, ModifierState{CtrlPressed: true}) {
		t.Fatal("Ctrl+Return should be handled")
	}
}

func TestCopyMoveDialogHandlerCancel(t *testing.T) {
	dialog := &fakeFilterSearchDialog{}
	handler := NewCopyMoveDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyEscape}, ModifierState{}) {
		t.Fatal("Escape should be handled")
	}
}

func TestCopyMoveDialogHandlerPlainDeleteOnlyClearsSearch(t *testing.T) {
	dialog := &fakeFilterSearchDialog{}
	handler := NewCopyMoveDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyDelete}, ModifierState{}) {
		t.Fatal("plain Delete should be handled")
	}
	if handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyDelete}, ModifierState{ShiftPressed: true}) {
		t.Fatal("Shift+Delete should not be handled by CopyMoveDialog")
	}
}

func TestCopyMoveDialogHandlerMoveKeys(t *testing.T) {
	dialog := &fakeFilterSearchDialog{}
	handler := NewCopyMoveDialogKeyHandler(dialog, func(string, ...interface{}) {})

	for _, key := range []fyne.KeyName{fyne.KeyUp, fyne.KeyDown} {
		if !handler.OnKeyActivated(&fyne.KeyEvent{Name: key}, ModifierState{}) {
			t.Fatalf("%s should be handled", key)
		}
		if !handler.OnKeyActivated(&fyne.KeyEvent{Name: key}, ModifierState{ShiftPressed: true}) {
			t.Fatalf("Shift+%s should be handled", key)
		}
	}
}
