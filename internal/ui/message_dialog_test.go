package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
)

func TestCompactMessageSinkClosesOnDismissKeyUp(t *testing.T) {
	tests := []fyne.KeyName{fyne.KeyReturn, fyne.KeyEnter, fyne.KeyEscape}
	for _, key := range tests {
		closed := false
		sink := newCompactMessageSink(nil, func() { closed = true })

		sink.KeyDown(&fyne.KeyEvent{Name: key})
		sink.TypedKey(&fyne.KeyEvent{Name: key})
		if closed {
			t.Fatalf("TypedKey(%s) closed dialog before key release", key)
		}

		sink.KeyUp(&fyne.KeyEvent{Name: key})

		if !closed {
			t.Fatalf("KeyUp(%s) did not close dialog", key)
		}
	}
}

func TestCompactMessageSinkTypedKeyStillClosesWithoutKeyDown(t *testing.T) {
	closed := false
	sink := newCompactMessageSink(nil, func() { closed = true })

	sink.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if !closed {
		t.Fatal("TypedKey(Return) without KeyDown should close dialog")
	}
}

func TestCompactMessageSinkIgnoresCarriedKeyUp(t *testing.T) {
	closed := false
	sink := newCompactMessageSink(nil, func() { closed = true })

	sink.KeyUp(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if closed {
		t.Fatal("KeyUp(Return) without matching KeyDown should not close dialog")
	}
}

func TestCompactMessageSinkIgnoresOtherKeys(t *testing.T) {
	closed := false
	sink := newCompactMessageSink(nil, func() { closed = true })

	sink.TypedKey(&fyne.KeyEvent{Name: fyne.KeyA})

	if closed {
		t.Fatal("non-dismiss key should not close dialog")
	}
}

func TestCompactMessageDialogSizesGrowForLongMessages(t *testing.T) {
	shortMessage, shortDialog := compactMessageDialogSizes("short")
	longMessage, longDialog := compactMessageDialogSizes(strings.Repeat("x", 180))

	if longMessage.Height <= shortMessage.Height {
		t.Fatalf("long message height = %v, want > %v", longMessage.Height, shortMessage.Height)
	}
	if longDialog.Height <= shortDialog.Height {
		t.Fatalf("long dialog height = %v, want > %v", longDialog.Height, shortDialog.Height)
	}
	if longMessage.Width != shortMessage.Width {
		t.Fatalf("message width changed from %v to %v", shortMessage.Width, longMessage.Width)
	}
}
