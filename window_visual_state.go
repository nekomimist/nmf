package main

import (
	"image/color"

	customtheme "nmf/internal/theme"
	"nmf/internal/ui"
)

const inactiveCursorAlphaScale = 0.38

type inactiveCursorTheme struct {
	base ui.ThemeColorProvider
}

func (t inactiveCursorTheme) GetCustomColor(colorType string) color.RGBA {
	c := t.base.GetCustomColor(colorType)
	if colorType == customtheme.ColorCursor {
		c.A = uint8(float32(c.A) * inactiveCursorAlphaScale)
	}
	return c
}

func (fm *FileManager) cursorThemeProvider() ui.ThemeColorProvider {
	if fm == nil {
		return nil
	}
	if fm.windowActive || fm.customTheme == nil {
		return fm.customTheme
	}
	return inactiveCursorTheme{base: fm.customTheme}
}

func (fm *FileManager) setWindowActive(active bool) {
	if fm == nil || fm.windowActive == active {
		return
	}
	// Fyne can deliver a focus change while the window is being torn down, and
	// this runs from the KeySink's focus callback (ui_setup.go). Every other
	// window-owned callback takes the same guard.
	if fm.isWindowClosed() {
		return
	}
	debugPrint("FileManager: window active change active=%t focused=%s path=%s", active, focusedObjectLabel(fm.window), fm.GetCurrentPath())
	fm.windowActive = active
	if fm.runtime != nil && fm.runtime.promptBroker != nil && fm.promptTargetID != 0 {
		fm.runtime.promptBroker.SetActive(fm.promptTargetID, active)
	}
	if active {
		bringFileManagerWindowsToFront(fm.runtime)
	}
	if fm.fileList != nil {
		fm.RefreshCursor()
	}
}

func (fm *FileManager) setWindowHighlight(active bool) {
	if fm == nil || fm.windowHighlight == nil {
		return
	}
	fm.windowHighlight.SetHighlighted(active)
	if fm.window != nil {
		fm.window.Canvas().Refresh(fm.windowHighlight)
	}
}

func clearFileManagerWindowHighlights(fm *FileManager) {
	for _, manager := range fm.registeredWindows() {
		manager.setWindowHighlight(false)
	}
}

func updateOpenPathHighlights(fm *FileManager, path string, openPaths map[string]bool, setOwnerHighlighted func(bool)) {
	open := openPaths[path]
	ownerHighlighted := open && fm != nil && sameDirectoryPath(path, fm.GetCurrentPath())
	if setOwnerHighlighted != nil {
		setOwnerHighlighted(ownerHighlighted)
	}
	if open {
		highlightFileManagerWindowForPath(fm, path)
		return
	}
	clearFileManagerWindowHighlights(fm)
}

func highlightFileManagerWindowForPath(fm *FileManager, path string) {
	clearFileManagerWindowHighlights(fm)
	if path == "" {
		return
	}
	for _, manager := range fm.registeredWindows() {
		if manager.GetCurrentPath() != path || fileManagerWindowIconified(manager) {
			continue
		}
		manager.setWindowHighlight(true)
	}
}

func fileManagerWindowIconified(fm *FileManager) bool {
	return fm != nil && fm.window != nil && windowIconified(fm.window)
}
