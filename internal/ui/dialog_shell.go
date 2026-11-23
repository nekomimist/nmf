package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	dialogShellHeaderTopPad    float32 = 10
	dialogShellHeaderBottomPad float32 = 8
	dialogShellHeaderSidePad   float32 = 14
	dialogShellBodyTopPad      float32 = 10
	dialogShellBodyBottomPad   float32 = 12
	dialogShellBodySidePad     float32 = 14
	dialogShellBorderWidth     float32 = 1
	dialogShellTitleSizeDelta  float32 = 1
)

type modalDialogOptions struct {
	headerTrailing fyne.CanvasObject
	highlight      *dialogHighlightState
	compactBody    bool
}

// modalDialog implements dialog.Dialog over widget.NewModalPopUp with an
// opaque surface, a 1px hairline border, a compact single-line header, and
// scoped dialog control padding.
type modalDialog struct {
	title       string
	parent      fyne.Window
	win         *widget.PopUp
	themeRoot   *container.ThemeOverride
	shell       *dialogShellWidget
	desiredSize fyne.Size
	onClosed    func()
}

var _ dialog.Dialog = (*modalDialog)(nil)

func newModalDialog(title string, content fyne.CanvasObject, parent fyne.Window) dialog.Dialog {
	return newModalDialogWithOptions(title, content, parent, modalDialogOptions{})
}

func newModalDialogWithOptions(title string, content fyne.CanvasObject, parent fyne.Window, opts modalDialogOptions) dialog.Dialog {
	var accentFrame *HighlightFrame
	if opts.highlight != nil {
		accentFrame = opts.highlight.attachDialogFrame()
	}

	shell := newDialogShellWidget(title, content, opts.headerTrailing, accentFrame, opts.compactBody)
	var popup *widget.PopUp
	var themeRoot *container.ThemeOverride
	if parent != nil && parent.Canvas() != nil {
		themed := dialogThemeOverride(shell)
		themeRoot, _ = themed.(*container.ThemeOverride)
		popup = widget.NewModalPopUp(themed, parent.Canvas())
	}
	return &modalDialog{
		title:     title,
		parent:    parent,
		win:       popup,
		themeRoot: themeRoot,
		shell:     shell,
	}
}

// NewModalDialog exposes newModalDialog to package main (drop_ui.go).
func NewModalDialog(title string, content fyne.CanvasObject, parent fyne.Window) dialog.Dialog {
	return newModalDialog(title, content, parent)
}

func (d *modalDialog) Show() {
	if d.win == nil {
		return
	}
	if !d.desiredSize.IsZero() {
		d.win.Resize(d.clampedSize(d.desiredSize))
	}
	d.win.Show()
}

func (d *modalDialog) Hide() {
	if d.win != nil {
		d.win.Hide()
	}
	if d.onClosed != nil {
		d.onClosed()
	}
}

func (d *modalDialog) Dismiss() {
	d.Hide()
}

func (d *modalDialog) SetDismissText(string) {}

func (d *modalDialog) SetOnClosed(closed func()) {
	if closed == nil {
		return
	}
	previous := d.onClosed
	d.onClosed = func() {
		if previous != nil {
			previous()
		}
		closed()
	}
}

func (d *modalDialog) Refresh() {
	if d.themeRoot != nil {
		d.themeRoot.Refresh()
	}
	if d.win != nil {
		d.win.Refresh()
	}
}

func (d *modalDialog) Resize(size fyne.Size) {
	d.desiredSize = d.clampedSize(size)
	if d.win != nil {
		d.win.Resize(d.desiredSize)
	}
}

func (d *modalDialog) MinSize() fyne.Size {
	if d.win == nil {
		if d.shell != nil {
			return d.shell.MinSize()
		}
		return fyne.NewSquareSize(1)
	}
	return d.win.MinSize()
}

func (d *modalDialog) clampedSize(size fyne.Size) fyne.Size {
	minSize := d.MinSize()
	width := max(size.Width, minSize.Width)
	height := max(size.Height, minSize.Height)
	return fyne.NewSize(width, height)
}

type dialogShellWidget struct {
	widget.BaseWidget

	title          string
	titleText      *canvas.Text
	headerTrailing fyne.CanvasObject
	headerDivider  *canvas.Rectangle
	body           fyne.CanvasObject
	accentFrame    *HighlightFrame
	background     *canvas.Rectangle
	content        fyne.CanvasObject
}

