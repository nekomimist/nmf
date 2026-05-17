package main

import "nmf/internal/ui"

func (fm *FileManager) ShowMessageDialog(title string, message string) {
	ui.ShowCompactMessageDialog(fm.window, title, message)
}
