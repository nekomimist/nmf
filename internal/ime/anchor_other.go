//go:build !windows

package ime

import "fyne.io/fyne/v2"

func setAnchor(_ fyne.Window, _ fyne.CanvasObject, _ fyne.Position, _ fyne.Size) bool {
	return false
}
