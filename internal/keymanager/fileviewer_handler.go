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
}

// FileViewerKeyHandler handles less-like keys for the built-in viewer.
type FileViewerKeyHandler struct {
	viewer     FileViewerInterface
	bindings   []keyBinding
	commands   map[string]func()
	debugPrint func(format string, args ...interface{})
}

func NewFileViewerKeyHandler(viewer FileViewerInterface, configuredBindings ...[]config.KeyBindingEntry) *FileViewerKeyHandler {
	var configured []config.KeyBindingEntry
	if len(configuredBindings) > 0 {
		configured = configuredBindings[0]
	}
	h := &FileViewerKeyHandler{
		viewer:     viewer,
		debugPrint: func(string, ...interface{}) {},
	}
	h.commands = h.defaultCommands()
	h.bindings = buildTargetKeyBindings(
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
	return h
}

func (h *FileViewerKeyHandler) GetName() string { return "FileViewer" }

func (h *FileViewerKeyHandler) OnKeyActivated(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	if ev == nil {
		return false
	}
	return h.executeBinding(ev, modifiers)
}

func (h *FileViewerKeyHandler) OnTypedRune(r rune, modifiers ModifierState) bool {
	ev, mods, ok := fileViewerRuneKey(r)
	if !ok {
		return false
	}
	return h.executeBinding(ev, mods)
}

func (h *FileViewerKeyHandler) executeBinding(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	for _, binding := range h.bindings {
		if !binding.matches(ev, modifiers) {
			continue
		}
		h.debugPrint("FileViewer: command=%s key=%s", binding.command, ev.Name)
		h.commands[binding.command]()
		return true
	}
	return false
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
