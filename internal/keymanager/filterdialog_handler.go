package keymanager

import (
	"unicode"

	"fyne.io/fyne/v2"
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
	filterDialog FilterDialogInterface
	debugPrint   func(format string, args ...interface{})
}

// NewFilterDialogKeyHandler creates a new filter dialog key handler
func NewFilterDialogKeyHandler(fd FilterDialogInterface, debugPrint func(format string, args ...interface{})) *FilterDialogKeyHandler {
	return &FilterDialogKeyHandler{
		filterDialog: fd,
		debugPrint:   debugPrint,
	}
}

// GetName returns the name of this handler
func (fh *FilterDialogKeyHandler) GetName() string {
	return "FilterDialog"
}

// OnKeyDown handles key press events
func (fh *FilterDialogKeyHandler) OnKeyDown(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	switch ev.Name {
	case fyne.KeyF:
		// Ctrl+F - Search functionality handled by focusless design
		if modifiers.CtrlPressed {
			return true
		}
	}

	return false
}

// OnKeyUp handles key release events
func (fh *FilterDialogKeyHandler) OnKeyUp(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	// Modifier key state is managed by KeyManager
	return false
}

// OnTypedKey handles typed key events in focusless mode
func (fh *FilterDialogKeyHandler) OnTypedKey(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	fh.debugPrint("FilterDialog: OnTypedKey %s", ev.Name)

	switch ev.Name {
	case fyne.KeyUp:
		if modifiers.ShiftPressed {
			fh.filterDialog.MoveToTop()
		} else {
			fh.filterDialog.MoveUp()
		}
		return true

	case fyne.KeyDown:
		if modifiers.ShiftPressed {
			fh.filterDialog.MoveToBottom()
		} else {
			fh.filterDialog.MoveDown()
		}
		return true

	case fyne.KeySpace:
		// Select current item
		fh.filterDialog.SelectCurrentItem()
		return true

	case fyne.KeyReturn:
		// Accept current selection and close dialog
		fh.filterDialog.AcceptSelection()
		return true

	case fyne.KeyEscape:
		// Cancel dialog
		fh.filterDialog.CancelDialog()
		return true

	case fyne.KeyBackspace:
		// Remove last character from search
		fh.filterDialog.BackspaceSearch()
		return true

	case fyne.KeyDelete:
		// Clear entire search
		fh.filterDialog.ClearSearch()
		return true

	default:
		// Non-handled key
	}

	return false
}

// OnTypedRune handles text input to update the search field
func (fh *FilterDialogKeyHandler) OnTypedRune(r rune, modifiers ModifierState) bool {
	// Accept printable, non-control runes
	if unicode.IsPrint(r) && !unicode.IsControl(r) {
		fh.filterDialog.AppendToSearch(string(r))
		return true
	}
	return false
}
