package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/keymanager"
)

// IMEEntry is a normal Fyne Entry that updates the native IME candidate anchor.
type IMEEntry struct {
	widget.Entry
	imeWindow      fyne.Window
	OnEscape       func()
	focused        bool
	disabled       bool
	keyManager     *keymanager.KeyManager
	onKeyActivated func(*fyne.KeyEvent, keymanager.ModifierState) bool
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

// SetKeyActivationHandler supplies optional non-text actions, such as help.
// Native clipboard shortcuts and printable text keep their Entry behavior.
func (e *IMEEntry) SetKeyActivationHandler(km *keymanager.KeyManager, handler func(*fyne.KeyEvent, keymanager.ModifierState) bool) {
	e.keyManager = km
	e.onKeyActivated = handler
}

func (e *IMEEntry) KeyDown(ev *fyne.KeyEvent) {
	if e.keyManager != nil {
		e.keyManager.HandleKeyDown(ev)
	}
	e.Entry.KeyDown(ev)
}

func (e *IMEEntry) KeyUp(ev *fyne.KeyEvent) {
	if e.keyManager != nil {
		e.keyManager.HandleKeyUp(ev)
	}
	e.Entry.KeyUp(ev)
}

func (e *IMEEntry) TypedShortcut(shortcut fyne.Shortcut) {
	defer e.UpdateIMEAnchor()
	if custom, ok := shortcut.(*desktop.CustomShortcut); ok && e.onKeyActivated != nil {
		mod := keymanager.ModifierState{
			CtrlPressed:  custom.Modifier&fyne.KeyModifierControl != 0,
			AltPressed:   custom.Modifier&fyne.KeyModifierAlt != 0,
			ShiftPressed: custom.Modifier&fyne.KeyModifierShift != 0,
		}
		if e.onKeyActivated(&fyne.KeyEvent{Name: custom.KeyName}, mod) {
			return
		}
	}
	e.Entry.TypedShortcut(shortcut)
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
	if e.keyManager != nil && e.onKeyActivated != nil && e.onKeyActivated(ev, e.keyManager.GetModifierState()) {
		e.UpdateIMEAnchor()
		return
	}
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

// SelectAll selects the entire entry text and keeps the IME anchor in sync.
func (e *IMEEntry) SelectAll() {
	e.Entry.TypedShortcut(&fyne.ShortcutSelectAll{})
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
