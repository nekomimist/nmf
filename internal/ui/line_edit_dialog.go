package ui

import (
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/keymanager"
)

const (
	lineEditDialogWidth  float32 = 520
	lineEditDialogHeight float32 = 150
)

// LineEditDialogOptions configures a single-line edit dialog.
type LineEditDialogOptions struct {
	Title       string
	Prompt      string
	CurrentText string
	InitialText string
	ConfirmText string
	Width       float32
	Height      float32
}

// LineEditDialog edits one line of text and commits it through a callback.
type LineEditDialog struct {
	opts       LineEditDialogOptions
	entry      *LineEditEntry
	keyManager *keymanager.KeyManager
	parent     fyne.Window
	dialog     dialog.Dialog
	closed     bool
	onAccept   func(string) bool
}

// NewLineEditDialog creates a configured one-line edit dialog.
func NewLineEditDialog(opts LineEditDialogOptions, km *keymanager.KeyManager) *LineEditDialog {
	if opts.ConfirmText == "" {
		opts.ConfirmText = "OK"
	}
	if opts.Width <= 0 {
		opts.Width = lineEditDialogWidth
	}
	if opts.Height <= 0 {
		opts.Height = lineEditDialogHeight
	}

	d := &LineEditDialog{
		opts:       opts,
		keyManager: km,
	}
	d.entry = NewLineEditEntry(d.CancelDialog)
	d.entry.SetText(opts.InitialText)
	d.entry.MoveCursorEnd()
	d.entry.OnSubmitted = func(_ string) {
		d.AcceptEdit()
	}
	return d
}

// ShowDialog displays the edit dialog.
func (d *LineEditDialog) ShowDialog(parent fyne.Window, onAccept func(string) bool) {
	d.parent = parent
	d.onAccept = onAccept

	content := container.NewVBox()
	if d.opts.CurrentText != "" {
		currentLabel := widget.NewLabel("Current:")
		currentName := widget.NewLabel(middleEllipsizeFileName(d.opts.CurrentText, renameDisplayedNameMax))
		currentName.Truncation = fyne.TextTruncateClip
		content.Add(container.NewBorder(nil, nil, currentLabel, nil, currentName))
	}
	if d.opts.Prompt != "" {
		content.Add(widget.NewLabel(d.opts.Prompt))
	}
	content.Add(d.entry)
	content.Add(container.NewGridWithColumns(
		2,
		widget.NewButton("Cancel", d.CancelDialog),
		widget.NewButton(d.opts.ConfirmText, d.AcceptEdit),
	))

	handler := keymanager.NewLineEditDialogKeyHandler(d)
	d.keyManager.PushHandler(handler)

	title := d.opts.Title
	if title == "" {
		title = "Edit"
	}
	d.dialog = dialog.NewCustomWithoutButtons(title, content, parent)
	d.dialog.SetOnClosed(func() {
		d.CancelDialog()
	})
	d.dialog.Show()
	d.dialog.Resize(fyne.NewSize(d.opts.Width, d.opts.Height))
	if d.parent != nil && d.entry != nil {
		d.parent.Canvas().Focus(d.entry)
	}
}

// AcceptEdit commits the entered value.
func (d *LineEditDialog) AcceptEdit() {
	if d.closed {
		return
	}
	if d.onAccept != nil && d.entry != nil {
		if !d.onAccept(d.entry.Text) {
			if d.parent != nil && d.entry != nil {
				d.parent.Canvas().Focus(d.entry)
			}
			return
		}
	}
	d.close()
}

// CancelDialog closes the dialog without committing.
func (d *LineEditDialog) CancelDialog() {
	if d.closed {
		return
	}
	d.close()
}

func (d *LineEditDialog) close() {
	d.closed = true
	d.keyManager.PopHandler()
	if d.dialog != nil {
		d.dialog.Hide()
	}
	if d.parent != nil {
		d.parent.Canvas().Unfocus()
	}
}

func (d *LineEditDialog) MoveCursorStart() {
	d.focusEntry()
	d.entry.MoveCursorStart()
}
func (d *LineEditDialog) MoveCursorEnd() {
	d.focusEntry()
	d.entry.MoveCursorEnd()
}
func (d *LineEditDialog) MoveCursorLeft() {
	d.focusEntry()
	d.entry.MoveCursorLeft()
}
func (d *LineEditDialog) MoveCursorRight() {
	d.focusEntry()
	d.entry.MoveCursorRight()
}
func (d *LineEditDialog) DeleteBeforeCursor() {
	d.focusEntry()
	d.entry.DeleteBeforeCursor()
}
func (d *LineEditDialog) DeleteAtCursor() {
	d.focusEntry()
	d.entry.DeleteAtCursor()
}
func (d *LineEditDialog) DeleteBeforeCursorToStart() {
	d.focusEntry()
	d.entry.DeleteBeforeCursorToStart()
}
func (d *LineEditDialog) DeleteAfterCursorToEnd() {
	d.focusEntry()
	d.entry.DeleteAfterCursorToEnd()
}
func (d *LineEditDialog) InsertRune(r rune) bool {
	if d.entryIsFocused() {
		return false
	}
	d.focusEntry()
	d.entry.InsertText(string(r))
	return true
}

func (d *LineEditDialog) focusEntry() {
	if d.parent != nil && d.entry != nil {
		d.parent.Canvas().Focus(d.entry)
	}
}

