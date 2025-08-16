package keymanager

import (
	"unicode"

	"fyne.io/fyne/v2"
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
	historyDialog HistoryDialogInterface
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
func (hh *HistoryDialogKeyHandler) OnKeyDown(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	switch ev.Name {
	case fyne.KeyF:
		// Ctrl+F - Search functionality handled by focusless design
		if modifiers.CtrlPressed {
			return true
		}

	case fyne.KeyReturn:
		if modifiers.CtrlPressed {
			// Ctrl+Enter - Accept direct path navigation
			hh.historyDialog.AcceptDirectPathNavigation()
		}
		return true
	}

	return false
}

// OnKeyUp handles key release events
func (hh *HistoryDialogKeyHandler) OnKeyUp(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	// Modifier key state is managed by KeyManager
	return false
}

// OnTypedKey handles typed key events in focusless mode
func (hh *HistoryDialogKeyHandler) OnTypedKey(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	hh.debugPrint("HistoryDialog: OnTypedKey %s", ev.Name)

	switch ev.Name {
	case fyne.KeyUp:
		if modifiers.ShiftPressed {
			hh.historyDialog.MoveToTop()
		} else {
			hh.historyDialog.MoveUp()
		}
		return true

	case fyne.KeyDown:
		if modifiers.ShiftPressed {
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
		// Enter - Accept current selection and close dialog
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

	case fyne.KeyTab:
		// Copy selected path to search entry
		hh.historyDialog.CopySelectedPathToSearch()
		return true

	default:
		// Non-handled key
	}

	return false
}

// OnTypedRune handles text input to update the search field
func (hh *HistoryDialogKeyHandler) OnTypedRune(r rune, modifiers ModifierState) bool {
	// Accept printable, non-control runes
	if unicode.IsPrint(r) && !unicode.IsControl(r) {
		hh.historyDialog.AppendToSearch(string(r))
		return true
	}
	return false
}
