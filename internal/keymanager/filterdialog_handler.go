package keymanager

import (
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// FilterDialogInterface defines the interface needed by FilterDialogKeyHandler
type FilterDialogInterface interface {
	// List navigation
	MoveUp()
	MoveDown()
	MoveToTop()
	MoveToBottom()

	// Search functionality
	FocusSearch()
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
	shiftPressed bool
	ctrlPressed  bool
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
func (fh *FilterDialogKeyHandler) OnKeyDown(ev *fyne.KeyEvent) bool {
	switch ev.Name {
	case desktop.KeyShiftLeft, desktop.KeyShiftRight:
		fh.shiftPressed = true
		fh.debugPrint("FilterDialog: Shift key pressed (state: %t)", fh.shiftPressed)
		return true

	case desktop.KeyControlLeft, desktop.KeyControlRight:
		fh.ctrlPressed = true
		fh.debugPrint("FilterDialog: Ctrl key pressed (state: %t)", fh.ctrlPressed)
		return true

	case fyne.KeyF:
		// Ctrl+F - Focus search
		if fh.ctrlPressed {
			fh.filterDialog.FocusSearch()
			return true
		}
	}

	return false
}

// OnKeyUp handles key release events
func (fh *FilterDialogKeyHandler) OnKeyUp(ev *fyne.KeyEvent) bool {
	switch ev.Name {
	case desktop.KeyShiftLeft, desktop.KeyShiftRight:
		fh.shiftPressed = false
		fh.debugPrint("FilterDialog: Shift key released (state: %t)", fh.shiftPressed)
		return true

	case desktop.KeyControlLeft, desktop.KeyControlRight:
		fh.ctrlPressed = false
		fh.debugPrint("FilterDialog: Ctrl key released (state: %t)", fh.ctrlPressed)
		return true
	}

	return false
}

// OnTypedKey handles typed key events in focusless mode
func (fh *FilterDialogKeyHandler) OnTypedKey(ev *fyne.KeyEvent) bool {
	fh.debugPrint("FilterDialog: OnTypedKey %s", ev.Name)

	switch ev.Name {
	case fyne.KeyUp:
		if fh.shiftPressed {
			fh.filterDialog.MoveToTop()
		} else {
			fh.filterDialog.MoveUp()
		}
		return true

	case fyne.KeyDown:
		if fh.shiftPressed {
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
func (fh *FilterDialogKeyHandler) OnTypedRune(r rune) bool {
	// Accept printable, non-control runes
	if unicode.IsPrint(r) && !unicode.IsControl(r) {
		fh.filterDialog.AppendToSearch(string(r))
		return true
	}
	return false
}
