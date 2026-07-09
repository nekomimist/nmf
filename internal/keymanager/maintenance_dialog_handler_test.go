package keymanager

import (
	"testing"

	"fyne.io/fyne/v2"
)

type fakeMaintenanceDialog struct {
	scanned  int
	applied  int
	canceled int
}

func (f *fakeMaintenanceDialog) Scan()   { f.scanned++ }
func (f *fakeMaintenanceDialog) Apply()  { f.applied++ }
func (f *fakeMaintenanceDialog) Cancel() { f.canceled++ }

func TestMaintenanceDialogHandlerKeys(t *testing.T) {
	dialog := &fakeMaintenanceDialog{}
	handler := NewMaintenanceDialogKeyHandler(dialog)

	if handler.GetName() != "MaintenanceDialog" {
		t.Fatalf("GetName() = %q, want %q", handler.GetName(), "MaintenanceDialog")
	}

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyF5}, ModifierState{}) {
		t.Fatal("F5 should be handled")
	}
	if dialog.scanned != 1 {
		t.Fatalf("scanned = %d, want 1", dialog.scanned)
	}

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyReturn}, ModifierState{}) {
		t.Fatal("Return should be handled")
	}
	if dialog.applied != 1 {
		t.Fatalf("applied = %d, want 1", dialog.applied)
	}

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyEscape}, ModifierState{}) {
		t.Fatal("Escape should be handled")
	}
	if dialog.canceled != 1 {
		t.Fatalf("canceled = %d, want 1", dialog.canceled)
	}
}

func TestMaintenanceDialogHandlerNilEvent(t *testing.T) {
	dialog := &fakeMaintenanceDialog{}
	handler := NewMaintenanceDialogKeyHandler(dialog)

	if handler.OnKeyActivated(nil, ModifierState{}) {
		t.Fatal("nil event should not be handled")
	}
}

func TestMaintenanceDialogHandlerModifiedKeysNoLongerMatch(t *testing.T) {
	dialog := &fakeMaintenanceDialog{}
	handler := NewMaintenanceDialogKeyHandler(dialog)

	// Previously the switch ignored modifiers entirely, so e.g. Ctrl+Return
	// also applied. Exact-match normalization means that no longer matches.
	if handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyReturn}, ModifierState{CtrlPressed: true}) {
		t.Fatal("Ctrl+Return should not be handled after exact-match normalization")
	}
	if dialog.applied != 0 {
		t.Fatalf("applied = %d, want 0", dialog.applied)
	}
}
