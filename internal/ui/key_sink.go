package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/keymanager"
)

// KeySink is a generic, focusable wrapper around any CanvasObject.
// When focused, it forwards all key events to the provided KeyManager
// and optionally captures Tab to prevent Fyne's default focus traversal.
type KeySink struct {
	widget.BaseWidget
	Content   fyne.CanvasObject
	km        *keymanager.KeyManager
	acceptTab bool
	onFocus   func(bool)
}

// KeySinkOption customizes KeySink behavior.
type KeySinkOption func(*KeySink)

// WithTabCapture toggles Tab key capture for focus traversal suppression.
func WithTabCapture(on bool) KeySinkOption { return func(k *KeySink) { k.acceptTab = on } }

// WithFocusChanged sets a callback invoked when the sink gains or loses focus.
func WithFocusChanged(callback func(bool)) KeySinkOption {
	return func(k *KeySink) { k.onFocus = callback }
}

// NewKeySink creates a new KeySink wrapping the given content.
// By default, Tab is captured (acceptTab=true).
func NewKeySink(content fyne.CanvasObject, km *keymanager.KeyManager, opts ...KeySinkOption) *KeySink {
	k := &KeySink{
		Content:   content,
		km:        km,
		acceptTab: true,
	}
	for _, o := range opts {
		o(k)
	}
	k.ExtendBaseWidget(k)
	return k
}

// CreateRenderer delegates rendering to the underlying content.
func (k *KeySink) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(k.Content)
}

// FocusGained implements fyne.Focusable.
func (k *KeySink) FocusGained() {
	if k.onFocus != nil {
		k.onFocus(true)
	}
}

// FocusLost implements fyne.Focusable.
func (k *KeySink) FocusLost() {
	if k.onFocus != nil {
		k.onFocus(false)
	}
}

// TypedKey forwards typed key events to KeyManager.
func (k *KeySink) TypedKey(ev *fyne.KeyEvent) {
	if k.km != nil {
		k.km.HandleTypedKey(ev)
	}
}

// TypedRune forwards typed runes to KeyManager.
func (k *KeySink) TypedRune(r rune) {
	if k.km != nil {
		k.km.HandleTypedRune(r)
	}
}

// TypedShortcut forwards desktop shortcut repeats to KeyManager.
func (k *KeySink) TypedShortcut(shortcut fyne.Shortcut) {
	if k.km == nil {
		return
	}
	s, ok := shortcut.(*desktop.CustomShortcut)
	if !ok {
		return
	}
	modifiers := keymanager.ModifierState{
		ShiftPressed: s.Modifier&fyne.KeyModifierShift != 0,
		CtrlPressed:  s.Modifier&fyne.KeyModifierControl != 0,
		AltPressed:   s.Modifier&fyne.KeyModifierAlt != 0,
	}
	k.km.HandleShortcutKey(&fyne.KeyEvent{Name: s.KeyName}, modifiers)
}

// KeyDown forwards desktop key down events to KeyManager.
func (k *KeySink) KeyDown(ev *fyne.KeyEvent) {
	if k.km != nil {
		k.km.HandleKeyDown(ev)
	}
}

// KeyUp forwards desktop key up events to KeyManager.
func (k *KeySink) KeyUp(ev *fyne.KeyEvent) {
	if k.km != nil {
		k.km.HandleKeyUp(ev)
	}
}

// AcceptsTab indicates whether to capture Tab, preventing focus traversal.
func (k *KeySink) AcceptsTab() bool { return k.acceptTab }
