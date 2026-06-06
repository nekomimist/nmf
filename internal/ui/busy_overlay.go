package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	customtheme "nmf/internal/theme"
)

// busyBlocker is a full-window widget that:
// - shows a semi-transparent backdrop + centered spinner+label
// - returns a Wait cursor
// - swallows taps so underlying UI cannot be interacted with
type busyBlocker struct {
	widget.BaseWidget
	content *fyne.Container
}

func newBusyBlocker(content *fyne.Container) *busyBlocker {
	b := &busyBlocker{content: content}
	b.ExtendBaseWidget(b)
	return b
}

// Implement a simple renderer wrapping content
func (b *busyBlocker) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(b.content)
}

// Swallow taps to block interactions underneath
func (b *busyBlocker) Tapped(_ *fyne.PointEvent)          {}
func (b *busyBlocker) TappedSecondary(_ *fyne.PointEvent) {}

// Provide a busy cursor when hovering over the overlay
// Note: We don't set a special cursor here to keep compatibility
// across older Fyne versions. The overlay visuals are sufficient.
// func (b *busyBlocker) Cursor() desktop.Cursor { return desktop.DefaultCursor }

// BusyOverlay provides an indeterminate progress indication overlay.
// It's intentionally lightweight and non-interactive while visible.
type BusyOverlay struct {
	blocker *busyBlocker
	spinner *widget.ProgressBarInfinite
	label   *shrinkingTextLabel
	root    *fyne.Container
	visible bool
}

func NewBusyOverlay(themeProvider ThemeColorProvider) *BusyOverlay {
	spinner := widget.NewProgressBarInfinite()

	text := canvas.NewText("Working...", currentAppThemeColor(fynetheme.ColorNameForeground))
	text.Alignment = fyne.TextAlignCenter
	lbl := newShrinkingTextLabel(text)

	// Semi-transparent backdrop
	bg := canvas.NewRectangle(themeProvider.GetCustomColor(customtheme.ColorBusyOverlayBackground))

	content := newBusyOverlayContent(spinner, lbl)

	// Stack background + center content
	max := container.NewMax(bg, content)
	blk := newBusyBlocker(max)

	// Root container holds the blocker (easy to Hide/Show)
	root := container.NewMax(blk)
	root.Hide()

	return &BusyOverlay{
		blocker: blk,
		spinner: spinner,
		label:   lbl,
		root:    root,
		visible: false,
	}
}

func (bo *BusyOverlay) GetContainer() *fyne.Container { return bo.root }

func (bo *BusyOverlay) Show(parent fyne.Window, text string) {
	if text != "" {
		bo.label.SetText(text)
	}
	bo.spinner.Start()
	bo.spinner.Refresh()
	if bo.visible {
		bo.refresh(parent)
		return
	}
	bo.visible = true
	bo.root.Show()
	bo.refresh(parent)
}

func (bo *BusyOverlay) Hide() {
	if !bo.visible {
		return
	}
	bo.visible = false
	bo.spinner.Stop()
	bo.root.Hide()
	canvas.Refresh(bo.root)
}

func (bo *BusyOverlay) IsVisible() bool { return bo.visible }

func (bo *BusyOverlay) refresh(parent fyne.Window) {
	canvas.Refresh(bo.root)
	if parent != nil && parent.Canvas() != nil {
		parent.Canvas().Refresh(bo.root)
	}
}

type busyOverlayContent struct {
	widget.BaseWidget
	spinner *widget.ProgressBarInfinite
	label   *shrinkingTextLabel
}

func newBusyOverlayContent(spinner *widget.ProgressBarInfinite, label *shrinkingTextLabel) *busyOverlayContent {
	content := &busyOverlayContent{
		spinner: spinner,
		label:   label,
	}
	content.ExtendBaseWidget(content)
	return content
}

func (c *busyOverlayContent) CreateRenderer() fyne.WidgetRenderer {
	return &busyOverlayContentRenderer{content: c}
}

type busyOverlayContentRenderer struct {
	content *busyOverlayContent
}

func (r *busyOverlayContentRenderer) Layout(size fyne.Size) {
	padding := fynetheme.Padding()
	availableWidth := size.Width - padding*4
	if availableWidth < 0 {
		availableWidth = 0
	}

	contentWidth := fyne.Min(availableWidth, 900)
	spinnerMin := r.content.spinner.MinSize()
	if contentWidth < spinnerMin.Width {
		contentWidth = fyne.Min(availableWidth, spinnerMin.Width)
	}

	spinnerHeight := spinnerMin.Height
	labelHeight := r.content.label.MinSize().Height
	totalHeight := spinnerHeight + padding + labelHeight
	x := (size.Width - contentWidth) / 2
	y := (size.Height - totalHeight) / 2

	r.content.spinner.Move(fyne.NewPos(x, y))
	r.content.spinner.Resize(fyne.NewSize(contentWidth, spinnerHeight))
	r.content.label.Move(fyne.NewPos(x, y+spinnerHeight+padding))
	r.content.label.Resize(fyne.NewSize(contentWidth, labelHeight))
}

func (r *busyOverlayContentRenderer) MinSize() fyne.Size {
	spinnerMin := r.content.spinner.MinSize()
	labelMin := r.content.label.MinSize()
	return fyne.NewSize(spinnerMin.Width, spinnerMin.Height+fynetheme.Padding()+labelMin.Height)
}

func (r *busyOverlayContentRenderer) Refresh() {
	r.content.label.text.Color = currentAppThemeColor(fynetheme.ColorNameForeground)
	r.Layout(r.content.Size())
	canvas.Refresh(r.content)
}

func (r *busyOverlayContentRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.content.spinner, r.content.label}
}

func (r *busyOverlayContentRenderer) Destroy() {}
