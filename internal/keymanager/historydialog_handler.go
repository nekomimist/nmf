package keymanager

import (
	"unicode"
)

// HistoryDialogInterface defines the interface needed by HistoryDialogKeyHandler
type HistoryDialogInterface interface {
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
	CopySelectedPathToSearch() // Copy selected path to search entry
	ScrollSelectedRight()
	ResetHorizontalScroll()
	UnpinSelectedPath()

	// Focus management (deprecated in focusless design)
	IsSearchFocused() bool
	FocusList()

	// Selection
	SelectCurrentItem()

	// Dialog control
	AcceptSelection()
	AcceptDirectPathNavigation() // Accept direct path navigation (Ctrl+Enter)
	CancelDialog()
}

// HistoryDialogKeyHandler handles keyboard events for the navigation history dialog
type HistoryDialogKeyHandler struct {
	*dialogKeyHandler
}

// NewHistoryDialogKeyHandler creates a new history dialog key handler
func NewHistoryDialogKeyHandler(hd HistoryDialogInterface, debugPrint func(format string, args ...interface{})) *HistoryDialogKeyHandler {
	base := newDialogKeyHandler("HistoryDialog", debugPrint, []dialogBinding{
		// Ctrl+F: search functionality is handled by the focusless design;
		// swallow it here so it doesn't fall through to MainScreen.
		{"C-F", func() {}},
		{"C-H", hd.BackspaceSearch},
		{"C-D", hd.UnpinSelectedPath},

		{"Up", hd.MoveUp},
		{"S-Up", hd.MoveToTop},
		{"Down", hd.MoveDown},
		{"S-Down", hd.MoveToBottom},

		{"Right", hd.ScrollSelectedRight},
		{"Left", hd.ResetHorizontalScroll},
		{"Space", hd.SelectCurrentItem},

		{"Return", hd.AcceptSelection},
		{"C-Return", hd.AcceptDirectPathNavigation},

		{"Escape", hd.CancelDialog},
		{"Backspace", hd.BackspaceSearch},
		// Plain Delete only: Shift+Delete arrives as a folded Cut shortcut and
		// has no binding here, so it falls through unmatched.
		{"Delete", hd.ClearSearch},
		{"Tab", hd.CopySelectedPathToSearch},
	}).withRune(func(r rune, modifiers ModifierState) bool {
		if unicode.IsPrint(r) && !unicode.IsControl(r) {
			hd.AppendToSearch(string(r))
			return true
		}
		return false
	})
	return &HistoryDialogKeyHandler{dialogKeyHandler: base}
}
