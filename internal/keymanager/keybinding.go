package keymanager

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"

	"nmf/internal/config"
)

const (
	KeyBindingTargetMain       = "main"
	KeyBindingTargetLineEdit   = "lineEdit"
	KeyBindingTargetFileViewer = "fileViewer"
)

// keyEventTyped is the only activation kind; bindings fire on TypedKey or
// TypedShortcut. It is kept as the CommandContext.Event value for
// compatibility with Starlark user commands that read ctx.event.
const keyEventTyped = "typed"

// CommandContext carries transient input state into command execution.
type CommandContext struct {
	Modifiers ModifierState
	Key       fyne.KeyName
	Event     string

	FileManager FileManagerInterface
	RunCommand  func(command string) bool

	RunExternalCommand func(command string, args []string, edit bool, cwd string) bool
	SetClipboard       func(text string) bool
	DeferTransition    func(label string, action func())

	// UI-launcher closures for the Show* dialogs/menus internal/configscript
	// exposes to Starlark commands (nmf.show_menu, nmf.message,
	// nmf.mkdir(edit=True), nmf.save_clipboard(edit=True)). These mirror the
	// same DialogActions MainScreenKeyHandler uses for its own built-in
	// commands; see mainscreen_handler.go's executeBinding.
	ShowCommandMenu             func(title string, items []CommandMenuItem)
	ShowMessageDialog           func(title string, message string)
	ShowCreateDirectoryDialog   func()
	ShowClipboardTextFileDialog func()
}

// CommandFunc executes an internal command.
type CommandFunc func(CommandContext)

// CommandRegistry maps stable command IDs to implementations.
type CommandRegistry map[string]CommandFunc

// CommandMenuItem describes a UI-agnostic command menu entry.
type CommandMenuItem struct {
	Label     string
	Key       string
	Separator bool
	Action    func()
}

type keyBinding struct {
	spec    keySpec
	command string
}

type keySpec struct {
	key fyne.KeyName
	mod ModifierState
}

func normalizeKeyBindingTarget(target string) string {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "", "main":
		return KeyBindingTargetMain
	case "lineedit", "line-edit", "line_edit":
		return KeyBindingTargetLineEdit
	case "fileviewer", "file-viewer", "file_viewer":
		return KeyBindingTargetFileViewer
	default:
		return strings.TrimSpace(target)
	}
}

// NormalizeKeyBindingTarget returns the canonical target name for user config.
func NormalizeKeyBindingTarget(target string) string {
	return normalizeKeyBindingTarget(target)
}

func buildTargetKeyBindings(
	handlerName string,
	target string,
	configured []config.KeyBindingEntry,
	defaults []config.KeyBindingEntry,
	commandExists func(string) bool,
	debugPrint func(format string, args ...interface{}),
) []keyBinding {
	target = normalizeKeyBindingTarget(target)
	entries := append(targetKeyBindingEntries(configured, target), defaults...)
	bindings := make([]keyBinding, 0, len(entries))
	for _, entry := range entries {
		spec, err := parseKeySpec(entry.Key)
		if err != nil {
			debugPrint("%s: WARNING invalid key binding target=%s key=%q command=%s err=%v", handlerName, target, entry.Key, entry.Command, err)
			continue
		}
		if entry.Event != "" {
			debugPrint("%s: WARNING key binding event=%q is deprecated and ignored target=%s key=%q command=%s", handlerName, entry.Event, target, entry.Key, entry.Command)
		}
		if !commandExists(entry.Command) {
			debugPrint("%s: WARNING invalid key binding target=%s unknown command=%s key=%q", handlerName, target, entry.Command, entry.Key)
			continue
		}
		bindings = append(bindings, keyBinding{spec: spec, command: entry.Command})
	}
	return bindings
}

// WarnUnknownKeyBindingTargets logs one warning per entry whose normalized
// target is not a recognized key-binding target (main, lineEdit, fileViewer).
// targetKeyBindingEntries filters entries by target before validation, so a
// config.json entry with a mistyped target (e.g. "fileviewr") would otherwise
// silently vanish from every target's binding list without any diagnostic.
// Call this once at startup, after the config's key bindings are loaded.
func WarnUnknownKeyBindingTargets(entries []config.KeyBindingEntry, debugPrint func(format string, args ...interface{})) {
	if debugPrint == nil {
		return
	}
	for _, entry := range entries {
		switch normalizeKeyBindingTarget(entry.Target) {
		case KeyBindingTargetMain, KeyBindingTargetLineEdit, KeyBindingTargetFileViewer:
			continue
		}
		debugPrint("KeyBinding: WARNING unknown key binding target=%q key=%q command=%s", entry.Target, entry.Key, entry.Command)
	}
}

func targetKeyBindingEntries(entries []config.KeyBindingEntry, target string) []config.KeyBindingEntry {
	var filtered []config.KeyBindingEntry
	for _, entry := range entries {
		if normalizeKeyBindingTarget(entry.Target) == target {
			filtered = append(filtered, entry)
		}
	}
	return filtered
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
	"SEMICOLON": fyne.KeySemicolon,
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

func (b keyBinding) matches(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	return b.spec.matches(ev, modifiers)
}

// matches reports whether ev/modifiers is an exact match for this spec: the
// key name (after folding KP_Enter to Return, same as the rest of keymanager)
// and every modifier bit must match precisely. This is the single definition
// of "exact match" shared by the configurable main-screen/file-viewer/line-edit
// bindings (via keyBinding.matches) and the static per-dialog bindings built
// by newDialogKeyHandler (dialog_handler.go).
func (s keySpec) matches(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	if ev == nil || normalizeDrainKey(ev.Name) != normalizeDrainKey(s.key) {
		return false
	}
	return s.mod.ShiftPressed == modifiers.ShiftPressed &&
		s.mod.CtrlPressed == modifiers.CtrlPressed &&
		s.mod.AltPressed == modifiers.AltPressed
}
