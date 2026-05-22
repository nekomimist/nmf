//go:build windows

package main

import "fyne.io/fyne/v2"

func applyWindowSwitchRects(candidates []windowSwitchCandidate) {
	for i := range candidates {
		if candidates[i].manager == nil || candidates[i].manager.window == nil {
			continue
		}
		rect, ok := fyneWindowSwitchRect(candidates[i].manager.window)
		candidates[i].rect = rect
		candidates[i].hasRect = ok
	}
}

func platformWindowSwitchRect(window fyne.Window) (windowSwitchRect, bool) {
	hwnd, ok := windowHWND(window)
	if !ok {
		return windowSwitchRect{}, false
	}

	if isWindowIconic(hwnd) {
		placement, ok := getWindowPlacement(hwnd)
		if ok {
			return windowSwitchRectFromWinRect(placement.RcNormalPosition), true
		}
		debugPrint("FileManager: minimized window placement unavailable for switch rect")
	}

	rect, ok := getWindowRect(hwnd)
	if !ok {
		return windowSwitchRect{}, false
	}
	return windowSwitchRectFromWinRect(rect), true
}

func windowSwitchRectFromWinRect(rect winRect) windowSwitchRect {
	return windowSwitchRect{
		Left:   rect.Left,
		Top:    rect.Top,
		Right:  rect.Right,
		Bottom: rect.Bottom,
	}
}
