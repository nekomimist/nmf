package ui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/keymanager"
)

// QuitConfirmDialog represents a quit confirmation dialog
type QuitConfirmDialog struct {
	keyManager *keymanager.KeyManager
	kmToken    keymanager.HandlerToken
	debugPrint func(format string, args ...interface{})
	dialog     dialog.Dialog
	callback   func(bool) // Callback function for quit confirmation
	parent     fyne.Window
	closed     bool // Prevent double-close/pop
	sink       *KeySink
	activeJobs int
}

// NewQuitConfirmDialog creates a new quit confirmation dialog
func NewQuitConfirmDialog(keyManager *keymanager.KeyManager, debugPrint func(format string, args ...interface{}), activeJobs int) *QuitConfirmDialog {
	return &QuitConfirmDialog{
		keyManager: keyManager,
		debugPrint: debugPrint,
		activeJobs: activeJobs,
	}
}

// ShowDialog shows the quit confirmation dialog
func (qcd *QuitConfirmDialog) ShowDialog(parent fyne.Window, callback func(bool)) {
	// Store callback and parent for later use
	qcd.callback = callback
	qcd.parent = parent

	// Create quit dialog key handler
	quitHandler := keymanager.NewQuitConfirmDialogKeyHandler(qcd, qcd.debugPrint)
	qcd.kmToken = qcd.keyManager.PushHandler(quitHandler)

	// Create message content
	message := widget.NewLabel(qcd.message())
	message.Alignment = fyne.TextAlignCenter
	message.Wrapping = fyne.TextWrapWord

	content := container.NewVBox(
		message,
		quitDialogSpacer(quitDialogGap),
		qcd.buttonRow(),
		quitDialogSpacer(quitDialogBottom),
	)
	fixedContent := container.NewGridWrap(metricsSize(quitDialogWidth, quitDialogHeight), content)

	// Wrap content with KeySink to capture keys and forward to KeyManager
	qcd.sink = NewKeySink(fixedContent, qcd.keyManager, WithTabCapture(false))

	// Set appropriate content size
	qcd.sink.Resize(metricsSize(quitDialogWidth, quitDialogHeight))

	qcd.dialog = dialog.NewCustomWithoutButtons("Quit Application", qcd.sink, parent)
	qcd.dialog.SetOnClosed(func() {
		qcd.CancelQuit()
	})

	// Show the dialog
	qcd.dialog.Show()

	// Ensure focus goes to our KeySink so keyboard events are captured
	if qcd.parent != nil && qcd.sink != nil {
		qcd.parent.Canvas().Focus(qcd.sink)
	}
}

// QuitDialogInterface implementation

func (qcd *QuitConfirmDialog) message() string {
	if qcd.activeJobs > 0 {
		return fmt.Sprintf("There are %d pending or running job(s). Quit anyway?", qcd.activeJobs)
	}
	return "Are you sure you want to quit the file manager?"
}

func (qcd *QuitConfirmDialog) buttonTexts() (confirmText string, cancelText string) {
	if qcd.activeJobs > 0 {
		return "Quit Anyway", "No"
	}
	return "Yes", "No"
}

func (qcd *QuitConfirmDialog) buttonRow() fyne.CanvasObject {
	confirmText, cancelText := qcd.buttonTexts()
	cancel := dialogCancelButton(cancelText, qcd.CancelQuit)
	if qcd.activeJobs <= 0 {
		return dialogButtonBar(cancel, dialogConfirmButton(confirmText, qcd.ConfirmQuit))
	}
	// With active jobs, Enter runs CancelQuit; blue marks that default.
	cancel.Importance = widget.HighImportance
	return dialogButtonBar(cancel, dialogDangerButton(confirmText, qcd.ConfirmQuit))
}

func quitDialogSpacer(height float32) fyne.CanvasObject {
	return container.NewGridWrap(
		metricsSize(quitDialogWidth, height),
		canvas.NewRectangle(color.Transparent),
	)
}

// DefaultQuitAction runs the action assigned to Enter.
func (qcd *QuitConfirmDialog) DefaultQuitAction() {
	if qcd.activeJobs > 0 {
		qcd.CancelQuit()
		return
	}
	qcd.ConfirmQuit()
}

// ConfirmQuit confirms the quit action
func (qcd *QuitConfirmDialog) ConfirmQuit() {
	if qcd.closed {
		return
	}
	qcd.closed = true

	qcd.debugPrint("QuitConfirmDialog: User confirmed quit via keyboard")

	deferDialogClose(qcd.keyManager, "quit.confirm", func() {
		// Pop the handler first
		qcd.keyManager.RemoveHandler(qcd.kmToken)

		// Hide the dialog
		if qcd.dialog != nil {
			qcd.dialog.Hide()
		}

		// Call the callback with true (confirmed)
		if qcd.callback != nil {
			qcd.callback(true)
		}
	})
}

// CancelQuit cancels the quit action
func (qcd *QuitConfirmDialog) CancelQuit() {
	if qcd.closed {
		return
	}
	qcd.closed = true

	qcd.debugPrint("QuitConfirmDialog: User cancelled quit via keyboard")

	deferDialogClose(qcd.keyManager, "quit.cancel", func() {
		// Pop the handler first
		qcd.keyManager.RemoveHandler(qcd.kmToken)

		// Hide the dialog
		if qcd.dialog != nil {
			qcd.dialog.Hide()
		}

		// Call the callback with false (cancelled)
		if qcd.callback != nil {
			qcd.callback(false)
		}
	})
}
