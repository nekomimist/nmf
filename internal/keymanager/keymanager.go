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
	AltPressed   bool
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
	handlers       []KeyHandler
	modifierState  ModifierState
	mutex          sync.RWMutex
	debugPrint     func(format string, args ...interface{})
	stackVersion   uint64
	pressedKeys    map[fyne.KeyName]struct{}
	pending        []pendingTransition
	suppressTyped  map[fyne.KeyName]struct{}
	suppressRune   bool
	activeTypedKey fyne.KeyName
}

type pendingTransition struct {
	label  string
	action func()
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
	km.stackVersion++
	km.debugPrint("KeyManager: push %s stack=%d", handler.GetName(), len(km.handlers))
}

// PopHandler removes the top key handler from the stack
func (km *KeyManager) PopHandler() KeyHandler {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	if len(km.handlers) == 0 {
		km.debugPrint("KeyManager: pop empty stack")
		return nil
	}

	// Get the last handler
	handler := km.handlers[len(km.handlers)-1]

	// Remove it from the slice
	km.handlers = km.handlers[:len(km.handlers)-1]
	km.stackVersion++
	km.modifierState = ModifierState{}

	km.debugPrint("KeyManager: pop %s stack=%d", handler.GetName(), len(km.handlers))
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

func (km *KeyManager) currentHandlerAndVersion() (KeyHandler, uint64) {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	if len(km.handlers) == 0 {
		return nil, km.stackVersion
	}
	return km.handlers[len(km.handlers)-1], km.stackVersion
}

func normalizeDrainKey(name fyne.KeyName) fyne.KeyName {
	switch name {
	case fyne.KeyEnter:
		return fyne.KeyReturn
	default:
		return name
	}
}

func (km *KeyManager) recordKeyDownLocked(ev *fyne.KeyEvent) {
	if ev == nil || ev.Name == "" {
		return
	}
	if km.pressedKeys == nil {
		km.pressedKeys = make(map[fyne.KeyName]struct{})
	}
	keyName := normalizeDrainKey(ev.Name)
	km.pressedKeys[keyName] = struct{}{}
	km.suppressRune = false
	if km.suppressTyped != nil {
		delete(km.suppressTyped, keyName)
		if len(km.suppressTyped) == 0 {
			km.suppressTyped = nil
		}
	}
}

func (km *KeyManager) recordKeyUpLocked(ev *fyne.KeyEvent) {
	if ev == nil || ev.Name == "" || km.pressedKeys == nil {
		return
	}
	keyName := normalizeDrainKey(ev.Name)
	if len(km.pending) > 0 {
		if km.suppressTyped == nil {
			km.suppressTyped = make(map[fyne.KeyName]struct{})
		}
		km.suppressTyped[keyName] = struct{}{}
	}
	delete(km.pressedKeys, keyName)
	if len(km.pressedKeys) == 0 {
		km.pressedKeys = nil
	}
}

func (km *KeyManager) hasPendingTransitionLocked() bool {
	return len(km.pending) > 0
}

func (km *KeyManager) flushPendingTransitionsLocked() []pendingTransition {
	if len(km.pressedKeys) > 0 || len(km.pending) == 0 {
		return nil
	}
	pending := km.pending
	km.pending = nil
	km.modifierState = ModifierState{}
	km.suppressRune = true
	return pending
}

func (km *KeyManager) shouldSuppressTypedKeyLocked(ev *fyne.KeyEvent) bool {
	if ev == nil || ev.Name == "" || km.suppressTyped == nil {
		return false
	}
	keyName := normalizeDrainKey(ev.Name)
	if _, ok := km.suppressTyped[keyName]; !ok {
		return false
	}
	delete(km.suppressTyped, keyName)
	if len(km.suppressTyped) == 0 {
		km.suppressTyped = nil
	}
	return true
}

func (km *KeyManager) shouldSuppressRuneLocked() bool {
	if !km.suppressRune {
		return false
	}
	km.suppressRune = false
	return true
}

func (km *KeyManager) runPendingTransitions(pending []pendingTransition) {
	for _, transition := range pending {
		km.debugPrint("KeyManager: transition run label=%s", transition.label)
		transition.action()
	}
}

// DeferUntilKeysReleased runs action after all currently pressed keys are released.
func (km *KeyManager) DeferUntilKeysReleased(label string, action func()) {
	if action == nil {
		return
	}

	km.mutex.Lock()
	if len(km.pressedKeys) == 0 && km.activeTypedKey != "" {
		if km.pressedKeys == nil {
			km.pressedKeys = make(map[fyne.KeyName]struct{})
		}
		km.pressedKeys[km.activeTypedKey] = struct{}{}
	}
	if len(km.pressedKeys) == 0 {
		km.modifierState = ModifierState{}
		km.mutex.Unlock()
		km.debugPrint("KeyManager: transition run label=%s", label)
		action()
		return
	}

	km.pending = append(km.pending, pendingTransition{label: label, action: action})
	pressed := len(km.pressedKeys)
	km.mutex.Unlock()

	km.debugPrint("KeyManager: transition defer label=%s pressed=%d", label, pressed)
}

// updateModifierState updates the modifier key state based on key events
func (km *KeyManager) updateModifierState(ev *fyne.KeyEvent, pressed bool) bool {
	switch ev.Name {
	case desktop.KeyShiftLeft, desktop.KeyShiftRight:
		km.modifierState.ShiftPressed = pressed
		km.debugPrint("KeyManager: Shift %s", map[bool]string{true: "down", false: "up"}[pressed])
		return true
	case desktop.KeyControlLeft, desktop.KeyControlRight:
		km.modifierState.CtrlPressed = pressed
		km.debugPrint("KeyManager: Ctrl %s", map[bool]string{true: "down", false: "up"}[pressed])
		return true
	case desktop.KeyAltLeft, desktop.KeyAltRight:
		km.modifierState.AltPressed = pressed
		km.debugPrint("KeyManager: Alt %s", map[bool]string{true: "down", false: "up"}[pressed])
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
	km.recordKeyDownLocked(ev)
	modifiers := km.modifierState
	pending := km.hasPendingTransitionLocked()
	km.mutex.Unlock()

	if pending {
		km.debugPrint("KeyManager: KeyDown gated key=%s mod=%t", ev.Name, modifierHandled)
		return
	}

	currentHandler, _ := km.currentHandlerAndVersion()

	if currentHandler != nil {
		handled := currentHandler.OnKeyDown(ev, modifiers)
		km.debugPrint("KeyManager: KeyDown %s handled=%t mod=%t", currentHandler.GetName(), handled, modifierHandled)
	} else {
		km.debugPrint("KeyManager: KeyDown no handler")
	}
}

// HandleKeyUp routes key up events to the current top handler
func (km *KeyManager) HandleKeyUp(ev *fyne.KeyEvent) {
	km.mutex.Lock()
	// Update modifier state first
	modifierHandled := km.updateModifierState(ev, false)
	km.recordKeyUpLocked(ev)
	modifiers := km.modifierState
	pending := km.hasPendingTransitionLocked()
	pendingTransitions := km.flushPendingTransitionsLocked()
	km.mutex.Unlock()

	if pending {
		km.debugPrint("KeyManager: KeyUp gated key=%s mod=%t", ev.Name, modifierHandled)
		km.runPendingTransitions(pendingTransitions)
		return
	}

	currentHandler, _ := km.currentHandlerAndVersion()

	if currentHandler != nil {
		handled := currentHandler.OnKeyUp(ev, modifiers)
		km.debugPrint("KeyManager: KeyUp %s handled=%t mod=%t", currentHandler.GetName(), handled, modifierHandled)
	} else {
		km.debugPrint("KeyManager: KeyUp no handler")
	}
}

// HandleTypedKey routes typed key events to the current top handler
func (km *KeyManager) HandleTypedKey(ev *fyne.KeyEvent) {
	km.mutex.Lock()
	suppressed := km.shouldSuppressTypedKeyLocked(ev)
	if suppressed {
		km.mutex.Unlock()
		km.debugPrint("KeyManager: TypedKey suppressed key=%s", ev.Name)
		return
	}
	pending := km.hasPendingTransitionLocked()
	modifiers := km.modifierState
	var active fyne.KeyName
	if ev != nil {
		active = normalizeDrainKey(ev.Name)
		km.activeTypedKey = active
	}
	km.mutex.Unlock()

	if pending {
		km.debugPrint("KeyManager: TypedKey gated key=%s", ev.Name)
		km.clearActiveTypedKey(active)
		return
	}

	handled := km.handleTypedKey(ev, modifiers)
	if handled {
		km.clearPressedKeyWithoutPending(active)
	}
	km.clearActiveTypedKey(active)
}

// HandleShortcutKey routes a shortcut-style key event to the current handler.
func (km *KeyManager) HandleShortcutKey(ev *fyne.KeyEvent, modifiers ModifierState) {
	km.mutex.Lock()
	suppressed := km.shouldSuppressTypedKeyLocked(ev)
	if suppressed {
		km.mutex.Unlock()
		km.debugPrint("KeyManager: ShortcutKey suppressed key=%s", ev.Name)
		return
	}
	pending := km.hasPendingTransitionLocked()
	var active fyne.KeyName
	if ev != nil {
		active = normalizeDrainKey(ev.Name)
		km.activeTypedKey = active
	}
	km.mutex.Unlock()

	if pending {
		km.debugPrint("KeyManager: ShortcutKey gated key=%s", ev.Name)
		km.clearActiveTypedKey(active)
		return
	}

	handled := km.handleTypedKey(ev, modifiers)
	if handled {
		km.clearPressedKeyWithoutPending(active)
	}
	km.clearActiveTypedKey(active)
}

func (km *KeyManager) clearPressedKeyWithoutPending(key fyne.KeyName) {
	if key == "" {
		return
	}
	km.mutex.Lock()
	if len(km.pending) == 0 && km.pressedKeys != nil {
		delete(km.pressedKeys, key)
		if len(km.pressedKeys) == 0 {
			km.pressedKeys = nil
		}
	}
	km.mutex.Unlock()
}

func (km *KeyManager) clearActiveTypedKey(key fyne.KeyName) {
	if key == "" {
		return
	}
	km.mutex.Lock()
	if km.activeTypedKey == key {
		km.activeTypedKey = ""
	}
	km.mutex.Unlock()
}

func (km *KeyManager) handleTypedKey(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	currentHandler, _ := km.currentHandlerAndVersion()

	if currentHandler != nil {
		handled := currentHandler.OnTypedKey(ev, modifiers)
		km.debugPrint("KeyManager: TypedKey %s handled=%t", currentHandler.GetName(), handled)
		return handled
	}
	km.debugPrint("KeyManager: TypedKey no handler")
	return false
}

// HandleTypedRune routes typed rune events to the current top handler
func (km *KeyManager) HandleTypedRune(r rune) {
	km.mutex.Lock()
	pending := km.hasPendingTransitionLocked()
	suppressed := km.shouldSuppressRuneLocked()
	modifiers := km.modifierState
	var currentHandler KeyHandler
	if len(km.handlers) > 0 {
		currentHandler = km.handlers[len(km.handlers)-1]
	}
	km.mutex.Unlock()

	if suppressed {
		km.debugPrint("KeyManager: TypedRune suppressed rune=%q", r)
		return
	}
	if pending {
		km.debugPrint("KeyManager: TypedRune gated rune=%q", r)
		return
	}

	if currentHandler != nil {
		handled := currentHandler.OnTypedRune(r, modifiers)
		km.debugPrint("KeyManager: TypedRune %s handled=%t", currentHandler.GetName(), handled)
	} else {
		km.debugPrint("KeyManager: TypedRune no handler")
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
