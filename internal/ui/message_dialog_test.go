package ui

import (
	"testing"

	"fyne.io/fyne/v2"
)

func TestCompactMessageSinkClosesOnEnterAndEscape(t *testing.T) {
	tests := []fyne.KeyName{fyne.KeyReturn, fyne.KeyEscape}
	for _, key := range tests {
		closed := false
		sink := newCompactMessageSink(nil, func() { closed = true })

		sink.TypedKey(&fyne.KeyEvent{Name: key})

		if !closed {
			t.Fatalf("TypedKey(%s) did not close dialog", key)
		}
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
