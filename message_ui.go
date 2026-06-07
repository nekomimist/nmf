package main

import "nmf/internal/ui"

func (fm *FileManager) ShowMessageDialog(title string, message string) {
	fm.showMessageDialog(func() {
		ui.ShowCompactMessageDialog(fm.window, title, message)
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
		fm.keyManager.DeferUntilKeysReleased("message.show", show)
		return
	}
	show()
}
