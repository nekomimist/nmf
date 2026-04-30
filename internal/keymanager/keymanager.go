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
	handlers      []KeyHandler
	modifierState ModifierState
	mutex         sync.RWMutex
	debugPrint    func(format string, args ...interface{})
	stackVersion  uint64
	drainingKeys  map[fyne.KeyName]struct{}
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

func (km *KeyManager) currentVersion() uint64 {
	km.mutex.RLock()
	defer km.mutex.RUnlock()
	return km.stackVersion
}

func normalizeDrainKey(name fyne.KeyName) fyne.KeyName {
	switch name {
	case fyne.KeyEnter:
		return fyne.KeyReturn
	default:
		return name
	}
}

func (km *KeyManager) isDrainingKeyLocked(name fyne.KeyName) bool {
	if km.drainingKeys == nil {
		return false
	}
	_, ok := km.drainingKeys[normalizeDrainKey(name)]
	return ok
}

func (km *KeyManager) shouldDrainKey(name fyne.KeyName) bool {
	km.mutex.RLock()
	defer km.mutex.RUnlock()
	return km.isDrainingKeyLocked(name)
}

func (km *KeyManager) markDrainKey(ev *fyne.KeyEvent) {
	if ev == nil || ev.Name == "" {
		return
	}

	km.mutex.Lock()
	defer km.mutex.Unlock()

	if km.drainingKeys == nil {
		km.drainingKeys = make(map[fyne.KeyName]struct{})
	}
	keyName := normalizeDrainKey(ev.Name)
	km.drainingKeys[keyName] = struct{}{}
	km.debugPrint("KeyManager: drain start key=%s", keyName)
}

func (km *KeyManager) clearDrainKeyLocked(name fyne.KeyName) bool {
	if km.drainingKeys == nil {
		return false
	}
	keyName := normalizeDrainKey(name)
	if _, ok := km.drainingKeys[keyName]; !ok {
		return false
	}
	delete(km.drainingKeys, keyName)
	km.debugPrint("KeyManager: drain end key=%s", keyName)
	return true
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
	modifiers := km.modifierState
	draining := km.isDrainingKeyLocked(ev.Name)
	km.mutex.Unlock()

	if draining {
		km.debugPrint("KeyManager: KeyDown drained key=%s mod=%t", ev.Name, modifierHandled)
		return
	}

	currentHandler, beforeVersion := km.currentHandlerAndVersion()

	if currentHandler != nil {
		handled := currentHandler.OnKeyDown(ev, modifiers)
		afterVersion := km.currentVersion()
		if afterVersion != beforeVersion {
			km.markDrainKey(ev)
		}
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
	modifiers := km.modifierState
	drained := km.clearDrainKeyLocked(ev.Name)
	km.mutex.Unlock()

	if drained {
		km.debugPrint("KeyManager: KeyUp drained key=%s mod=%t", ev.Name, modifierHandled)
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
	if km.shouldDrainKey(ev.Name) {
		km.debugPrint("KeyManager: TypedKey drained key=%s", ev.Name)
		return
	}

	km.mutex.RLock()
	modifiers := km.modifierState
	km.mutex.RUnlock()

	currentHandler, beforeVersion := km.currentHandlerAndVersion()

	if currentHandler != nil {
		handled := currentHandler.OnTypedKey(ev, modifiers)
		afterVersion := km.currentVersion()
		if afterVersion != beforeVersion {
			km.markDrainKey(ev)
		}
		km.debugPrint("KeyManager: TypedKey %s handled=%t", currentHandler.GetName(), handled)
	} else {
		km.debugPrint("KeyManager: TypedKey no handler")
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
