package keymanager

import (
	"unicode"

	"fyne.io/fyne/v2"
)

// DirectoryJumpDialogInterface defines the interface needed by DirectoryJumpDialogKeyHandler.
type DirectoryJumpDialogInterface interface {
	MoveUp()
	MoveDown()
	MoveToTop()
	MoveToBottom()

	ClearSearch()
	AppendToSearch(char string)
	BackspaceSearch()
	CopySelectedShortcutToSearch()

	SelectCurrentItem()
	AcceptSelection()
	CancelDialog()
}

// DirectoryJumpDialogKeyHandler handles keyboard events for the directory jump dialog.
type DirectoryJumpDialogKeyHandler struct {
	dialog     DirectoryJumpDialogInterface
	debugPrint func(format string, args ...interface{})
}

// NewDirectoryJumpDialogKeyHandler creates a new directory jump dialog key handler.
func NewDirectoryJumpDialogKeyHandler(d DirectoryJumpDialogInterface, debugPrint func(format string, args ...interface{})) *DirectoryJumpDialogKeyHandler {
	return &DirectoryJumpDialogKeyHandler{
		dialog:     d,
		debugPrint: debugPrint,
	}
}

// GetName returns the handler name.
func (h *DirectoryJumpDialogKeyHandler) GetName() string {
	return "DirectoryJumpDialog"
}

// OnKeyActivated handles key activations.
func (h *DirectoryJumpDialogKeyHandler) OnKeyActivated(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	h.debugPrint("DirectoryJumpDialog: OnKeyActivated %s", ev.Name)

	switch ev.Name {
	case fyne.KeyH:
		if modifiers.CtrlPressed {
			h.dialog.BackspaceSearch()
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
		// Plain Delete only: Shift+Delete arrives here as a folded Cut shortcut.
		if !modifiers.None() {
			return false
		}
		h.dialog.ClearSearch()
		return true
	case fyne.KeyTab:
		h.dialog.CopySelectedShortcutToSearch()
		return true
	}
	return false
}

// OnTypedRune handles text input to update the search field.
func (h *DirectoryJumpDialogKeyHandler) OnTypedRune(r rune, modifiers ModifierState) bool {
	if modifiers.AltPressed {
		return true
	}
	if unicode.IsPrint(r) && !unicode.IsControl(r) {
		h.dialog.AppendToSearch(string(r))
		return true
	}
	return false
}
