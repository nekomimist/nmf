package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/keymanager"
)

// CommandErrorDialog displays script failures without terminating the app.
// All methods are confined to the UI thread.
type CommandErrorDialog struct {
	keyManager *keymanager.KeyManager
	token      keymanager.HandlerToken
	parent     fyne.Window
	dialog     *dialog.CustomDialog
	sink       *KeySink
	label      *widget.Label
	scroll     *container.Scroll
	copyButton *widget.Button
	okButton   *widget.Button
	onClosed   func()
	closed     bool
}

func NewCommandErrorDialog(action, details string, km *keymanager.KeyManager) *CommandErrorDialog {
	d := &CommandErrorDialog{keyManager: km}
	d.label = widget.NewLabel(commandErrorText(action, details))
	d.label.Wrapping = fyne.TextWrapBreak
	d.label.TextStyle.Monospace = true
	d.scroll = container.NewVScroll(d.label)
	d.scroll.SetMinSize(metricsSize(320, 120))
	d.copyButton = dialogAuxButton("Copy details", theme.ContentCopyIcon(), d.CopyDetails)
	d.okButton = dialogConfirmButton("OK", d.Close)
	return d
}

func commandErrorText(action, details string) string {
	return "Action: " + action +
		"\n\nCommand execution was interrupted. NMF can continue.\n" +
		"Actions already completed have not been undone.\n\n" + details
}

// Append keeps failures from one script chain in a single acknowledgement
// dialog instead of stacking multiple modal input owners.
func (d *CommandErrorDialog) Append(action, details string) {
	d.label.SetText(d.label.Text + "\n\n---\n\n" + commandErrorText(action, details))
}

func (d *CommandErrorDialog) Show(parent fyne.Window, onClosed func()) {
	d.parent = parent
	d.onClosed = onClosed
	content := container.NewBorder(nil, dialogButtonBar(d.copyButton, d.okButton), nil, nil, d.scroll)
	d.sink = NewKeySink(content, d.keyManager, WithTabCapture(true), WithTapFocus(true))
	d.token = d.keyManager.PushHandler(keymanager.NewCommandErrorDialogHandler(d))
	d.dialog = dialog.NewCustomWithoutButtons("Starlark command error", d.sink, parent)
	d.dialog.SetOnClosed(d.Close)
	size := metricsSize(760, 480)
	if parent != nil {
		viewport := parent.Canvas().Size()
		size.Width = min(size.Width, viewport.Width*0.9)
		size.Height = min(size.Height, viewport.Height*0.9)
	}
	d.dialog.Resize(size)
	d.dialog.Show()
	d.refocus()
}

func (d *CommandErrorDialog) CopyDetails() {
	if app := fyne.CurrentApp(); app != nil && app.Clipboard() != nil {
		app.Clipboard().SetContent(d.label.Text)
	}
	// Buttons unfocus the canvas before running their callbacks.
	d.refocus()
}

func (d *CommandErrorDialog) ScrollDetails(lines int) {
	lineHeight := d.label.Theme().Size(theme.SizeNameText) + d.label.Theme().Size(theme.SizeNameLineSpacing)
	d.scroll.ScrollToOffset(fyne.NewPos(0, d.scroll.Offset.Y+float32(lines)*lineHeight))
}

func (d *CommandErrorDialog) refocus() {
	if !d.closed && d.parent != nil && d.sink != nil {
		d.parent.Canvas().Focus(d.sink)
	}
}

func (d *CommandErrorDialog) Close() {
	if d.closed {
		return
	}
	d.closed = true
	deferDialogClose(d.keyManager, "command-error.close", d.Dismiss)
}

// Dismiss also supports synchronous cleanup when the owning window closes.
func (d *CommandErrorDialog) Dismiss() {
	d.closed = true
	if d.token != 0 {
		d.keyManager.RemoveHandler(d.token)
		d.token = 0
	}
	if d.dialog != nil {
		dlg := d.dialog
		d.dialog = nil
		dlg.Hide()
	}
	if d.onClosed != nil {
		callback := d.onClosed
		d.onClosed = nil
		callback()
	}
}
