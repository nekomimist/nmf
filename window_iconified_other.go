//go:build !windows

package main

import "fyne.io/fyne/v2"

func windowIconified(fyne.Window) bool {
	return false
}
