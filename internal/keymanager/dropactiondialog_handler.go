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
	// A printable key press delivers both a TypedKey and a TypedRune (see
	// docs/architecture/ui-input.md, driver fact 6), so each spec belongs to
	// exactly one of the two paths. C and M are letters and live on the rune
	// path, matching SortDialog's o/d and the viewer's letter bindings; only
	// Escape, which produces no rune, stays on the typed-key path.
	base := newDialogKeyHandler("DropActionDialog", nil, []dialogBinding{
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
