package keymanager

import (
	"fmt"
	"sort"
	"strings"
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

// None reports whether no modifier key is active.
func (m ModifierState) None() bool {
	return !m.ShiftPressed && !m.CtrlPressed && !m.AltPressed
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

// HandlerToken identifies one pushed stack entry. The zero value is never
// issued, so removing a zero token is a safe no-op.
type HandlerToken uint64

type handlerEntry struct {
	token   HandlerToken
	handler KeyHandler
}

// KeyManager manages a stack of key handlers
type KeyManager struct {
	handlers       []handlerEntry
	nextToken      HandlerToken
	modifierState  ModifierState
	mutex          sync.RWMutex
	debugPrint     func(format string, args ...interface{})
	stackVersion   uint64
	pressedKeys    map[fyne.KeyName]struct{}
	pending        []pendingTransition
	suppressTyped  map[fyne.KeyName]struct{}
	suppressRune   bool
	activeTypedKey fyne.KeyName
	lastKeyDown    fyne.KeyName
}

type pendingTransition struct {
	label  string
	action func()
}

// NewKeyManager creates a new KeyManager instance
func NewKeyManager(debugPrint func(format string, args ...interface{})) *KeyManager {
	return &KeyManager{
		handlers:   make([]handlerEntry, 0),
		debugPrint: debugPrint,
	}
}

// PushHandler adds a new key handler to the top of the stack and returns a
// token that removes exactly this entry via RemoveHandler.
func (km *KeyManager) PushHandler(handler KeyHandler) HandlerToken {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	km.nextToken++
	token := km.nextToken
	km.handlers = append(km.handlers, handlerEntry{token: token, handler: handler})
	km.stackVersion++
	km.debugPrint("KeyManager: push %s token=%d stack=%d", handler.GetName(), token, len(km.handlers))
	return token
}

// RemoveHandler removes the stack entry identified by token, wherever it is.
// Unlike a blind pop, an out-of-order or duplicate removal cannot evict a
// handler owned by someone else; such calls only log a warning.
func (km *KeyManager) RemoveHandler(token HandlerToken) KeyHandler {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	idx := -1
	for i := len(km.handlers) - 1; i >= 0; i-- {
		if km.handlers[i].token == token {
			idx = i
			break
		}
	}
	if idx < 0 {
		km.debugPrint("KeyManager: WARNING remove unknown token=%d stack=%d", token, len(km.handlers))
		return nil
	}

	entry := km.handlers[idx]
	wasTop := idx == len(km.handlers)-1
	km.handlers = append(km.handlers[:idx], km.handlers[idx+1:]...)
	km.stackVersion++
	if wasTop {
		// The active input owner changed; modifier state belongs to it.
		km.modifierState = ModifierState{}
		km.debugPrint("KeyManager: remove %s token=%d stack=%d", entry.handler.GetName(), token, len(km.handlers))
	} else {
		km.debugPrint("KeyManager: WARNING out-of-order remove %s token=%d index=%d stack=%d", entry.handler.GetName(), token, idx, len(km.handlers))
	}
	return entry.handler
}

// GetCurrentHandler returns the top handler without removing it
func (km *KeyManager) GetCurrentHandler() KeyHandler {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	if len(km.handlers) == 0 {
		return nil
	}

	return km.handlers[len(km.handlers)-1].handler
}

func (km *KeyManager) currentHandlerAndVersion() (KeyHandler, uint64) {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	if len(km.handlers) == 0 {
		return nil, km.stackVersion
	}
	return km.handlers[len(km.handlers)-1].handler, km.stackVersion
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

// ForceReleaseAllKeys clears tracked key state and runs pending transitions.
func (km *KeyManager) ForceReleaseAllKeys(label string) {
	km.mutex.Lock()
	pressed := len(km.pressedKeys)
	pendingCount := len(km.pending)
	if pressed > 0 {
		if km.suppressTyped == nil {
			km.suppressTyped = make(map[fyne.KeyName]struct{})
		}
		for key := range km.pressedKeys {
			km.suppressTyped[key] = struct{}{}
		}
	}
	km.pressedKeys = nil
	km.modifierState = ModifierState{}
	km.activeTypedKey = ""
	km.lastKeyDown = ""
	pendingTransitions := km.flushPendingTransitionsLocked()
	km.mutex.Unlock()

	km.debugPrint("KeyManager: force release label=%s pressed=%d pending=%d", label, pressed, pendingCount)
	km.runPendingTransitions(pendingTransitions)
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
	if !modifierHandled && ev != nil && ev.Name != "" {
		// Remembered to reconstruct the physical key behind folded standard
		// shortcuts (e.g. Ctrl+Insert vs Ctrl+C both arrive as ShortcutCopy).
		km.lastKeyDown = normalizeDrainKey(ev.Name)
	}
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

// HandleShortcut routes any Fyne shortcut to the current handler as a
// shortcut-style key event. Fyne's GLFW driver folds some physical
// combinations into standard shortcuts before they can reach TypedKey
// (Ctrl+C/X/V/A/Z/Y/Insert and Shift+Insert/Delete); the original key is
// reconstructed from the most recent non-modifier key down of the same press.
func (km *KeyManager) HandleShortcut(shortcut fyne.Shortcut) {
	ev, modifiers, ok := km.normalizeShortcut(shortcut)
	if !ok {
		km.debugPrint("KeyManager: Shortcut ignored name=%s", shortcut.ShortcutName())
		return
	}
	km.HandleShortcutKey(ev, modifiers)
}

func (km *KeyManager) normalizeShortcut(shortcut fyne.Shortcut) (*fyne.KeyEvent, ModifierState, bool) {
	if custom, ok := shortcut.(*desktop.CustomShortcut); ok {
		return &fyne.KeyEvent{Name: custom.KeyName}, ModifierState{
			ShiftPressed: custom.Modifier&fyne.KeyModifierShift != 0,
			CtrlPressed:  custom.Modifier&fyne.KeyModifierControl != 0,
			AltPressed:   custom.Modifier&fyne.KeyModifierAlt != 0,
		}, true
	}

	km.mutex.RLock()
	lastKeyDown := km.lastKeyDown
	km.mutex.RUnlock()

	switch shortcut.(type) {
	case *fyne.ShortcutCopy:
		if lastKeyDown == fyne.KeyInsert {
			return &fyne.KeyEvent{Name: fyne.KeyInsert}, ModifierState{CtrlPressed: true}, true
		}
		return &fyne.KeyEvent{Name: fyne.KeyC}, ModifierState{CtrlPressed: true}, true
	case *fyne.ShortcutCut:
		if lastKeyDown == fyne.KeyDelete {
			return &fyne.KeyEvent{Name: fyne.KeyDelete}, ModifierState{ShiftPressed: true}, true
		}
		return &fyne.KeyEvent{Name: fyne.KeyX}, ModifierState{CtrlPressed: true}, true
	case *fyne.ShortcutPaste:
		if lastKeyDown == fyne.KeyInsert {
			return &fyne.KeyEvent{Name: fyne.KeyInsert}, ModifierState{ShiftPressed: true}, true
		}
		return &fyne.KeyEvent{Name: fyne.KeyV}, ModifierState{CtrlPressed: true}, true
	case *fyne.ShortcutSelectAll:
		return &fyne.KeyEvent{Name: fyne.KeyA}, ModifierState{CtrlPressed: true}, true
	case *fyne.ShortcutUndo:
		return &fyne.KeyEvent{Name: fyne.KeyZ}, ModifierState{CtrlPressed: true}, true
	case *fyne.ShortcutRedo:
		return &fyne.KeyEvent{Name: fyne.KeyY}, ModifierState{CtrlPressed: true}, true
	}
	return nil, ModifierState{}, false
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
		currentHandler = km.handlers[len(km.handlers)-1].handler
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
	for i, entry := range km.handlers {
		names[i] = entry.handler.GetName()
	}

	return names
}

// DumpState returns a stable multi-line snapshot of the key routing state.
func (km *KeyManager) DumpState() string {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	handlers := make([]string, len(km.handlers))
	for i, entry := range km.handlers {
		handlers[i] = entry.handler.GetName()
	}
	pending := make([]string, len(km.pending))
	for i, transition := range km.pending {
		pending[i] = transition.label
	}

	lines := []string{
		fmt.Sprintf("stackVersion=%d", km.stackVersion),
		fmt.Sprintf("handlers=%s", formatStateList(handlers)),
		fmt.Sprintf("modifiers=shift:%t ctrl:%t alt:%t", km.modifierState.ShiftPressed, km.modifierState.CtrlPressed, km.modifierState.AltPressed),
		fmt.Sprintf("pressedKeys=%s", formatKeySet(km.pressedKeys)),
		fmt.Sprintf("pendingTransitions=%s", formatStateList(pending)),
		fmt.Sprintf("suppressTyped=%s", formatKeySet(km.suppressTyped)),
		fmt.Sprintf("suppressRune=%t", km.suppressRune),
		fmt.Sprintf("activeTypedKey=%s", km.activeTypedKey),
	}
	return strings.Join(lines, "\n")
}

func formatKeySet(keys map[fyne.KeyName]struct{}) string {
	if len(keys) == 0 {
		return "[]"
	}
	names := make([]string, 0, len(keys))
	for key := range keys {
		names = append(names, string(key))
	}
	sort.Strings(names)
	return formatStateList(names)
}

func formatStateList(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	return "[" + strings.Join(values, ",") + "]"
}
