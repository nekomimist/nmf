package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/config"
)

const backgroundCursorAlphaScale = 0.38

// FileListRow is the reusable visual template for a file list item.
//
// Its renderer owns a fixed set of canvas objects. List UpdateItem callbacks
// update row state and child widgets, but never replace the canvas object tree.
type FileListRow struct {
	widget.BaseWidget

	Icon      *TappableIcon
	NameLabel *FileNameLabel
	InfoLabel *widget.Label

	content        *fyne.Container
	cursorStyle    config.CursorStyleConfig
	hasStatus      bool
	statusColor    color.RGBA
	selected       bool
	selectionColor color.RGBA
	cursor         bool
	cursorColor    color.RGBA
}

// NewFileListRow creates a reusable file-list row with fixed content and
// decoration layers.
func NewFileListRow(cursorStyle config.CursorStyleConfig, nameColor color.RGBA) *FileListRow {
	icon := NewTappableIcon(theme.FolderIcon(), nil)
	name := NewFileNameLabel("filename", nameColor)
	info := widget.NewLabel("info")
	info.TextStyle = fyne.TextStyle{Monospace: true}

	textSize := fyne.CurrentApp().Settings().Theme().Size(theme.SizeNameText)
	icon.Resize(fyne.NewSize(textSize, textSize))

	row := &FileListRow{
		Icon:        icon,
		NameLabel:   name,
		InfoLabel:   info,
		content:     container.NewBorder(nil, nil, icon, info, name),
		cursorStyle: cursorStyle,
	}
	row.ExtendBaseWidget(row)
	return row
}

// SetDecorations updates the row's status, selection, and cursor state.
// The renderer keeps its CanvasObject identities stable across calls.
func (r *FileListRow) SetDecorations(
	statusColor *color.RGBA,
	selected bool,
	selectionColor color.RGBA,
	cursor bool,
	cursorColor color.RGBA,
) {
	hasStatus := statusColor != nil
	nextStatusColor := color.RGBA{}
	if hasStatus {
		nextStatusColor = *statusColor
	}
	nextSelectionColor := color.RGBA{}
	if selected {
		nextSelectionColor = selectionColor
	}
	nextCursorColor := color.RGBA{}
	if cursor {
		nextCursorColor = cursorColor
	}

	if r.hasStatus == hasStatus &&
		r.statusColor == nextStatusColor &&
		r.selected == selected &&
		r.selectionColor == nextSelectionColor &&
		r.cursor == cursor &&
		r.cursorColor == nextCursorColor {
		return
	}

	r.hasStatus = hasStatus
	r.statusColor = nextStatusColor
	r.selected = selected
	r.selectionColor = nextSelectionColor
	r.cursor = cursor
	r.cursorColor = nextCursorColor
	r.Refresh()
}

// CreateRenderer builds the fixed layers used for every update of this row.
func (r *FileListRow) CreateRenderer() fyne.WidgetRenderer {
	r.ExtendBaseWidget(r)

	renderer := &fileListRowRenderer{
		row: r,
	}
	renderer.status = canvas.NewRectangle(&renderer.statusFill)
	renderer.selection = canvas.NewRectangle(&renderer.selectionFill)
	renderer.cursorBackground = canvas.NewRectangle(&renderer.cursorBackgroundFill)
	renderer.cursorTop = canvas.NewRectangle(&renderer.cursorTopFill)
	renderer.cursorBottom = canvas.NewRectangle(&renderer.cursorBottomFill)
	renderer.cursorLeft = canvas.NewRectangle(&renderer.cursorLeftFill)
	renderer.cursorRight = canvas.NewRectangle(&renderer.cursorRightFill)
	renderer.objects = []fyne.CanvasObject{
		r.content,
		renderer.status,
		renderer.selection,
		renderer.cursorBackground,
		renderer.cursorTop,
		renderer.cursorBottom,
		renderer.cursorLeft,
		renderer.cursorRight,
	}
	renderer.applyColors(false)
	return renderer
}

type fileListRowRenderer struct {
	objects          []fyne.CanvasObject
	row              *FileListRow
	status           *canvas.Rectangle
	selection        *canvas.Rectangle
	cursorBackground *canvas.Rectangle
	cursorTop        *canvas.Rectangle
	cursorBottom     *canvas.Rectangle
	cursorLeft       *canvas.Rectangle
	cursorRight      *canvas.Rectangle

	statusFill           color.RGBA
	selectionFill        color.RGBA
	cursorBackgroundFill color.RGBA
	cursorTopFill        color.RGBA
	cursorBottomFill     color.RGBA
	cursorLeftFill       color.RGBA
	cursorRightFill      color.RGBA
}

