package keymanager

import (
	"unicode"
)

// FilterDialogInterface defines the interface needed by FilterDialogKeyHandler
type FilterDialogInterface interface {
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
	AcceptDirectInput()
	DeleteSelectedEntry()

	// Focus management (deprecated in focusless design)
	IsSearchFocused() bool
	FocusList()

	// Selection
	SelectCurrentItem()

	// Dialog control
	AcceptSelection()
	CancelDialog()
}

// FilterDialogKeyHandler handles keyboard events for the file filter dialog
type FilterDialogKeyHandler struct {
	*dialogKeyHandler
}

// NewFilterDialogKeyHandler creates a new filter dialog key handler
func NewFilterDialogKeyHandler(fd FilterDialogInterface, debugPrint func(format string, args ...interface{})) *FilterDialogKeyHandler {
	base := newDialogKeyHandler("FilterDialog", debugPrint, []dialogBinding{
		// Ctrl+F: search functionality is handled by the focusless design;
		// swallow it here so it doesn't fall through to MainScreen.
		{"C-F", func() {}},
		{"C-H", fd.BackspaceSearch},
		{"C-D", fd.DeleteSelectedEntry},

		{"Up", fd.MoveUp},
		{"S-Up", fd.MoveToTop},
		{"Down", fd.MoveDown},
		{"S-Down", fd.MoveToBottom},

		{"Return", fd.AcceptSelection},
		{"C-Return", fd.AcceptDirectInput},

		{"Escape", fd.CancelDialog},
		{"Backspace", fd.BackspaceSearch},
		// Plain Delete only: Shift+Delete arrives as a folded Cut shortcut and
		// has no binding here, so it falls through unmatched.
		{"Delete", fd.ClearSearch},
	}).withRune(func(r rune, modifiers ModifierState) bool {
		if unicode.IsPrint(r) && !unicode.IsControl(r) {
			fd.AppendToSearch(string(r))
			return true
		}
		return false
	})
	return &FilterDialogKeyHandler{dialogKeyHandler: base}
}
