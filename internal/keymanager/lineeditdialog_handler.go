package keymanager

import (
	"unicode"

	"fyne.io/fyne/v2"

	"nmf/internal/config"
)

const (
	CommandLineEditAccept            = "lineEdit.accept"
	CommandLineEditCancel            = "lineEdit.cancel"
	CommandLineEditCursorStart       = "lineEdit.cursor.start"
	CommandLineEditCursorEnd         = "lineEdit.cursor.end"
	CommandLineEditCursorLeft        = "lineEdit.cursor.left"
	CommandLineEditCursorRight       = "lineEdit.cursor.right"
	CommandLineEditDeleteBefore      = "lineEdit.delete.before"
	CommandLineEditDeleteAt          = "lineEdit.delete.at"
	CommandLineEditDeleteBeforeStart = "lineEdit.delete.beforeStart"
	CommandLineEditDeleteAfterEnd    = "lineEdit.delete.afterEnd"
	CommandLineEditPaste             = "lineEdit.paste"
)

// LineEditDialogInterface defines the operations used by the line edit dialog handler.
type LineEditDialogInterface interface {
	AcceptEdit()
	CancelDialog()
	MoveCursorStart()
	MoveCursorEnd()
	MoveCursorLeft()
	MoveCursorRight()
	DeleteBeforeCursor()
	DeleteAtCursor()
	DeleteBeforeCursorToStart()
	DeleteAfterCursorToEnd()
	PasteFromClipboard()
	InsertRune(r rune) bool
}

// LineEditDialogKeyHandler handles commit/cancel and readline-style edit keys.
type LineEditDialogKeyHandler struct {
	dialog     LineEditDialogInterface
	bindings   []keyBinding
	commands   map[string]func()
	history    *dialogKeyHandler
	debugPrint func(format string, args ...interface{})
	onHelp     func()
}

// NewLineEditDialogKeyHandler creates a line edit dialog key handler.
func NewLineEditDialogKeyHandler(d LineEditDialogInterface, debugPrint func(format string, args ...interface{}), configuredBindings ...[]config.KeyBindingEntry) *LineEditDialogKeyHandler {
	var configured []config.KeyBindingEntry
	if len(configuredBindings) > 0 {
		configured = configuredBindings[0]
	}
	h := &LineEditDialogKeyHandler{dialog: d}
	if debugPrint != nil {
		h.debugPrint = debugPrint
	} else {
		h.debugPrint = func(string, ...interface{}) {}
	}
	h.commands = h.defaultCommands()
	h.bindings = buildLineEditBindings(configured, h.debugPrint)
	return h
}

// SetHelpAction supplies the host's input-owner transition and help presenter.
func (h *LineEditDialogKeyHandler) SetHelpAction(action func()) { h.onHelp = action }

// SetHistoryActions adds history keys after the common editing bindings.
func (h *LineEditDialogKeyHandler) SetHistoryActions(older, newer func()) {
	h.history = newDialogKeyHandler(h.GetName(), h.debugPrint, []dialogBinding{
		{"Up", older},
		{"C-P", older},
		{"Down", newer},
		{"C-N", newer},
	})
}

func buildLineEditBindings(configured []config.KeyBindingEntry, debugPrint func(string, ...interface{})) []keyBinding {
	return buildTargetKeyBindings(
		"LineEditDialog",
		KeyBindingTargetLineEdit,
		configured,
		defaultLineEditBindings(),
		func(command string) bool {
			_, ok := lineEditCommands[command]
			return ok || command == CommandDescribeBindings
		},
		debugPrint,
	)
}

// GetName returns the handler name.
func (h *LineEditDialogKeyHandler) GetName() string { return "LineEditDialog" }

// OnKeyActivated handles key activations.
func (h *LineEditDialogKeyHandler) OnKeyActivated(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	for _, binding := range h.bindings {
		if !binding.matches(ev, modifiers) {
			continue
		}
		h.debugPrint("LineEditDialog: command=%s key=%s", binding.command, ev.Name)
		h.commands[binding.command]()
		return true
	}
	return h.history != nil && h.history.OnKeyActivated(ev, modifiers)
}

// OnTypedRune handles text input when focus has drifted away from the entry.
func (h *LineEditDialogKeyHandler) OnTypedRune(r rune, _ ModifierState) bool {
	if unicode.IsPrint(r) && !unicode.IsControl(r) {
		return h.dialog.InsertRune(r)
	}
	return false
}

func (h *LineEditDialogKeyHandler) defaultCommands() map[string]func() {
	commands := map[string]func(){
		CommandDescribeBindings: func() {
			if h.onHelp != nil {
				h.onHelp()
			}
		},
	}
	for id, command := range lineEditCommands {
		commands[id] = func() { command(h.dialog) }
	}
	return commands
}

var lineEditCommands = map[string]func(LineEditDialogInterface){
	CommandLineEditAccept:            LineEditDialogInterface.AcceptEdit,
	CommandLineEditCancel:            LineEditDialogInterface.CancelDialog,
	CommandLineEditCursorStart:       LineEditDialogInterface.MoveCursorStart,
	CommandLineEditCursorEnd:         LineEditDialogInterface.MoveCursorEnd,
	CommandLineEditCursorLeft:        LineEditDialogInterface.MoveCursorLeft,
	CommandLineEditCursorRight:       LineEditDialogInterface.MoveCursorRight,
	CommandLineEditDeleteBefore:      LineEditDialogInterface.DeleteBeforeCursor,
	CommandLineEditDeleteAt:          LineEditDialogInterface.DeleteAtCursor,
	CommandLineEditDeleteBeforeStart: LineEditDialogInterface.DeleteBeforeCursorToStart,
	CommandLineEditDeleteAfterEnd:    LineEditDialogInterface.DeleteAfterCursorToEnd,
	CommandLineEditPaste:             LineEditDialogInterface.PasteFromClipboard,
	CommandNoop:                      func(LineEditDialogInterface) {},
}

func defaultLineEditBindings() []config.KeyBindingEntry {
	return []config.KeyBindingEntry{
		{Key: "F1", Command: CommandDescribeBindings},
		{Key: "Return", Command: CommandLineEditAccept},
		{Key: "KP_Enter", Command: CommandLineEditAccept},
		{Key: "Escape", Command: CommandLineEditCancel},
		{Key: "Backspace", Command: CommandLineEditDeleteBefore},
		{Key: "Delete", Command: CommandLineEditDeleteAt},
		{Key: "Left", Command: CommandLineEditCursorLeft},
		{Key: "Right", Command: CommandLineEditCursorRight},
		{Key: "Home", Command: CommandLineEditCursorStart},
		{Key: "End", Command: CommandLineEditCursorEnd},
		{Key: "C-A", Command: CommandLineEditCursorStart},
		{Key: "C-E", Command: CommandLineEditCursorEnd},
		{Key: "C-B", Command: CommandLineEditCursorLeft},
		{Key: "C-F", Command: CommandLineEditCursorRight},
		{Key: "C-H", Command: CommandLineEditDeleteBefore},
		{Key: "C-D", Command: CommandLineEditDeleteAt},
		{Key: "C-U", Command: CommandLineEditDeleteBeforeStart},
		{Key: "C-K", Command: CommandLineEditDeleteAfterEnd},
		{Key: "C-Y", Command: CommandLineEditPaste},
	}
}
