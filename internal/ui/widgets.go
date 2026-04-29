package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// TappableIcon is a custom icon widget that can handle tap events
type TappableIcon struct {
	widget.BaseWidget
	icon     *widget.Icon
	onTapped func()
}

// NewTappableIcon creates a new tappable icon widget
func NewTappableIcon(resource fyne.Resource, onTapped func()) *TappableIcon {
	icon := widget.NewIcon(resource)
	ti := &TappableIcon{
		icon:     icon,
		onTapped: onTapped,
	}
	ti.ExtendBaseWidget(ti)
	return ti
}

// Tapped handles tap events on the icon
func (ti *TappableIcon) Tapped(_ *fyne.PointEvent) {
	if ti.onTapped != nil {
		ti.onTapped()
	}
}

// SetResource sets the icon resource
func (ti *TappableIcon) SetResource(resource fyne.Resource) {
	ti.icon.SetResource(resource)
	ti.Refresh()
}

// SetOnTapped sets the tap handler function
func (ti *TappableIcon) SetOnTapped(onTapped func()) {
	ti.onTapped = onTapped
}

// CreateRenderer creates the widget renderer
func (ti *TappableIcon) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(ti.icon)
}

// FileNameLabel draws a file name that shrinks to its assigned width.
type FileNameLabel struct {
	widget.BaseWidget
	name    string
	color   color.RGBA
	deleted bool
	text    *canvas.Text
}

func NewFileNameLabel(name string, textColor color.RGBA) *FileNameLabel {
	label := &FileNameLabel{
		name:  name,
		color: textColor,
		text:  canvas.NewText(name, textColor),
	}
	label.text.TextSize = fyne.CurrentApp().Settings().Theme().Size(theme.SizeNameText)
	label.ExtendBaseWidget(label)
	return label
}

func (l *FileNameLabel) SetFile(name string, textColor color.RGBA, deleted bool) {
	l.name = name
	l.color = textColor
	l.deleted = deleted
	l.text.Color = textColor
	l.text.TextSize = fyne.CurrentApp().Settings().Theme().Size(theme.SizeNameText)
	l.Refresh()
}

func (l *FileNameLabel) displayText(width float32) string {
	name := l.name
	if l.deleted {
		name = "\u22a0 " + name
	}
	if width <= 0 {
		return ""
	}
	if textWidth(name, l.text.TextSize, l.text.TextStyle) <= width {
		return name
	}

	const ellipsis = "..."
	runes := []rune(name)
	if textWidth(ellipsis, l.text.TextSize, l.text.TextStyle) > width {
		return ""
	}
	for keep := len(runes) - 1; keep > 0; keep-- {
		prefix := keep / 2
		suffix := keep - prefix
		candidate := string(runes[:prefix]) + ellipsis + string(runes[len(runes)-suffix:])
		if textWidth(candidate, l.text.TextSize, l.text.TextStyle) <= width {
			return candidate
		}
	}
	return ellipsis
}

func textWidth(text string, size float32, style fyne.TextStyle) float32 {
	return fyne.MeasureText(text, size, style).Width
}

func (l *FileNameLabel) CreateRenderer() fyne.WidgetRenderer {
	return &fileNameLabelRenderer{label: l}
}

type fileNameLabelRenderer struct {
	label *FileNameLabel
}

func (r *fileNameLabelRenderer) Layout(size fyne.Size) {
	r.label.text.Text = r.label.displayText(size.Width)
	textSize := r.label.text.MinSize()
	r.label.text.Move(fyne.NewPos(0, (size.Height-textSize.Height)/2))
	r.label.text.Resize(fyne.NewSize(size.Width, textSize.Height))
}

func (r *fileNameLabelRenderer) MinSize() fyne.Size {
	textSize := fyne.MeasureText("M", r.label.text.TextSize, r.label.text.TextStyle)
	return fyne.NewSize(0, textSize.Height)
}

func (r *fileNameLabelRenderer) Refresh() {
	r.label.text.Color = r.label.color
	r.label.text.TextSize = fyne.CurrentApp().Settings().Theme().Size(theme.SizeNameText)
	r.Layout(r.label.Size())
	canvas.Refresh(r.label)
}

func (r *fileNameLabelRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.label.text}
}

func (r *fileNameLabelRenderer) Destroy() {}
