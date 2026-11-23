package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	customtheme "nmf/internal/theme"
)

func currentThemeColorProvider() ThemeColorProvider {
	if fyne.CurrentApp() == nil {
		return nil
	}
	themeProvider, ok := fyne.CurrentApp().Settings().Theme().(ThemeColorProvider)
	if !ok {
		return nil
	}
	return themeProvider
}

func lineEditThemeOverride(obj fyne.CanvasObject) fyne.CanvasObject {
	themeProvider := currentThemeColorProvider()
	if themeProvider == nil {
		return obj
	}
	base := fyne.CurrentApp().Settings().Theme()
	return container.NewThemeOverride(obj, customtheme.NewLineEditOverrideTheme(base, themeProvider))
}

func dialogListThemeOverride(obj fyne.CanvasObject) fyne.CanvasObject {
	themeProvider := currentThemeColorProvider()
	if themeProvider == nil {
		return obj
	}
	base := fyne.CurrentApp().Settings().Theme()
	return container.NewThemeOverride(obj, customtheme.NewDialogListOverrideTheme(base, themeProvider))
}

func menuThemeOverride(obj fyne.CanvasObject) fyne.CanvasObject {
	themeProvider := currentThemeColorProvider()
	if themeProvider == nil {
		return obj
	}
	base := fyne.CurrentApp().Settings().Theme()
	return container.NewThemeOverride(obj, customtheme.NewMenuOverrideTheme(base, themeProvider))
}

func dialogThemeOverride(obj fyne.CanvasObject) fyne.CanvasObject {
	if obj == nil || fyne.CurrentApp() == nil {
		return obj
	}
	base := fyne.CurrentApp().Settings().Theme()
	return container.NewThemeOverride(obj, customtheme.NewDialogOverrideTheme(base))
}

// DialogThemeOverride exposes dialogThemeOverride to package main (drop_ui.go).
func DialogThemeOverride(obj fyne.CanvasObject) fyne.CanvasObject {
	return dialogThemeOverride(obj)
}
