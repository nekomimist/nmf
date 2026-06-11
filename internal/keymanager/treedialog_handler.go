package keymanager

import (
	"fyne.io/fyne/v2"
)

// TreeDialogInterface defines the interface needed by TreeDialogKeyHandler
type TreeDialogInterface interface {
	// Tree navigation
	MoveUp()
	MoveDown()
	ExpandNode()
	CollapseNode()
	SelectCurrentNode()

	// Dialog control
	AcceptSelection()
	CancelDialog()

	// Mode switching
	ToggleRootMode()
}

// TreeDialogKeyHandler handles keyboard events for the directory tree dialog
type TreeDialogKeyHandler struct {
	treeDialog TreeDialogInterface
	debugPrint func(format string, args ...interface{})
}

// NewTreeDialogKeyHandler creates a new tree dialog key handler
func NewTreeDialogKeyHandler(td TreeDialogInterface, debugPrint func(format string, args ...interface{})) *TreeDialogKeyHandler {
	return &TreeDialogKeyHandler{
		treeDialog: td,
		debugPrint: debugPrint,
	}
}

// GetName returns the name of this handler
func (th *TreeDialogKeyHandler) GetName() string {
	return "TreeDialog"
}

// OnKeyActivated handles key activations
func (th *TreeDialogKeyHandler) OnKeyActivated(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	switch ev.Name {
	case fyne.KeyR:
		// Ctrl+R - Toggle root mode
		if modifiers.CtrlPressed {
			th.treeDialog.ToggleRootMode()
			return true
		}

	case fyne.KeyUp:
		if modifiers.ShiftPressed {
			// Fast move up (multiple nodes)
			for i := 0; i < 5; i++ {
				th.treeDialog.MoveUp()
			}
		} else {
			th.treeDialog.MoveUp()
		}
		return true

	case fyne.KeyDown:
		if modifiers.ShiftPressed {
			// Fast move down (multiple nodes)
			for i := 0; i < 5; i++ {
				th.treeDialog.MoveDown()
			}
		} else {
			th.treeDialog.MoveDown()
		}
		return true

	case fyne.KeyRight:
		// Expand current node or move to child
		th.treeDialog.ExpandNode()
		return true

	case fyne.KeyLeft:
		// Collapse current node or move to parent
		th.treeDialog.CollapseNode()
		return true

	case fyne.KeySpace:
		// Select current node
		th.treeDialog.SelectCurrentNode()
		return true

	case fyne.KeyReturn:
		// Accept current selection and close dialog
		th.treeDialog.AcceptSelection()
		return true

	case fyne.KeyEscape:
		// Cancel dialog
		th.treeDialog.CancelDialog()
		return true

	case fyne.KeyTab:
		// Toggle between root modes
		th.treeDialog.ToggleRootMode()
		return true
	}

	return false
}

// OnTypedRune handles text input (not used in tree dialog)
func (th *TreeDialogKeyHandler) OnTypedRune(r rune, modifiers ModifierState) bool {
	return false
}
