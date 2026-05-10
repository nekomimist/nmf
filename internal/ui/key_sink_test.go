package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/keymanager"
)

type shortcutRecordingHandler struct {
	keys      []fyne.KeyName
	modifiers []keymanager.ModifierState
}

func (h *shortcutRecordingHandler) OnKeyDown(_ *fyne.KeyEvent, _ keymanager.ModifierState) bool {
	return false
}
func (h *shortcutRecordingHandler) OnKeyUp(_ *fyne.KeyEvent, _ keymanager.ModifierState) bool {
	return false
}
func (h *shortcutRecordingHandler) OnTypedKey(ev *fyne.KeyEvent, modifiers keymanager.ModifierState) bool {
	h.keys = append(h.keys, ev.Name)
	h.modifiers = append(h.modifiers, modifiers)
	return true
}
func (h *shortcutRecordingHandler) OnTypedRune(_ rune, _ keymanager.ModifierState) bool {
	return false
}
func (h *shortcutRecordingHandler) GetName() string { return "shortcutRecording" }

func TestKeySinkForwardsCustomShortcuts(t *testing.T) {
	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	handler := &shortcutRecordingHandler{}
	km.PushHandler(handler)
	sink := NewKeySink(widget.NewLabel("content"), km)

	sink.TypedShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyH,
		Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift,
	})

	if len(handler.keys) != 1 || handler.keys[0] != fyne.KeyH {
		t.Fatalf("keys = %v, want [H]", handler.keys)
	}
	if len(handler.modifiers) != 1 || !handler.modifiers[0].CtrlPressed || !handler.modifiers[0].ShiftPressed {
		t.Fatalf("modifiers = %+v, want ctrl and shift", handler.modifiers)
	}
}
