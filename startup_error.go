package main

import (
	"fmt"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"

	"nmf/internal/config"
	customtheme "nmf/internal/theme"
	"nmf/internal/ui"
)

// showStartupErrorAndExit shows a modal error dialog in a transient Fyne app
// and blocks until it is dismissed. cfg may be nil; defaults are used for theming.
func showStartupErrorAndExit(cfg *config.Config, title, message string) {
	if cfg == nil {
		cfg = config.Default()
	}

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
		title,
		message,
		func() {
			fyneApp.Quit()
		},
	)
	fyneApp.Run()
}

func showStartupConfigScriptErrorAndExit(cfg *config.Config, err error) {
	showStartupErrorAndExit(cfg, "init.star error", startupConfigScriptErrorMessage(err))
}

// startupFailureMessage formats a startup failure for the error dialog.
func startupFailureMessage(what string, err error) string {
	return fmt.Sprintf("%s.\n\n%s\n\nnmf will exit.", what, err)
}

func startupConfigScriptErrorMessage(err error) string {
	return startupFailureMessage("Failed to load init.star", err)
}

func startupErrorWindowSize() fyne.Size {
	return fyne.NewSize(640, 360)
}
