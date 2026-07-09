package keymanager

// DeleteConfirmDialogInterface defines keyboard actions for delete confirmation.
type DeleteConfirmDialogInterface interface {
	ConfirmDelete()
	CancelDelete()
}

// DeleteConfirmDialogKeyHandler handles keyboard events for delete confirmation.
type DeleteConfirmDialogKeyHandler struct {
	*dialogKeyHandler
}

func NewDeleteConfirmDialogKeyHandler(d DeleteConfirmDialogInterface) *DeleteConfirmDialogKeyHandler {
	base := newDialogKeyHandler("DeleteConfirmDialog", nil, []dialogBinding{
		{"Return", d.ConfirmDelete},
		{"Escape", d.CancelDelete},
	})
	return &DeleteConfirmDialogKeyHandler{dialogKeyHandler: base}
}
