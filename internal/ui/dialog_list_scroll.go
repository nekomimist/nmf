package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
)

func newScrollableDialogList(list fyne.CanvasObject, contentWidth, viewportWidth, viewportHeight float32) *container.Scroll {
	if contentWidth < viewportWidth {
		contentWidth = viewportWidth
	}
	wrapped := container.NewGridWrap(fyne.NewSize(contentWidth, viewportHeight), dialogListThemeOverride(list))
	scroll := container.NewScroll(wrapped)
	scroll.SetMinSize(fyne.NewSize(viewportWidth, viewportHeight))
	return scroll
}

func dialogTextWidth(values []string, minimum float32) float32 {
	width := minimum
	textSize := theme.TextSize()
	padding := theme.Padding()
	for _, value := range values {
		measured := fyne.MeasureText(value, textSize, fyne.TextStyle{}).Width + padding*4
		if measured > width {
			width = measured
		}
	}
	return width
}
