package keymanager

// JobsDialogInterface defines navigation and actions for the Jobs dialog
type JobsDialogInterface interface {
	MoveUp()
	MoveDown()
	MoveToTop()
	MoveToBottom()
	CancelSelected()
	CloseDialog()
}

// JobsDialogKeyHandler handles keys while Jobs dialog is open
type JobsDialogKeyHandler struct {
	*dialogKeyHandler
}

func NewJobsDialogKeyHandler(d JobsDialogInterface, debugPrint func(format string, args ...interface{})) *JobsDialogKeyHandler {
	base := newDialogKeyHandler("JobsDialog", debugPrint, []dialogBinding{
		{"Up", d.MoveUp},
		{"S-Up", d.MoveToTop},
		{"Down", d.MoveDown},
		{"S-Down", d.MoveToBottom},

		// Plain Delete only: Shift+Delete arrives as a folded Cut shortcut and
		// has no binding here, so it falls through unmatched.
		{"Delete", d.CancelSelected},

		{"Return", d.CloseDialog},
		{"Escape", d.CloseDialog},
	})
	return &JobsDialogKeyHandler{dialogKeyHandler: base}
}