func (d *LineEditDialog) entryIsFocused() bool {
	return d.parent != nil && d.entry != nil && d.parent.Canvas().Focused() == d.entry
}

// LineEditEntry is a single-line entry with small readline-style edit helpers.
type LineEditEntry struct {
	TabEntry
	onCancel func()
	ctrlDown bool
}

// NewLineEditEntry creates an entry for LineEditDialog.
func NewLineEditEntry(onCancel func()) *LineEditEntry {
	e := &LineEditEntry{onCancel: onCancel}
	e.acceptTab = true
	e.Wrapping = fyne.TextWrap(fyne.TextTruncateClip)
	e.ExtendBaseWidget(e)
	return e
}

func (e *LineEditEntry) TypedKey(ev *fyne.KeyEvent) {
	if ev.Name == fyne.KeyEscape {
		if e.onCancel != nil {
			e.onCancel()
		}
		return
	}
	e.TabEntry.TypedKey(ev)
}

func (e *LineEditEntry) FocusLost() {
	e.ctrlDown = false
	e.TabEntry.FocusLost()
}

func (e *LineEditEntry) KeyDown(ev *fyne.KeyEvent) {
	switch ev.Name {
	case desktop.KeyControlLeft, desktop.KeyControlRight:
		e.ctrlDown = true
		e.TabEntry.KeyDown(ev)
		return
	}
	if e.ctrlDown && e.handleReadlineKey(ev.Name) {
		return
	}
	e.TabEntry.KeyDown(ev)
}

func (e *LineEditEntry) KeyUp(ev *fyne.KeyEvent) {
	switch ev.Name {
	case desktop.KeyControlLeft, desktop.KeyControlRight:
		e.ctrlDown = false
	}
	e.TabEntry.KeyUp(ev)
}

func (e *LineEditEntry) TypedShortcut(shortcut fyne.Shortcut) {
	switch shortcut.(type) {
	case *fyne.ShortcutSelectAll:
		e.MoveCursorStart()
	default:
		e.TabEntry.TypedShortcut(shortcut)
	}
}

func (e *LineEditEntry) handleReadlineKey(name fyne.KeyName) bool {
	switch name {
	case fyne.KeyA:
		e.MoveCursorStart()
	case fyne.KeyE:
		e.MoveCursorEnd()
	case fyne.KeyB:
		e.MoveCursorLeft()
	case fyne.KeyF:
		e.MoveCursorRight()
	case fyne.KeyH:
		e.DeleteBeforeCursor()
	case fyne.KeyD:
		e.DeleteAtCursor()
	case fyne.KeyU:
		e.DeleteBeforeCursorToStart()
	case fyne.KeyK:
		e.DeleteAfterCursorToEnd()
	default:
		return false
	}
	return true
}

func (e *LineEditEntry) MoveCursorStart() {
	e.setCursor(0)
}

func (e *LineEditEntry) MoveCursorEnd() {
	e.setCursor(utf8.RuneCountInString(e.Text))
}

func (e *LineEditEntry) MoveCursorLeft() {
	e.setCursor(e.CursorColumn - 1)
}

func (e *LineEditEntry) MoveCursorRight() {
	e.setCursor(e.CursorColumn + 1)
}

func (e *LineEditEntry) DeleteBeforeCursor() {
	pos := e.normalizedCursor()
	if pos <= 0 {
		return
	}
	e.replaceRunes(pos-1, pos, "")
}

func (e *LineEditEntry) DeleteAtCursor() {
	pos := e.normalizedCursor()
	if pos >= utf8.RuneCountInString(e.Text) {
		return
	}
	e.replaceRunes(pos, pos+1, "")
}

func (e *LineEditEntry) DeleteBeforeCursorToStart() {
	pos := e.normalizedCursor()
	if pos <= 0 {
		return
	}
	e.replaceRunes(0, pos, "")
}

func (e *LineEditEntry) DeleteAfterCursorToEnd() {
	pos := e.normalizedCursor()
	if pos >= utf8.RuneCountInString(e.Text) {
		return
	}
	e.replaceRunes(pos, utf8.RuneCountInString(e.Text), "")
}

func (e *LineEditEntry) InsertText(text string) {
	e.replaceRunes(e.normalizedCursor(), e.normalizedCursor(), text)
}

func (e *LineEditEntry) replaceRunes(start, end int, replacement string) {
	runes := []rune(e.Text)
	if start < 0 {
		start = 0
	}
	if end > len(runes) {
		end = len(runes)
	}
	if start > end {
		start = end
	}
	next := string(runes[:start]) + replacement + string(runes[end:])
	e.SetText(next)
	e.setCursor(start + utf8.RuneCountInString(replacement))
}

func (e *LineEditEntry) normalizedCursor() int {
	max := utf8.RuneCountInString(e.Text)
	if e.CursorColumn < 0 {
		return 0
	}
	if e.CursorColumn > max {
		return max
	}
	return e.CursorColumn
}

func (e *LineEditEntry) setCursor(pos int) {
	max := utf8.RuneCountInString(e.Text)
	if pos < 0 {
		pos = 0
	}
	if pos > max {
		pos = max
	}
	e.CursorRow = 0
	e.CursorColumn = pos
	e.Refresh()
}
