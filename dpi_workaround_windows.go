//go:build windows

package main

import "os"

const fyneDisableDPIDetection = "FYNE_DISABLE_DPI_DETECTION"

func applyDisplayDPIWorkaround() {
	if err := os.Setenv(fyneDisableDPIDetection, "1"); err != nil {
		debugPrint("Display: failed to disable Fyne DPI detection: %v", err)
		return
	}

	debugPrint("Display: disabled Fyne DPI detection workaround")
}
