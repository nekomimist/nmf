package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

// ShowMessageDialog displays a simple OK dialog with a title and message.
// It returns immediately after showing.
func ShowMessageDialog(parent fyne.Window, title, message string) {
	d := dialog.NewInformation(title, message, parent)
	d.Show()
}
