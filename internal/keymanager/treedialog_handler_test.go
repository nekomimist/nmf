package keymanager

import (
	"testing"

	"fyne.io/fyne/v2"
)

type fakeTreeDialog struct {
	up         int
	down       int
	expanded   int
	collapsed  int
	selected   int
	accepted   int
	cancelled  int
	rootToggle int
}

func (f *fakeTreeDialog) MoveUp()            { f.up++ }
func (f *fakeTreeDialog) MoveDown()          { f.down++ }
func (f *fakeTreeDialog) ExpandNode()        { f.expanded++ }
func (f *fakeTreeDialog) CollapseNode()      { f.collapsed++ }
func (f *fakeTreeDialog) SelectCurrentNode() { f.selected++ }
func (f *fakeTreeDialog) AcceptSelection()   { f.accepted++ }
func (f *fakeTreeDialog) CancelDialog()      { f.cancelled++ }
func (f *fakeTreeDialog) ToggleRootMode()    { f.rootToggle++ }

func TestTreeDialogHandlerAcceptAndCancel(t *testing.T) {
	dialog := &fakeTreeDialog{}
	handler := NewTreeDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if handler.GetName() != "TreeDialog" {
		t.Fatalf("GetName() = %q, want %q", handler.GetName(), "TreeDialog")
	}

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyReturn}, ModifierState{}) {
		t.Fatal("Return should be handled")
	}
	if dialog.accepted != 1 {
		t.Fatalf("accepted = %d, want 1", dialog.accepted)
	}

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyEscape}, ModifierState{}) {
		t.Fatal("Escape should be handled")
	}
	if dialog.cancelled != 1 {
		t.Fatalf("cancelled = %d, want 1", dialog.cancelled)
	}
}

func TestTreeDialogHandlerShiftUpFastMoves5(t *testing.T) {
	dialog := &fakeTreeDialog{}
	handler := NewTreeDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyUp}, ModifierState{}) {
		t.Fatal("Up should be handled")
	}
	if dialog.up != 1 {
		t.Fatalf("up = %d, want 1", dialog.up)
	}

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyUp}, ModifierState{ShiftPressed: true}) {
		t.Fatal("Shift+Up should be handled")
	}
	if dialog.up != 6 {
		t.Fatalf("up after Shift+Up = %d, want 6 (1 + 5 fast moves)", dialog.up)
	}
}

func TestTreeDialogHandlerShiftDownFastMoves5(t *testing.T) {
	dialog := &fakeTreeDialog{}
	handler := NewTreeDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyDown}, ModifierState{ShiftPressed: true}) {
		t.Fatal("Shift+Down should be handled")
	}
	if dialog.down != 5 {
		t.Fatalf("down = %d, want 5", dialog.down)
	}
}

func TestTreeDialogHandlerRootModeToggles(t *testing.T) {
	dialog := &fakeTreeDialog{}
	handler := NewTreeDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyR}, ModifierState{CtrlPressed: true}) {
		t.Fatal("Ctrl+R should be handled")
	}
	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyTab}, ModifierState{}) {
		t.Fatal("Tab should be handled")
	}
	if dialog.rootToggle != 2 {
		t.Fatalf("rootToggle = %d, want 2", dialog.rootToggle)
	}

	// Plain R (no Ctrl) is not bound.
	if handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyR}, ModifierState{}) {
		t.Fatal("plain R should not be handled")
	}
}

func TestTreeDialogHandlerExpandCollapse(t *testing.T) {
	dialog := &fakeTreeDialog{}
	handler := NewTreeDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyRight}, ModifierState{}) {
		t.Fatal("Right should be handled")
	}
	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyLeft}, ModifierState{}) {
		t.Fatal("Left should be handled")
	}
	if dialog.expanded != 1 || dialog.collapsed != 1 {
		t.Fatalf("expanded=%d collapsed=%d, want 1/1", dialog.expanded, dialog.collapsed)
	}
}
