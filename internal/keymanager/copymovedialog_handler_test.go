package keymanager

import (
	"testing"

	"fyne.io/fyne/v2"
)

func TestCopyMoveDialogHandlerCtrlRTogglesHistoryOrder(t *testing.T) {
	dialog := &fakeFilterSearchDialog{}
	handler := NewCopyMoveDialogKeyHandler(dialog, nil)
	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyR}, ModifierState{CtrlPressed: true}) {
		t.Fatal("Ctrl+R should be handled")
	}
	if dialog.order != 1 || dialog.search != "" {
		t.Fatal("Ctrl+R should toggle once without changing the query")
	}
	if handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyR}, ModifierState{CtrlPressed: true, ShiftPressed: true}) {
		t.Fatal("Ctrl+Shift+R should not match Ctrl+R")
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
