package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// TappableIcon is a custom icon widget that can handle tap events
type TappableIcon struct {
	widget.BaseWidget
	icon      *widget.Icon
	onTapped  func()
	onDragged func()
	dragging  bool
	pressed   bool
	pressPos  fyne.Position
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

// MouseDown records the initial press for native file drag startup.
func (ti *TappableIcon) MouseDown(ev *desktop.MouseEvent) {
	if ev.Button != desktop.MouseButtonPrimary {
		return
	}
	ti.pressed = true
	ti.dragging = false
	ti.pressPos = ev.AbsolutePosition
}

// MouseUp clears any pending native drag startup.
func (ti *TappableIcon) MouseUp(ev *desktop.MouseEvent) {
	if ev.Button == desktop.MouseButtonPrimary {
		ti.pressed = false
		ti.dragging = false
	}
}

// MouseIn implements desktop.Hoverable.
func (ti *TappableIcon) MouseIn(_ *desktop.MouseEvent) {}

// MouseMoved starts a native drag after pointer movement exceeds a small threshold.
func (ti *TappableIcon) MouseMoved(ev *desktop.MouseEvent) {
	if !ti.pressed || ev.Button != desktop.MouseButtonPrimary {
		return
	}
	if ti.dragging {
		return
	}
	dx := ev.AbsolutePosition.X - ti.pressPos.X
	dy := ev.AbsolutePosition.Y - ti.pressPos.Y
	const dragStartDistance float32 = 4
	if dx*dx+dy*dy < dragStartDistance*dragStartDistance {
		return
	}
	ti.dragging = true
	if ti.onDragged != nil {
		ti.onDragged()
	}
	// Native drag loops can consume the pointer release before Fyne dispatches
	// DragEnd, so do not keep the local guard latched after the start callback.
	ti.dragging = false
}

// MouseOut implements desktop.Hoverable.
func (ti *TappableIcon) MouseOut() {
	ti.dragging = false
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

// SetOnDragged sets the callback invoked when a drag starts.
func (ti *TappableIcon) SetOnDragged(onDragged func()) {
	ti.onDragged = onDragged
	ti.dragging = false
	ti.pressed = false
}

// CreateRenderer creates the widget renderer
func (ti *TappableIcon) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(ti.icon)
}

// FileNameLabel draws a file name that shrinks to its assigned width.
type FileNameLabel struct {
	widget.BaseWidget
	name          string
	color         color.RGBA
	deleted       bool
	text          *canvas.Text
	onTapped      func(fyne.KeyModifier)
	onDragged     func()
	dragging      bool
	pressed       bool
	suppressTap   bool
	pressPos      fyne.Position
	pressModifier fyne.KeyModifier
}

func NewFileNameLabel(name string, textColor color.RGBA) *FileNameLabel {
	label := &FileNameLabel{
		name:  name,
		color: textColor,
		text:  canvas.NewText(name, textColor),
	}
	label.text.TextStyle = fyne.TextStyle{Monospace: true}
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

// Tapped handles left-click actions on the file name area.
func (l *FileNameLabel) Tapped(_ *fyne.PointEvent) {
	if l.suppressTap {
		l.suppressTap = false
		return
	}
	if l.onTapped != nil {
		l.onTapped(l.pressModifier)
	}
}

// MouseDown records the initial press for click modifiers and drag startup.
func (l *FileNameLabel) MouseDown(ev *desktop.MouseEvent) {
	if ev.Button != desktop.MouseButtonPrimary {
		return
	}
	l.pressed = true
	l.dragging = false
	l.suppressTap = false
	l.pressPos = ev.AbsolutePosition
	l.pressModifier = ev.Modifier
}

// MouseUp clears any pending drag startup.
func (l *FileNameLabel) MouseUp(ev *desktop.MouseEvent) {
	if ev.Button == desktop.MouseButtonPrimary {
		l.pressed = false
		l.dragging = false
	}
}

// MouseIn implements desktop.Hoverable.
func (l *FileNameLabel) MouseIn(_ *desktop.MouseEvent) {}

// MouseMoved starts a native drag after pointer movement exceeds a small threshold.
func (l *FileNameLabel) MouseMoved(ev *desktop.MouseEvent) {
	if !l.pressed || ev.Button != desktop.MouseButtonPrimary {
		return
	}
	if l.dragging {
		return
	}
	dx := ev.AbsolutePosition.X - l.pressPos.X
	dy := ev.AbsolutePosition.Y - l.pressPos.Y
	const dragStartDistance float32 = 4
	if dx*dx+dy*dy < dragStartDistance*dragStartDistance {
		return
	}
	l.dragging = true
	l.suppressTap = true
	if l.onDragged != nil {
		l.onDragged()
	}
	l.pressed = false
	l.dragging = false
}

// MouseOut implements desktop.Hoverable.
func (l *FileNameLabel) MouseOut() {
	l.dragging = false
}

// SetOnTapped sets the callback invoked when the file name is clicked.
func (l *FileNameLabel) SetOnTapped(onTapped func(fyne.KeyModifier)) {
	l.onTapped = onTapped
	l.pressModifier = 0
	l.suppressTap = false
}

// SetOnDragged sets the callback invoked when a drag starts.
func (l *FileNameLabel) SetOnDragged(onDragged func()) {
	l.onDragged = onDragged
	l.dragging = false
	l.pressed = false
	l.suppressTap = false
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
