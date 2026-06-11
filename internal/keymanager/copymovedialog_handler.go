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
	ScrollSelectedRight()
	ResetHorizontalScroll()

	// Selection
	SelectCurrentItem()

	// Dialog control
	AcceptSelection()
	AcceptDirectPath() // Ctrl+Enter: use search text as destination directly
	OpenDestination()
	CancelDialog()
}

// CopyMoveDialogKeyHandler handles keyboard events for the copy/move dialog
type CopyMoveDialogKeyHandler struct {
	dialog     CopyMoveDialogInterface
	debugPrint func(format string, args ...interface{})
}

// NewCopyMoveDialogKeyHandler creates a new copy/move dialog key handler
func NewCopyMoveDialogKeyHandler(d CopyMoveDialogInterface, debugPrint func(format string, args ...interface{})) *CopyMoveDialogKeyHandler {
	return &CopyMoveDialogKeyHandler{dialog: d, debugPrint: debugPrint}
}

// GetName returns the handler name
func (h *CopyMoveDialogKeyHandler) GetName() string { return "CopyMoveDialog" }

// OnKeyActivated handles key activations
func (h *CopyMoveDialogKeyHandler) OnKeyActivated(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	switch ev.Name {
	case fyne.KeyH:
		if modifiers.CtrlPressed {
			h.dialog.BackspaceSearch()
			return true
		}
	case fyne.KeyN:
		if modifiers.CtrlPressed {
			h.dialog.OpenDestination()
			return true
		}

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
	case fyne.KeyRight:
		h.dialog.ScrollSelectedRight()
		return true
	case fyne.KeyLeft:
		h.dialog.ResetHorizontalScroll()
		return true
	case fyne.KeySpace:
		h.dialog.SelectCurrentItem()
		return true
	case fyne.KeyReturn:
		if modifiers.CtrlPressed {
			h.dialog.AcceptDirectPath()
			return true
		}
		h.dialog.AcceptSelection()
		return true
	case fyne.KeyEscape:
		h.dialog.CancelDialog()
		return true
	case fyne.KeyBackspace:
		h.dialog.BackspaceSearch()
		return true
	case fyne.KeyDelete:
		// Plain Delete only: Shift+Delete arrives here as a folded Cut shortcut.
		if !modifiers.None() {
			return false
		}
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
	if unicode.IsPrint(r) && !unicode.IsControl(r) {
		h.dialog.AppendToSearch(string(r))
		return true
	}
	return false
}
