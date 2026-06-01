package keymanager

import (
	"testing"

	"fyne.io/fyne/v2"
)

func TestBusyKeyHandlerEscapeCancelsAndSwallows(t *testing.T) {
	cancelled := 0
	handler := NewBusyKeyHandler(func() { cancelled++ })

	if !handler.OnKeyDown(&fyne.KeyEvent{Name: fyne.KeyEscape}, ModifierState{}) {
		t.Fatal("Escape KeyDown should be handled")
	}
	if cancelled != 1 {
		t.Fatalf("cancelled after KeyDown = %d, want 1", cancelled)
	}

	if !handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape}, ModifierState{}) {
		t.Fatal("Escape TypedKey should be handled")
	}
	if cancelled != 2 {
		t.Fatalf("cancelled after TypedKey = %d, want 2", cancelled)
	}
}

func TestBusyKeyHandlerSwallowsNonEscapeInput(t *testing.T) {
	cancelled := 0
	handler := NewBusyKeyHandler(func() { cancelled++ })

	if !handler.OnKeyDown(&fyne.KeyEvent{Name: fyne.KeyA}, ModifierState{}) {
		t.Fatal("non-Escape KeyDown should be handled")
	}
	if !handler.OnTypedRune('x', ModifierState{}) {
		t.Fatal("typed rune should be handled")
	}
	if cancelled != 0 {
		t.Fatalf("cancelled = %d, want 0", cancelled)
	}
}
