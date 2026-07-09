package keymanager

// DialogActions holds the UI-launcher closures MainScreenKeyHandler invokes
// for commands that open a dialog, menu, or the file viewer. Bootstrap wires
// this once from FileManager method values (see SetActions), so keymanager
// no longer needs FileManagerInterface to declare every Show* method itself.
//
// A nil field means no action is registered for that command; the command
// logs a debug warning and no-ops instead of panicking, the same spirit as
// executeCommand's unknown-command handling.
type DialogActions struct {
	ShowDirectoryTreeDialog     func()
	ShowNavigationHistoryDialog func()
	ShowDirectoryJumpDialog     func()

	ShowFilterDialog            func()
	ShowIncrementalSearchDialog func()
	ShowSortDialog              func()
	ShowJobsDialog              func()
	ShowPathEditDialog          func()
	ShowCreateDirectoryDialog   func()
	ShowClipboardTextFileDialog func()
	ShowMessageDialog           func(title string, message string)

	ShowCopyDialog           func()
	ShowMoveDialog           func()
	ShowExtractArchiveDialog func()
	ShowCompareDialog        func()
	ShowRenameDialog         func()
	ShowDeleteDialog         func(permanent bool)
	ShowExplorerContextMenu  func()
	ShowExternalCommandMenu  func()
	ShowFileViewer           func()
	ShowMaintenanceDialog    func()
	ShowCommandMenu          func(title string, items []CommandMenuItem)
}
