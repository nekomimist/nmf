package ui

import (
	"image/color"
	"strings"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/keymanager"
	customtheme "nmf/internal/theme"
)

const (
	lineEditEntryHorizontalInset         float32 = 4
	lineEditEntryVerticalInset           float32 = 6
	lineEditEntryTrailingCaretClearance  float32 = 2
	lineEditEntryMinimumCaretStrokeWidth float32 = 1
)

// LineEditSelection describes an initial single-line selection using rune
// offsets. The cursor is placed at End.
type LineEditSelection struct {
	Start int
	End   int
}

// LineEditDialogOptions configures a single-line edit dialog.
type LineEditDialogOptions struct {
	Title            string
	Prompt           string
	CurrentText      string
	InitialText      string
	InitialSelection *LineEditSelection
	ConfirmText      string
	Width            float32
	Height           float32
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
	if opts.InitialSelection != nil {
		d.entry.SelectRange(opts.InitialSelection.Start, opts.InitialSelection.End)
	} else {
		d.entry.MoveCursorEnd()
	}
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
	content.Add(lineEditThemeOverride(d.entry))
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
		d.entry.SetIMEWindow(d.parent)
		d.parent.Canvas().Focus(d.entry)
		d.entry.UpdateIMEAnchor()
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
	deferDialogClose(d.keyManager, "lineEdit.close", func() {
		d.keyManager.PopHandler()
		if d.dialog != nil {
			d.dialog.Hide()
		}
		unfocusIfDialogOwned(d.parent, d.entry)
	})
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
func (d *LineEditDialog) PasteFromClipboard() {
	d.focusEntry()
	d.entry.PasteFromClipboard()
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
	onCancel  func()
	imeWindow fyne.Window
	focused   bool
	disabled  bool
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
	e.UpdateIMEAnchor()
}

func (e *LineEditEntry) TypedRune(r rune) {
	e.TabEntry.TypedRune(r)
	e.UpdateIMEAnchor()
}

func (e *LineEditEntry) FocusGained() {
	e.focused = true
	e.TabEntry.FocusGained()
	e.UpdateIMEAnchor()
}

func (e *LineEditEntry) FocusLost() {
	e.focused = false
	e.TabEntry.FocusLost()
}

func (e *LineEditEntry) Disable() {
	e.disabled = true
	e.TabEntry.Disable()
}

func (e *LineEditEntry) Enable() {
	e.disabled = false
	e.TabEntry.Enable()
}

func (e *LineEditEntry) SetIMEWindow(window fyne.Window) {
	e.imeWindow = window
	e.UpdateIMEAnchor()
}

func (e *LineEditEntry) SetText(text string) {
	e.TabEntry.SetText(text)
	e.UpdateIMEAnchor()
}

func (e *LineEditEntry) UpdateIMEAnchor() {
	if e.disabled {
		return
	}
	setIMEAnchorAtTextEnd(e.imeWindow, e, string([]rune(e.Text)[:e.normalizedCursor()]), e.TextStyle)
}

func (e *LineEditEntry) CreateRenderer() fyne.WidgetRenderer {
	caret := canvas.NewRectangle(color.Transparent)
	caret.Hide()
	return &lineEditEntryRenderer{
		entry: e,
		base:  e.TabEntry.CreateRenderer(),
		caret: caret,
	}
}

func (e *LineEditEntry) lineEditFocused() bool {
	return e.focused
}

func (e *LineEditEntry) lineEditDisabled() bool {
	return e.disabled
}

func (e *LineEditEntry) lineEditTextStyle() fyne.TextStyle {
	return e.TextStyle
}

func (e *LineEditEntry) KeyDown(ev *fyne.KeyEvent) {
	e.TabEntry.KeyDown(ev)
}

func (e *LineEditEntry) KeyUp(ev *fyne.KeyEvent) {
	e.TabEntry.KeyUp(ev)
}

func (e *LineEditEntry) TypedShortcut(shortcut fyne.Shortcut) {
	defer e.UpdateIMEAnchor()
	switch s := shortcut.(type) {
	case *fyne.ShortcutSelectAll:
		e.MoveCursorStart()
	case *desktop.CustomShortcut:
		if s.Modifier == fyne.KeyModifierControl && e.handleReadlineKey(s.KeyName) {
			return
		}
		e.TabEntry.TypedShortcut(shortcut)
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
	case fyne.KeyY:
		e.PasteFromClipboard()
	default:
		return false
	}
	return true
}

func (e *LineEditEntry) MoveCursorStart() {
	e.clearSelection()
	e.setCursor(0)
}

func (e *LineEditEntry) MoveCursorEnd() {
	e.clearSelection()
	e.setCursor(utf8.RuneCountInString(e.Text))
}

func (e *LineEditEntry) MoveCursorLeft() {
	pos := e.normalizedCursor()
	e.clearSelection()
	e.setCursor(pos - 1)
}

func (e *LineEditEntry) MoveCursorRight() {
	pos := e.normalizedCursor()
	e.clearSelection()
	e.setCursor(pos + 1)
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
	e.setClipboardText(string([]rune(e.Text)[:pos]))
	e.replaceRunes(0, pos, "")
}

func (e *LineEditEntry) DeleteAfterCursorToEnd() {
	pos := e.normalizedCursor()
	runes := []rune(e.Text)
	if pos >= len(runes) {
		return
	}
	e.setClipboardText(string(runes[pos:]))
	e.replaceRunes(pos, len(runes), "")
}

func (e *LineEditEntry) InsertText(text string) {
	e.replaceRunes(e.normalizedCursor(), e.normalizedCursor(), text)
}

func (e *LineEditEntry) PasteFromClipboard() {
	text, ok := e.clipboardText()
	if !ok {
		return
	}
	if !e.MultiLine {
		text = strings.ReplaceAll(text, "\n", " ")
	}
	e.InsertText(text)
}

// SelectRange selects text from start to end using rune offsets and places the
// cursor at end.
func (e *LineEditEntry) SelectRange(start, end int) {
	max := utf8.RuneCountInString(e.Text)
	start = clampLineEditOffset(start, max)
	end = clampLineEditOffset(end, max)
	if start > end {
		start, end = end, start
	}
	e.setCursor(start)
	if start == end {
		return
	}
	e.KeyDown(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})
	for e.CursorColumn < end {
		e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyRight})
	}
	e.KeyUp(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})
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

