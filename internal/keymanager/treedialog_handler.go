package keymanager

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
	*dialogKeyHandler
}

// NewTreeDialogKeyHandler creates a new tree dialog key handler
func NewTreeDialogKeyHandler(td TreeDialogInterface, debugPrint func(format string, args ...interface{})) *TreeDialogKeyHandler {
	fastUp := func() {
		for i := 0; i < 5; i++ {
			td.MoveUp()
		}
	}
	fastDown := func() {
		for i := 0; i < 5; i++ {
			td.MoveDown()
		}
	}
	base := newDialogKeyHandler("TreeDialog", debugPrint, []dialogBinding{
		{"C-R", td.ToggleRootMode},

		// Shift+Up/Down: fast move (multiple nodes).
		{"Up", td.MoveUp},
		{"S-Up", fastUp},
		{"Down", td.MoveDown},
		{"S-Down", fastDown},

		// Expand current node or move to child.
		{"Right", td.ExpandNode},
		// Collapse current node or move to parent.
		{"Left", td.CollapseNode},

		{"Space", td.SelectCurrentNode},
		{"Return", td.AcceptSelection},
		{"Escape", td.CancelDialog},
		{"Tab", td.ToggleRootMode},
	})
	return &TreeDialogKeyHandler{dialogKeyHandler: base}
}
