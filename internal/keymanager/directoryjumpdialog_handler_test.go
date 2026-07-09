package keymanager

import (
	"testing"

	"fyne.io/fyne/v2"
)

func TestDirectoryJumpDialogHandlerAcceptAndCancel(t *testing.T) {
	dialog := &fakeFilterSearchDialog{}
	handler := NewDirectoryJumpDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if handler.GetName() != "DirectoryJumpDialog" {
		t.Fatalf("GetName() = %q, want %q", handler.GetName(), "DirectoryJumpDialog")
	}

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyReturn}, ModifierState{}) {
		t.Fatal("Return should be handled")
	}
	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyEscape}, ModifierState{}) {
		t.Fatal("Escape should be handled")
	}
}

func TestDirectoryJumpDialogHandlerPlainDeleteOnlyClearsSearch(t *testing.T) {
	dialog := &fakeFilterSearchDialog{}
	handler := NewDirectoryJumpDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyDelete}, ModifierState{}) {
		t.Fatal("plain Delete should be handled")
	}
	if handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyDelete}, ModifierState{ShiftPressed: true}) {
		t.Fatal("Shift+Delete should not be handled by DirectoryJumpDialog")
	}
}

func TestDirectoryJumpDialogHandlerAltRuneSwallowed(t *testing.T) {
	dialog := &fakeFilterSearchDialog{}
	handler := NewDirectoryJumpDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if !handler.OnTypedRune('w', ModifierState{AltPressed: true}) {
		t.Fatal("Alt-modified rune should be swallowed (handled=true, no search append)")
	}
	if dialog.search != "" {
		t.Fatalf("search = %q, want empty (Alt rune must not append)", dialog.search)
	}
}

func TestDirectoryJumpDialogHandlerTabCopiesShortcut(t *testing.T) {
	dialog := &fakeFilterSearchDialog{}
	handler := NewDirectoryJumpDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyTab}, ModifierState{}) {
		t.Fatal("Tab should be handled")
	}
}
