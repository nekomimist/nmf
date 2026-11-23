package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	insetSurfaceBorderWidth  float32 = 1
	insetSurfaceVerticalPad  float32 = 2
	insetSurfaceSidePad      float32 = 6
	dialogSectionGap         float32 = 6
	dialogFormLabelWidth     float32 = 84
	dialogConflictLabelWidth float32 = 76
)

// insetSurface wraps a list, tree, or scrollable area in a rounded 1px
// hairline inset frame with light horizontal breathing room and minimal
// vertical overhead.
type insetSurface struct {
	widget.BaseWidget

	background *canvas.Rectangle
	content    fyne.CanvasObject
}

func newInsetListSurface(content fyne.CanvasObject) fyne.CanvasObject {
	return newInsetSurfaceWithPadding(content, insetSurfaceVerticalPad, insetSurfaceVerticalPad, insetSurfaceSidePad, insetSurfaceSidePad)
}

func newInsetSurfaceWithPadding(content fyne.CanvasObject, top, bottom, left, right float32) fyne.CanvasObject {
	padded := container.New(layout.NewCustomPaddedLayout(top, bottom, left, right), content)
	s := &insetSurface{
		background: canvas.NewRectangle(color.Transparent),
		content:    padded,
	}
	s.ExtendBaseWidget(s)
	return s
}

// NewInsetListSurface exposes newInsetListSurface to package main (drop_ui.go).
func NewInsetListSurface(content fyne.CanvasObject) fyne.CanvasObject {
	return newInsetListSurface(content)
}

func (s *insetSurface) CreateRenderer() fyne.WidgetRenderer {
	r := &insetSurfaceRenderer{
		surface: s,
		objects: []fyne.CanvasObject{s.background, s.content},
	}
	r.applyTheme()
	return r
}

type insetSurfaceRenderer struct {
	surface *insetSurface
	objects []fyne.CanvasObject
}

func (r *insetSurfaceRenderer) Destroy() {}

func (r *insetSurfaceRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *insetSurfaceRenderer) MinSize() fyne.Size {
	return r.surface.content.MinSize()
}

func (r *insetSurfaceRenderer) Layout(size fyne.Size) {
	r.surface.background.Move(fyne.NewPos(0, 0))
	r.surface.background.Resize(size)
	r.surface.content.Move(fyne.NewPos(0, 0))
	r.surface.content.Resize(size)
}

func (r *insetSurfaceRenderer) Refresh() {
	r.applyTheme()
	r.surface.background.Refresh()
	r.surface.content.Refresh()
}

func (r *insetSurfaceRenderer) applyTheme() {
	th := r.surface.Theme()
	variant := theme.VariantDark
	if app := fyne.CurrentApp(); app != nil {
		variant = app.Settings().ThemeVariant()
	}
	r.surface.background.FillColor = th.Color(theme.ColorNameInputBackground, variant)
	r.surface.background.StrokeColor = th.Color(theme.ColorNameInputBorder, variant)
	r.surface.background.StrokeWidth = insetSurfaceBorderWidth
	r.surface.background.CornerRadius = th.Size(theme.SizeNameInputRadius)
}

func newDialogHintLabel(text string) *widget.Label {
	label := widget.NewLabel(text)
	label.SizeName = theme.SizeNameCaptionText
	label.Importance = widget.LowImportance
	return label
}

func newDialogGap(height float32) fyne.CanvasObject {
	rect := canvas.NewRectangle(color.Transparent)
	rect.SetMinSize(fyne.NewSize(0, height))
	return rect
}

func newFormRow(labelWidth float32, label string, field fyne.CanvasObject) fyne.CanvasObject {
	lbl := widget.NewLabel(label)
	return container.NewBorder(
		nil,
		nil,
		container.New(minWidthLayout{min: labelWidth}, lbl),
		nil,
		field,
	)
}
