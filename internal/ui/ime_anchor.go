package ui

import (
	"fyne.io/fyne/v2"
	fynetheme "fyne.io/fyne/v2/theme"

	"nmf/internal/ime"
)

func setIMEAnchorAtTextEnd(window fyne.Window, object fyne.CanvasObject, text string, style fyne.TextStyle) bool {
	if window == nil || object == nil {
		return false
	}
	textSize := currentAppThemeSize(fynetheme.SizeNameText)
	line := fyne.MeasureText("M", textSize, style).Height
	x := fyne.MeasureText(text, textSize, style).Width
	if x < 0 {
		x = 0
	}
	objectSize := object.Size()
	if objectSize.Width <= 0 || objectSize.Height <= 0 {
		objectSize = object.MinSize()
	}
	maxX := objectSize.Width - lineEditEntryHorizontalInset
	if maxX > lineEditEntryHorizontalInset && x > maxX {
		x = maxX
	}
	return ime.SetAnchor(window, object, fyne.NewPos(lineEditEntryHorizontalInset+x, lineEditEntryVerticalInset), fyne.NewSize(1, line))
}

func currentAppThemeSize(name fyne.ThemeSizeName) float32 {
	if fyne.CurrentApp() == nil || fyne.CurrentApp().Settings().Theme() == nil {
		return fynetheme.Size(name)
	}
	return fyne.CurrentApp().Settings().Theme().Size(name)
}
