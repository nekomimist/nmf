package main

import (
	"nmf/internal/config"
	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
	"nmf/internal/ui"
)

// showBindingHelp is called on the UI thread after the owner's transition gate.
func (fm *FileManager) showBindingHelp(target string) {
	if fm.isWindowClosed() || fm.bindingHelp != nil {
		return
	}
	var commandNames map[string]string
	if fm.runtime != nil && fm.runtime.configScript != nil {
		commandNames = fm.runtime.configScript.CommandNames
	}
	modes := map[string][]keymanager.BindingDescription{
		keymanager.KeyBindingTargetMain:       fm.mainKeyHandler.BindingDescriptions(commandNames),
		keymanager.KeyBindingTargetLineEdit:   keymanager.LineEditBindingDescriptions(fm.config.UI.KeyBindings),
		keymanager.KeyBindingTargetFileViewer: keymanager.FileViewerBindingDescriptions(fm.config.UI.KeyBindings),
	}
	text := ui.BindingHelpMarkdown(target, modes)
	preview := &fileinfo.PreviewFile{
		Path: "Key Bindings", Name: "Key Bindings", Text: text, Data: []byte(text), Markdown: true,
		Encoding: "UTF-8", Size: int64(len(text)), SizeKnown: true,
	}
	help := ui.NewFileViewerDialog(preview, fm.keyManager)
	help.SetMaxSize(fm.config.UI.Viewer.MaxWidth, fm.config.UI.Viewer.MaxHeight)
	help.SetDefaultPane("markdown")
	help.SetDefaultWrap(true)
	help.SetDebugPrint(debugPrint)
	// Keep an unconditional keyboard exit even when normal viewer Escape is
	// remapped, and consume help requests without stacking another viewer.
	bindings := []config.KeyBindingEntry{
		{Target: keymanager.KeyBindingTargetFileViewer, Key: "Escape", Command: keymanager.CommandFileViewerClose},
		{Target: keymanager.KeyBindingTargetFileViewer, Key: "F1", Command: keymanager.CommandNoop},
	}
	help.SetKeyBindings(append(bindings, fm.config.UI.KeyBindings...))
	focus := fm.window.Canvas().Focused()
	owner := fm.keyManager.GetCurrentHandler()
	help.SetAfterClosed(func() {
		fm.bindingHelp = nil
		if fm.isWindowClosed() {
			return
		}
		if focus != nil && fm.keyManager.GetCurrentHandler() == owner {
			fm.window.Canvas().Focus(focus)
		} else if fm.keyManager.GetCurrentHandler() == fm.mainKeyHandler {
			fm.FocusFileList()
		}
	})
	fm.bindingHelp = help
	help.ShowDialog(fm.window)
}
