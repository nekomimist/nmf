package keymanager

import (
	"unicode"
)

// CompareDialogInterface defines the interface needed by Compare dialog key handler.
type CompareDialogInterface interface {
	MoveUp()
	MoveDown()
	MoveToTop()
	MoveToBottom()

	ClearSearch()
	AppendToSearch(char string)
	BackspaceSearch()
	CopySelectedPathToSearch()
	ScrollSelectedRight()
	ResetHorizontalScroll()

	SelectCurrentItem()
	AcceptSelection()
	AcceptDirectPath()
	CancelDialog()
	NextMethod()
	PreviousMethod()
	SelectMissingOrNewer()
	SelectMissing()
	SelectNewer()
	SelectSizeEqual()
	SelectSizeTimeEqual()
	SelectSizeContentEqual()
}

// CompareDialogKeyHandler handles keyboard events for the compare dialog.
type CompareDialogKeyHandler struct {
	*dialogKeyHandler
}

// NewCompareDialogKeyHandler creates a new compare dialog key handler.
func NewCompareDialogKeyHandler(d CompareDialogInterface, debugPrint func(format string, args ...interface{})) *CompareDialogKeyHandler {
	base := newDialogKeyHandler("CompareDialog", debugPrint, []dialogBinding{
		{"A-U", d.SelectMissingOrNewer},
		{"A-M", d.SelectMissing},
		{"A-N", d.SelectNewer},
		{"A-S", d.SelectSizeEqual},
		{"A-T", d.SelectSizeTimeEqual},
		{"A-C", d.SelectSizeContentEqual},

		{"C-H", d.BackspaceSearch},

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

		{"PageDown", d.NextMethod},
		{"PageUp", d.PreviousMethod},
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
	return &CompareDialogKeyHandler{dialogKeyHandler: base}
}
