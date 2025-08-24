package keymanager

import (
	"fyne.io/fyne/v2"
)

// BusyKeyHandler swallows all key input while a busy operation is active.
// Optionally, it can allow a cancel callback on Esc in the future, but for now
// we just block all inputs to avoid reentrancy during async operations.
type BusyKeyHandler struct{}

func NewBusyKeyHandler() *BusyKeyHandler { return &BusyKeyHandler{} }

func (b *BusyKeyHandler) GetName() string { return "BusyGuard" }

func (b *BusyKeyHandler) OnKeyDown(_ *fyne.KeyEvent, _ ModifierState) bool  { return true }
func (b *BusyKeyHandler) OnKeyUp(_ *fyne.KeyEvent, _ ModifierState) bool    { return true }
func (b *BusyKeyHandler) OnTypedKey(_ *fyne.KeyEvent, _ ModifierState) bool { return true }
func (b *BusyKeyHandler) OnTypedRune(_ rune, _ ModifierState) bool          { return true }
