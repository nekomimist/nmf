package keymanager

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
)

const (
	keyEventTyped = "typed"
	keyEventDown  = "down"
	keyEventUp    = "up"
)

// CommandContext carries transient input state into command execution.
type CommandContext struct {
	Modifiers ModifierState
}

// CommandFunc executes an internal command.
type CommandFunc func(CommandContext)

// CommandRegistry maps stable command IDs to implementations.
type CommandRegistry map[string]CommandFunc

type keyBinding struct {
	spec    keySpec
	event   string
	command string
}

type keySpec struct {
	key fyne.KeyName
	mod ModifierState
}

func parseKeySpec(input string) (keySpec, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return keySpec{}, fmt.Errorf("empty key specification")
	}
	parts := strings.Split(raw, "-")
	modParts := parts[:len(parts)-1]
	keyToken := strings.TrimSpace(parts[len(parts)-1])
	if keyToken == "" {
		if raw == "-" {
			keyToken = "-"
			modParts = nil
		} else if strings.HasSuffix(raw, "--") {
			keyToken = "-"
			modParts = parts[:len(parts)-2]
		}
	}
	if keyToken == "" {
		return keySpec{}, fmt.Errorf("missing key name in %q", input)
	}

	keyName, err := normalizeKeyName(keyToken)
	if err != nil {
		return keySpec{}, err
	}

	spec := keySpec{key: keyName}
	for _, part := range modParts {
		switch strings.ToUpper(strings.TrimSpace(part)) {
		case "S":
			spec.mod.ShiftPressed = true
		case "C":
			spec.mod.CtrlPressed = true
		case "A":
			spec.mod.AltPressed = true
		case "":
		default:
			return keySpec{}, fmt.Errorf("unknown modifier %q in %q", part, input)
		}
	}

	return spec, nil
}

func normalizeKeyName(name string) (fyne.KeyName, error) {
	key := strings.TrimSpace(name)
	if key == "" {
		return fyne.KeyUnknown, fmt.Errorf("empty key name")
	}
	if alias, ok := keyNameAliases[strings.ToUpper(key)]; ok {
		return alias, nil
	}
	if _, ok := validKeyNames[fyne.KeyName(key)]; ok {
		return fyne.KeyName(key), nil
	}
	upper := strings.ToUpper(key)
	if _, ok := validKeyNames[fyne.KeyName(upper)]; ok {
		return fyne.KeyName(upper), nil
	}
	return fyne.KeyUnknown, fmt.Errorf("unknown key name %q", name)
}

var keyNameAliases = map[string]fyne.KeyName{
	"BACKSPACE": fyne.KeyBackspace,
	"BACKTICK":  fyne.KeyBackTick,
	"BACKQUOTE": fyne.KeyBackTick,
	"COMMA":     fyne.KeyComma,
	"DEL":       fyne.KeyDelete,
	"DOT":       fyne.KeyPeriod,
	"ENTER":     fyne.KeyReturn,
	"ESC":       fyne.KeyEscape,
	"PAGEUP":    fyne.KeyPageUp,
	"PAGEDOWN":  fyne.KeyPageDown,
	"PERIOD":    fyne.KeyPeriod,
}

var validKeyNames = map[fyne.KeyName]struct{}{
	fyne.KeyEscape:       {},
	fyne.KeyReturn:       {},
	fyne.KeyTab:          {},
	fyne.KeyBackspace:    {},
	fyne.KeyInsert:       {},
	fyne.KeyDelete:       {},
	fyne.KeyRight:        {},
	fyne.KeyLeft:         {},
	fyne.KeyDown:         {},
	fyne.KeyUp:           {},
	fyne.KeyPageUp:       {},
	fyne.KeyPageDown:     {},
	fyne.KeyHome:         {},
	fyne.KeyEnd:          {},
	fyne.KeyF1:           {},
	fyne.KeyF2:           {},
	fyne.KeyF3:           {},
	fyne.KeyF4:           {},
	fyne.KeyF5:           {},
	fyne.KeyF6:           {},
	fyne.KeyF7:           {},
	fyne.KeyF8:           {},
	fyne.KeyF9:           {},
	fyne.KeyF10:          {},
	fyne.KeyF11:          {},
	fyne.KeyF12:          {},
	fyne.KeyEnter:        {},
	fyne.Key0:            {},
	fyne.Key1:            {},
	fyne.Key2:            {},
	fyne.Key3:            {},
	fyne.Key4:            {},
	fyne.Key5:            {},
	fyne.Key6:            {},
	fyne.Key7:            {},
	fyne.Key8:            {},
	fyne.Key9:            {},
	fyne.KeyA:            {},
	fyne.KeyB:            {},
	fyne.KeyC:            {},
	fyne.KeyD:            {},
	fyne.KeyE:            {},
	fyne.KeyF:            {},
	fyne.KeyG:            {},
	fyne.KeyH:            {},
	fyne.KeyI:            {},
	fyne.KeyJ:            {},
	fyne.KeyK:            {},
	fyne.KeyL:            {},
	fyne.KeyM:            {},
	fyne.KeyN:            {},
	fyne.KeyO:            {},
	fyne.KeyP:            {},
	fyne.KeyQ:            {},
	fyne.KeyR:            {},
	fyne.KeyS:            {},
	fyne.KeyT:            {},
	fyne.KeyU:            {},
	fyne.KeyV:            {},
	fyne.KeyW:            {},
	fyne.KeyX:            {},
	fyne.KeyY:            {},
	fyne.KeyZ:            {},
	fyne.KeySpace:        {},
	fyne.KeyApostrophe:   {},
	fyne.KeyComma:        {},
	fyne.KeyMinus:        {},
	fyne.KeyPeriod:       {},
	fyne.KeySlash:        {},
	fyne.KeyBackslash:    {},
	fyne.KeyLeftBracket:  {},
	fyne.KeyRightBracket: {},
	fyne.KeySemicolon:    {},
	fyne.KeyEqual:        {},
	fyne.KeyAsterisk:     {},
	fyne.KeyPlus:         {},
	fyne.KeyBackTick:     {},
}

func normalizeEventName(event string, spec keySpec) string {
	switch strings.ToLower(strings.TrimSpace(event)) {
	case keyEventDown, keyEventUp, keyEventTyped:
		return strings.ToLower(strings.TrimSpace(event))
	case "":
		if spec.mod.CtrlPressed || spec.mod.AltPressed {
			return keyEventDown
		}
		return keyEventTyped
	default:
		return ""
	}
}

func (b keyBinding) matches(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	if ev == nil || normalizeDrainKey(ev.Name) != normalizeDrainKey(b.spec.key) {
		return false
	}
	return b.spec.mod.ShiftPressed == modifiers.ShiftPressed &&
		b.spec.mod.CtrlPressed == modifiers.CtrlPressed &&
		b.spec.mod.AltPressed == modifiers.AltPressed
}
