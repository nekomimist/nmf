package keymanager

import "fyne.io/fyne/v2"

type MaintenanceDialogInterface interface {
	Scan()
	Apply()
	Cancel()
}

type MaintenanceDialogKeyHandler struct {
	dialog MaintenanceDialogInterface
}

func NewMaintenanceDialogKeyHandler(dialog MaintenanceDialogInterface) *MaintenanceDialogKeyHandler {
	return &MaintenanceDialogKeyHandler{dialog: dialog}
}

func (h *MaintenanceDialogKeyHandler) GetName() string {
	return "MaintenanceDialog"
}

func (h *MaintenanceDialogKeyHandler) OnKeyActivated(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	if ev == nil {
		return false
	}
	switch ev.Name {
	case fyne.KeyEscape:
		h.dialog.Cancel()
		return true
	case fyne.KeyReturn, fyne.KeyEnter:
		h.dialog.Apply()
		return true
	case fyne.KeyF5:
		h.dialog.Scan()
		return true
	default:
		return false
	}
}

func (h *MaintenanceDialogKeyHandler) OnTypedRune(r rune, modifiers ModifierState) bool {
	return false
}
