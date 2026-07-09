package keymanager

// SortDialogInterface defines the interface needed by SortDialogKeyHandler
type SortDialogInterface interface {
	// Field navigation
	MoveToPreviousField()
	MoveToNextField()

	// Field interaction
	ToggleCurrentField()

	// Dialog control
	AcceptSettings()
	CancelDialog()

	// Shortcut methods
	SetSortByName()
	SetSortBySize()
	SetSortByModified()
	SetSortByExtension()
	ToggleSortOrder()
	ToggleDirectoriesFirst()
}

// SortDialogKeyHandler handles keyboard events for the sort configuration dialog
type SortDialogKeyHandler struct {
	*dialogKeyHandler
}

// NewSortDialogHandler creates a new sort dialog keyboard handler
func NewSortDialogHandler(sortDialog SortDialogInterface,
	debugPrint func(format string, args ...interface{})) *SortDialogKeyHandler {
	base := newDialogKeyHandler("SortDialog", debugPrint, []dialogBinding{
		// Tab navigation between fields.
		{"Tab", sortDialog.MoveToNextField},
		{"S-Tab", sortDialog.MoveToPreviousField},

		// Space: toggle current field (radio button/checkbox).
		{"Space", sortDialog.ToggleCurrentField},

		// Enter: apply settings.
		{"Return", sortDialog.AcceptSettings},

		// Escape: cancel dialog.
		{"Escape", sortDialog.CancelDialog},

		// Number keys for sort type shortcuts.
		{"1", sortDialog.SetSortByName},
		{"2", sortDialog.SetSortBySize},
		{"3", sortDialog.SetSortByModified},
		{"4", sortDialog.SetSortByExtension},
	}).withRune(func(r rune, modifiers ModifierState) bool {
		switch r {
		case 'o', 'O':
			// O: toggle sort order
			sortDialog.ToggleSortOrder()
			return true
		case 'd', 'D':
			// D: toggle directories first
			sortDialog.ToggleDirectoriesFirst()
			return true
		}
		return false
	})
	return &SortDialogKeyHandler{dialogKeyHandler: base}
}
