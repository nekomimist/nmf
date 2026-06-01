package keymanager

import (
	"unicode"

	"fyne.io/fyne/v2"
)

// LineEditDialogInterface defines the operations used by the line edit dialog handler.
type LineEditDialogInterface interface {
	AcceptEdit()
	CancelDialog()
	MoveCursorStart()
	MoveCursorEnd()
	MoveCursorLeft()
	MoveCursorRight()
	DeleteBeforeCursor()
	DeleteAtCursor()
	DeleteBeforeCursorToStart()
	DeleteAfterCursorToEnd()
	PasteFromClipboard()
	InsertRune(r rune) bool
}

// LineEditDialogKeyHandler handles commit/cancel and readline-style edit keys.
type LineEditDialogKeyHandler struct {
	dialog LineEditDialogInterface
}

// NewLineEditDialogKeyHandler creates a line edit dialog key handler.
func NewLineEditDialogKeyHandler(d LineEditDialogInterface) *LineEditDialogKeyHandler {
	return &LineEditDialogKeyHandler{dialog: d}
}

// GetName returns the handler name.
func (h *LineEditDialogKeyHandler) GetName() string { return "LineEditDialog" }

// OnKeyDown handles desktop key events.
func (h *LineEditDialogKeyHandler) OnKeyDown(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	if modifiers.CtrlPressed {
		switch ev.Name {
		case fyne.KeyA:
			h.dialog.MoveCursorStart()
			return true
		case fyne.KeyE:
			h.dialog.MoveCursorEnd()
			return true
		case fyne.KeyB:
			h.dialog.MoveCursorLeft()
			return true
		case fyne.KeyF:
			h.dialog.MoveCursorRight()
			return true
		case fyne.KeyH:
			h.dialog.DeleteBeforeCursor()
			return true
		case fyne.KeyD:
			h.dialog.DeleteAtCursor()
			return true
		case fyne.KeyU:
			h.dialog.DeleteBeforeCursorToStart()
			return true
		case fyne.KeyK:
			h.dialog.DeleteAfterCursorToEnd()
			return true
		case fyne.KeyY:
			h.dialog.PasteFromClipboard()
			return true
		}
	}

	switch ev.Name {
	case fyne.KeyEscape:
		h.dialog.CancelDialog()
		return true
	}
	return false
}

// OnKeyUp handles key release events.
func (h *LineEditDialogKeyHandler) OnKeyUp(_ *fyne.KeyEvent, _ ModifierState) bool {
	return false
}

// OnTypedKey handles special typed keys.
func (h *LineEditDialogKeyHandler) OnTypedKey(ev *fyne.KeyEvent, _ ModifierState) bool {
	switch ev.Name {
	case fyne.KeyReturn, fyne.KeyEnter:
		h.dialog.AcceptEdit()
		return true
	case fyne.KeyEscape:
		h.dialog.CancelDialog()
		return true
	case fyne.KeyBackspace:
		h.dialog.DeleteBeforeCursor()
		return true
	case fyne.KeyDelete:
		h.dialog.DeleteAtCursor()
		return true
	case fyne.KeyLeft:
		h.dialog.MoveCursorLeft()
		return true
	case fyne.KeyRight:
		h.dialog.MoveCursorRight()
		return true
	case fyne.KeyHome:
		h.dialog.MoveCursorStart()
		return true
	case fyne.KeyEnd:
		h.dialog.MoveCursorEnd()
		return true
	}
	return false
}

// OnTypedRune handles text input when focus has drifted away from the entry.
func (h *LineEditDialogKeyHandler) OnTypedRune(r rune, _ ModifierState) bool {
	if unicode.IsPrint(r) && !unicode.IsControl(r) {
		return h.dialog.InsertRune(r)
	}
	return false
}
