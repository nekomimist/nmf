package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	customtheme "nmf/internal/theme"
)

const (
	highlightFrameStrokeWidth  float32 = 4
	highlightFrameContentInset float32 = 4
)

var defaultHighlightFrameColor = color.RGBA{R: 30, G: 120, B: 80, A: 255}

// HighlightFrame draws the shared open-directory accent around its bounds.
type HighlightFrame struct {
	widget.BaseWidget

	themeProvider ThemeColorProvider
	border        *canvas.Rectangle
	highlighted   bool
}

// NewHighlightFrame creates an initially transparent highlight frame.
func NewHighlightFrame(themeProvider ThemeColorProvider) *HighlightFrame {
	frame := &HighlightFrame{
		themeProvider: themeProvider,
		border:        canvas.NewRectangle(color.Transparent),
	}
	frame.border.StrokeColor = color.Transparent
	frame.border.StrokeWidth = highlightFrameStrokeWidth
	frame.ExtendBaseWidget(frame)
	return frame
}

// WrapContent keeps content clear of the frame's in-content stroke while the
// frame itself continues to cover the full available bounds. The inset is
// deliberately fixed rather than theme padding so compact item-spacing themes
// retain enough room for the highlight and a scrollbar at the window edge.
func (f *HighlightFrame) WrapContent(content fyne.CanvasObject) fyne.CanvasObject {
	if f == nil {
		return content
	}
	padding := layout.NewCustomPaddedLayout(
		highlightFrameContentInset,
		highlightFrameContentInset,
		highlightFrameContentInset,
		highlightFrameContentInset,
	)
	return container.NewMax(container.New(padding, content), f)
}

// SetHighlighted changes whether the frame uses the open-directory accent.
func (f *HighlightFrame) SetHighlighted(highlighted bool) {
	if f == nil || f.highlighted == highlighted {
		return
	}
	f.highlighted = highlighted
	f.Refresh()
}

// IsHighlighted reports whether the frame is active.
func (f *HighlightFrame) IsHighlighted() bool {
	return f != nil && f.highlighted
}

// Refresh updates the frame color after state or theme changes.
func (f *HighlightFrame) Refresh() {
	if f == nil || f.border == nil {
		return
	}
	f.updateBorderColor()
	f.border.Refresh()
	f.BaseWidget.Refresh()
}

func (f *HighlightFrame) updateBorderColor() {
	f.border.StrokeColor = color.Transparent
	if f.highlighted {
		f.border.StrokeColor = f.highlightColor()
	}
}

// CreateRenderer implements fyne.Widget.
func (f *HighlightFrame) CreateRenderer() fyne.WidgetRenderer {
	f.updateBorderColor()
	return widget.NewSimpleRenderer(f.border)
}

func (f *HighlightFrame) highlightColor() color.Color {
	provider := f.themeProvider
	if provider == nil {
		provider = currentThemeColorProvider()
	}
	if provider != nil {
		return provider.GetCustomColor(customtheme.ColorCopyMoveOpenDestination)
	}
	return defaultHighlightFrameColor
}
