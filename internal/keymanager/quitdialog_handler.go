package keymanager

import (
	"fyne.io/fyne/v2"
)

// QuitDialogInterface defines the interface needed by QuitConfirmDialogKeyHandler
type QuitDialogInterface interface {
	// Dialog control
	ConfirmQuit()
	CancelQuit()
}

// QuitConfirmDialogKeyHandler handles keyboard events for the quit confirmation dialog
type QuitConfirmDialogKeyHandler struct {
	quitDialog QuitDialogInterface
	debugPrint func(format string, args ...interface{})
}

// NewQuitConfirmDialogKeyHandler creates a new quit confirmation dialog key handler
func NewQuitConfirmDialogKeyHandler(qd QuitDialogInterface, debugPrint func(format string, args ...interface{})) *QuitConfirmDialogKeyHandler {
	return &QuitConfirmDialogKeyHandler{
		quitDialog: qd,
		debugPrint: debugPrint,
	}
}

// GetName returns the name of this handler
func (qh *QuitConfirmDialogKeyHandler) GetName() string {
	return "QuitConfirmDialog"
}

// OnKeyDown handles key press events
func (qh *QuitConfirmDialogKeyHandler) OnKeyDown(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	// Consume all key down events to prevent them from reaching MainScreen
	return true
}

// OnKeyUp handles key release events
func (qh *QuitConfirmDialogKeyHandler) OnKeyUp(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	// Consume all key up events to prevent them from reaching MainScreen
	return true
}

// OnTypedKey handles typed key events
func (qh *QuitConfirmDialogKeyHandler) OnTypedKey(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	switch ev.Name {
	case fyne.KeyReturn:
		fallthrough
	case fyne.KeyY:
		// Y/Enter - Confirm quit
		qh.debugPrint("QuitConfirmDialog: Enter detected - confirming quit")
		qh.quitDialog.ConfirmQuit()

	case fyne.KeyEscape:
		fallthrough
	case fyne.KeyN:
		// N/Escape - Cancel quit
		qh.debugPrint("QuitConfirmDialog: Escape detected - cancelling quit")
		qh.quitDialog.CancelQuit()

	default:
		// Consume all other typed key events to prevent them from reaching MainScreen
		qh.debugPrint("QuitConfirmDialog: Consuming key event: %s", ev.Name)
	}
	return true
}

// OnTypedRune handles text input (consume all to prevent reaching MainScreen)
func (qh *QuitConfirmDialogKeyHandler) OnTypedRune(r rune, modifiers ModifierState) bool {
	// Consume all rune events to prevent them from reaching MainScreen
	return true
}
