package keymanager

import (
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

	case fyne.KeySlash:
		// Add slash to search (no special behavior)
		hh.historyDialog.AppendToSearch("/")
		return true

	default:
		// Handle printable characters
		if hh.isPrintableKey(ev) {
			hh.historyDialog.AppendToSearch(hh.keyToString(ev))
			return true
		}
	}

	return false
}

// isPrintableKey checks if the key represents a printable character
func (hh *HistoryDialogKeyHandler) isPrintableKey(ev *fyne.KeyEvent) bool {
	// Handle common printable keys
	switch ev.Name {
	case fyne.KeyA, fyne.KeyB, fyne.KeyC, fyne.KeyD, fyne.KeyE, fyne.KeyF, fyne.KeyG, fyne.KeyH, fyne.KeyI, fyne.KeyJ,
		fyne.KeyK, fyne.KeyL, fyne.KeyM, fyne.KeyN, fyne.KeyO, fyne.KeyP, fyne.KeyQ, fyne.KeyR, fyne.KeyS, fyne.KeyT,
		fyne.KeyU, fyne.KeyV, fyne.KeyW, fyne.KeyX, fyne.KeyY, fyne.KeyZ,
		fyne.Key0, fyne.Key1, fyne.Key2, fyne.Key3, fyne.Key4, fyne.Key5, fyne.Key6, fyne.Key7, fyne.Key8, fyne.Key9,
		fyne.KeyPeriod, fyne.KeyComma, fyne.KeyMinus, fyne.KeySlash, fyne.KeySemicolon, fyne.KeyEqual,
		fyne.KeyLeftBracket, fyne.KeyRightBracket, fyne.KeyBackslash, fyne.KeyApostrophe:
		return true
	}
	return false
}

// keyToString converts a key event to its string representation
func (hh *HistoryDialogKeyHandler) keyToString(ev *fyne.KeyEvent) string {
	switch ev.Name {
	case fyne.KeyA:
		return "a"
	case fyne.KeyB:
		return "b"
	case fyne.KeyC:
		return "c"
	case fyne.KeyD:
		return "d"
	case fyne.KeyE:
		return "e"
	case fyne.KeyF:
		return "f"
	case fyne.KeyG:
		return "g"
	case fyne.KeyH:
		return "h"
	case fyne.KeyI:
		return "i"
	case fyne.KeyJ:
		return "j"
	case fyne.KeyK:
		return "k"
	case fyne.KeyL:
		return "l"
	case fyne.KeyM:
		return "m"
	case fyne.KeyN:
		return "n"
	case fyne.KeyO:
		return "o"
	case fyne.KeyP:
		return "p"
	case fyne.KeyQ:
		return "q"
	case fyne.KeyR:
		return "r"
	case fyne.KeyS:
		return "s"
	case fyne.KeyT:
		return "t"
	case fyne.KeyU:
		return "u"
	case fyne.KeyV:
		return "v"
	case fyne.KeyW:
		return "w"
	case fyne.KeyX:
		return "x"
	case fyne.KeyY:
		return "y"
	case fyne.KeyZ:
		return "z"
	case fyne.Key0:
		return "0"
	case fyne.Key1:
		return "1"
	case fyne.Key2:
		return "2"
	case fyne.Key3:
		return "3"
	case fyne.Key4:
		return "4"
	case fyne.Key5:
		return "5"
	case fyne.Key6:
		return "6"
	case fyne.Key7:
		return "7"
	case fyne.Key8:
		return "8"
	case fyne.Key9:
		return "9"
	case fyne.KeyPeriod:
		return "."
	case fyne.KeyComma:
		return ","
	case fyne.KeyMinus:
		return "-"
	case fyne.KeySlash:
		return "/"
	case fyne.KeySemicolon:
		return ";"
	case fyne.KeyEqual:
		return "="
	case fyne.KeyLeftBracket:
		return "["
	case fyne.KeyRightBracket:
		return "]"
	case fyne.KeyBackslash:
		return "\\"
	case fyne.KeyApostrophe:
		return "'"
	case fyne.KeySpace:
		return " "
	}
	return ""
}
