//go:build !windows

package main

import (
	"fyne.io/fyne/v2"

	"nmf/internal/config"
)

func applyInitialWindowPosition(fyne.Window, config.WindowConfig) {
}

func positionWindowNextTo(_ *ApplicationRuntime, parent, child fyne.Window) {
}

func restoreWindowBeforeFocus(fyne.Window) {
}
