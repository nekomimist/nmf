package keymanager

import (
	"testing"

	"fyne.io/fyne/v2"
)

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
