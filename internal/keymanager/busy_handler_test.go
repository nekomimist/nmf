package keymanager

import (
	"testing"

	"fyne.io/fyne/v2"
)

func TestBusyKeyHandlerEscapeCancelsAndSwallows(t *testing.T) {
	cancelled := 0
	handler := NewBusyKeyHandler(func() { cancelled++ })

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyEscape}, ModifierState{}) {
		t.Fatal("Escape activation should be handled")
	}
	if cancelled != 1 {
		t.Fatalf("cancelled after activation = %d, want 1", cancelled)
	}
}

func TestBusyKeyHandlerSwallowsNonEscapeInput(t *testing.T) {
	cancelled := 0
	handler := NewBusyKeyHandler(func() { cancelled++ })

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyA}, ModifierState{}) {
		t.Fatal("non-Escape activation should be handled")
	}
	if !handler.OnTypedRune('x', ModifierState{}) {
		t.Fatal("typed rune should be handled")
	}
	if cancelled != 0 {
		t.Fatalf("cancelled = %d, want 0", cancelled)
	}
}
