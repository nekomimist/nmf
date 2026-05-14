package main

import (
	"fyne.io/fyne/v2"

	"nmf/internal/fileinfo"
	"nmf/internal/ui"
)

// SetClipboardText writes text to the application clipboard.
func (fm *FileManager) SetClipboardText(text string) bool {
	app := fyne.CurrentApp()
	if app == nil {
		debugPrint("FileManager: clipboard unavailable app=nil")
		return false
	}
	clip := app.Clipboard()
	if clip == nil {
		debugPrint("FileManager: clipboard unavailable")
		return false
	}
	clip.SetContent(text)
	debugPrint("FileManager: clipboard set bytes=%d", len(text))
	return true
}

// ShowClipboardTextFileDialog asks for a file name and creates it from clipboard text.
func (fm *FileManager) ShowClipboardTextFileDialog() {
	dlg := ui.NewLineEditDialog(ui.LineEditDialogOptions{
		Title:       "Create Text File",
		Prompt:      "File name:",
		ConfirmText: "Create",
	}, fm.keyManager)
	dlg.ShowDialog(fm.window, func(name string) bool {
		return fm.CreateClipboardTextFile(name)
	})
}

// CreateClipboardTextFile creates a local text file from the current clipboard text.
func (fm *FileManager) CreateClipboardTextFile(name string) bool {
	text, ok := fm.clipboardText()
	if !ok {
		ui.ShowMessageDialog(fm.window, "Clipboard unavailable", "The application clipboard is not available.")
		return false
	}
	if text == "" {
		ui.ShowMessageDialog(fm.window, "Clipboard is empty", "There is no text to save.")
		return false
	}

	newPath, err := fileinfo.CreateTextFilePortable(fm.currentPath, name, text)
	if err != nil {
		debugPrint("FileManager: Create text file failed parent=%s name=%s err=%v", fm.currentPath, name, err)
		ui.ShowMessageDialog(fm.window, "Create text file failed", err.Error())
		return false
	}

	fm.applyCreatedPathToList(newPath, false)
	debugPrint("FileManager: Created text file from clipboard %s", newPath)
	fm.FocusFileList()
	return true
}

func (fm *FileManager) clipboardText() (string, bool) {
	app := fyne.CurrentApp()
	if app == nil {
		debugPrint("FileManager: clipboard unavailable app=nil")
		return "", false
	}
	clip := app.Clipboard()
	if clip == nil {
		debugPrint("FileManager: clipboard unavailable")
		return "", false
	}
	return clip.Content(), true
}
