package ui

import (
	"errors"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
)

// SMBCredentialsProvider prompts the user for SMB credentials.
type SMBCredentialsProvider struct {
	parent   fyne.Window
	km       *keymanager.KeyManager
	bindings []config.KeyBindingEntry
}

func NewSMBCredentialsProvider(parent fyne.Window, km *keymanager.KeyManager, bindings ...[]config.KeyBindingEntry) *SMBCredentialsProvider {
	p := &SMBCredentialsProvider{parent: parent, km: km}
	if len(bindings) > 0 {
		p.bindings = bindings[0]
	}
	return p
}

func (p *SMBCredentialsProvider) Get(host, share, _ string) (fileinfo.Credentials, error) {
	var mu sync.Mutex
	var creds fileinfo.Credentials
	var retErr error
	var finishOnce sync.Once
	done := make(chan struct{})

	fyne.Do(func() {
		d := newSMBLoginDialog(host, share, p.parent, p.km, p.bindings, func(ok bool, c fileinfo.Credentials) {
			finishOnce.Do(func() {
				defer close(done)
				mu.Lock()
				defer mu.Unlock()
				if !ok {
					retErr = errors.New("login cancelled")
					return
				}
				creds = c
			})
		})
		d.show()
	})

	<-done
	mu.Lock()
	defer mu.Unlock()
	return creds, retErr
}

type smbLoginDialog struct {
	host       string
	share      string
	parent     fyne.Window
	km         *keymanager.KeyManager
	kmToken    keymanager.HandlerToken
	bindings   []config.KeyBindingEntry
	dialog     dialog.Dialog
	domain     *LineEditEntry
	username   *LineEditEntry
	password   *LineEditEntry
	saveCheck  *widget.Check
	active     int
	closed     bool
	onFinished func(bool, fileinfo.Credentials)
}

func newSMBLoginDialog(host, share string, parent fyne.Window, km *keymanager.KeyManager, bindings []config.KeyBindingEntry, onFinished func(bool, fileinfo.Credentials)) *smbLoginDialog {
	d := &smbLoginDialog{
		host:       host,
		share:      share,
		parent:     parent,
		km:         km,
		bindings:   bindings,
		active:     1,
		onFinished: onFinished,
	}
	d.domain = d.newEntry("domain (optional)", false)
	d.username = d.newEntry("username", false)
	d.password = d.newEntry("password", true)
	d.saveCheck = widget.NewCheck("この端末に保存 (keyring)", nil)
	return d
}

func (d *smbLoginDialog) newEntry(placeholder string, password bool) *LineEditEntry {
	entry := NewLineEditEntry(d.CancelDialog, d.km)
	entry.SetPlaceHolder(placeholder)
	entry.SetIMEWindow(d.parent)
	entry.Password = password
	if password {
		entry.Refresh()
	}
	entry.OnSubmitted = func(string) {
		d.AcceptLogin()
	}
	return entry
}

func (d *smbLoginDialog) show() {
	content := container.NewVBox(
		container.NewBorder(nil, nil, widget.NewLabel("Domain"), nil, lineEditThemeOverride(d.domain)),
		container.NewBorder(nil, nil, widget.NewLabel("Username"), nil, lineEditThemeOverride(d.username)),
		container.NewBorder(nil, nil, widget.NewLabel("Password"), nil, lineEditThemeOverride(d.password)),
		d.saveCheck,
		dialogButtonRow("Cancel", d.CancelDialog, "Login", d.AcceptLogin),
	)

	if d.km != nil {
		handler := newSMBLoginKeyHandler(d, d.bindings)
		d.kmToken = d.km.PushHandler(handler)
	}

	d.dialog = dialog.NewCustomWithoutButtons("SMB Login: "+d.host+"/"+d.share, content, d.parent)
	d.dialog.SetOnClosed(func() {
		d.CancelDialog()
	})
	d.dialog.Show()
	d.dialog.Resize(metricsSize(smbLoginDialogWidth, smbLoginDialogHeight))
	d.focusEntry(1)
}

func (d *smbLoginDialog) AcceptLogin() {
	if d.closed {
		return
	}
	creds := fileinfo.Credentials{
		Domain:   d.domain.Text,
		Username: d.username.Text,
		Password: d.password.Text,
		Persist:  d.saveCheck.Checked,
	}
	d.close(true, creds)
}

