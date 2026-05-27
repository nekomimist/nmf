package main

import (
	"fmt"

	"fyne.io/fyne/v2"
)

func focusedObjectLabel(w fyne.Window) string {
	if w == nil {
		return "<nil-window>"
	}
	focused := w.Canvas().Focused()
	if focused == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%T", focused)
}
