package keymanager

// ConflictDialogInterface defines keyboard operations for the name conflict dialog.
type ConflictDialogInterface interface {
	Continue()
	CancelJob()
	SelectOverwriteIfNewer()
	SelectOverwrite()
	SelectAutoName()
	SelectRename()
	SelectSkip()
}

// ConflictDialogKeyHandler handles commit/cancel keys while resolving a copy/move conflict.
type ConflictDialogKeyHandler struct {
	*dialogKeyHandler
}

// NewConflictDialogKeyHandler creates a conflict dialog key handler.
func NewConflictDialogKeyHandler(d ConflictDialogInterface) *ConflictDialogKeyHandler {
	base := newDialogKeyHandler("ConflictDialog", nil, []dialogBinding{
		{"A-N", d.SelectOverwriteIfNewer},
		{"A-O", d.SelectOverwrite},
		{"A-A", d.SelectAutoName},
		{"A-R", d.SelectRename},
		{"A-S", d.SelectSkip},

		{"Return", d.Continue},
		{"Escape", d.CancelJob},
	})
	return &ConflictDialogKeyHandler{dialogKeyHandler: base}
}
