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
	CommandFileViewerShowImage      = "fileViewer.pane.image"
	CommandFileViewerShowText       = "fileViewer.pane.text"
	CommandFileViewerShowMarkdown   = "fileViewer.pane.markdown"
	CommandFileViewerShowHex        = "fileViewer.pane.hex"
	CommandFileViewerSearchNext     = "fileViewer.search.next"
	CommandFileViewerSearchPrevious = "fileViewer.search.previous"
	CommandFileViewerFocusSearch    = "fileViewer.search.focus"
	CommandFileViewerFocusLine      = "fileViewer.line.focus"
	CommandFileViewerCopySelection  = "fileViewer.selection.copy"
	CommandFileViewerSelectAll      = "fileViewer.selection.selectAll"
	CommandFileViewerClearSelection = "fileViewer.selection.clear"
	CommandFileViewerImageToggleFit = "fileViewer.image.zoom.toggle"
	CommandFileViewerImageZoomIn    = "fileViewer.image.zoom.in"
	CommandFileViewerImageZoomOut   = "fileViewer.image.zoom.out"
	CommandFileViewerFilePrevious   = "fileViewer.file.previous"
	CommandFileViewerFileNext       = "fileViewer.file.next"
	CommandFileViewerMarkToggle     = "fileViewer.file.markToggle"
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
	ViewerShowImage()
	ViewerShowText()
	ViewerShowMarkdown()
	ViewerShowHex()
	ViewerSearchNext()
	ViewerSearchPrevious()
	ViewerFocusSearch()
	ViewerFocusLine()
	ViewerCopySelection()
	ViewerSelectAll()
	ViewerClearSelection()
	ViewerImageToggleFit()
	ViewerImageZoomIn()
	ViewerImageZoomOut()
	ViewerFilePrevious()
	ViewerFileNext()
	ViewerMarkToggle()
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
	onHelp       func()
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
	bindings := buildFileViewerBindings(configured, h.debugPrint)
	for _, binding := range bindings {
		if fileViewerRunePathSpec(binding.spec) {
			h.runeBindings = append(h.runeBindings, binding)
		} else {
			h.keyBindings = append(h.keyBindings, binding)
		}
	}
	return h
}

// SetHelpAction supplies the host's input-owner transition and help presenter.
func (h *FileViewerKeyHandler) SetHelpAction(action func()) { h.onHelp = action }

func buildFileViewerBindings(configured []config.KeyBindingEntry, debugPrint func(string, ...interface{})) []keyBinding {
	return buildTargetKeyBindings("FileViewer", KeyBindingTargetFileViewer,
		configured, defaultFileViewerBindings(), func(command string) bool {
			_, ok := fileViewerCommands[command]
			return ok || command == CommandDescribeBindings
		}, debugPrint)
}

func (h *FileViewerKeyHandler) GetName() string { return "FileViewer" }

func (h *FileViewerKeyHandler) OnKeyActivated(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	if ev == nil {
		return false
	}
	return h.executeBinding(h.keyBindings, ev, modifiers)
}

// OnHelpKeyActivated allows viewer text inputs to request help without taking
// over their native editing keys. Printable rune bindings stay with text entry.
func (h *FileViewerKeyHandler) OnHelpKeyActivated(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	for _, binding := range h.keyBindings {
		if binding.matches(ev, modifiers) {
			if binding.command != CommandDescribeBindings {
				return false
			}
			h.commands[binding.command]()
			return true
		}
	}
	return false
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
// none or Shift-only, KeySlash with no modifiers, KeySemicolon with
// Shift-only, or the image viewer's =/+/- controls. Everything else (arrows,
// Escape, Space, Ctrl/Alt combos, F-keys, ...) stays on the key path.
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
	case spec.key == fyne.KeyEqual:
		return true
	case spec.key == fyne.KeyMinus:
		return !spec.mod.ShiftPressed
	default:
		return false
	}
}

func (h *FileViewerKeyHandler) defaultCommands() map[string]func() {
	commands := map[string]func(){
		CommandDescribeBindings: func() {
			if h.onHelp != nil {
				h.onHelp()
			}
		},
	}
	for id, command := range fileViewerCommands {
		commands[id] = func() { command(h.viewer) }
	}
	return commands
}

