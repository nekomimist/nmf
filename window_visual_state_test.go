package main

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	customtheme "nmf/internal/theme"
	"nmf/internal/ui"
)

type visualStateTheme struct{}

func (visualStateTheme) GetCustomColor(colorType string) color.RGBA {
	if colorType == customtheme.ColorCursor {
		return color.RGBA{R: 10, G: 20, B: 30, A: 200}
	}
	return color.RGBA{}
}

func TestInactiveCursorThemeDimsCursorAlphaOnly(t *testing.T) {
	theme := inactiveCursorTheme{base: visualStateTheme{}}

	cursor := theme.GetCustomColor(customtheme.ColorCursor)
	if cursor != (color.RGBA{R: 10, G: 20, B: 30, A: 76}) {
		t.Fatalf("inactive cursor color = %#v, want alpha-dimmed cursor", cursor)
	}

	other := theme.GetCustomColor(customtheme.ColorFileRegular)
	if other.A != 0 {
		t.Fatalf("non-cursor color = %#v, want unchanged zero color", other)
	}
}

func TestFocusFileListRestoresWindowActive(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	window := app.NewWindow("active")
	fileListView := ui.NewKeySink(widget.NewLabel("files"), nil)
	window.SetContent(fileListView)
	fm := &FileManager{
		window:       window,
		fileListView: fileListView,
		windowActive: false,
		currentPath:  "/tmp",
	}

	fm.focusFileList("test")

	if !fm.windowActive {
		t.Fatal("focusFileList should restore active state")
	}
	if window.Canvas().Focused() != fileListView {
		t.Fatalf("focused object = %T, want fileListView", window.Canvas().Focused())
	}
}

func TestHighlightFileManagerWindowForPathHighlightsOpenWindow(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	resetFileManagerWindowTestRegistry(t)

	current := &FileManager{
		window:          app.NewWindow("current"),
		currentPath:     "/current",
		windowHighlight: canvas.NewRectangle(color.Transparent),
	}
	target := &FileManager{
		window:          app.NewWindow("target"),
		currentPath:     "/target",
		windowHighlight: canvas.NewRectangle(color.Transparent),
	}
	current.windowHighlight.StrokeColor = color.Transparent
	target.windowHighlight.StrokeColor = color.Transparent
	registerFileManagerWindow(current)
	registerFileManagerWindow(target)

	highlightFileManagerWindowForPath("/target")

	if color.RGBAModel.Convert(current.windowHighlight.StrokeColor).(color.RGBA).A != 0 {
		t.Fatal("current window highlight should stay transparent")
	}
	if color.RGBAModel.Convert(target.windowHighlight.StrokeColor).(color.RGBA).A == 0 {
		t.Fatal("target window highlight should be visible")
	}

	clearFileManagerWindowHighlights()
	if color.RGBAModel.Convert(target.windowHighlight.StrokeColor).(color.RGBA).A != 0 {
		t.Fatal("target window highlight should be cleared")
	}
}