func newDialogShellWidget(
	title string,
	body fyne.CanvasObject,
	headerTrailing fyne.CanvasObject,
	accentFrame *HighlightFrame,
	compactBody bool,
) *dialogShellWidget {
	topPad := dialogShellBodyTopPad
	bottomPad := dialogShellBodyBottomPad
	sidePad := dialogShellBodySidePad
	if compactBody {
		topPad = 8
		bottomPad = 8
		sidePad = 10
	}

	paddedBody := container.New(
		layout.NewCustomPaddedLayout(topPad, bottomPad, sidePad, sidePad),
		body,
	)

	s := &dialogShellWidget{
		title:          title,
		headerTrailing: headerTrailing,
		body:           body,
		accentFrame:    accentFrame,
		background:     canvas.NewRectangle(color.Transparent),
	}

	var top fyne.CanvasObject
	if title != "" || headerTrailing != nil {
		s.titleText = canvas.NewText(title, color.Transparent)
		s.titleText.TextStyle = fyne.TextStyle{Bold: true}
		headerRow := container.NewBorder(nil, nil, nil, headerTrailing, s.titleText)
		paddedHeader := container.New(
			layout.NewCustomPaddedLayout(
				dialogShellHeaderTopPad,
				dialogShellHeaderBottomPad,
				dialogShellHeaderSidePad,
				dialogShellHeaderSidePad,
			),
			headerRow,
		)
		s.headerDivider = canvas.NewRectangle(color.Transparent)
		s.headerDivider.SetMinSize(fyne.NewSize(0, 1))
		top = container.NewVBox(paddedHeader, s.headerDivider)
	}

	s.content = container.NewBorder(top, nil, nil, nil, paddedBody)
	s.ExtendBaseWidget(s)
	return s
}

func (s *dialogShellWidget) CreateRenderer() fyne.WidgetRenderer {
	objects := []fyne.CanvasObject{s.background, s.content}
	if s.accentFrame != nil {
		objects = append(objects, s.accentFrame)
	}
	r := &dialogShellRenderer{shell: s, objects: objects}
	r.applyTheme()
	return r
}

type dialogShellRenderer struct {
	shell   *dialogShellWidget
	objects []fyne.CanvasObject
}

func (r *dialogShellRenderer) Destroy() {}

func (r *dialogShellRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *dialogShellRenderer) MinSize() fyne.Size {
	return r.shell.content.MinSize()
}

func (r *dialogShellRenderer) Layout(size fyne.Size) {
	r.shell.background.Move(fyne.NewPos(0, 0))
	r.shell.background.Resize(size)
	r.shell.content.Move(fyne.NewPos(0, 0))
	r.shell.content.Resize(size)
	if r.shell.accentFrame != nil {
		r.shell.accentFrame.Move(fyne.NewPos(0, 0))
		r.shell.accentFrame.Resize(size)
	}
}

func (r *dialogShellRenderer) Refresh() {
	r.applyTheme()
	r.shell.background.Refresh()
	if r.shell.titleText != nil {
		r.shell.titleText.Refresh()
	}
	if r.shell.headerDivider != nil {
		r.shell.headerDivider.Refresh()
	}
	r.shell.content.Refresh()
	if r.shell.accentFrame != nil {
		r.shell.accentFrame.Refresh()
	}
}

func (r *dialogShellRenderer) applyTheme() {
	th := r.shell.Theme()
	variant := theme.VariantDark
	if app := fyne.CurrentApp(); app != nil {
		variant = app.Settings().ThemeVariant()
	}

	radius := th.Size(theme.SizeNamePopupRadius)
	if radius <= 0 {
		radius = th.Size(theme.SizeNameDialogRadius)
	}

	r.shell.background.FillColor = th.Color(theme.ColorNameOverlayBackground, variant)
	r.shell.background.StrokeColor = th.Color(theme.ColorNameInputBorder, variant)
	r.shell.background.StrokeWidth = dialogShellBorderWidth
	r.shell.background.CornerRadius = radius

	if r.shell.titleText != nil {
		r.shell.titleText.Color = th.Color(theme.ColorNameForeground, variant)
		r.shell.titleText.TextSize = th.Size(theme.SizeNameText) + dialogShellTitleSizeDelta
	}
	if r.shell.headerDivider != nil {
		r.shell.headerDivider.FillColor = th.Color(theme.ColorNameSeparator, variant)
	}
	if r.shell.accentFrame != nil {
		r.shell.accentFrame.SetCornerRadius(radius)
	}
}
