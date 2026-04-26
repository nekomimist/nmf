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
	CopySelectedPathToSearch()

	SelectCurrentItem()
	AcceptSelection()
	AcceptShortcut(shortcut string)
	CancelDialog()
}

// DirectoryJumpDialogKeyHandler handles keyboard events for the directory jump dialog.
type DirectoryJumpDialogKeyHandler struct {
	dialog       DirectoryJumpDialogInterface
	debugPrint   func(format string, args ...interface{})
	skipNextRune bool
}

// NewDirectoryJumpDialogKeyHandler creates a new directory jump dialog key handler.
func NewDirectoryJumpDialogKeyHandler(d DirectoryJumpDialogInterface, debugPrint func(format string, args ...interface{})) *DirectoryJumpDialogKeyHandler {
	return &DirectoryJumpDialogKeyHandler{
		dialog:       d,
		debugPrint:   debugPrint,
		skipNextRune: true,
	}
}

// GetName returns the handler name.
func (h *DirectoryJumpDialogKeyHandler) GetName() string {
	return "DirectoryJumpDialog"
}

// OnKeyDown handles key press events.
func (h *DirectoryJumpDialogKeyHandler) OnKeyDown(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	if modifiers.AltPressed {
		shortcut := string(ev.Name)
		if shortcut != "" {
			h.dialog.AcceptShortcut(shortcut)
			return true
		}
	}
	return false
}

// OnKeyUp handles key release events.
func (h *DirectoryJumpDialogKeyHandler) OnKeyUp(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	return false
}

// OnTypedKey handles non-text keys.
func (h *DirectoryJumpDialogKeyHandler) OnTypedKey(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	h.debugPrint("DirectoryJumpDialog: OnTypedKey %s", ev.Name)

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

// OnTypedRune handles text input to update the search field.
func (h *DirectoryJumpDialogKeyHandler) OnTypedRune(r rune, modifiers ModifierState) bool {
	if h.skipNextRune {
		// Swallow the Shift+J rune that opened the dialog.
		h.skipNextRune = false
		return true
	}
	if modifiers.AltPressed {
		return true
	}
	if unicode.IsPrint(r) && !unicode.IsControl(r) {
		h.dialog.AppendToSearch(string(r))
		return true
	}
	return false
}
