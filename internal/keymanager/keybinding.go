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
	last := strings.TrimSpace(parts[len(parts)-1])
	if last == "" {
		return keySpec{}, fmt.Errorf("missing key name in %q", input)
	}

	spec := keySpec{key: normalizeKeyName(last)}
	for _, part := range parts[:len(parts)-1] {
		switch strings.ToUpper(strings.TrimSpace(part)) {
		case "S", "SHIFT":
			spec.mod.ShiftPressed = true
		case "C", "CTRL", "CONTROL", "^":
			spec.mod.CtrlPressed = true
		case "A", "ALT", "M", "META":
			spec.mod.AltPressed = true
		case "":
		default:
			return keySpec{}, fmt.Errorf("unknown modifier %q in %q", part, input)
		}
	}

	for strings.HasPrefix(last, "^") {
		spec.mod.CtrlPressed = true
		last = strings.TrimPrefix(last, "^")
		if last == "" {
			return keySpec{}, fmt.Errorf("missing key name in %q", input)
		}
		spec.key = normalizeKeyName(last)
	}

	return spec, nil
}

func normalizeKeyName(name string) fyne.KeyName {
	upper := strings.ToUpper(strings.TrimSpace(name))
	switch upper {
	case "ENTER":
		return fyne.KeyReturn
	case "RETURN":
		return fyne.KeyReturn
	case "ESC":
		return fyne.KeyEscape
	case "SPACE":
		return fyne.KeySpace
	case "TAB":
		return fyne.KeyTab
	case "BACKSPACE":
		return fyne.KeyBackspace
	case "DELETE", "DEL":
		return fyne.KeyDelete
	case "UP":
		return fyne.KeyUp
	case "DOWN":
		return fyne.KeyDown
	case "LEFT":
		return fyne.KeyLeft
	case "RIGHT":
		return fyne.KeyRight
	case "COMMA":
		return fyne.KeyComma
	case "PERIOD", "DOT":
		return fyne.KeyPeriod
	case "BACKTICK", "BACKQUOTE":
		return fyne.KeyBackTick
	default:
		if strings.HasPrefix(upper, "F") {
			return fyne.KeyName(upper)
		}
		if len([]rune(upper)) == 1 {
			return fyne.KeyName(upper)
		}
		return fyne.KeyName(name)
	}
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
