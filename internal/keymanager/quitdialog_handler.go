package keymanager

import "fyne.io/fyne/v2"

// QuitDialogInterface defines the interface needed by QuitConfirmDialogKeyHandler
type QuitDialogInterface interface {
	// Dialog control
	ConfirmQuit()
	CancelQuit()
	DefaultQuitAction()
}

// QuitConfirmDialogKeyHandler handles keyboard events for the quit confirmation dialog
type QuitConfirmDialogKeyHandler struct {
	*dialogKeyHandler
}

// NewQuitConfirmDialogKeyHandler creates a new quit confirmation dialog key handler
func NewQuitConfirmDialogKeyHandler(qd QuitDialogInterface, debugPrint func(format string, args ...interface{})) *QuitConfirmDialogKeyHandler {
	if debugPrint == nil {
		debugPrint = func(string, ...interface{}) {}
	}

	// Enter follows the dialog's current default action; the same binding
	// covers both Return and the numeric-keypad Enter (keySpec.matches folds
	// them together, same as everywhere else in keymanager).
	acceptDefault := func() {
		debugPrint("QuitConfirmDialog: Enter detected - running default action")
		qd.DefaultQuitAction()
	}
	confirm := func() {
		debugPrint("QuitConfirmDialog: Y detected - confirming quit")
		qd.ConfirmQuit()
	}
	// N/Escape - Cancel quit.
	cancel := func() {
		debugPrint("QuitConfirmDialog: Escape detected - cancelling quit")
		qd.CancelQuit()
	}

	base := newDialogKeyHandler("QuitConfirmDialog", debugPrint, []dialogBinding{
		{"Return", acceptDefault},
		{"Y", confirm},
		{"Escape", cancel},
		{"N", cancel},
	}).withFallback(func(ev *fyne.KeyEvent, modifiers ModifierState) bool {
		// Every other key activation is consumed here, matched or not, so
		// nothing leaks through to MainScreen while the dialog is open
		// (e.g. Ctrl+Y arriving as a folded Redo shortcut is not a dialog
		// answer).
		if !modifiers.None() {
			debugPrint("QuitConfirmDialog: Consuming modified key event: %s", ev.Name)
		} else {
			debugPrint("QuitConfirmDialog: Consuming key event: %s", ev.Name)
		}
		return true
	}).withRune(func(r rune, modifiers ModifierState) bool {
		// Consume all rune events to prevent them from reaching MainScreen.
		return true
	})
	return &QuitConfirmDialogKeyHandler{dialogKeyHandler: base}
}
