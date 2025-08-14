package keymanager

import (
	"fyne.io/fyne/v2"
)

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
	sortDialog SortDialogInterface
	debugPrint func(format string, args ...interface{})
}

// NewSortDialogHandler creates a new sort dialog keyboard handler
func NewSortDialogHandler(sortDialog SortDialogInterface,
	debugPrint func(format string, args ...interface{})) *SortDialogKeyHandler {
	return &SortDialogKeyHandler{
		sortDialog: sortDialog,
		debugPrint: debugPrint,
	}
}

// GetName returns the name of this handler
func (h *SortDialogKeyHandler) GetName() string {
	return "SortDialog"
}

// OnKeyDown handles key press events
func (h *SortDialogKeyHandler) OnKeyDown(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	h.debugPrint("SortDialogKeyHandler: OnKeyDown called with key: %s", ev.Name)

	switch ev.Name {
	case fyne.KeyTab:
		// Tab navigation between fields
		if modifiers.ShiftPressed {
			// Shift+Tab: move to previous field
			h.sortDialog.MoveToPreviousField()
		} else {
			// Tab: move to next field
			h.sortDialog.MoveToNextField()
		}
		return true

	case fyne.KeySpace:
		// Space: toggle current field (radio button/checkbox)
		h.sortDialog.ToggleCurrentField()
		return true

	case fyne.KeyReturn, fyne.KeyEnter:
		// Enter: apply settings
		h.sortDialog.AcceptSettings()
		return true

	case fyne.KeyEscape:
		// Escape: cancel dialog
		h.sortDialog.CancelDialog()
		return true

	// Number keys for sort type shortcuts
	case fyne.Key1:
		h.sortDialog.SetSortByName()
		return true
	case fyne.Key2:
		h.sortDialog.SetSortBySize()
		return true
	case fyne.Key3:
		h.sortDialog.SetSortByModified()
		return true
	case fyne.Key4:
		h.sortDialog.SetSortByExtension()
		return true
	}

	return false
}

// OnKeyUp handles key release events
func (h *SortDialogKeyHandler) OnKeyUp(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	// Modifier key state is managed by KeyManager
	return false
}

// OnTypedKey handles typed key events
func (h *SortDialogKeyHandler) OnTypedKey(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	return false
}

// OnTypedRune handles text input runes
func (h *SortDialogKeyHandler) OnTypedRune(r rune, modifiers ModifierState) bool {
	h.debugPrint("SortDialogKeyHandler: OnTypedRune called with rune: %c", r)

	switch r {
	case 'o', 'O':
		// O: toggle sort order
		h.sortDialog.ToggleSortOrder()
		return true
	case 'd', 'D':
		// D: toggle directories first
		h.sortDialog.ToggleDirectoriesFirst()
		return true
	}

	return false
}
