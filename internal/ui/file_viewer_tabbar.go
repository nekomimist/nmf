package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var fileViewerTabBarLabels = map[string]string{
	viewerPaneImage:    "Image (i)",
	viewerPaneText:     "Text (t)",
	viewerPaneMarkdown: "Markdown (m)",
	viewerPaneHex:      "Hex (x)",
}

var fileViewerTabBarOrder = []string{viewerPaneText, viewerPaneMarkdown, viewerPaneHex}
var fileViewerImageTabBarOrder = []string{viewerPaneImage, viewerPaneHex}

const fileViewerTabIndicatorHeight float32 = 2

// fileViewerTabBar is a flat underline-style pane switcher built from stock
// widget.Button instances and a thin bottom accent indicator so the active tab
// stays distinct without competing visually with primary action toggles.
type fileViewerTabBar struct {
	buttons    map[string]*widget.Button
	indicators map[string]*canvas.Rectangle
	order      []string
	active     string
	bar        *fyne.Container
}

func newFileViewerTabBar(order []string, onSelect func(pane string)) *fileViewerTabBar {
	if len(order) == 0 {
		order = fileViewerTabBarOrder
	}
	order = append([]string(nil), order...)
	t := &fileViewerTabBar{
		buttons:    make(map[string]*widget.Button, len(order)),
		indicators: make(map[string]*canvas.Rectangle, len(order)),
		order:      order,
	}
	segments := make([]fyne.CanvasObject, 0, len(t.order))
	for _, pane := range t.order {
		button := widget.NewButton(fileViewerTabBarLabels[pane], func() {
			if onSelect != nil {
				onSelect(pane)
			}
		})
		button.Importance = widget.LowImportance
		indicator := canvas.NewRectangle(color.Transparent)
		indicator.SetMinSize(fyne.NewSize(0, fileViewerTabIndicatorHeight))
		indicator.CornerRadius = 1
		t.buttons[pane] = button
		t.indicators[pane] = indicator
		segments = append(segments, container.NewBorder(nil, indicator, nil, nil, button))
	}
	t.bar = container.NewVBox(container.NewHBox(segments...), widget.NewSeparator())
	return t
}

func (t *fileViewerTabBar) Container() fyne.CanvasObject {
	return t.bar
}

func (t *fileViewerTabBar) SetActive(pane string) {
	if pane == t.active {
		return
	}
	if button, ok := t.buttons[t.active]; ok {
		button.Importance = widget.LowImportance
		button.Refresh()
	}
	if indicator, ok := t.indicators[t.active]; ok {
		indicator.FillColor = color.Transparent
		indicator.Refresh()
	}
	if button, ok := t.buttons[pane]; ok {
		button.Importance = widget.MediumImportance
		button.Refresh()
	}
	if indicator, ok := t.indicators[pane]; ok {
		indicator.FillColor = currentAppThemeColor(theme.ColorNamePrimary)
		indicator.Refresh()
	}
	t.active = pane
}

// Label returns the display label for pane; used by tests.
func (t *fileViewerTabBar) Label(pane string) string {
	if button, ok := t.buttons[pane]; ok {
		return button.Text
	}
	return ""
}
