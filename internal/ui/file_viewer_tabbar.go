package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

var fileViewerTabBarLabels = map[string]string{
	viewerPaneText:     "Text (t)",
	viewerPaneMarkdown: "Markdown (m)",
	viewerPaneHex:      "Hex (x)",
}

var fileViewerTabBarOrder = []string{viewerPaneText, viewerPaneMarkdown, viewerPaneHex}

// fileViewerTabBar is a segmented-control style pane switcher built entirely
// from stock widget.Button instances; it does not extend widget.BaseWidget,
// so all rendering is delegated to the buttons themselves.
type fileViewerTabBar struct {
	buttons map[string]*widget.Button
	order   []string
	active  string
	bar     *fyne.Container
}

func newFileViewerTabBar(onSelect func(pane string)) *fileViewerTabBar {
	t := &fileViewerTabBar{
		buttons: make(map[string]*widget.Button, len(fileViewerTabBarOrder)),
		order:   fileViewerTabBarOrder,
	}
	segments := make([]fyne.CanvasObject, 0, len(t.order))
	for _, pane := range t.order {
		button := widget.NewButton(fileViewerTabBarLabels[pane], func() {
			if onSelect != nil {
				onSelect(pane)
			}
		})
		button.Importance = widget.MediumImportance
		t.buttons[pane] = button
		segments = append(segments, button)
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
		button.Importance = widget.MediumImportance
		button.Refresh()
	}
	if button, ok := t.buttons[pane]; ok {
		button.Importance = widget.HighImportance
		button.Refresh()
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
