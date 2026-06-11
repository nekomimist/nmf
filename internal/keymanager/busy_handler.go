package keymanager

import (
	"fyne.io/fyne/v2"
)

// BusyKeyHandler swallows all key input while a busy operation is active.
// Escape can trigger an optional cancel callback.
type BusyKeyHandler struct {
	onCancel func()
}

func NewBusyKeyHandler(onCancel ...func()) *BusyKeyHandler {
	var cancel func()
	if len(onCancel) > 0 {
		cancel = onCancel[0]
	}
	return &BusyKeyHandler{onCancel: cancel}
}

func (b *BusyKeyHandler) GetName() string { return "BusyGuard" }

func (b *BusyKeyHandler) OnKeyActivated(ev *fyne.KeyEvent, _ ModifierState) bool {
	b.cancelIfEscape(ev)
	return true
}

func (b *BusyKeyHandler) OnTypedRune(_ rune, _ ModifierState) bool { return true }

func (b *BusyKeyHandler) cancelIfEscape(ev *fyne.KeyEvent) {
	if ev != nil && ev.Name == fyne.KeyEscape && b.onCancel != nil {
		b.onCancel()
	}
}
