package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func dialogCancelButton(text string, tapped func()) *widget.Button {
	return widget.NewButtonWithIcon(text, theme.CancelIcon(), tapped)
}

func dialogConfirmButton(text string, tapped func()) *widget.Button {
	button := widget.NewButtonWithIcon(text, theme.ConfirmIcon(), tapped)
	button.Importance = widget.HighImportance
	return button
}

func dialogButtonRow(cancelText string, cancelTapped func(), confirmText string, confirmTapped func()) fyne.CanvasObject {
	return container.NewGridWithColumns(
		2,
		dialogCancelButton(cancelText, cancelTapped),
		dialogConfirmButton(confirmText, confirmTapped),
	)
}

func dialogOKButtonRow(tapped func()) fyne.CanvasObject {
	return container.NewGridWithColumns(1, dialogConfirmButton("OK", tapped))
}
