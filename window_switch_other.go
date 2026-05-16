//go:build !windows

package main

import "fyne.io/fyne/v2"

func applyWindowSwitchRects([]windowSwitchCandidate) {
}

func platformWindowSwitchRect(fyne.Window) (windowSwitchRect, bool) {
	return windowSwitchRect{}, false
}
