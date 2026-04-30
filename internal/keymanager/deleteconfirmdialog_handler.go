package keymanager

import "fyne.io/fyne/v2"

// DeleteConfirmDialogInterface defines keyboard actions for delete confirmation.
type DeleteConfirmDialogInterface interface {
	ConfirmDelete()
	CancelDelete()
}

// DeleteConfirmDialogKeyHandler handles keyboard events for delete confirmation.
type DeleteConfirmDialogKeyHandler struct {
	dialog DeleteConfirmDialogInterface
}

func NewDeleteConfirmDialogKeyHandler(d DeleteConfirmDialogInterface) *DeleteConfirmDialogKeyHandler {
	return &DeleteConfirmDialogKeyHandler{dialog: d}
}

func (h *DeleteConfirmDialogKeyHandler) GetName() string { return "DeleteConfirmDialog" }

func (h *DeleteConfirmDialogKeyHandler) OnKeyDown(_ *fyne.KeyEvent, _ ModifierState) bool {
	return false
}

func (h *DeleteConfirmDialogKeyHandler) OnKeyUp(_ *fyne.KeyEvent, _ ModifierState) bool {
	return false
}

func (h *DeleteConfirmDialogKeyHandler) OnTypedKey(ev *fyne.KeyEvent, _ ModifierState) bool {
	switch ev.Name {
	case fyne.KeyReturn:
		h.dialog.ConfirmDelete()
		return true
	case fyne.KeyEscape:
		h.dialog.CancelDelete()
		return true
	default:
		return false
	}
}

func (h *DeleteConfirmDialogKeyHandler) OnTypedRune(_ rune, _ ModifierState) bool {
	return false
}
