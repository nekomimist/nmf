package keymanager

import (
	"unicode"

	"fyne.io/fyne/v2"

	"nmf/internal/config"
)

const (
	CommandFileViewerClose          = "fileViewer.close"
	CommandFileViewerLineDown       = "fileViewer.line.down"
	CommandFileViewerLineUp         = "fileViewer.line.up"
	CommandFileViewerPageDown       = "fileViewer.page.down"
	CommandFileViewerPageUp         = "fileViewer.page.up"
	CommandFileViewerHome           = "fileViewer.home"
	CommandFileViewerEnd            = "fileViewer.end"
	CommandFileViewerColumnLeft     = "fileViewer.column.left"
	CommandFileViewerColumnRight    = "fileViewer.column.right"
	CommandFileViewerToggleWrap     = "fileViewer.wrap.toggle"
	CommandFileViewerShowText       = "fileViewer.pane.text"
	CommandFileViewerShowMarkdown   = "fileViewer.pane.markdown"
	CommandFileViewerShowHex        = "fileViewer.pane.hex"
	CommandFileViewerSearchNext     = "fileViewer.search.next"
	CommandFileViewerSearchPrevious = "fileViewer.search.previous"
	CommandFileViewerFocusSearch    = "fileViewer.search.focus"
	CommandFileViewerFocusLine      = "fileViewer.line.focus"
	CommandFileViewerCopySelection  = "fileViewer.selection.copy"
	CommandFileViewerSelectAll      = "fileViewer.selection.selectAll"
)

// FileViewerInterface defines keyboard actions for the built-in viewer.
type FileViewerInterface interface {
	CloseViewer()
	ViewerLineDown()
	ViewerLineUp()
	ViewerPageDown()
	ViewerPageUp()
	ViewerHome()
	ViewerEnd()
	ViewerColumnLeft()
	ViewerColumnRight()
	ViewerToggleWrap()
	ViewerShowText()
	ViewerShowMarkdown()
	ViewerShowHex()
	ViewerSearchNext()
	ViewerSearchPrevious()
	ViewerFocusSearch()
	ViewerFocusLine()
	ViewerCopySelection()
	ViewerSelectAll()
}

// FileViewerKeyHandler handles less-like keys for the built-in viewer.
//
// One physical printable-letter press delivers both a TypedKey and a
// TypedRune from Fyne's GLFW driver; both reach this handler as separate
// activations (OnKeyActivated then OnTypedRune). Bindings are split at
// construction time into keyBindings (matched only from OnKeyActivated) and
// runeBindings (matched only from OnTypedRune, via fileViewerRuneKey's
// reverse mapping) so each binding fires on exactly one of the two paths
// instead of matching — and executing — on both.
type FileViewerKeyHandler struct {
	viewer       FileViewerInterface
	keyBindings  []keyBinding
	runeBindings []keyBinding
	commands     map[string]func()
	debugPrint   func(format string, args ...interface{})
}

func NewFileViewerKeyHandler(viewer FileViewerInterface, debugPrint func(format string, args ...interface{}), configuredBindings ...[]config.KeyBindingEntry) *FileViewerKeyHandler {
	var configured []config.KeyBindingEntry
	if len(configuredBindings) > 0 {
		configured = configuredBindings[0]
	}
	h := &FileViewerKeyHandler{viewer: viewer}
	if debugPrint != nil {
		h.debugPrint = debugPrint
	} else {
		h.debugPrint = func(string, ...interface{}) {}
	}
	h.commands = h.defaultCommands()
	bindings := buildTargetKeyBindings(
		"FileViewer",
		KeyBindingTargetFileViewer,
		configured,
		defaultFileViewerBindings(),
		func(command string) bool {
			_, ok := h.commands[command]
			return ok
		},
		h.debugPrint,
	)
	for _, binding := range bindings {
		if fileViewerRunePathSpec(binding.spec) {
			h.runeBindings = append(h.runeBindings, binding)
		} else {
			h.keyBindings = append(h.keyBindings, binding)
		}
	}
	return h
}

func (h *FileViewerKeyHandler) GetName() string { return "FileViewer" }

func (h *FileViewerKeyHandler) OnKeyActivated(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	if ev == nil {
		return false
	}
	return h.executeBinding(h.keyBindings, ev, modifiers)
}

func (h *FileViewerKeyHandler) OnTypedRune(r rune, modifiers ModifierState) bool {
	ev, mods, ok := fileViewerRuneKey(r)
	if !ok {
		return false
	}
	return h.executeBinding(h.runeBindings, ev, mods)
}

func (h *FileViewerKeyHandler) executeBinding(bindings []keyBinding, ev *fyne.KeyEvent, modifiers ModifierState) bool {
	for _, binding := range bindings {
		if !binding.matches(ev, modifiers) {
			continue
		}
		h.debugPrint("FileViewer: command=%s key=%s", binding.command, ev.Name)
		h.commands[binding.command]()
		return true
	}
	return false
}