var fileViewerCommands = map[string]func(FileViewerInterface){
	CommandFileViewerClose:          FileViewerInterface.CloseViewer,
	CommandFileViewerLineDown:       FileViewerInterface.ViewerLineDown,
	CommandFileViewerLineUp:         FileViewerInterface.ViewerLineUp,
	CommandFileViewerPageDown:       FileViewerInterface.ViewerPageDown,
	CommandFileViewerPageUp:         FileViewerInterface.ViewerPageUp,
	CommandFileViewerHome:           FileViewerInterface.ViewerHome,
	CommandFileViewerEnd:            FileViewerInterface.ViewerEnd,
	CommandFileViewerColumnLeft:     FileViewerInterface.ViewerColumnLeft,
	CommandFileViewerColumnRight:    FileViewerInterface.ViewerColumnRight,
	CommandFileViewerToggleWrap:     FileViewerInterface.ViewerToggleWrap,
	CommandFileViewerShowImage:      FileViewerInterface.ViewerShowImage,
	CommandFileViewerShowText:       FileViewerInterface.ViewerShowText,
	CommandFileViewerShowMarkdown:   FileViewerInterface.ViewerShowMarkdown,
	CommandFileViewerShowHex:        FileViewerInterface.ViewerShowHex,
	CommandFileViewerSearchNext:     FileViewerInterface.ViewerSearchNext,
	CommandFileViewerSearchPrevious: FileViewerInterface.ViewerSearchPrevious,
	CommandFileViewerFocusSearch:    FileViewerInterface.ViewerFocusSearch,
	CommandFileViewerFocusLine:      FileViewerInterface.ViewerFocusLine,
	CommandFileViewerCopySelection:  FileViewerInterface.ViewerCopySelection,
	CommandFileViewerSelectAll:      FileViewerInterface.ViewerSelectAll,
	CommandFileViewerClearSelection: FileViewerInterface.ViewerClearSelection,
	CommandFileViewerImageToggleFit: FileViewerInterface.ViewerImageToggleFit,
	CommandFileViewerImageZoomIn:    FileViewerInterface.ViewerImageZoomIn,
	CommandFileViewerImageZoomOut:   FileViewerInterface.ViewerImageZoomOut,
	CommandFileViewerFilePrevious:   FileViewerInterface.ViewerFilePrevious,
	CommandFileViewerFileNext:       FileViewerInterface.ViewerFileNext,
	CommandFileViewerMarkToggle:     FileViewerInterface.ViewerMarkToggle,
	CommandNoop:                     func(FileViewerInterface) {},
}

func defaultFileViewerBindings() []config.KeyBindingEntry {
	return []config.KeyBindingEntry{
		{Key: "F1", Command: CommandDescribeBindings},
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
		{Key: "C-S-A", Command: CommandFileViewerClearSelection},
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
		{Key: "I", Command: CommandFileViewerShowImage},
		{Key: "T", Command: CommandFileViewerShowText},
		{Key: "M", Command: CommandFileViewerShowMarkdown},
		{Key: "X", Command: CommandFileViewerShowHex},
		{Key: "N", Command: CommandFileViewerSearchNext},
		{Key: "S-N", Command: CommandFileViewerSearchPrevious},
		{Key: "S-Semicolon", Command: CommandFileViewerFocusLine},
		{Key: "=", Command: CommandFileViewerImageToggleFit},
		{Key: "S-=", Command: CommandFileViewerImageZoomIn},
		{Key: "-", Command: CommandFileViewerImageZoomOut},
		{Key: "S-Up", Command: CommandFileViewerFilePrevious},
		{Key: "S-Down", Command: CommandFileViewerFileNext},
		{Key: "S-Space", Command: CommandFileViewerMarkToggle},
	}
}

func fileViewerRuneKey(r rune) (*fyne.KeyEvent, ModifierState, bool) {
	switch r {
	case '/':
		return &fyne.KeyEvent{Name: fyne.KeySlash}, ModifierState{}, true
	case ':':
		return &fyne.KeyEvent{Name: fyne.KeySemicolon}, ModifierState{ShiftPressed: true}, true
	case '=':
		return &fyne.KeyEvent{Name: fyne.KeyEqual}, ModifierState{}, true
	case '+':
		return &fyne.KeyEvent{Name: fyne.KeyEqual}, ModifierState{ShiftPressed: true}, true
	case '-':
		return &fyne.KeyEvent{Name: fyne.KeyMinus}, ModifierState{}, true
	}
	if r >= 'a' && r <= 'z' {
		return &fyne.KeyEvent{Name: fyne.KeyName(string(unicode.ToUpper(r)))}, ModifierState{}, true
	}
	if r >= 'A' && r <= 'Z' {
		return &fyne.KeyEvent{Name: fyne.KeyName(string(r))}, ModifierState{ShiftPressed: true}, true
	}
	return nil, ModifierState{}, false
}
