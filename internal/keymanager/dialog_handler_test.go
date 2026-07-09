package keymanager

import (
	"testing"

	"fyne.io/fyne/v2"
)

func TestNewDialogKeyHandlerPanicsOnInvalidSpec(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for invalid key spec")
		}
	}()
	newDialogKeyHandler("BrokenDialog", nil, []dialogBinding{
		{"NotAKey", func() {}},
	})
}

func TestDialogKeyHandlerExactModifierMatch(t *testing.T) {
	upCalls, shiftUpCalls := 0, 0
	h := newDialogKeyHandler("TestDialog", nil, []dialogBinding{
		{"Up", func() { upCalls++ }},
		{"S-Up", func() { shiftUpCalls++ }},
	})

	if !h.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyUp}, ModifierState{}) {
		t.Fatal("plain Up should be handled")
	}
	if upCalls != 1 || shiftUpCalls != 0 {
		t.Fatalf("after plain Up: upCalls=%d shiftUpCalls=%d, want 1/0", upCalls, shiftUpCalls)
	}

	if !h.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyUp}, ModifierState{ShiftPressed: true}) {
		t.Fatal("Shift+Up should be handled")
	}
	if upCalls != 1 || shiftUpCalls != 1 {
		t.Fatalf("after Shift+Up: upCalls=%d shiftUpCalls=%d, want 1/1", upCalls, shiftUpCalls)
	}

	// Ctrl+Up matches neither spec exactly (Up requires no modifiers, S-Up
	// requires Shift only), so it must fall through unhandled.
	if h.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyUp}, ModifierState{CtrlPressed: true}) {
		t.Fatal("Ctrl+Up should not match either binding")
	}
	if upCalls != 1 || shiftUpCalls != 1 {
		t.Fatalf("after Ctrl+Up: upCalls=%d shiftUpCalls=%d, want unchanged 1/1", upCalls, shiftUpCalls)
	}

	// Ctrl+Shift+Up doesn't match S-Up either (extra Ctrl bit).
	if h.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyUp}, ModifierState{ShiftPressed: true, CtrlPressed: true}) {
		t.Fatal("Ctrl+Shift+Up should not match S-Up")
	}
}

func TestDialogKeyHandlerUnmatchedReturnsFalseWithoutFallback(t *testing.T) {
	h := newDialogKeyHandler("TestDialog", nil, []dialogBinding{
		{"Escape", func() {}},
	})
	if h.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyA}, ModifierState{}) {
		t.Fatal("unmatched key should return false when there is no fallback")
	}
}

func TestDialogKeyHandlerNilEventNeverMatches(t *testing.T) {
	calls := 0
	h := newDialogKeyHandler("TestDialog", nil, []dialogBinding{
		{"Escape", func() { calls++ }},
	})
	if h.OnKeyActivated(nil, ModifierState{}) {
		t.Fatal("nil event should not be handled")
	}
	if calls != 0 {
		t.Fatalf("calls = %d, want 0", calls)
	}
}

func TestDialogKeyHandlerFallback(t *testing.T) {
	fallbackCalls := 0
	h := newDialogKeyHandler("TestDialog", nil, []dialogBinding{
		{"Escape", func() {}},
	}).withFallback(func(ev *fyne.KeyEvent, modifiers ModifierState) bool {
		fallbackCalls++
		return true
	})

	if !h.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyA}, ModifierState{}) {
		t.Fatal("fallback should have consumed the unmatched key")
	}
	if fallbackCalls != 1 {
		t.Fatalf("fallbackCalls = %d, want 1", fallbackCalls)
	}

	// A matched binding must not also invoke the fallback.
	if !h.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyEscape}, ModifierState{}) {
		t.Fatal("Escape should be handled by its own binding")
	}
	if fallbackCalls != 1 {
		t.Fatalf("fallbackCalls after matched binding = %d, want unchanged 1", fallbackCalls)
	}
}

func TestDialogKeyHandlerRuneDispatch(t *testing.T) {
	var got rune
	h := newDialogKeyHandler("TestDialog", nil, nil).withRune(func(r rune, modifiers ModifierState) bool {
		got = r
		return true
	})

	if !h.OnTypedRune('x', ModifierState{}) {
		t.Fatal("rune should be handled")
	}
	if got != 'x' {
		t.Fatalf("got rune %q, want %q", got, 'x')
	}
}

func TestDialogKeyHandlerRuneWithoutHandlerReturnsFalse(t *testing.T) {
	h := newDialogKeyHandler("TestDialog", nil, nil)
	if h.OnTypedRune('x', ModifierState{}) {
		t.Fatal("no rune handler was attached; OnTypedRune should return false")
	}
}

func TestDialogKeyHandlerGetName(t *testing.T) {
	h := newDialogKeyHandler("TestDialog", nil, nil)
	if h.GetName() != "TestDialog" {
		t.Fatalf("GetName() = %q, want %q", h.GetName(), "TestDialog")
	}
}
