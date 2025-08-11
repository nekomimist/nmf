package keymanager

import (
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// HistoryDialogInterface defines the interface needed by HistoryDialogKeyHandler
type HistoryDialogInterface interface {
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

// HistoryDialogKeyHandler handles keyboard events for the navigation history dialog
type HistoryDialogKeyHandler struct {
	historyDialog HistoryDialogInterface
	shiftPressed  bool
	ctrlPressed   bool
	debugPrint    func(format string, args ...interface{})
}

// NewHistoryDialogKeyHandler creates a new history dialog key handler
func NewHistoryDialogKeyHandler(hd HistoryDialogInterface, debugPrint func(format string, args ...interface{})) *HistoryDialogKeyHandler {
	return &HistoryDialogKeyHandler{
		historyDialog: hd,
		debugPrint:    debugPrint,
	}
}

// GetName returns the name of this handler
func (hh *HistoryDialogKeyHandler) GetName() string {
	return "HistoryDialog"
}

// OnKeyDown handles key press events
func (hh *HistoryDialogKeyHandler) OnKeyDown(ev *fyne.KeyEvent) bool {
	switch ev.Name {
	case desktop.KeyShiftLeft, desktop.KeyShiftRight:
		hh.shiftPressed = true
		hh.debugPrint("HistoryDialog: Shift key pressed (state: %t)", hh.shiftPressed)
		return true

	case desktop.KeyControlLeft, desktop.KeyControlRight:
		hh.ctrlPressed = true
		hh.debugPrint("HistoryDialog: Ctrl key pressed (state: %t)", hh.ctrlPressed)
		return true

	case fyne.KeyF:
		// Ctrl+F - Focus search
		if hh.ctrlPressed {
			hh.historyDialog.FocusSearch()
			return true
		}
	}

	return false
}

// OnKeyUp handles key release events
func (hh *HistoryDialogKeyHandler) OnKeyUp(ev *fyne.KeyEvent) bool {
	switch ev.Name {
	case desktop.KeyShiftLeft, desktop.KeyShiftRight:
		hh.shiftPressed = false
		hh.debugPrint("HistoryDialog: Shift key released (state: %t)", hh.shiftPressed)
		return true

	case desktop.KeyControlLeft, desktop.KeyControlRight:
		hh.ctrlPressed = false
		hh.debugPrint("HistoryDialog: Ctrl key released (state: %t)", hh.ctrlPressed)
		return true
	}

	return false
}

// OnTypedKey handles typed key events in focusless mode
func (hh *HistoryDialogKeyHandler) OnTypedKey(ev *fyne.KeyEvent) bool {
	hh.debugPrint("HistoryDialog: OnTypedKey %s", ev.Name)

	switch ev.Name {
	case fyne.KeyUp:
		if hh.shiftPressed {
			hh.historyDialog.MoveToTop()
		} else {
			hh.historyDialog.MoveUp()
		}
		return true

	case fyne.KeyDown:
		if hh.shiftPressed {
			hh.historyDialog.MoveToBottom()
		} else {
			hh.historyDialog.MoveDown()
		}
		return true

	case fyne.KeySpace:
		// Select current item
		hh.historyDialog.SelectCurrentItem()
		return true

	case fyne.KeyReturn:
		// Accept current selection and close dialog
		hh.historyDialog.AcceptSelection()
		return true

	case fyne.KeyEscape:
		// Cancel dialog
		hh.historyDialog.CancelDialog()
		return true

	case fyne.KeyBackspace:
		// Remove last character from search
		hh.historyDialog.BackspaceSearch()
		return true

	case fyne.KeyDelete:
		// Clear entire search
		hh.historyDialog.ClearSearch()
		return true

	default:
		// Non-handled key
	}

	return false
}

// OnTypedRune handles text input to update the search field
func (hh *HistoryDialogKeyHandler) OnTypedRune(r rune) bool {
	// Accept printable, non-control runes
	if unicode.IsPrint(r) && !unicode.IsControl(r) {
		hh.historyDialog.AppendToSearch(string(r))
		return true
	}
	return false
}
