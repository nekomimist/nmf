package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

const (
	navigationHistoryOrderFrecency = "Frecency"
	navigationHistoryOrderLastUsed = "Last Access"
)

func newHistoryOrderRadio(onChanged func(string)) *widget.RadioGroup {
	radio := widget.NewRadioGroup([]string{
		navigationHistoryOrderFrecency,
		navigationHistoryOrderLastUsed,
	}, onChanged)
	radio.Horizontal = true
	radio.Required = true
	radio.SetSelected(navigationHistoryOrderFrecency)
	return radio
}

func historyOrderSection(radio *widget.RadioGroup) fyne.CanvasObject {
	return container.NewBorder(nil, nil, widget.NewLabel("Order (Ctrl+R):"), nil, radio)
}

func toggleHistoryOrder(radio *widget.RadioGroup) {
	if radio == nil {
		return
	}
	if radio.Selected == navigationHistoryOrderLastUsed {
		radio.SetSelected(navigationHistoryOrderFrecency)
		return
	}
	radio.SetSelected(navigationHistoryOrderLastUsed)
}
