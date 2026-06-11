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

func (h *shortcutRecordingHandler) OnKeyActivated(ev *fyne.KeyEvent, modifiers keymanager.ModifierState) bool {
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

	sink.KeyDown(&fyne.KeyEvent{Name: fyne.KeyH}) // fresh press arms the gate
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

func TestKeySinkForwardsFoldedStandardShortcuts(t *testing.T) {
	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	handler := &shortcutRecordingHandler{}
	km.PushHandler(handler)
	sink := NewKeySink(widget.NewLabel("content"), km)

	sink.KeyDown(&fyne.KeyEvent{Name: fyne.KeyDelete})
	sink.TypedShortcut(&fyne.ShortcutCut{})

	if len(handler.keys) != 1 || handler.keys[0] != fyne.KeyDelete {
		t.Fatalf("keys = %v, want [Delete]", handler.keys)
	}
	if len(handler.modifiers) != 1 || !handler.modifiers[0].ShiftPressed || handler.modifiers[0].CtrlPressed {
		t.Fatalf("modifiers = %+v, want shift only", handler.modifiers)
	}
}

func TestKeySinkReportsFocusChanges(t *testing.T) {
	var got []bool
	sink := NewKeySink(widget.NewLabel("content"), nil, WithFocusChanged(func(active bool) {
		got = append(got, active)
	}))

	sink.FocusGained()
	sink.FocusLost()

	if len(got) != 2 || !got[0] || got[1] {
		t.Fatalf("focus changes = %v, want [true false]", got)
	}
}
