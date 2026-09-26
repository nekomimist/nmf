package main

import "nmf/internal/ui"

func (fm *FileManager) ShowMessageDialog(title string, message string) {
	fm.showMessageDialog(func() {
		ui.ShowCompactMessageDialog(fm.window, title, message)
	})
}

func (fm *FileManager) ShowCommandError(action, details string) {
	fm.showMessageDialog(func() {
		if fm.isWindowClosed() || fm.window == nil {
			return
		}
		if fm.commandErrorDialog != nil {
			fm.commandErrorDialog.Append(action, details)
			return
		}
		d := ui.NewCommandErrorDialog(action, details, fm.keyManager)
		fm.commandErrorDialog = d
		d.Show(fm.window, func() {
			fm.commandErrorDialog = nil
			if !fm.isWindowClosed() {
				fm.focusFileList("command-error-closed")
			}
		})
	})
}

func (fm *FileManager) ShowVersionDialog() {
	fm.showMessageDialog(func() {
		ui.ShowCompactVersionDialog(fm.window, appFullName, appRepository, appVersion())
	})
}

func versionDialogMessage() string {
	return "Software: " + appFullName +
		"\nRepository: " + appRepository +
		"\nVersion: " + appVersion()
}

func (fm *FileManager) showMessageDialog(show func()) {
	if show == nil {
		return
	}
	if fm != nil && fm.keyManager != nil {
		fm.keyManager.BeginOwnerTransition("message.show", show)
		return
	}
	show()
}
