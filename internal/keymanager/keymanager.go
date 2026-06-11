package keymanager

import (
	"fmt"
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

// KeyHandler defines the interface for handling keyboard events.
// Handlers only see key activations: TypedKey and TypedShortcut events,
// including key repeats, merged into OnKeyActivated. Raw key down/up are
// KeyManager-internal plumbing (modifier tracking, folded-shortcut
// reconstruction, gate arming) and are never dispatched to handlers.
type KeyHandler interface {
	// OnKeyActivated handles a key activation (typed key or shortcut)
	OnKeyActivated(ev *fyne.KeyEvent, modifiers ModifierState) bool // returns true if handled

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

// KeyManager manages a stack of key handlers.
//
// Input gating model: activation events (typed keys, shortcuts, runes) are
// delivered only while the gate is armed. The gate disarms on every input
// owner change (handler push/remove, queued owner transition, focus change)
// and re-arms on the next fresh non-modifier key down. Because key repeats
// never produce key-down events, a key held across an owner change can never
// fire into the new owner; the arming press itself is fully delivered since
// its key-down precedes its typed events.
type KeyManager struct {
	handlers          []handlerEntry
	nextToken         HandlerToken
	modifierState     ModifierState
	mutex             sync.RWMutex
	debugPrint        func(format string, args ...interface{})
	stackVersion      uint64
	lastKeyDown       fyne.KeyName
	armed             bool
	queuedTransitions int
	queueOnMain       func(func())
}

// NewKeyManager creates a new KeyManager instance
func NewKeyManager(debugPrint func(format string, args ...interface{})) *KeyManager {
	return &KeyManager{
		handlers:    make([]handlerEntry, 0),
		debugPrint:  debugPrint,
		queueOnMain: queueOnFyneMain,
	}
}

// queueOnFyneMain schedules fn onto the next Fyne main-loop iteration, after
// the current event batch (including the trailing TypedRune of the same key
// press) has been processed. Without a running app (headless tests) it runs
// fn synchronously.
func queueOnFyneMain(fn func()) {
	if fyne.CurrentApp() == nil {
		fn()
		return
	}
	fyne.Do(fn)
}

// PushHandler adds a new key handler to the top of the stack and returns a
// token that removes exactly this entry via RemoveHandler. The input owner
// changes, so the gate disarms until the next fresh key press.
func (km *KeyManager) PushHandler(handler KeyHandler) HandlerToken {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	km.nextToken++
	token := km.nextToken
	km.handlers = append(km.handlers, handlerEntry{token: token, handler: handler})
	km.stackVersion++
	km.armed = false
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
		// The active input owner changed; transient input state belongs to it.
		km.modifierState = ModifierState{}
		km.armed = false
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

// BeginOwnerTransition queues action onto the next Fyne main-loop iteration
// and gates activation events until a fresh key press after it has run. Use
// it for any action that changes the input owner (opening or closing a
// dialog, moving window focus, entering an input mode): the remaining events
// of the triggering key press are delivered to the old owner in the current
// iteration, and held-key repeats can never fire into the new owner because
// repeats do not produce key-down events. Events arriving while the
// transition is queued are dropped, not queued.
func (km *KeyManager) BeginOwnerTransition(label string, action func()) {
	if action == nil {
		return
	}
	km.mutex.Lock()
	km.queuedTransitions++
	km.mutex.Unlock()
	km.debugPrint("KeyManager: transition queued label=%s", label)

	km.queueOnMain(func() {
		km.debugPrint("KeyManager: transition run label=%s", label)
		action()
		km.mutex.Lock()
		km.queuedTransitions--
		km.armed = false
		km.mutex.Unlock()
	})
}

// ResetTransientState clears per-press input state (modifiers, gate arming,
// folded-shortcut reconstruction). Called on focus changes so that state from
// before a focus loss can never leak into the new focus owner.
func (km *KeyManager) ResetTransientState(label string) {
	km.mutex.Lock()
	km.modifierState = ModifierState{}
	km.lastKeyDown = ""
	km.armed = false
	km.mutex.Unlock()
	km.debugPrint("KeyManager: transient state reset label=%s", label)
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

// HandleKeyDown records key press plumbing state: modifier tracking, the key
// behind folded standard shortcuts, and gate arming. A fresh non-modifier
// press arms the gate (unless an owner transition is still queued), so the
// same press's typed events are delivered. It never dispatches to handlers.
func (km *KeyManager) HandleKeyDown(ev *fyne.KeyEvent) {
	km.mutex.Lock()
	modifierHandled := km.updateModifierState(ev, true)
	armedNow := false
	if !modifierHandled && ev != nil && ev.Name != "" {
		// Remembered to reconstruct the physical key behind folded standard
		// shortcuts (e.g. Ctrl+Insert vs Ctrl+C both arrive as ShortcutCopy).
		km.lastKeyDown = normalizeDrainKey(ev.Name)
		if km.queuedTransitions == 0 && !km.armed {
			km.armed = true
			armedNow = true
		}
	}
	km.mutex.Unlock()

	km.debugPrint("KeyManager: KeyDown recorded key=%s mod=%t armed=%t", ev.Name, modifierHandled, armedNow)
}

// HandleKeyUp records modifier key releases. It never dispatches to handlers.
func (km *KeyManager) HandleKeyUp(ev *fyne.KeyEvent) {
	km.mutex.Lock()
	modifierHandled := km.updateModifierState(ev, false)
	km.mutex.Unlock()

	km.debugPrint("KeyManager: KeyUp recorded key=%s mod=%t", ev.Name, modifierHandled)
}

func (km *KeyManager) gateOpen() (bool, ModifierState) {
	km.mutex.RLock()
	defer km.mutex.RUnlock()
	return km.armed && km.queuedTransitions == 0, km.modifierState
}

// HandleTypedKey routes typed key events to the current top handler
func (km *KeyManager) HandleTypedKey(ev *fyne.KeyEvent) {
	open, modifiers := km.gateOpen()
	if !open {
		km.debugPrint("KeyManager: TypedKey gated key=%s", ev.Name)
		return
	}
	km.handleKeyActivated(ev, modifiers)
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
// Modifiers come from the shortcut event itself, not from tracked state.
func (km *KeyManager) HandleShortcutKey(ev *fyne.KeyEvent, modifiers ModifierState) {
	open, _ := km.gateOpen()
	if !open {
		km.debugPrint("KeyManager: ShortcutKey gated key=%s", ev.Name)
		return
	}
	km.handleKeyActivated(ev, modifiers)
}

func (km *KeyManager) handleKeyActivated(ev *fyne.KeyEvent, modifiers ModifierState) {
	currentHandler, _ := km.currentHandlerAndVersion()

	if currentHandler != nil {
		handled := currentHandler.OnKeyActivated(ev, modifiers)
		km.debugPrint("KeyManager: KeyActivated %s handled=%t", currentHandler.GetName(), handled)
		return
	}
	km.debugPrint("KeyManager: KeyActivated no handler")
}

// HandleTypedRune routes typed rune events to the current top handler
func (km *KeyManager) HandleTypedRune(r rune) {
	open, modifiers := km.gateOpen()
	if !open {
		km.debugPrint("KeyManager: TypedRune gated rune=%q", r)
		return
	}

	currentHandler, _ := km.currentHandlerAndVersion()
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

	lines := []string{
		fmt.Sprintf("stackVersion=%d", km.stackVersion),
		fmt.Sprintf("handlers=%s", formatStateList(handlers)),
		fmt.Sprintf("modifiers=shift:%t ctrl:%t alt:%t", km.modifierState.ShiftPressed, km.modifierState.CtrlPressed, km.modifierState.AltPressed),
		fmt.Sprintf("armed=%t", km.armed),
		fmt.Sprintf("queuedTransitions=%d", km.queuedTransitions),
		fmt.Sprintf("lastKeyDown=%s", km.lastKeyDown),
	}
	return strings.Join(lines, "\n")
}

func formatStateList(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	return "[" + strings.Join(values, ",") + "]"
}
