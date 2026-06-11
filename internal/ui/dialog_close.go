package ui

import (
	"fyne.io/fyne/v2"

	"nmf/internal/keymanager"
)

// deferDialogClose runs a dialog close path through the KeyManager's owner
// transition gate: the close executes on the next main-loop iteration and
// held-key repeats cannot fall through to the handler underneath.
func deferDialogClose(km *keymanager.KeyManager, label string, action func()) {
	if action == nil {
		return
	}
	if km == nil {
		action()
		return
	}
	km.BeginOwnerTransition(label, action)
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
