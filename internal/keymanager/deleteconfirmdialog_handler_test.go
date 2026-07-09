package keymanager

import (
	"testing"

	"fyne.io/fyne/v2"
)

type fakeDeleteConfirmDialog struct {
	confirmed int
	cancelled int
}

func (f *fakeDeleteConfirmDialog) ConfirmDelete() { f.confirmed++ }
func (f *fakeDeleteConfirmDialog) CancelDelete()  { f.cancelled++ }

func TestDeleteConfirmDialogHandlerConfirmAndCancel(t *testing.T) {
	dialog := &fakeDeleteConfirmDialog{}
	handler := NewDeleteConfirmDialogKeyHandler(dialog)

	if handler.GetName() != "DeleteConfirmDialog" {
		t.Fatalf("GetName() = %q, want %q", handler.GetName(), "DeleteConfirmDialog")
	}

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyReturn}, ModifierState{}) {
		t.Fatal("Return should be handled")
	}
	if dialog.confirmed != 1 {
		t.Fatalf("confirmed = %d, want 1", dialog.confirmed)
	}

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyEscape}, ModifierState{}) {
		t.Fatal("Escape should be handled")
	}
	if dialog.cancelled != 1 {
		t.Fatalf("cancelled = %d, want 1", dialog.cancelled)
	}
}

func TestDeleteConfirmDialogHandlerUnrelatedKeyUnhandled(t *testing.T) {
	dialog := &fakeDeleteConfirmDialog{}
	handler := NewDeleteConfirmDialogKeyHandler(dialog)

	if handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyA}, ModifierState{}) {
		t.Fatal("unrelated key should not be handled")
	}
	if handler.OnTypedRune('a', ModifierState{}) {
		t.Fatal("rune input should not be handled")
	}
}
