package keymanager

import (
	"testing"

	"fyne.io/fyne/v2"
)

type fakeLineEditDialog struct {
	accepted     int
	cancelled    int
	start        int
	end          int
	left         int
	right        int
	deleteBefore int
	deleteAt     int
	deleteStart  int
	deleteEnd    int
	paste        int
	inserted     string
}

func (f *fakeLineEditDialog) AcceptEdit()                { f.accepted++ }
func (f *fakeLineEditDialog) CancelDialog()              { f.cancelled++ }
func (f *fakeLineEditDialog) MoveCursorStart()           { f.start++ }
func (f *fakeLineEditDialog) MoveCursorEnd()             { f.end++ }
func (f *fakeLineEditDialog) MoveCursorLeft()            { f.left++ }
func (f *fakeLineEditDialog) MoveCursorRight()           { f.right++ }
func (f *fakeLineEditDialog) DeleteBeforeCursor()        { f.deleteBefore++ }
func (f *fakeLineEditDialog) DeleteAtCursor()            { f.deleteAt++ }
func (f *fakeLineEditDialog) DeleteBeforeCursorToStart() { f.deleteStart++ }
func (f *fakeLineEditDialog) DeleteAfterCursorToEnd()    { f.deleteEnd++ }
func (f *fakeLineEditDialog) PasteFromClipboard()        { f.paste++ }
func (f *fakeLineEditDialog) InsertRune(r rune) bool {
	f.inserted += string(r)
	return true
}

func TestLineEditDialogHandlerReadlineKeys(t *testing.T) {
	tests := []struct {
		name string
		key  fyne.KeyName
		want func(*fakeLineEditDialog) int
	}{
		{name: "ctrl a", key: fyne.KeyA, want: func(f *fakeLineEditDialog) int { return f.start }},
		{name: "ctrl e", key: fyne.KeyE, want: func(f *fakeLineEditDialog) int { return f.end }},
		{name: "ctrl b", key: fyne.KeyB, want: func(f *fakeLineEditDialog) int { return f.left }},
		{name: "ctrl f", key: fyne.KeyF, want: func(f *fakeLineEditDialog) int { return f.right }},
		{name: "ctrl h", key: fyne.KeyH, want: func(f *fakeLineEditDialog) int { return f.deleteBefore }},
		{name: "ctrl d", key: fyne.KeyD, want: func(f *fakeLineEditDialog) int { return f.deleteAt }},
		{name: "ctrl u", key: fyne.KeyU, want: func(f *fakeLineEditDialog) int { return f.deleteStart }},
		{name: "ctrl k", key: fyne.KeyK, want: func(f *fakeLineEditDialog) int { return f.deleteEnd }},
		{name: "ctrl y", key: fyne.KeyY, want: func(f *fakeLineEditDialog) int { return f.paste }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dialog := &fakeLineEditDialog{}
			handler := NewLineEditDialogKeyHandler(dialog)

			handled := handler.OnKeyDown(&fyne.KeyEvent{Name: tt.key}, ModifierState{CtrlPressed: true})

			if !handled {
				t.Fatal("OnKeyDown should handle readline key")
			}
			if got := tt.want(dialog); got != 1 {
				t.Fatalf("handler count = %d, want 1", got)
			}
		})
	}
}

func TestLineEditDialogHandlerAcceptsAndCancels(t *testing.T) {
	dialog := &fakeLineEditDialog{}
	handler := NewLineEditDialogKeyHandler(dialog)

	if !handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn}, ModifierState{}) {
		t.Fatal("Return should be handled")
	}
	if dialog.accepted != 1 {
		t.Fatalf("accepted = %d, want 1", dialog.accepted)
	}
	if !handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape}, ModifierState{}) {
		t.Fatal("Escape should be handled")
	}
	if dialog.cancelled != 1 {
		t.Fatalf("cancelled = %d, want 1", dialog.cancelled)
	}
}

func TestLineEditDialogHandlerFallbackTypedKeyEditing(t *testing.T) {
	tests := []struct {
		name string
		key  fyne.KeyName
		want func(*fakeLineEditDialog) int
	}{
		{name: "backspace", key: fyne.KeyBackspace, want: func(f *fakeLineEditDialog) int { return f.deleteBefore }},
		{name: "delete", key: fyne.KeyDelete, want: func(f *fakeLineEditDialog) int { return f.deleteAt }},
		{name: "left", key: fyne.KeyLeft, want: func(f *fakeLineEditDialog) int { return f.left }},
		{name: "right", key: fyne.KeyRight, want: func(f *fakeLineEditDialog) int { return f.right }},
		{name: "home", key: fyne.KeyHome, want: func(f *fakeLineEditDialog) int { return f.start }},
		{name: "end", key: fyne.KeyEnd, want: func(f *fakeLineEditDialog) int { return f.end }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dialog := &fakeLineEditDialog{}
			handler := NewLineEditDialogKeyHandler(dialog)

			if !handler.OnTypedKey(&fyne.KeyEvent{Name: tt.key}, ModifierState{}) {
				t.Fatal("OnTypedKey should handle fallback edit key")
			}
			if got := tt.want(dialog); got != 1 {
				t.Fatalf("handler count = %d, want 1", got)
			}
		})
	}
}

func TestLineEditDialogHandlerFallbackTypedRune(t *testing.T) {
	dialog := &fakeLineEditDialog{}
	handler := NewLineEditDialogKeyHandler(dialog)

	if !handler.OnTypedRune('x', ModifierState{}) {
		t.Fatal("printable rune should be handled")
	}
	if dialog.inserted != "x" {
		t.Fatalf("inserted = %q, want x", dialog.inserted)
	}
	if handler.OnTypedRune('\n', ModifierState{}) {
		t.Fatal("control rune should not be handled")
	}
}
