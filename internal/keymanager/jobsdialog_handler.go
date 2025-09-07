package keymanager

import (
	"fyne.io/fyne/v2"
)

// JobsDialogInterface defines navigation and actions for the Jobs dialog
type JobsDialogInterface interface {
	MoveUp()
	MoveDown()
	MoveToTop()
	MoveToBottom()
	CancelSelected()
	CloseDialog()
}

// JobsDialogKeyHandler handles keys while Jobs dialog is open
type JobsDialogKeyHandler struct {
	dlg        JobsDialogInterface
	debugPrint func(format string, args ...interface{})
}

func NewJobsDialogKeyHandler(d JobsDialogInterface, debugPrint func(format string, args ...interface{})) *JobsDialogKeyHandler {
	return &JobsDialogKeyHandler{dlg: d, debugPrint: debugPrint}
}

func (h *JobsDialogKeyHandler) GetName() string { return "JobsDialog" }

func (h *JobsDialogKeyHandler) OnKeyDown(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	return false
}
func (h *JobsDialogKeyHandler) OnKeyUp(ev *fyne.KeyEvent, modifiers ModifierState) bool { return false }

func (h *JobsDialogKeyHandler) OnTypedKey(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	switch ev.Name {
	case fyne.KeyUp:
		if modifiers.ShiftPressed {
			h.dlg.MoveToTop()
		} else {
			h.dlg.MoveUp()
		}
		return true
	case fyne.KeyDown:
		if modifiers.ShiftPressed {
			h.dlg.MoveToBottom()
		} else {
			h.dlg.MoveDown()
		}
		return true
	case fyne.KeyDelete:
		h.dlg.CancelSelected()
		return true
	case fyne.KeyEscape:
		h.dlg.CloseDialog()
		return true
	}
	return false
}

func (h *JobsDialogKeyHandler) OnTypedRune(r rune, modifiers ModifierState) bool { return false }
