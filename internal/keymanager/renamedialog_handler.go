package keymanager

import "fyne.io/fyne/v2"

// RenameDialogInterface defines the operations used by the rename dialog handler.
type RenameDialogInterface interface {
	AcceptRename()
	CancelDialog()
}

// RenameDialogKeyHandler handles commit/cancel keys while the rename dialog is open.
type RenameDialogKeyHandler struct {
	dialog RenameDialogInterface
}

// NewRenameDialogKeyHandler creates a rename dialog key handler.
func NewRenameDialogKeyHandler(d RenameDialogInterface) *RenameDialogKeyHandler {
	return &RenameDialogKeyHandler{dialog: d}
}

// GetName returns the handler name.
func (h *RenameDialogKeyHandler) GetName() string { return "RenameDialog" }

// OnKeyDown handles desktop key events.
func (h *RenameDialogKeyHandler) OnKeyDown(ev *fyne.KeyEvent, _ ModifierState) bool {
	switch ev.Name {
	case fyne.KeyEscape:
		h.dialog.CancelDialog()
		return true
	}
	return false
}

// OnKeyUp handles key release events.
func (h *RenameDialogKeyHandler) OnKeyUp(_ *fyne.KeyEvent, _ ModifierState) bool {
	return false
}

// OnTypedKey handles special typed keys.
func (h *RenameDialogKeyHandler) OnTypedKey(ev *fyne.KeyEvent, _ ModifierState) bool {
	switch ev.Name {
	case fyne.KeyReturn, fyne.KeyEnter:
		h.dialog.AcceptRename()
		return true
	case fyne.KeyEscape:
		h.dialog.CancelDialog()
		return true
	}
	return false
}

// OnTypedRune lets the focused entry handle text input.
func (h *RenameDialogKeyHandler) OnTypedRune(_ rune, _ ModifierState) bool {
	return false
}
