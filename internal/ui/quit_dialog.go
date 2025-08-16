package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/keymanager"
)

// QuitConfirmDialog represents a quit confirmation dialog
type QuitConfirmDialog struct {
	keyManager *keymanager.KeyManager
	debugPrint func(format string, args ...interface{})
	dialog     dialog.Dialog
	callback   func(bool) // Callback function for quit confirmation
	parent     fyne.Window
	closed     bool // Prevent double-close/pop
	sink       *KeySink
}

// NewQuitConfirmDialog creates a new quit confirmation dialog
func NewQuitConfirmDialog(keyManager *keymanager.KeyManager, debugPrint func(format string, args ...interface{})) *QuitConfirmDialog {
	return &QuitConfirmDialog{
		keyManager: keyManager,
		debugPrint: debugPrint,
	}
}

// ShowDialog shows the quit confirmation dialog
func (qcd *QuitConfirmDialog) ShowDialog(parent fyne.Window, callback func(bool)) {
	// Store callback and parent for later use
	qcd.callback = callback
	qcd.parent = parent

	// Create quit dialog key handler
	quitHandler := keymanager.NewQuitConfirmDialogKeyHandler(qcd, qcd.debugPrint)
	qcd.keyManager.PushHandler(quitHandler)

	// Create message content
	message := widget.NewLabel("Are you sure you want to quit the file manager?")
	message.Alignment = fyne.TextAlignCenter

	// Wrap content with KeySink to capture keys and forward to KeyManager
	qcd.sink = NewKeySink(message, qcd.keyManager, WithTabCapture(false))

	// Set appropriate content size
	qcd.sink.Resize(fyne.NewSize(400, 100))

	// Create custom confirm dialog
	qcd.dialog = dialog.NewCustomConfirm(
		"Quit Application",
		"Yes",
		"No",
		qcd.sink,
		func(confirmed bool) {
			if qcd.closed {
				return
			}
			qcd.closed = true

			// Pop the handler first
			qcd.keyManager.PopHandler()

			// Call the callback
			if qcd.callback != nil {
				qcd.callback(confirmed)
			}
		},
		parent,
	)

	// Show the dialog
	qcd.dialog.Show()

	// Ensure focus goes to our KeySink so keyboard events are captured
	if qcd.parent != nil && qcd.sink != nil {
		qcd.parent.Canvas().Focus(qcd.sink)
	}
}

// QuitDialogInterface implementation

// ConfirmQuit confirms the quit action
func (qcd *QuitConfirmDialog) ConfirmQuit() {
	if qcd.closed {
		return
	}
	qcd.closed = true

	qcd.debugPrint("QuitConfirmDialog: User confirmed quit via keyboard")

	// Pop the handler first
	qcd.keyManager.PopHandler()

	// Hide the dialog
	if qcd.dialog != nil {
		qcd.dialog.Hide()
	}

	// Call the callback with true (confirmed)
	if qcd.callback != nil {
		qcd.callback(true)
	}
}

// CancelQuit cancels the quit action
func (qcd *QuitConfirmDialog) CancelQuit() {
	if qcd.closed {
		return
	}
	qcd.closed = true

	qcd.debugPrint("QuitConfirmDialog: User cancelled quit via keyboard")

	// Pop the handler first
	qcd.keyManager.PopHandler()

	// Hide the dialog
	if qcd.dialog != nil {
		qcd.dialog.Hide()
	}

	// Call the callback with false (cancelled)
	if qcd.callback != nil {
		qcd.callback(false)
	}
}
