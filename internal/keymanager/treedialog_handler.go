package keymanager

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
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
	treeDialog   TreeDialogInterface
	shiftPressed bool
	ctrlPressed  bool
	debugPrint   func(format string, args ...interface{})
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

// OnKeyDown handles key press events
func (th *TreeDialogKeyHandler) OnKeyDown(ev *fyne.KeyEvent) bool {
	switch ev.Name {
	case desktop.KeyShiftLeft, desktop.KeyShiftRight:
		th.shiftPressed = true
		th.debugPrint("TreeDialog: Shift key pressed (state: %t)", th.shiftPressed)
		return true

	case desktop.KeyControlLeft, desktop.KeyControlRight:
		th.ctrlPressed = true
		th.debugPrint("TreeDialog: Ctrl key pressed (state: %t)", th.ctrlPressed)
		return true

	case fyne.KeyR:
		// Ctrl+R - Toggle root mode
		if th.ctrlPressed {
			th.treeDialog.ToggleRootMode()
			return true
		}
	}

	return false
}

// OnKeyUp handles key release events
func (th *TreeDialogKeyHandler) OnKeyUp(ev *fyne.KeyEvent) bool {
	switch ev.Name {
	case desktop.KeyShiftLeft, desktop.KeyShiftRight:
		th.shiftPressed = false
		th.debugPrint("TreeDialog: Shift key released (state: %t)", th.shiftPressed)
		return true

	case desktop.KeyControlLeft, desktop.KeyControlRight:
		th.ctrlPressed = false
		th.debugPrint("TreeDialog: Ctrl key released (state: %t)", th.ctrlPressed)
		return true
	}

	return false
}

// OnTypedKey handles typed key events
func (th *TreeDialogKeyHandler) OnTypedKey(ev *fyne.KeyEvent) bool {
	switch ev.Name {
	case fyne.KeyUp:
		if th.shiftPressed {
			// Fast move up (multiple nodes)
			for i := 0; i < 5; i++ {
				th.treeDialog.MoveUp()
			}
		} else {
			th.treeDialog.MoveUp()
		}
		return true

	case fyne.KeyDown:
		if th.shiftPressed {
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