func (d *smbLoginDialog) AcceptEdit() {
	d.AcceptLogin()
}

func (d *smbLoginDialog) CancelDialog() {
	if d.closed {
		return
	}
	d.close(false, fileinfo.Credentials{})
}

func (d *smbLoginDialog) close(ok bool, creds fileinfo.Credentials) {
	if d.closed {
		return
	}
	d.closed = true
	deferDialogClose(d.km, "smbLogin.close", func() {
		if d.km != nil {
			d.km.RemoveHandler(d.kmToken)
		}
		if d.dialog != nil {
			d.dialog.Hide()
		}
		unfocusIfDialogOwned(d.parent, d.domain, d.username, d.password)
		if d.onFinished != nil {
			d.onFinished(ok, creds)
		}
	})
}

func (d *smbLoginDialog) MoveToNextField() {
	d.focusEntry((d.active + 1) % len(d.entries()))
}

func (d *smbLoginDialog) MoveToPreviousField() {
	entries := d.entries()
	d.focusEntry((d.active + len(entries) - 1) % len(entries))
}

func (d *smbLoginDialog) MoveCursorStart() {
	d.currentEntry().MoveCursorStart()
}
func (d *smbLoginDialog) MoveCursorEnd() {
	d.currentEntry().MoveCursorEnd()
}
func (d *smbLoginDialog) MoveCursorLeft() {
	d.currentEntry().MoveCursorLeft()
}
func (d *smbLoginDialog) MoveCursorRight() {
	d.currentEntry().MoveCursorRight()
}
func (d *smbLoginDialog) DeleteBeforeCursor() {
	d.currentEntry().DeleteBeforeCursor()
}
func (d *smbLoginDialog) DeleteAtCursor() {
	d.currentEntry().DeleteAtCursor()
}
func (d *smbLoginDialog) DeleteBeforeCursorToStart() {
	d.currentEntry().DeleteBeforeCursorToStart()
}
func (d *smbLoginDialog) DeleteAfterCursorToEnd() {
	d.currentEntry().DeleteAfterCursorToEnd()
}
func (d *smbLoginDialog) PasteFromClipboard() {
	d.currentEntry().PasteFromClipboard()
}
func (d *smbLoginDialog) InsertRune(r rune) bool {
	entry := d.currentEntry()
	if d.parent != nil && d.parent.Canvas().Focused() == entry {
		return false
	}
	entry.InsertText(string(r))
	return true
}

func (d *smbLoginDialog) focusEntry(index int) {
	entries := d.entries()
	if len(entries) == 0 {
		return
	}
	if index < 0 {
		index = 0
	}
	if index >= len(entries) {
		index = len(entries) - 1
	}
	d.active = index
	if d.parent != nil {
		d.parent.Canvas().Focus(entries[index])
	}
	entries[index].UpdateIMEAnchor()
}

func (d *smbLoginDialog) currentEntry() *LineEditEntry {
	entries := d.entries()
	if d.active < 0 || d.active >= len(entries) {
		d.active = 0
	}
	return entries[d.active]
}

func (d *smbLoginDialog) entries() []*LineEditEntry {
	return []*LineEditEntry{d.domain, d.username, d.password}
}

type smbLoginKeyHandler struct {
	dialog   *smbLoginDialog
	lineEdit *keymanager.LineEditDialogKeyHandler
}

func newSMBLoginKeyHandler(d *smbLoginDialog, bindings []config.KeyBindingEntry) *smbLoginKeyHandler {
	return &smbLoginKeyHandler{
		dialog:   d,
		lineEdit: keymanager.NewLineEditDialogKeyHandler(d, bindings),
	}
}

func (h *smbLoginKeyHandler) GetName() string { return "SMBLoginDialog" }

func (h *smbLoginKeyHandler) OnKeyActivated(ev *fyne.KeyEvent, modifiers keymanager.ModifierState) bool {
	if ev != nil && ev.Name == fyne.KeyTab {
		if modifiers.ShiftPressed {
			h.dialog.MoveToPreviousField()
		} else {
			h.dialog.MoveToNextField()
		}
		return true
	}
	return h.lineEdit.OnKeyActivated(ev, modifiers)
}

func (h *smbLoginKeyHandler) OnTypedRune(r rune, modifiers keymanager.ModifierState) bool {
	return h.lineEdit.OnTypedRune(r, modifiers)
}
