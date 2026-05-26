//go:build windows

package main

import "fyne.io/fyne/v2"

func windowIconified(window fyne.Window) bool {
	hwnd, ok := windowHWND(window)
	return ok && isWindowIconic(hwnd)
}
