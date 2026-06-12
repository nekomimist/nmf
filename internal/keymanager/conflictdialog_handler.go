package keymanager

import "fyne.io/fyne/v2"

// ConflictDialogInterface defines keyboard operations for the name conflict dialog.
type ConflictDialogInterface interface {
	Continue()
	CancelJob()
	SelectOverwriteIfNewer()
	SelectOverwrite()
	SelectAutoName()
	SelectRename()
	SelectSkip()
}

// ConflictDialogKeyHandler handles commit/cancel keys while resolving a copy/move conflict.
type ConflictDialogKeyHandler struct {
	dialog ConflictDialogInterface
}

// NewConflictDialogKeyHandler creates a conflict dialog key handler.
func NewConflictDialogKeyHandler(d ConflictDialogInterface) *ConflictDialogKeyHandler {
	return &ConflictDialogKeyHandler{dialog: d}
}

// GetName returns the handler name.
func (h *ConflictDialogKeyHandler) GetName() string { return "ConflictDialog" }

// OnKeyActivated handles key activations.
func (h *ConflictDialogKeyHandler) OnKeyActivated(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	if modifiers.AltPressed {
		switch ev.Name {
		case fyne.KeyN:
			h.dialog.SelectOverwriteIfNewer()
			return true
		case fyne.KeyO:
			h.dialog.SelectOverwrite()
			return true
		case fyne.KeyA:
			h.dialog.SelectAutoName()
			return true
		case fyne.KeyR:
			h.dialog.SelectRename()
			return true
		case fyne.KeyS:
			h.dialog.SelectSkip()
			return true
		}
	}
	switch ev.Name {
	case fyne.KeyReturn, fyne.KeyEnter:
		h.dialog.Continue()
		return true
	case fyne.KeyEscape:
		h.dialog.CancelJob()
		return true
	}
	return false
}

// OnTypedRune lets focused widgets handle text input.
func (h *ConflictDialogKeyHandler) OnTypedRune(_ rune, _ ModifierState) bool {
	return false
}
