package keymanager

import (
	"testing"

	"fyne.io/fyne/v2"
)

type fakeConflictDialog struct {
	overwriteIfNewer int
	overwrite        int
	autoName         int
	rename           int
	skip             int
}

func (f *fakeConflictDialog) Continue()               {}
func (f *fakeConflictDialog) CancelJob()              {}
func (f *fakeConflictDialog) SelectOverwriteIfNewer() { f.overwriteIfNewer++ }
func (f *fakeConflictDialog) SelectOverwrite()        { f.overwrite++ }
func (f *fakeConflictDialog) SelectAutoName()         { f.autoName++ }
func (f *fakeConflictDialog) SelectRename()           { f.rename++ }
func (f *fakeConflictDialog) SelectSkip()             { f.skip++ }

func TestConflictDialogAltShortcutsSelectChoices(t *testing.T) {
	dialog := &fakeConflictDialog{}
	handler := NewConflictDialogKeyHandler(dialog)
	modifiers := ModifierState{AltPressed: true}

	tests := []struct {
		name string
		key  fyne.KeyName
		want func() int
	}{
		{name: "overwrite if newer", key: fyne.KeyN, want: func() int { return dialog.overwriteIfNewer }},
		{name: "overwrite", key: fyne.KeyO, want: func() int { return dialog.overwrite }},
		{name: "auto name", key: fyne.KeyA, want: func() int { return dialog.autoName }},
		{name: "rename", key: fyne.KeyR, want: func() int { return dialog.rename }},
		{name: "skip", key: fyne.KeyS, want: func() int { return dialog.skip }},
	}
	for _, tt := range tests {
		before := tt.want()
		if !handler.OnKeyActivated(&fyne.KeyEvent{Name: tt.key}, modifiers) {
			t.Fatalf("%s shortcut was not handled", tt.name)
		}
		if got := tt.want(); got != before+1 {
			t.Fatalf("%s calls: got %d want %d", tt.name, got, before+1)
		}
	}
}
