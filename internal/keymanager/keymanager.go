package keymanager

import (
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// ModifierState holds the current state of modifier keys
type ModifierState struct {
	ShiftPressed bool
	CtrlPressed  bool
}

// KeyHandler defines the interface for handling keyboard events
type KeyHandler interface {
	// OnKeyDown handles key press events
	OnKeyDown(ev *fyne.KeyEvent, modifiers ModifierState) bool // returns true if handled

	// OnKeyUp handles key release events
	OnKeyUp(ev *fyne.KeyEvent, modifiers ModifierState) bool // returns true if handled

	// OnTypedKey handles typed key events
	OnTypedKey(ev *fyne.KeyEvent, modifiers ModifierState) bool // returns true if handled

	// OnTypedRune handles text input runes
	OnTypedRune(r rune, modifiers ModifierState) bool // returns true if handled

	// GetName returns a descriptive name for this handler (for debugging)
	GetName() string
}

// KeyManager manages a stack of key handlers
type KeyManager struct {
	handlers      []KeyHandler
	modifierState ModifierState
	mutex         sync.RWMutex
	debugPrint    func(format string, args ...interface{})
}

// NewKeyManager creates a new KeyManager instance
func NewKeyManager(debugPrint func(format string, args ...interface{})) *KeyManager {
	return &KeyManager{
		handlers:   make([]KeyHandler, 0),
		debugPrint: debugPrint,
	}
}

// PushHandler adds a new key handler to the top of the stack
func (km *KeyManager) PushHandler(handler KeyHandler) {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	km.handlers = append(km.handlers, handler)
	km.debugPrint("KeyManager: Pushed handler '%s', stack size: %d", handler.GetName(), len(km.handlers))
}

// PopHandler removes the top key handler from the stack
func (km *KeyManager) PopHandler() KeyHandler {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	if len(km.handlers) == 0 {
		km.debugPrint("KeyManager: Attempted to pop from empty stack")
		return nil
	}

	// Get the last handler
	handler := km.handlers[len(km.handlers)-1]

	// Remove it from the slice
	km.handlers = km.handlers[:len(km.handlers)-1]

	km.debugPrint("KeyManager: Popped handler '%s', stack size: %d", handler.GetName(), len(km.handlers))
	return handler
}

// GetCurrentHandler returns the top handler without removing it
func (km *KeyManager) GetCurrentHandler() KeyHandler {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	if len(km.handlers) == 0 {
		return nil
	}

	return km.handlers[len(km.handlers)-1]
}

// updateModifierState updates the modifier key state based on key events
func (km *KeyManager) updateModifierState(ev *fyne.KeyEvent, pressed bool) bool {
	switch ev.Name {
	case desktop.KeyShiftLeft, desktop.KeyShiftRight:
		km.modifierState.ShiftPressed = pressed
		km.debugPrint("KeyManager: Shift key %s (state: %t)", map[bool]string{true: "pressed", false: "released"}[pressed], km.modifierState.ShiftPressed)
		return true
	case desktop.KeyControlLeft, desktop.KeyControlRight:
		km.modifierState.CtrlPressed = pressed
		km.debugPrint("KeyManager: Ctrl key %s (state: %t)", map[bool]string{true: "pressed", false: "released"}[pressed], km.modifierState.CtrlPressed)
		return true
	}
	return false
}

// GetModifierState returns a copy of the current modifier state
func (km *KeyManager) GetModifierState() ModifierState {
	km.mutex.RLock()
	defer km.mutex.RUnlock()
	return km.modifierState
}

// HandleKeyDown routes key down events to the current top handler
func (km *KeyManager) HandleKeyDown(ev *fyne.KeyEvent) {
	km.mutex.Lock()
	// Update modifier state first
	modifierHandled := km.updateModifierState(ev, true)
	modifiers := km.modifierState
	km.mutex.Unlock()

	// Get current handler
	km.mutex.RLock()
	currentHandler := km.GetCurrentHandler()
	km.mutex.RUnlock()

	if currentHandler != nil {
		handled := currentHandler.OnKeyDown(ev, modifiers)
		km.debugPrint("KeyManager: KeyDown event handled by '%s': %t (modifier: %t)", currentHandler.GetName(), handled, modifierHandled)
	} else {
		km.debugPrint("KeyManager: No handler available for KeyDown event")
	}
}

// HandleKeyUp routes key up events to the current top handler
func (km *KeyManager) HandleKeyUp(ev *fyne.KeyEvent) {
	km.mutex.Lock()
	// Update modifier state first
	modifierHandled := km.updateModifierState(ev, false)
	modifiers := km.modifierState
	km.mutex.Unlock()

	// Get current handler
	km.mutex.RLock()
	currentHandler := km.GetCurrentHandler()
	km.mutex.RUnlock()

	if currentHandler != nil {
		handled := currentHandler.OnKeyUp(ev, modifiers)
		km.debugPrint("KeyManager: KeyUp event handled by '%s': %t (modifier: %t)", currentHandler.GetName(), handled, modifierHandled)
	} else {
		km.debugPrint("KeyManager: No handler available for KeyUp event")
	}
}

// HandleTypedKey routes typed key events to the current top handler
func (km *KeyManager) HandleTypedKey(ev *fyne.KeyEvent) {
	km.mutex.RLock()
	modifiers := km.modifierState
	currentHandler := km.GetCurrentHandler()
	km.mutex.RUnlock()

	if currentHandler != nil {
		handled := currentHandler.OnTypedKey(ev, modifiers)
		km.debugPrint("KeyManager: TypedKey event handled by '%s': %t", currentHandler.GetName(), handled)
	} else {
		km.debugPrint("KeyManager: No handler available for TypedKey event")
	}
}

// HandleTypedRune routes typed rune events to the current top handler
func (km *KeyManager) HandleTypedRune(r rune) {
	km.mutex.RLock()
	modifiers := km.modifierState
	currentHandler := km.GetCurrentHandler()
	km.mutex.RUnlock()

	if currentHandler != nil {
		handled := currentHandler.OnTypedRune(r, modifiers)
		km.debugPrint("KeyManager: TypedRune event handled by '%s': %t", currentHandler.GetName(), handled)
	} else {
		km.debugPrint("KeyManager: No handler available for TypedRune event")
	}
}

// GetStackSize returns the current number of handlers in the stack
func (km *KeyManager) GetStackSize() int {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	return len(km.handlers)
}

// ListHandlers returns the names of all handlers in the stack (for debugging)
func (km *KeyManager) ListHandlers() []string {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	names := make([]string, len(km.handlers))
	for i, handler := range km.handlers {
		names[i] = handler.GetName()
	}

	return names
}
