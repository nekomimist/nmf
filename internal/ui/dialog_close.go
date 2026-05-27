package ui

import (
	"fyne.io/fyne/v2"

	"nmf/internal/keymanager"
)

func deferDialogClose(km *keymanager.KeyManager, label string, action func()) {
	if action == nil {
		return
	}
	if km == nil {
		action()
		return
	}
	km.DeferUntilKeysReleased(label, action)
}

func unfocusIfDialogOwned(parent fyne.Window, owned ...fyne.Focusable) {
	if parent == nil {
		return
	}
	focused := parent.Canvas().Focused()
	if focused == nil {
		return
	}
	for _, owner := range owned {
		if owner != nil && focused == owner {
			parent.Canvas().Unfocus()
			return
		}
	}
}
