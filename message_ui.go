package main

import "nmf/internal/ui"

func (fm *FileManager) ShowMessageDialog(title string, message string) {
	fm.showMessageDialog(func() {
		ui.ShowCompactMessageDialog(fm.window, title, message)
	})
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
