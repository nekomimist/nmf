package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
)

type dialogListScroller struct {
	*container.Scroll
	viewportWidth float32
}

func newScrollableDialogList(list fyne.CanvasObject, contentWidth, viewportWidth, viewportHeight float32) *container.Scroll {
	return newDialogListScroller(list, contentWidth, viewportWidth, viewportHeight).Scroll
}

func newDialogListScroller(list fyne.CanvasObject, contentWidth, viewportWidth, viewportHeight float32) *dialogListScroller {
	if contentWidth < viewportWidth {
		contentWidth = viewportWidth
	}
	wrapped := container.NewGridWrap(fyne.NewSize(contentWidth, viewportHeight), dialogListThemeOverride(list))
	scroll := container.NewScroll(wrapped)
	scroll.SetMinSize(fyne.NewSize(viewportWidth, viewportHeight))
	return &dialogListScroller{Scroll: scroll, viewportWidth: viewportWidth}
}

func (s *dialogListScroller) ScrollPathRight(path string) {
	if s == nil || s.Scroll == nil {
		return
	}
	width := dialogTextWidth([]string{path}, s.viewportWidth)
	offsetX := width - s.viewportWidth
	if offsetX < 0 {
		offsetX = 0
	}
	s.ScrollToOffset(fyne.NewPos(offsetX, s.Offset.Y))
}

func (s *dialogListScroller) ResetHorizontalScroll() {
	if s == nil || s.Scroll == nil {
		return
	}
	s.ScrollToOffset(fyne.NewPos(0, s.Offset.Y))
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
