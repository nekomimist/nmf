package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
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
	label   *widget.Label
	root    *fyne.Container
	visible bool
}

func NewBusyOverlay() *BusyOverlay {
	spinner := widget.NewProgressBarInfinite()
	spinner.Start()

	lbl := widget.NewLabel("Working...")
	lbl.Alignment = fyne.TextAlignCenter
	lbl.Importance = widget.HighImportance

	// Semi-transparent backdrop
	bg := canvas.NewRectangle(color.NRGBA{R: 0, G: 0, B: 0, A: 96})

	// Center panel
	panel := container.NewVBox(spinner, lbl)
	padded := container.NewPadded(panel)
	centered := container.NewCenter(padded)

	// Stack background + center content
	max := container.NewMax(bg, centered)
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

func (bo *BusyOverlay) Show(_ fyne.Window, text string) {
	if text != "" {
		bo.label.SetText(text)
	}
	if bo.visible {
		return
	}
	bo.visible = true
	bo.root.Show()
}

func (bo *BusyOverlay) Hide() {
	if !bo.visible {
		return
	}
	bo.visible = false
	bo.root.Hide()
}

func (bo *BusyOverlay) IsVisible() bool { return bo.visible }
