package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// IMEEntry is a normal Fyne Entry that updates the native IME candidate anchor.
type IMEEntry struct {
	widget.Entry
	imeWindow fyne.Window
	OnEscape  func()
	focused   bool
	disabled  bool
}

func NewIMEEntry(window fyne.Window) *IMEEntry {
	e := &IMEEntry{imeWindow: window}
	e.Wrapping = fyne.TextWrap(fyne.TextTruncateClip)
	e.ExtendBaseWidget(e)
	return e
}

func (e *IMEEntry) SetIMEWindow(window fyne.Window) {
	e.imeWindow = window
	e.UpdateIMEAnchor()
}

func (e *IMEEntry) SetText(text string) {
	e.Entry.SetText(text)
	e.UpdateIMEAnchor()
}

func (e *IMEEntry) FocusGained() {
	e.focused = true
	e.Entry.FocusGained()
	e.UpdateIMEAnchor()
}

func (e *IMEEntry) FocusLost() {
	e.focused = false
	e.Entry.FocusLost()
}

func (e *IMEEntry) Disable() {
	e.disabled = true
	e.Entry.Disable()
}

func (e *IMEEntry) Enable() {
	e.disabled = false
	e.Entry.Enable()
}

func (e *IMEEntry) TypedKey(ev *fyne.KeyEvent) {
	if ev != nil && ev.Name == fyne.KeyEscape && e.OnEscape != nil {
		e.OnEscape()
		e.UpdateIMEAnchor()
		return
	}
	e.Entry.TypedKey(ev)
	e.UpdateIMEAnchor()
}

func (e *IMEEntry) TypedRune(r rune) {
	e.Entry.TypedRune(r)
	e.UpdateIMEAnchor()
}

func (e *IMEEntry) UpdateIMEAnchor() {
	setIMEAnchorAtTextEnd(e.imeWindow, e, e.Text, e.TextStyle)
}

func (e *IMEEntry) CreateRenderer() fyne.WidgetRenderer {
	caret := canvas.NewRectangle(color.Transparent)
	caret.Hide()
	return &lineEditEntryRenderer{
		entry: e,
		base:  e.Entry.CreateRenderer(),
		caret: caret,
	}
}

func (e *IMEEntry) lineEditFocused() bool {
	return e.focused
}

func (e *IMEEntry) lineEditDisabled() bool {
	return e.disabled
}

func (e *IMEEntry) lineEditTextStyle() fyne.TextStyle {
	return e.TextStyle
}
