package keymanager

import (
	"unicode"

	"fyne.io/fyne/v2"
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
	dialog     CompareDialogInterface
	debugPrint func(format string, args ...interface{})
}

// NewCompareDialogKeyHandler creates a new compare dialog key handler.
func NewCompareDialogKeyHandler(d CompareDialogInterface, debugPrint func(format string, args ...interface{})) *CompareDialogKeyHandler {
	return &CompareDialogKeyHandler{dialog: d, debugPrint: debugPrint}
}

func (h *CompareDialogKeyHandler) GetName() string { return "CompareDialog" }

func (h *CompareDialogKeyHandler) OnKeyDown(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	if ev.Name == fyne.KeyReturn && modifiers.CtrlPressed {
		h.dialog.AcceptDirectPath()
		return true
	}
	if modifiers.AltPressed {
		switch ev.Name {
		case fyne.KeyU:
			h.dialog.SelectMissingOrNewer()
			return true
		case fyne.KeyM:
			h.dialog.SelectMissing()
			return true
		case fyne.KeyN:
			h.dialog.SelectNewer()
			return true
		case fyne.KeyS:
			h.dialog.SelectSizeEqual()
			return true
		case fyne.KeyT:
			h.dialog.SelectSizeTimeEqual()
			return true
		case fyne.KeyC:
			h.dialog.SelectSizeContentEqual()
			return true
		}
	}
	return false
}

func (h *CompareDialogKeyHandler) OnKeyUp(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	return false
}

func (h *CompareDialogKeyHandler) OnTypedKey(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	h.debugPrint("CompareDialog: OnTypedKey %s", ev.Name)

	switch ev.Name {
	case fyne.KeyH:
		if modifiers.CtrlPressed {
			h.dialog.BackspaceSearch()
			return true
		}
	case fyne.KeyUp:
		if modifiers.ShiftPressed {
			h.dialog.MoveToTop()
		} else {
			h.dialog.MoveUp()
		}
		return true
	case fyne.KeyDown:
		if modifiers.ShiftPressed {
			h.dialog.MoveToBottom()
		} else {
			h.dialog.MoveDown()
		}
		return true
	case fyne.KeyRight:
		h.dialog.ScrollSelectedRight()
		return true
	case fyne.KeyLeft:
		h.dialog.ResetHorizontalScroll()
		return true
	case fyne.KeySpace:
		h.dialog.SelectCurrentItem()
		return true
	case fyne.KeyReturn:
		h.dialog.AcceptSelection()
		return true
	case fyne.KeyEscape:
		h.dialog.CancelDialog()
		return true
	case fyne.KeyBackspace:
		h.dialog.BackspaceSearch()
		return true
	case fyne.KeyDelete:
		// Plain Delete only: Shift+Delete arrives here as a folded Cut shortcut.
		if !modifiers.None() {
			return false
		}
		h.dialog.ClearSearch()
		return true
	case fyne.KeyTab:
		h.dialog.CopySelectedPathToSearch()
		return true
	case fyne.KeyPageDown:
		h.dialog.NextMethod()
		return true
	case fyne.KeyPageUp:
		h.dialog.PreviousMethod()
		return true
	}
	return false
}

func (h *CompareDialogKeyHandler) OnTypedRune(r rune, modifiers ModifierState) bool {
	if modifiers.AltPressed {
		return true
	}
	if unicode.IsPrint(r) && !unicode.IsControl(r) {
		h.dialog.AppendToSearch(string(r))
		return true
	}
	return false
}
