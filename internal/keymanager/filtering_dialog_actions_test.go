package keymanager

import (
	"testing"

	"fyne.io/fyne/v2"
)

func TestFilteringDialogActionBindings(t *testing.T) {
	type binding struct {
		key       fyne.KeyName
		modifiers ModifierState
		action    string
	}
	common := []binding{
		{fyne.KeyReturn, ModifierState{}, "accept"},
		{fyne.KeyEscape, ModifierState{}, "cancel"},
		{fyne.KeyUp, ModifierState{}, "up"},
		{fyne.KeyDown, ModifierState{}, "down"},
		{fyne.KeyUp, ModifierState{ShiftPressed: true}, "top"},
		{fyne.KeyDown, ModifierState{ShiftPressed: true}, "bottom"},
	}
	for _, tt := range []struct {
		name    string
		handler func(*fakeFilterSearchDialog) KeyHandler
		extra   []binding
	}{
		{"copy move", func(d *fakeFilterSearchDialog) KeyHandler {
			return NewCopyMoveDialogKeyHandler(d, nil)
		}, []binding{
			{fyne.KeyReturn, ModifierState{CtrlPressed: true}, "direct path"},
			{fyne.KeyTab, ModifierState{}, "copy path"},
		}},
		{"directory jump", func(d *fakeFilterSearchDialog) KeyHandler {
			return NewDirectoryJumpDialogKeyHandler(d, nil)
		}, []binding{{fyne.KeyTab, ModifierState{}, "copy shortcut"}}},
		{"filter", func(d *fakeFilterSearchDialog) KeyHandler {
			return NewFilterDialogKeyHandler(d, nil)
		}, nil},
		{"history", func(d *fakeFilterSearchDialog) KeyHandler {
			return NewHistoryDialogKeyHandler(d, nil)
		}, []binding{{fyne.KeyTab, ModifierState{}, "copy path"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			bindings := append(append([]binding(nil), common...), tt.extra...)
			for _, binding := range bindings {
				t.Run(binding.action, func(t *testing.T) {
					dialog := &fakeFilterSearchDialog{}
					handler := tt.handler(dialog)
					if !handler.OnKeyActivated(&fyne.KeyEvent{Name: binding.key}, binding.modifiers) {
						t.Fatal("action key was not handled")
					}
					if len(dialog.actions) != 1 || dialog.actions[0] != binding.action {
						t.Fatalf("actions = %v, want only %s", dialog.actions, binding.action)
					}
				})
			}
		})
	}
}
