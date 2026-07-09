package ui

import (
	"context"
	"errors"
	"path/filepath"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
)

// ArchivePasswordProvider prompts the user for encrypted archive passwords.
type ArchivePasswordProvider struct {
	parent   fyne.Window
	km       *keymanager.KeyManager
	bindings []config.KeyBindingEntry
}

func NewArchivePasswordProvider(parent fyne.Window, km *keymanager.KeyManager, bindings ...[]config.KeyBindingEntry) *ArchivePasswordProvider {
	p := &ArchivePasswordProvider{parent: parent, km: km}
	if len(bindings) > 0 {
		p.bindings = bindings[0]
	}
	return p
}

func (p *ArchivePasswordProvider) GetArchivePassword(ctx context.Context, req fileinfo.ArchivePasswordRequest) (string, error) {
	var mu sync.Mutex
	var password string
	var retErr error
	var finishOnce sync.Once
	done := make(chan struct{})

	fyne.Do(func() {
		d := newArchivePasswordDialog(req, p.parent, p.km, p.bindings, func(ok bool, pass string) {
			finishOnce.Do(func() {
				defer close(done)
				mu.Lock()
				defer mu.Unlock()
				if !ok {
					retErr = errors.New("archive password cancelled")
					return
				}
				password = pass
			})
		})
		d.show()
	})

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-done:
	}
	mu.Lock()
	defer mu.Unlock()
	return password, retErr
}

// archivePasswordDialog is a single-field password prompt that follows the
// repo's keyboard-driven dialog lifecycle (see docs/architecture/ui-input.md,
// "Dialog Handler Lifecycle Pattern"): a key handler is pushed before Show()
// and removed on every close path, and the entry routes key events through
// the KeyManager so Escape cancels and Enter submits.
type archivePasswordDialog struct {
	req        fileinfo.ArchivePasswordRequest
	parent     fyne.Window
	km         *keymanager.KeyManager
	kmToken    keymanager.HandlerToken
	bindings   []config.KeyBindingEntry
	dialog     dialog.Dialog
	entry      *LineEditEntry
	closed     bool
	onFinished func(bool, string)
}

func newArchivePasswordDialog(req fileinfo.ArchivePasswordRequest, parent fyne.Window, km *keymanager.KeyManager, bindings []config.KeyBindingEntry, onFinished func(bool, string)) *archivePasswordDialog {
	d := &archivePasswordDialog{
		req:        req,
		parent:     parent,
		km:         km,
		bindings:   bindings,
		onFinished: onFinished,
	}
	d.entry = NewLineEditEntry(d.CancelDialog, d.km)
	d.entry.SetPlaceHolder("password")
	d.entry.SetIMEWindow(d.parent)
	d.entry.Password = true
	d.entry.Refresh()
	d.entry.OnSubmitted = func(string) {
		d.AcceptEdit()
	}
	return d
}

func (d *archivePasswordDialog) show() {
	title := "Archive Password: " + filepath.Base(d.req.ArchivePath)
	if d.req.Retry {
		title = "Archive Password Retry: " + filepath.Base(d.req.ArchivePath)
	}
	label := "Password"
	if d.req.Format != "" {
		label = d.req.Format + " Password"
	}

	content := container.NewVBox(
		container.NewBorder(nil, nil, widget.NewLabel(label), nil, lineEditThemeOverride(d.entry)),
		dialogButtonRow("Cancel", d.CancelDialog, "Open", d.AcceptEdit),
	)

	if d.km != nil {
		handler := keymanager.NewLineEditDialogKeyHandler(d, d.km.Debugf, d.bindings)
		d.kmToken = d.km.PushHandler(handler)
	}

	d.dialog = dialog.NewCustomWithoutButtons(title, content, d.parent)
	d.dialog.SetOnClosed(func() {
		d.CancelDialog()
	})
	d.dialog.Show()
	d.dialog.Resize(metricsSize(archivePasswordDialogWidth, archivePasswordDialogHeight))
	d.focusEntry()
	d.entry.UpdateIMEAnchor()
}

// AcceptEdit commits the entered password (Open button, Enter).
func (d *archivePasswordDialog) AcceptEdit() {
	if d.closed {
		return
	}
	d.close(true, d.entry.Text)
}

// CancelDialog closes the dialog without committing (Cancel button, Escape,
// SetOnClosed).
func (d *archivePasswordDialog) CancelDialog() {
	if d.closed {
		return
	}
	d.close(false, "")
}

func (d *archivePasswordDialog) close(ok bool, password string) {
	if d.closed {
		return
	}
	d.closed = true
	deferDialogClose(d.km, "archivePassword.close", func() {
		if d.km != nil {
			d.km.RemoveHandler(d.kmToken)
		}
		if d.dialog != nil {
			d.dialog.Hide()
		}
		unfocusIfDialogOwned(d.parent, d.entry)
		if d.onFinished != nil {
			d.onFinished(ok, password)
		}
	})
}

func (d *archivePasswordDialog) MoveCursorStart() {
	d.focusEntry()
	d.entry.MoveCursorStart()
}
func (d *archivePasswordDialog) MoveCursorEnd() {
	d.focusEntry()
	d.entry.MoveCursorEnd()
}
func (d *archivePasswordDialog) MoveCursorLeft() {
	d.focusEntry()
	d.entry.MoveCursorLeft()
}
func (d *archivePasswordDialog) MoveCursorRight() {
	d.focusEntry()
	d.entry.MoveCursorRight()
}
func (d *archivePasswordDialog) DeleteBeforeCursor() {
	d.focusEntry()
	d.entry.DeleteBeforeCursor()
}
func (d *archivePasswordDialog) DeleteAtCursor() {
	d.focusEntry()
	d.entry.DeleteAtCursor()
}
func (d *archivePasswordDialog) DeleteBeforeCursorToStart() {
	d.focusEntry()
	d.entry.DeleteBeforeCursorToStart()
}
func (d *archivePasswordDialog) DeleteAfterCursorToEnd() {
	d.focusEntry()
	d.entry.DeleteAfterCursorToEnd()
}
func (d *archivePasswordDialog) PasteFromClipboard() {
	d.focusEntry()
	d.entry.PasteFromClipboard()
}
func (d *archivePasswordDialog) InsertRune(r rune) bool {
	if d.entryIsFocused() {
		return false
	}
	d.focusEntry()
	d.entry.InsertText(string(r))
	return true
}

func (d *archivePasswordDialog) focusEntry() {
	if d.parent != nil {
		d.parent.Canvas().Focus(d.entry)
	}
}

func (d *archivePasswordDialog) entryIsFocused() bool {
	return d.parent != nil && d.parent.Canvas().Focused() == d.entry
}
