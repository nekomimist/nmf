package keymanager

import (
	"unicode"
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
	*dialogKeyHandler
}

// NewDirectoryJumpDialogKeyHandler creates a new directory jump dialog key handler.
func NewDirectoryJumpDialogKeyHandler(d DirectoryJumpDialogInterface, debugPrint func(format string, args ...interface{})) *DirectoryJumpDialogKeyHandler {
	base := newDialogKeyHandler("DirectoryJumpDialog", debugPrint, []dialogBinding{
		{"C-H", d.BackspaceSearch},

		{"Up", d.MoveUp},
		{"S-Up", d.MoveToTop},
		{"Down", d.MoveDown},
		{"S-Down", d.MoveToBottom},

		{"Space", d.SelectCurrentItem},
		{"Return", d.AcceptSelection},
		{"Escape", d.CancelDialog},
		{"Backspace", d.BackspaceSearch},
		// Plain Delete only: Shift+Delete arrives as a folded Cut shortcut and
		// has no binding here, so it falls through unmatched.
		{"Delete", d.ClearSearch},
		{"Tab", d.CopySelectedShortcutToSearch},
	}).withRune(func(r rune, modifiers ModifierState) bool {
		if modifiers.AltPressed {
			return true
		}
		if unicode.IsPrint(r) && !unicode.IsControl(r) {
			d.AppendToSearch(string(r))
			return true
		}
		return false
	})
	return &DirectoryJumpDialogKeyHandler{dialogKeyHandler: base}
}