func (r *fileListRowRenderer) Destroy() {}

func (r *fileListRowRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *fileListRowRenderer) Layout(size fyne.Size) {
	r.row.content.Resize(size)
	r.status.Resize(size)
	r.selection.Resize(size)
	r.cursorBackground.Resize(size)

	thickness := r.cursorThickness()
	horizontalThickness := min(thickness, size.Height)
	verticalThickness := min(thickness, size.Width)

	r.cursorTop.Move(fyne.NewPos(0, 0))
	r.cursorTop.Resize(fyne.NewSize(size.Width, horizontalThickness))
	r.cursorBottom.Move(fyne.NewPos(0, max(0, size.Height-horizontalThickness)))
	r.cursorBottom.Resize(fyne.NewSize(size.Width, horizontalThickness))
	r.cursorLeft.Move(fyne.NewPos(0, 0))
	r.cursorLeft.Resize(fyne.NewSize(verticalThickness, size.Height))
	r.cursorRight.Move(fyne.NewPos(max(0, size.Width-verticalThickness), 0))
	r.cursorRight.Resize(fyne.NewSize(verticalThickness, size.Height))
}

func (r *fileListRowRenderer) MinSize() fyne.Size {
	return r.row.content.MinSize()
}

func (r *fileListRowRenderer) Refresh() {
	r.applyColors(true)
}

func (r *fileListRowRenderer) applyColors(refresh bool) {
	transparent := color.RGBA{}
	statusColor := transparent
	if r.row.hasStatus {
		statusColor = r.row.statusColor
	}
	selectionColor := transparent
	if r.row.selected {
		selectionColor = r.row.selectionColor
	}

	cursorBackgroundColor := transparent
	cursorLineColor := transparent
	if r.row.cursor {
		switch r.cursorStyle() {
		case "background":
			cursorBackgroundColor = scaleAlpha(r.row.cursorColor, backgroundCursorAlphaScale)
		case "border", "underline":
			cursorLineColor = r.row.cursorColor
		}
	}

	cursorTopColor := transparent
	cursorBottomColor := transparent
	cursorLeftColor := transparent
	cursorRightColor := transparent

	switch r.cursorStyle() {
	case "border":
		cursorTopColor = cursorLineColor
		cursorBottomColor = cursorLineColor
		cursorLeftColor = cursorLineColor
		cursorRightColor = cursorLineColor
	case "underline":
		cursorBottomColor = cursorLineColor
	}

	setRectangleColor(&r.statusFill, r.status, statusColor, refresh)
	setRectangleColor(&r.selectionFill, r.selection, selectionColor, refresh)
	setRectangleColor(&r.cursorBackgroundFill, r.cursorBackground, cursorBackgroundColor, refresh)
	setRectangleColor(&r.cursorTopFill, r.cursorTop, cursorTopColor, refresh)
	setRectangleColor(&r.cursorBottomFill, r.cursorBottom, cursorBottomColor, refresh)
	setRectangleColor(&r.cursorLeftFill, r.cursorLeft, cursorLeftColor, refresh)
	setRectangleColor(&r.cursorRightFill, r.cursorRight, cursorRightColor, refresh)
}

func (r *fileListRowRenderer) cursorStyle() string {
	switch r.row.cursorStyle.Type {
	case "border", "background":
		return r.row.cursorStyle.Type
	default:
		return "underline"
	}
}

func (r *fileListRowRenderer) cursorThickness() float32 {
	thickness := float32(r.row.cursorStyle.Thickness)
	if thickness > 0 {
		return thickness
	}
	if r.cursorStyle() == "border" {
		return 1
	}
	return 2
}

func scaleAlpha(c color.RGBA, scale float32) color.RGBA {
	c.A = uint8(float32(c.A) * scale)
	return c
}

func setRectangleColor(current *color.RGBA, rect *canvas.Rectangle, next color.RGBA, refresh bool) {
	if *current == next {
		return
	}
	*current = next
	if refresh {
		rect.Refresh()
	}
}
