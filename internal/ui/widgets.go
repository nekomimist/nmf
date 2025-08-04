package ui

import (
	"fyne.io/fyne/v2"
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
