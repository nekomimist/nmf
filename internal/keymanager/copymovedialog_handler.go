package keymanager

import (
	"unicode"

	"fyne.io/fyne/v2"
)

// CopyMoveDialogInterface defines the interface needed by Copy/Move dialog key handler
type CopyMoveDialogInterface interface {
	// List navigation
	MoveUp()
	MoveDown()
	MoveToTop()
	MoveToBottom()

	// Search functionality
	ClearSearch()
	AppendToSearch(char string)
	BackspaceSearch()
	GetSearchText() string
	CopySelectedPathToSearch()

	// Selection
	SelectCurrentItem()

	// Dialog control
	AcceptSelection()
	AcceptDirectPath() // Ctrl+Enter: use search text as destination directly
	CancelDialog()
}

// CopyMoveDialogKeyHandler handles keyboard events for the copy/move dialog
type CopyMoveDialogKeyHandler struct {
	dialog       CopyMoveDialogInterface
	debugPrint   func(format string, args ...interface{})
	skipNextRune bool // swallow the triggering 'c'/'m' rune injected after opening
}

// NewCopyMoveDialogKeyHandler creates a new copy/move dialog key handler
func NewCopyMoveDialogKeyHandler(d CopyMoveDialogInterface, debugPrint func(format string, args ...interface{})) *CopyMoveDialogKeyHandler {
	return &CopyMoveDialogKeyHandler{dialog: d, debugPrint: debugPrint, skipNextRune: true}
}

// GetName returns the handler name
func (h *CopyMoveDialogKeyHandler) GetName() string { return "CopyMoveDialog" }

// OnKeyDown handles key press events
func (h *CopyMoveDialogKeyHandler) OnKeyDown(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	switch ev.Name {
	case fyne.KeyReturn:
		if modifiers.CtrlPressed {
			h.dialog.AcceptDirectPath()
		}
		return true
	}
	return false
}

// OnKeyUp handles key release events
func (h *CopyMoveDialogKeyHandler) OnKeyUp(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	return false
}

// OnTypedKey handles non-text keys
func (h *CopyMoveDialogKeyHandler) OnTypedKey(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	switch ev.Name {
	case fyne.KeyUp:
		if modifiers.ShiftPressed {
			h.dialog.MoveToTop()
		} else {
			h.dialog.MoveUp()
		}
		return true
	case fyne.KeyDown:
		if modifiers.ShiftPressed {
			h.dialog.MoveToBottom()
		} else {
			h.dialog.MoveDown()
		}
		return true
	case fyne.KeySpace:
		h.dialog.SelectCurrentItem()
		return true
	case fyne.KeyReturn:
		h.dialog.AcceptSelection()
		return true
	case fyne.KeyEscape:
		h.dialog.CancelDialog()
		return true
	case fyne.KeyBackspace:
		h.dialog.BackspaceSearch()
		return true
	case fyne.KeyDelete:
		h.dialog.ClearSearch()
		return true
	case fyne.KeyTab:
		h.dialog.CopySelectedPathToSearch()
		return true
	}
	return false
}

// OnTypedRune handles text input to update search
func (h *CopyMoveDialogKeyHandler) OnTypedRune(r rune, modifiers ModifierState) bool {
	if h.skipNextRune {
		// Swallow the first printable rune (the 'c'/'m' that opened the dialog)
		h.skipNextRune = false
		return true
	}
	if unicode.IsPrint(r) && !unicode.IsControl(r) {
		h.dialog.AppendToSearch(string(r))
		return true
	}
	return false
}
