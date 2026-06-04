package keymanager

import (
	"testing"

	"fyne.io/fyne/v2"
)

type fakeQuitDialog struct {
	confirmed int
	cancelled int
	defaults  int
}

func (f *fakeQuitDialog) ConfirmQuit() { f.confirmed++ }
func (f *fakeQuitDialog) CancelQuit()  { f.cancelled++ }
func (f *fakeQuitDialog) DefaultQuitAction() {
	f.defaults++
}

func TestQuitConfirmDialogHandlerReturnUsesDefaultAction(t *testing.T) {
	dialog := &fakeQuitDialog{}
	handler := NewQuitConfirmDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if !handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn}, ModifierState{}) {
		t.Fatal("Return should be handled")
	}
	if dialog.defaults != 1 {
		t.Fatalf("default action count = %d, want 1", dialog.defaults)
	}
	if dialog.confirmed != 0 || dialog.cancelled != 0 {
		t.Fatalf("Return should only use default action, confirmed=%d cancelled=%d", dialog.confirmed, dialog.cancelled)
	}
}

func TestQuitConfirmDialogHandlerEnterUsesDefaultAction(t *testing.T) {
	dialog := &fakeQuitDialog{}
	handler := NewQuitConfirmDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if !handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyEnter}, ModifierState{}) {
		t.Fatal("Enter should be handled")
	}
	if dialog.defaults != 1 {
		t.Fatalf("default action count = %d, want 1", dialog.defaults)
	}
}

func TestQuitConfirmDialogHandlerExplicitChoices(t *testing.T) {
	dialog := &fakeQuitDialog{}
	handler := NewQuitConfirmDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if !handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyY}, ModifierState{}) {
		t.Fatal("Y should be handled")
	}
	if dialog.confirmed != 1 {
		t.Fatalf("confirmed count = %d, want 1", dialog.confirmed)
	}

	if !handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyN}, ModifierState{}) {
		t.Fatal("N should be handled")
	}
	if dialog.cancelled != 1 {
		t.Fatalf("cancelled count = %d, want 1", dialog.cancelled)
	}
}
