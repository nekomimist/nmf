package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"

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
	debugPrint("FileManager: window active change active=%t focused=%s path=%s", active, focusedObjectLabel(fm.window), fm.currentPath)
	fm.windowActive = active
	if fm.fileList != nil {
		fm.RefreshCursor()
	}
}

func (fm *FileManager) setWindowHighlight(active bool) {
	if fm == nil || fm.windowHighlight == nil {
		return
	}
	if active {
		fm.windowHighlight.StrokeColor = fm.windowHighlightColor()
	} else {
		fm.windowHighlight.StrokeColor = color.Transparent
	}
	canvas.Refresh(fm.windowHighlight)
	if fm.window != nil {
		fm.window.Canvas().Refresh(fm.windowHighlight)
	}
}

func (fm *FileManager) windowHighlightColor() color.Color {
	if fm != nil && fm.customTheme != nil {
		return fm.customTheme.GetCustomColor(customtheme.ColorCopyMoveOpenDestination)
	}
	return currentAppColor(customtheme.ColorCopyMoveOpenDestination, color.RGBA{R: 30, G: 120, B: 80, A: 255})
}

func currentAppColor(name string, fallback color.RGBA) color.Color {
	if app := fyne.CurrentApp(); app != nil {
		if provider, ok := app.Settings().Theme().(interface {
			GetCustomColor(string) color.RGBA
		}); ok {
			return provider.GetCustomColor(name)
		}
	}
	return fallback
}

func clearFileManagerWindowHighlights() {
	for _, manager := range snapshotFileManagerWindows() {
		manager.setWindowHighlight(false)
	}
}

func highlightFileManagerWindowForPath(path string) {
	clearFileManagerWindowHighlights()
	if path == "" {
		return
	}
	for _, manager := range snapshotFileManagerWindows() {
		if manager.currentPath != path || fileManagerWindowIconified(manager) {
			continue
		}
		manager.setWindowHighlight(true)
		return
	}
}

func fileManagerWindowIconified(fm *FileManager) bool {
	return fm != nil && fm.window != nil && windowIconified(fm.window)
}
