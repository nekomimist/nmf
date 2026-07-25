package keymanager

import (
	"testing"

	"fyne.io/fyne/v2"
)

type fakeDropActionDialog struct {
	copied    int
	moved     int
	cancelled int
}

func (f *fakeDropActionDialog) CopyDropped() { f.copied++ }
func (f *fakeDropActionDialog) MoveDropped() { f.moved++ }
func (f *fakeDropActionDialog) CancelDrop()  { f.cancelled++ }

func TestDropActionDialogHandlerActions(t *testing.T) {
	d := &fakeDropActionDialog{}
	handler := NewDropActionDialogKeyHandler(d)

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyEscape}, ModifierState{}) {
		t.Fatal("Escape should be handled")
	}
	if d.cancelled != 1 {
		t.Fatalf("cancel count = %d, want 1", d.cancelled)
	}
}

// Driver fact 6 (docs/architecture/ui-input.md): a printable key press delivers
// both a TypedKey and a TypedRune, so a spec matched on both paths fires twice.
// C and M belong to the rune path only.
func TestDropActionDialogHandlerBindsLettersOnOnePathOnly(t *testing.T) {
	d := &fakeDropActionDialog{}
	handler := NewDropActionDialogKeyHandler(d)

	for _, key := range []fyne.KeyName{fyne.KeyC, fyne.KeyM} {
		if handler.OnKeyActivated(&fyne.KeyEvent{Name: key}, ModifierState{}) {
			t.Fatalf("%s must not also be handled on the typed-key path", key)
		}
	}
	if d.copied != 0 || d.moved != 0 {
		t.Fatalf("typed-key path ran an action: copy=%d move=%d", d.copied, d.moved)
	}
}

func TestDropActionDialogHandlerRejectsModifiedAndUnrelatedKeys(t *testing.T) {
	d := &fakeDropActionDialog{}
	handler := NewDropActionDialogKeyHandler(d)

	if handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyC}, ModifierState{CtrlPressed: true}) {
		t.Fatal("Ctrl+C should not trigger dropped-file copy")
	}
	if handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyDown}, ModifierState{}) {
		t.Fatal("Down should not be handled")
	}
	if handler.OnTypedRune('x', ModifierState{}) {
		t.Fatal("unrelated rune should not be handled")
	}
	if d.copied != 0 || d.moved != 0 || d.cancelled != 0 {
		t.Fatalf("unexpected action counts: copy=%d move=%d cancel=%d", d.copied, d.moved, d.cancelled)
	}
}

func TestDropActionDialogHandlerRuneActions(t *testing.T) {
	d := &fakeDropActionDialog{}
	handler := NewDropActionDialogKeyHandler(d)

	if !handler.OnTypedRune('c', ModifierState{}) {
		t.Fatal("c rune should be handled")
	}
	if !handler.OnTypedRune('M', ModifierState{ShiftPressed: true}) {
		t.Fatal("M rune should be handled")
	}
	if handler.OnTypedRune('c', ModifierState{CtrlPressed: true}) {
		t.Fatal("Ctrl+C rune should not trigger dropped-file copy")
	}
	if d.copied != 1 || d.moved != 1 || d.cancelled != 0 {
		t.Fatalf("action counts: copy=%d move=%d cancel=%d; want 1, 1, 0", d.copied, d.moved, d.cancelled)
	}
}