func (e *LineEditEntry) clearSelection() {
	if e.SelectedText() == "" {
		return
	}
	e.KeyUp(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})
	e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyLeft})
}

func (e *LineEditEntry) setClipboardText(text string) bool {
	app := fyne.CurrentApp()
	if app == nil || app.Clipboard() == nil {
		return false
	}
	app.Clipboard().SetContent(text)
	return true
}

func (e *LineEditEntry) clipboardText() (string, bool) {
	app := fyne.CurrentApp()
	if app == nil || app.Clipboard() == nil {
		return "", false
	}
	return app.Clipboard().Content(), true
}

func clampLineEditOffset(pos, max int) int {
	if pos < 0 {
		return 0
	}
	if pos > max {
		return max
	}
	return pos
}

func (e *LineEditEntry) normalizedCursor() int {
	max := utf8.RuneCountInString(e.Text)
	return clampLineEditOffset(e.CursorColumn, max)
}

func (e *LineEditEntry) setCursor(pos int) {
	max := utf8.RuneCountInString(e.Text)
	pos = clampLineEditOffset(pos, max)
	e.CursorRow = 0
	e.CursorColumn = pos
	e.Refresh()
	e.UpdateIMEAnchor()
}

type lineEditEntryRenderer struct {
	entry lineEditEntryRendererTarget
	base  fyne.WidgetRenderer
	caret *canvas.Rectangle
}

type lineEditEntryRendererTarget interface {
	fyne.Widget
	Theme() fyne.Theme
	CursorPosition() fyne.Position
	lineEditFocused() bool
	lineEditDisabled() bool
	lineEditTextStyle() fyne.TextStyle
}

func (r *lineEditEntryRenderer) Destroy() {
	r.base.Destroy()
}

