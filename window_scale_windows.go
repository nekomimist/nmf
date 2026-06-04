//go:build windows

package main

import (
	"math"

	"fyne.io/fyne/v2"
)

const windowsDefaultDPI = 96.0

var procGetDpiForWindow = winUser32.NewProc("GetDpiForWindow")

func resyncWindowScaleForReset(window fyne.Window) {
	if window == nil || window.Canvas() == nil {
		return
	}

	canvasScale := window.Canvas().Scale()
	windowScale, ok := currentWindowScale(window)
	if !ok {
		debugPrint("FileManager: DPI scale check unavailable canvas_scale=%.2f", canvasScale)
		return
	}
	if math.Abs(float64(canvasScale-windowScale)) < 0.05 {
		debugPrint("FileManager: DPI scale unchanged canvas_scale=%.2f dpi_scale=%.2f", canvasScale, windowScale)
		return
	}

	app := fyne.CurrentApp()
	if app == nil || app.Settings() == nil || app.Settings().Theme() == nil {
		debugPrint("FileManager: DPI scale changed but theme refresh unavailable canvas_scale=%.2f dpi_scale=%.2f", canvasScale, windowScale)
		return
	}

	debugPrint("FileManager: DPI scale changed canvas_scale=%.2f dpi_scale=%.2f; refreshing theme", canvasScale, windowScale)
	app.Settings().SetTheme(app.Settings().Theme())
	debugPrint("FileManager: DPI scale refresh complete canvas_scale=%.2f", window.Canvas().Scale())
}

func currentWindowScale(window fyne.Window) (float32, bool) {
	hwnd, ok := windowHWND(window)
	if !ok || hwnd == 0 {
		return 0, false
	}

	if err := procGetDpiForWindow.Find(); err != nil {
		return 0, false
	}
	dpi, _, _ := procGetDpiForWindow.Call(hwnd)
	if dpi == 0 {
		return 0, false
	}
	return float32(float64(dpi) / windowsDefaultDPI), true
}
