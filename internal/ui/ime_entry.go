package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// IMEEntry is a normal Fyne Entry that updates the native IME candidate anchor.
type IMEEntry struct {
	widget.Entry
	imeWindow fyne.Window
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
	e.Entry.FocusGained()
	e.UpdateIMEAnchor()
}

func (e *IMEEntry) TypedKey(ev *fyne.KeyEvent) {
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
