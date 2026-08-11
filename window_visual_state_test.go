package main

import (
	"image/color"
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
		browser:      newTestBrowser(testBrowserOptions{path: "/tmp"}),
	}

	fm.focusFileList("test")

	if !fm.windowActive {
		t.Fatal("focusFileList should restore active state")
	}
	if window.Canvas().Focused() != fileListView {
		t.Fatalf("focused object = %T, want fileListView", window.Canvas().Focused())
	}
}

func TestMainScreenPointerActionRestoresFocusBeforeAction(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	window := app.NewWindow("pointer action")
	fm := &FileManager{
		window:       window,
		windowActive: true,
		browser:      newTestBrowser(testBrowserOptions{path: "/tmp"}),
	}
	button := widget.NewButton("Action", fm.mainScreenPointerAction(func() {
		if window.Canvas().Focused() != fm.fileListView {
			t.Fatalf("action started with focus on %T, want fileListView", window.Canvas().Focused())
		}
		if !fm.windowActive {
			t.Fatal("action started with inactive File Manager window")
		}
	}))
	fm.fileListView = ui.NewKeySink(button, nil, ui.WithFocusChanged(fm.setWindowActive))
	window.SetContent(fm.fileListView)
	window.Canvas().Focus(fm.fileListView)

	test.Tap(button)

	if got := window.Canvas().Focused(); got != fm.fileListView {
		t.Fatalf("focused object after action = %T, want fileListView", got)
	}
}

func TestHighlightFileManagerWindowForPathHighlightsOpenWindow(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	runtime := newFileManagerWindowTestRuntime(t)

	current := &FileManager{
		runtime:         runtime,
		window:          app.NewWindow("current"),
		browser:         newTestBrowser(testBrowserOptions{path: "/current"}),
		windowHighlight: ui.NewHighlightFrame(nil),
	}
	target := &FileManager{
		runtime:         runtime,
		window:          app.NewWindow("target"),
		browser:         newTestBrowser(testBrowserOptions{path: "/target"}),
		windowHighlight: ui.NewHighlightFrame(nil),
	}
	registerFileManagerWindow(current)
	registerFileManagerWindow(target)

	highlightFileManagerWindowForPath(current, "/target")

	if current.windowHighlight.IsHighlighted() {
		t.Fatal("current window highlight should stay inactive")
	}
	if !target.windowHighlight.IsHighlighted() {
		t.Fatal("target window highlight should be active")
	}

	clearFileManagerWindowHighlights(current)
	if target.windowHighlight.IsHighlighted() {
		t.Fatal("target window highlight should be cleared")
	}
}

func TestHighlightFileManagerWindowForPathHighlightsEveryMatchingWindow(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	runtime := newFileManagerWindowTestRuntime(t)

	first := &FileManager{
		runtime:         runtime,
		window:          app.NewWindow("first"),
		browser:         newTestBrowser(testBrowserOptions{path: "/shared"}),
		windowHighlight: ui.NewHighlightFrame(nil),
	}
	second := &FileManager{
		runtime:         runtime,
		window:          app.NewWindow("second"),
		browser:         newTestBrowser(testBrowserOptions{path: "/shared"}),
		windowHighlight: ui.NewHighlightFrame(nil),
	}
	other := &FileManager{
		runtime:         runtime,
		window:          app.NewWindow("other"),
		browser:         newTestBrowser(testBrowserOptions{path: "/other"}),
		windowHighlight: ui.NewHighlightFrame(nil),
	}
	registerFileManagerWindow(first)
	registerFileManagerWindow(second)
	registerFileManagerWindow(other)

	highlightFileManagerWindowForPath(first, "/shared")

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
	runtime := newFileManagerWindowTestRuntime(t)

	current := &FileManager{
		runtime:         runtime,
		window:          app.NewWindow("current"),
		browser:         newTestBrowser(testBrowserOptions{path: "/current"}),
		windowHighlight: ui.NewHighlightFrame(nil),
	}
	target := &FileManager{
		runtime:         runtime,
		window:          app.NewWindow("target"),
		browser:         newTestBrowser(testBrowserOptions{path: "/target"}),
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
	runtime := newFileManagerWindowTestRuntime(t)

	current := &FileManager{
		runtime:         runtime,
		window:          app.NewWindow("current"),
		browser:         newTestBrowser(testBrowserOptions{path: "/current"}),
		windowHighlight: ui.NewHighlightFrame(nil),
	}
	target := &FileManager{
		runtime:         runtime,
		window:          app.NewWindow("target"),
		browser:         newTestBrowser(testBrowserOptions{path: "/target"}),
		windowHighlight: ui.NewHighlightFrame(nil),
	}
	registerFileManagerWindow(current)
	registerFileManagerWindow(target)
	highlightFileManagerWindowForPath(current, "/target")

	if !target.windowHighlight.IsHighlighted() {
		t.Fatal("target window highlight should be active before close")
	}

	current.closeWindow()

	if target.windowHighlight.IsHighlighted() {
		t.Fatal("target window highlight should be cleared when source window closes")
	}
}
