package keymanager

import (
	"fyne.io/fyne/v2"
)

// QuitDialogInterface defines the interface needed by QuitConfirmDialogKeyHandler
type QuitDialogInterface interface {
	// Dialog control
	ConfirmQuit()
	CancelQuit()
	DefaultQuitAction()
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

// OnKeyActivated handles key activations
func (qh *QuitConfirmDialogKeyHandler) OnKeyActivated(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	if !modifiers.None() {
		// Modified combos (e.g. Ctrl+Y arriving as a folded Redo shortcut) are
		// not dialog answers; consume them so they do not reach MainScreen.
		qh.debugPrint("QuitConfirmDialog: Consuming modified key event: %s", ev.Name)
		return true
	}
	switch ev.Name {
	case fyne.KeyReturn:
		fallthrough
	case fyne.KeyEnter:
		// Enter follows the dialog's current default action.
		qh.debugPrint("QuitConfirmDialog: Enter detected - running default action")
		qh.quitDialog.DefaultQuitAction()

	case fyne.KeyY:
		// Y - Confirm quit
		qh.debugPrint("QuitConfirmDialog: Y detected - confirming quit")
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
