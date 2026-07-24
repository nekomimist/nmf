package keymanager

// DropActionDialogInterface defines keyboard actions for dropped files.
type DropActionDialogInterface interface {
	CopyDropped()
	MoveDropped()
	CancelDrop()
}

// DropActionDialogKeyHandler handles action selection for dropped files.
type DropActionDialogKeyHandler struct {
	*dialogKeyHandler
}

// NewDropActionDialogKeyHandler creates a drop action dialog key handler.
func NewDropActionDialogKeyHandler(d DropActionDialogInterface) *DropActionDialogKeyHandler {
	base := newDialogKeyHandler("DropActionDialog", nil, []dialogBinding{
		{"C", d.CopyDropped},
		{"M", d.MoveDropped},
		{"Escape", d.CancelDrop},
	}).withRune(func(r rune, modifiers ModifierState) bool {
		if modifiers.CtrlPressed || modifiers.AltPressed {
			return false
		}
		switch r {
		case 'c', 'C':
			d.CopyDropped()
			return true
		case 'm', 'M':
			d.MoveDropped()
			return true
		default:
			return false
		}
	})
	return &DropActionDialogKeyHandler{dialogKeyHandler: base}
}