func (r *lineEditEntryRenderer) Layout(size fyne.Size) {
	r.base.Layout(size)
	r.applyContentInset()
	r.updateCaret()
}

func (r *lineEditEntryRenderer) MinSize() fyne.Size {
	min := r.base.MinSize()
	return fyne.NewSize(
		min.Width+lineEditEntryHorizontalInset*2,
		min.Height+lineEditEntryVerticalInset*2,
	)
}

func (r *lineEditEntryRenderer) Objects() []fyne.CanvasObject {
	return append(r.base.Objects(), r.caret)
}

func (r *lineEditEntryRenderer) Refresh() {
	r.base.Refresh()
	r.restoreFocusedBorderColor()
	r.updateCaret()
}

func (r *lineEditEntryRenderer) restoreFocusedBorderColor() {
	if r.entry == nil || !r.entry.lineEditFocused() || r.entry.lineEditDisabled() {
		return
	}
	border := r.borderRectangle()
	if border == nil {
		return
	}
	border.StrokeColor = currentAppThemeColor(fynetheme.ColorNamePrimary)
	border.Refresh()
}

func (r *lineEditEntryRenderer) borderRectangle() *canvas.Rectangle {
	for _, obj := range r.base.Objects() {
		rect, ok := obj.(*canvas.Rectangle)
		if ok && rect.StrokeWidth > 0 {
			return rect
		}
	}
	return nil
}

func (r *lineEditEntryRenderer) contentObject() fyne.CanvasObject {
	for _, obj := range r.base.Objects() {
		if _, ok := obj.(*canvas.Rectangle); ok {
			continue
		}
		return obj
	}
	return nil
}

func (r *lineEditEntryRenderer) applyContentInset() {
	content := r.contentObject()
	if content == nil {
		return
	}
	content.Move(content.Position().Add(fyne.NewPos(lineEditEntryHorizontalInset, 0)))
	size := content.Size().Subtract(fyne.NewSize(
		lineEditEntryHorizontalInset*2,
		0,
	))
	if size.Width < 0 {
		size.Width = 0
	}
	content.Resize(size)
}

func (r *lineEditEntryRenderer) updateCaret() {
	if r.entry == nil || r.caret == nil || !r.entry.lineEditFocused() || r.entry.lineEditDisabled() {
		r.caret.Hide()
		return
	}

	th := r.entry.Theme()
	inputBorder := th.Size(fynetheme.SizeNameInputBorder)
	textSize := th.Size(fynetheme.SizeNameText)
	lineHeight := fyne.MeasureText("M", textSize, r.entry.lineEditTextStyle()).Height
	caretWidth := inputBorder
	if caretWidth < lineEditEntryMinimumCaretStrokeWidth {
		caretWidth = lineEditEntryMinimumCaretStrokeWidth
	}
	pos := r.entry.CursorPosition()
	if content := r.contentObject(); content != nil {
		pos = pos.Add(content.Position())
	}
	maxX := r.entry.Size().Width - lineEditEntryHorizontalInset - caretWidth - lineEditEntryTrailingCaretClearance
	if pos.X > maxX {
		pos.X = maxX
	}
	if pos.X < lineEditEntryHorizontalInset {
		pos.X = lineEditEntryHorizontalInset
	}

	r.caret.FillColor = currentLineEditColor(customtheme.ColorLineEditCursor)
	r.caret.Resize(fyne.NewSize(caretWidth, lineHeight))
	r.caret.Move(pos)
	r.caret.Show()
	r.caret.Refresh()
}

func currentAppThemeColor(name fyne.ThemeColorName) color.Color {
	if fyne.CurrentApp() == nil || fyne.CurrentApp().Settings().Theme() == nil {
		return fynetheme.Color(name)
	}
	return fyne.CurrentApp().Settings().Theme().Color(name, fyne.CurrentApp().Settings().ThemeVariant())
}

func currentLineEditColor(name string) color.Color {
	themeProvider := currentThemeColorProvider()
	if themeProvider == nil {
		return currentAppThemeColor(fynetheme.ColorNamePrimary)
	}
	return themeProvider.GetCustomColor(name)
}
