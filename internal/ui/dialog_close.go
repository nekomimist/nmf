package ui

import "nmf/internal/keymanager"

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
