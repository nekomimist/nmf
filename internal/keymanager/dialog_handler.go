package keymanager

import (
	"fmt"

	"fyne.io/fyne/v2"
)

// dialogBinding pairs a static key spec with the action to run on an exact
// match. spec uses the same syntax as configured key bindings (keybinding.go:
// parseKeySpec) -- "Esc", "Return", "C-Return", "S-Up", "Space", "1", "Tab",
// "S-Tab", and so on.
type dialogBinding struct {
	spec   string
	action func()
}

// dialogKeyHandler is the shared base for per-dialog KeyHandlers whose
// OnKeyActivated used to be a hand-written switch over ev.Name/modifiers
// implementing the same Esc=cancel / Enter=accept / arrows=move shape. It
// mirrors the main screen's configurable-binding model (keybinding.go) but
// with a static table instead of config-driven entries: dialog bindings are
// not user-configurable, so there's no config file to read and no target
// selection.
//
// Matching is exact-modifier, identical to keyBinding.matches (they share
// keySpec.matches): a binding only fires when every modifier bit is exactly
// as specified, same semantics as the main screen's declarative bindings.
//
// Construct with newDialogKeyHandler and optionally chain withRune /
// withFallback. Each per-dialog handler type embeds *dialogKeyHandler and
// gets GetName/OnKeyActivated/OnTypedRune for free; a handler that needs a
// custom GetName or wants to intercept OnTypedRune itself can just shadow the
// embedded method.
type dialogKeyHandler struct {
	name       string
	debugPrint func(format string, args ...interface{})
	specs      []keySpec
	actions    []func()
	runeFunc   func(r rune, modifiers ModifierState) bool
	fallback   func(ev *fyne.KeyEvent, modifiers ModifierState) bool
}

// newDialogKeyHandler builds a base handler from a static binding table. Each
// spec is parsed once, here, via parseKeySpec -- the same parser configured
// key bindings use. Specs in this package are static string literals written
// by a programmer, not user config, so a spec that fails to parse is a typo
// caught at construction: it panics immediately rather than silently
// dropping a binding the way a bad config.json entry is just warned about and
// skipped (buildTargetKeyBindings). Failing loudly here means a broken dialog
// binding cannot ship unnoticed.
func newDialogKeyHandler(name string, debugPrint func(format string, args ...interface{}), bindings []dialogBinding) *dialogKeyHandler {
	if debugPrint == nil {
		debugPrint = func(string, ...interface{}) {}
	}
	h := &dialogKeyHandler{
		name:       name,
		debugPrint: debugPrint,
		specs:      make([]keySpec, 0, len(bindings)),
		actions:    make([]func(), 0, len(bindings)),
	}
	for _, b := range bindings {
		spec, err := parseKeySpec(b.spec)
		if err != nil {
			panic(fmt.Sprintf("keymanager: %s: invalid static key spec %q: %v", name, b.spec, err))
		}
		h.specs = append(h.specs, spec)
		h.actions = append(h.actions, b.action)
	}
	return h
}

// withRune attaches an OnTypedRune implementation, for dialogs that consume
// typed text (search/filter entry) or bare-rune shortcuts (sort's o/d).
// Returns h so it can be chained onto newDialogKeyHandler.
func (h *dialogKeyHandler) withRune(fn func(r rune, modifiers ModifierState) bool) *dialogKeyHandler {
	h.runeFunc = fn
	return h
}

// withFallback attaches an escape hatch invoked when no binding matches,
// for behavior that genuinely isn't a fixed key->action table (e.g. the quit
// dialog consuming every unmatched key so nothing leaks to MainScreen).
// Returns h so it can be chained onto newDialogKeyHandler.
func (h *dialogKeyHandler) withFallback(fn func(ev *fyne.KeyEvent, modifiers ModifierState) bool) *dialogKeyHandler {
	h.fallback = fn
	return h
}

// GetName returns the handler name passed to newDialogKeyHandler. Kept
// identical to each dialog's pre-conversion GetName() value since debug logs
// and tests depend on it.
func (h *dialogKeyHandler) GetName() string { return h.name }

// OnKeyActivated matches ev/modifiers against the binding table in order and
// runs the first exact match. A nil ev never matches (keySpec.matches
// guards it) and is not passed to debugPrint.
func (h *dialogKeyHandler) OnKeyActivated(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	if ev != nil {
		h.debugPrint("%s: OnKeyActivated %s", h.name, ev.Name)
	}
	for i, spec := range h.specs {
		if !spec.matches(ev, modifiers) {
			continue
		}
		h.actions[i]()
		return true
	}
	if h.fallback != nil {
		return h.fallback(ev, modifiers)
	}
	return false
}

// OnTypedRune delegates to the rune handler attached via withRune, if any.
func (h *dialogKeyHandler) OnTypedRune(r rune, modifiers ModifierState) bool {
	if h.runeFunc != nil {
		return h.runeFunc(r, modifiers)
	}
	return false
}
