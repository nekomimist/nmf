//go:build windows

package main

import "os"

const fyneDisableDPIDetection = "FYNE_DISABLE_DPI_DETECTION"

// Fyne 2.8.1 guards monitor video-mode queries when displays disappear.
// Keep the previous workaround available for RDP regression testing, but do
// not disable Fyne's per-monitor DPI detection by default.
const forceDisableFyneDPIDetection = false

func applyDisplayDPIWorkaround() {
	if !forceDisableFyneDPIDetection {
		debugPrint("Display: Fyne DPI detection workaround inactive")
		return
	}

	if err := os.Setenv(fyneDisableDPIDetection, "1"); err != nil {
		debugPrint("Display: failed to disable Fyne DPI detection: %v", err)
		return
	}

	debugPrint("Display: disabled Fyne DPI detection workaround")
}
