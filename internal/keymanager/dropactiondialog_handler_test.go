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

	tests := []struct {
		name string
		key  fyne.KeyName
		want func() int
	}{
		{name: "copy", key: fyne.KeyC, want: func() int { return d.copied }},
		{name: "move", key: fyne.KeyM, want: func() int { return d.moved }},
		{name: "cancel", key: fyne.KeyEscape, want: func() int { return d.cancelled }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !handler.OnKeyActivated(&fyne.KeyEvent{Name: tt.key}, ModifierState{}) {
				t.Fatalf("%s should be handled", tt.key)
			}
			if got := tt.want(); got != 1 {
				t.Fatalf("action count = %d, want 1", got)
			}
		})
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