// fileViewerRunePathSpec reports whether a key spec could be produced by
// fileViewerRuneKey's reverse mapping, i.e. whether the binding belongs on
// the OnTypedRune path rather than OnKeyActivated: key A-Z with modifiers
// none or Shift-only, KeySlash with no modifiers, or KeySemicolon with
// Shift-only. Everything else (arrows, Escape, Space, Ctrl/Alt combos,
// F-keys, ...) never arrives as a TypedRune and stays on the key path.
func fileViewerRunePathSpec(spec keySpec) bool {
	if spec.mod.CtrlPressed || spec.mod.AltPressed {
		return false
	}
	switch {
	case len(spec.key) == 1 && spec.key[0] >= 'A' && spec.key[0] <= 'Z':
		return true
	case spec.key == fyne.KeySlash:
		return !spec.mod.ShiftPressed
	case spec.key == fyne.KeySemicolon:
		return spec.mod.ShiftPressed
	default:
		return false
	}
}

func (h *FileViewerKeyHandler) defaultCommands() map[string]func() {
	return map[string]func(){
		CommandFileViewerClose:          h.viewer.CloseViewer,
		CommandFileViewerLineDown:       h.viewer.ViewerLineDown,
		CommandFileViewerLineUp:         h.viewer.ViewerLineUp,
		CommandFileViewerPageDown:       h.viewer.ViewerPageDown,
		CommandFileViewerPageUp:         h.viewer.ViewerPageUp,
		CommandFileViewerHome:           h.viewer.ViewerHome,
		CommandFileViewerEnd:            h.viewer.ViewerEnd,
		CommandFileViewerColumnLeft:     h.viewer.ViewerColumnLeft,
		CommandFileViewerColumnRight:    h.viewer.ViewerColumnRight,
		CommandFileViewerToggleWrap:     h.viewer.ViewerToggleWrap,
		CommandFileViewerShowText:       h.viewer.ViewerShowText,
		CommandFileViewerShowMarkdown:   h.viewer.ViewerShowMarkdown,
		CommandFileViewerShowHex:        h.viewer.ViewerShowHex,
		CommandFileViewerSearchNext:     h.viewer.ViewerSearchNext,
		CommandFileViewerSearchPrevious: h.viewer.ViewerSearchPrevious,
		CommandFileViewerFocusSearch:    h.viewer.ViewerFocusSearch,
		CommandFileViewerFocusLine:      h.viewer.ViewerFocusLine,
		CommandFileViewerCopySelection:  h.viewer.ViewerCopySelection,
		CommandFileViewerSelectAll:      h.viewer.ViewerSelectAll,
		CommandNoop:                     func() {},
	}
}

func defaultFileViewerBindings() []config.KeyBindingEntry {
	return []config.KeyBindingEntry{
		{Key: "Up", Command: CommandFileViewerLineUp},
		{Key: "Down", Command: CommandFileViewerLineDown},
		{Key: "Left", Command: CommandFileViewerColumnLeft},
		{Key: "Right", Command: CommandFileViewerColumnRight},
		{Key: "PageUp", Command: CommandFileViewerPageUp},
		{Key: "PageDown", Command: CommandFileViewerPageDown},
		{Key: "Home", Command: CommandFileViewerHome},
		{Key: "End", Command: CommandFileViewerEnd},
		{Key: "Escape", Command: CommandFileViewerClose},
		{Key: "Space", Command: CommandFileViewerPageDown},
		{Key: "/", Command: CommandFileViewerFocusSearch},
		{Key: "C-A", Command: CommandFileViewerSelectAll},
		{Key: "C-C", Command: CommandFileViewerCopySelection},
		{Key: "Q", Command: CommandFileViewerClose},
		{Key: "J", Command: CommandFileViewerLineDown},
		{Key: "K", Command: CommandFileViewerLineUp},
		{Key: "H", Command: CommandFileViewerColumnLeft},
		{Key: "L", Command: CommandFileViewerColumnRight},
		{Key: "F", Command: CommandFileViewerPageDown},
		{Key: "B", Command: CommandFileViewerPageUp},
		{Key: "G", Command: CommandFileViewerHome},
		{Key: "S-G", Command: CommandFileViewerEnd},
		{Key: "W", Command: CommandFileViewerToggleWrap},
		{Key: "T", Command: CommandFileViewerShowText},
		{Key: "M", Command: CommandFileViewerShowMarkdown},
		{Key: "X", Command: CommandFileViewerShowHex},
		{Key: "N", Command: CommandFileViewerSearchNext},
		{Key: "S-N", Command: CommandFileViewerSearchPrevious},
		{Key: "S-Semicolon", Command: CommandFileViewerFocusLine},
	}
}

func fileViewerRuneKey(r rune) (*fyne.KeyEvent, ModifierState, bool) {
	switch r {
	case '/':
		return &fyne.KeyEvent{Name: fyne.KeySlash}, ModifierState{}, true
	case ':':
		return &fyne.KeyEvent{Name: fyne.KeySemicolon}, ModifierState{ShiftPressed: true}, true
	}
	if r >= 'a' && r <= 'z' {
		return &fyne.KeyEvent{Name: fyne.KeyName(string(unicode.ToUpper(r)))}, ModifierState{}, true
	}
	if r >= 'A' && r <= 'Z' {
		return &fyne.KeyEvent{Name: fyne.KeyName(string(r))}, ModifierState{ShiftPressed: true}, true
	}
	return nil, ModifierState{}, false
}
