package main

import (
	"image/color"
	"sync/atomic"
	"testing"

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
		windowHighlight: ui.NewHighlightFrame(nil),
	}
	target := &FileManager{
		window:          app.NewWindow("target"),
		currentPath:     "/target",
		windowHighlight: ui.NewHighlightFrame(nil),
	}
	registerFileManagerWindow(current)
	registerFileManagerWindow(target)

	highlightFileManagerWindowForPath("/target")

	if current.windowHighlight.IsHighlighted() {
		t.Fatal("current window highlight should stay inactive")
	}
	if !target.windowHighlight.IsHighlighted() {
		t.Fatal("target window highlight should be active")
	}

	clearFileManagerWindowHighlights()
	if target.windowHighlight.IsHighlighted() {
		t.Fatal("target window highlight should be cleared")
	}
}

func TestHighlightFileManagerWindowForPathHighlightsEveryMatchingWindow(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	resetFileManagerWindowTestRegistry(t)

	first := &FileManager{
		window:          app.NewWindow("first"),
		currentPath:     "/shared",
		windowHighlight: ui.NewHighlightFrame(nil),
	}
	second := &FileManager{
		window:          app.NewWindow("second"),
		currentPath:     "/shared",
		windowHighlight: ui.NewHighlightFrame(nil),
	}
	other := &FileManager{
		window:          app.NewWindow("other"),
		currentPath:     "/other",
		windowHighlight: ui.NewHighlightFrame(nil),
	}
	registerFileManagerWindow(first)
	registerFileManagerWindow(second)
	registerFileManagerWindow(other)

	highlightFileManagerWindowForPath("/shared")

	if !first.windowHighlight.IsHighlighted() {
		t.Fatal("first matching window highlight should be active")
	}
	if !second.windowHighlight.IsHighlighted() {
		t.Fatal("second matching window highlight should be active")
	}
	if other.windowHighlight.IsHighlighted() {
		t.Fatal("non-matching window highlight should stay inactive")
	}
}

func TestUpdateOpenPathHighlightsMarksDialogOwnerAndClears(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	resetFileManagerWindowTestRegistry(t)

	current := &FileManager{
		window:          app.NewWindow("current"),
		currentPath:     "/current",
		windowHighlight: ui.NewHighlightFrame(nil),
	}
	target := &FileManager{
		window:          app.NewWindow("target"),
		currentPath:     "/target",
		windowHighlight: ui.NewHighlightFrame(nil),
	}
	registerFileManagerWindow(current)
	registerFileManagerWindow(target)

	ownerHighlighted := false
	setOwnerHighlighted := func(highlighted bool) {
		ownerHighlighted = highlighted
	}
	openPaths := map[string]bool{"/current": true, "/target": true}

	updateOpenPathHighlights(current, "/current", openPaths, setOwnerHighlighted)

	if !ownerHighlighted {
		t.Fatal("current open path should highlight the owning dialog")
	}
	if !current.windowHighlight.IsHighlighted() {
		t.Fatal("current open path should keep the dimmed window frame active")
	}
	if target.windowHighlight.IsHighlighted() {
		t.Fatal("non-matching target should stay inactive")
	}

	updateOpenPathHighlights(current, "/target", openPaths, setOwnerHighlighted)

	if ownerHighlighted {
		t.Fatal("other open path should clear the owning dialog highlight")
	}
	if current.windowHighlight.IsHighlighted() {
		t.Fatal("current window frame should clear for another path")
	}
	if !target.windowHighlight.IsHighlighted() {
		t.Fatal("matching target window should be active")
	}

	updateOpenPathHighlights(current, "/history", openPaths, setOwnerHighlighted)

	if ownerHighlighted || current.windowHighlight.IsHighlighted() || target.windowHighlight.IsHighlighted() {
		t.Fatal("non-open path should clear dialog and window highlights")
	}
}

func TestCloseWindowClearsOtherWindowHighlight(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	resetFileManagerWindowTestRegistry(t)
	atomic.StoreInt32(&windowCount, 2)
	t.Cleanup(func() {
		atomic.StoreInt32(&windowCount, 0)
	})

	current := &FileManager{
		window:          app.NewWindow("current"),
		currentPath:     "/current",
		windowHighlight: ui.NewHighlightFrame(nil),
	}
	target := &FileManager{
		window:          app.NewWindow("target"),
		currentPath:     "/target",
		windowHighlight: ui.NewHighlightFrame(nil),
	}
	registerFileManagerWindow(current)
	registerFileManagerWindow(target)
	highlightFileManagerWindowForPath("/target")

	if !target.windowHighlight.IsHighlighted() {
		t.Fatal("target window highlight should be active before close")
	}

	current.closeWindow()

	if target.windowHighlight.IsHighlighted() {
		t.Fatal("target window highlight should be cleared when source window closes")
	}
}
