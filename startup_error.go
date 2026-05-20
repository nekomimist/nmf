package main

import (
	"fmt"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"

	"nmf/internal/config"
	customtheme "nmf/internal/theme"
	"nmf/internal/ui"
)

func showStartupConfigScriptErrorAndExit(cfg *config.Config, err error) {
	fyneApp := fyneapp.NewWithID(appID)
	fyneApp.SetIcon(appIconResource)
	fyneApp.Settings().SetTheme(customtheme.NewCustomTheme(cfg, debugPrint))

	window := fyneApp.NewWindow("nmf startup error")
	window.SetCloseIntercept(func() {
		fyneApp.Quit()
	})
	window.Resize(startupErrorWindowSize())
	window.Show()

	ui.ShowCompactMessageDialogWithOnClose(
		window,
		"init.star error",
		startupConfigScriptErrorMessage(err),
		func() {
			fyneApp.Quit()
		},
	)
	fyneApp.Run()
}

func startupConfigScriptErrorMessage(err error) string {
	return fmt.Sprintf("Failed to load init.star.\n\n%s\n\nnmf will exit.", err)
}

func startupErrorWindowSize() fyne.Size {
	return fyne.NewSize(640, 360)
}
