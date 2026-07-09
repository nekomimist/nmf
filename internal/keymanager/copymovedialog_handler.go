package keymanager

import (
	"unicode"
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
	*dialogKeyHandler
}

// NewCopyMoveDialogKeyHandler creates a new copy/move dialog key handler
func NewCopyMoveDialogKeyHandler(d CopyMoveDialogInterface, debugPrint func(format string, args ...interface{})) *CopyMoveDialogKeyHandler {
	base := newDialogKeyHandler("CopyMoveDialog", debugPrint, []dialogBinding{
		{"C-H", d.BackspaceSearch},
		{"C-N", d.OpenDestination},

		{"Up", d.MoveUp},
		{"S-Up", d.MoveToTop},
		{"Down", d.MoveDown},
		{"S-Down", d.MoveToBottom},

		{"Right", d.ScrollSelectedRight},
		{"Left", d.ResetHorizontalScroll},
		{"Space", d.SelectCurrentItem},

		{"Return", d.AcceptSelection},
		{"C-Return", d.AcceptDirectPath},

		{"Escape", d.CancelDialog},
		{"Backspace", d.BackspaceSearch},
		// Plain Delete only: Shift+Delete arrives as a folded Cut shortcut and
		// has no binding here, so it falls through unmatched.
		{"Delete", d.ClearSearch},
		{"Tab", d.CopySelectedPathToSearch},
	}).withRune(func(r rune, modifiers ModifierState) bool {
		if unicode.IsPrint(r) && !unicode.IsControl(r) {
			d.AppendToSearch(string(r))
			return true
		}
		return false
	})
	return &CopyMoveDialogKeyHandler{dialogKeyHandler: base}
}
