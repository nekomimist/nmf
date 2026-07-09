package keymanager

type MaintenanceDialogInterface interface {
	Scan()
	Apply()
	Cancel()
}

type MaintenanceDialogKeyHandler struct {
	*dialogKeyHandler
}

func NewMaintenanceDialogKeyHandler(dialog MaintenanceDialogInterface) *MaintenanceDialogKeyHandler {
	base := newDialogKeyHandler("MaintenanceDialog", nil, []dialogBinding{
		{"Escape", dialog.Cancel},
		{"Return", dialog.Apply},
		{"F5", dialog.Scan},
	})
	return &MaintenanceDialogKeyHandler{dialogKeyHandler: base}
}
