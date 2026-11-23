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

// keyCapBadge renders a compact rounded keycap tag for shortcut columns.
// When text is empty the badge frame hides itself to keep blank rows clean.
type keyCapBadge struct {
	widget.BaseWidget

	text       string
	background *canvas.Rectangle
	label      *canvas.Text
}

func newKeyCapBadge(text string) *keyCapBadge {
	b := &keyCapBadge{
		text:       text,
		background: canvas.NewRectangle(color.Transparent),
		label:      canvas.NewText(text, color.Transparent),
	}
	b.label.Alignment = fyne.TextAlignCenter
	b.label.TextStyle = fyne.TextStyle{Monospace: true}
	b.ExtendBaseWidget(b)
	return b
}

func (b *keyCapBadge) SetText(text string) {
	if b.text == text {
		return
	}
	b.text = text
	b.label.Text = text
	b.Refresh()
}

func (b *keyCapBadge) CreateRenderer() fyne.WidgetRenderer {
	r := &keyCapBadgeRenderer{
		badge:   b,
		objects: []fyne.CanvasObject{b.background, b.label},
	}
	r.applyTheme()
	return r
}

type keyCapBadgeRenderer struct {
	badge   *keyCapBadge
	objects []fyne.CanvasObject
}

func (r *keyCapBadgeRenderer) Destroy() {}

func (r *keyCapBadgeRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

const keyCapBadgeHorizPad float32 = 5

func (r *keyCapBadgeRenderer) MinSize() fyne.Size {
	th := r.badge.Theme()
	textSize := th.Size(theme.SizeNameCaptionText)
	if textSize <= 0 {
		textSize = th.Size(theme.SizeNameText)
	}
	sample := r.badge.text
	if sample == "" {
		sample = "M"
	}
	measured := fyne.MeasureText(sample, textSize, r.badge.label.TextStyle)
	return fyne.NewSize(measured.Width+keyCapBadgeHorizPad*2, measured.Height+4)
}

func (r *keyCapBadgeRenderer) Layout(size fyne.Size) {
	const vInset float32 = 1
	bgHeight := max(0, size.Height-vInset*2)
	r.badge.background.Move(fyne.NewPos(0, vInset))
	r.badge.background.Resize(fyne.NewSize(size.Width, bgHeight))

	display := r.badge.text
	if size.Width > 0 {
		avail := max(0, size.Width-keyCapBadgeHorizPad*2)
		display = truncateBadgeText(r.badge.text, avail, r.badge.label.TextSize, r.badge.label.TextStyle)
	}
	if r.badge.label.Text != display {
		r.badge.label.Text = display
		r.badge.label.Refresh()
	}

	labelMin := r.badge.label.MinSize()
	y := (size.Height - labelMin.Height) / 2
	r.badge.label.Move(fyne.NewPos(0, y))
	r.badge.label.Resize(fyne.NewSize(size.Width, labelMin.Height))
}

func (r *keyCapBadgeRenderer) Refresh() {
	r.applyTheme()
	r.Layout(r.badge.Size())
	r.badge.background.Refresh()
	r.badge.label.Refresh()
}

func truncateBadgeText(text string, maxWidth, textSize float32, style fyne.TextStyle) string {
	if text == "" || maxWidth <= 0 {
		return ""
	}
	if fyne.MeasureText(text, textSize, style).Width <= maxWidth {
		return text
	}
	const ellipsis = "..."
	runes := []rune(text)
	if fyne.MeasureText(ellipsis, textSize, style).Width > maxWidth {
		lo, hi := 0, len(runes)
		for lo < hi {
			mid := (lo + hi + 1) / 2
			if fyne.MeasureText(string(runes[:mid]), textSize, style).Width <= maxWidth {
				lo = mid
			} else {
				hi = mid - 1
			}
		}
		return string(runes[:lo])
	}
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		candidate := string(runes[:mid]) + ellipsis
		if fyne.MeasureText(candidate, textSize, style).Width <= maxWidth {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return string(runes[:lo]) + ellipsis
}

func (r *keyCapBadgeRenderer) applyTheme() {
	th := r.badge.Theme()
	variant := theme.VariantDark
	if app := fyne.CurrentApp(); app != nil {
		variant = app.Settings().ThemeVariant()
	}
	textSize := th.Size(theme.SizeNameCaptionText)
	if textSize <= 0 {
		textSize = th.Size(theme.SizeNameText)
	}
	r.badge.label.TextSize = textSize
	r.badge.label.Color = th.Color(theme.ColorNameForeground, variant)
	r.badge.background.CornerRadius = th.Size(theme.SizeNameSelectionRadius)
	if r.badge.text == "" {
		r.badge.background.FillColor = color.Transparent
		r.badge.background.StrokeColor = color.Transparent
		r.badge.background.StrokeWidth = 0
		return
	}
	r.badge.background.FillColor = th.Color(theme.ColorNameButton, variant)
	r.badge.background.StrokeColor = th.Color(theme.ColorNameInputBorder, variant)
	r.badge.background.StrokeWidth = 1
}
