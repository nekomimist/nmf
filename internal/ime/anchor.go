package ime

import (
	"sync/atomic"

	"fyne.io/fyne/v2"
)

var enabled atomic.Bool

func init() {
	enabled.Store(true)
}

// SetEnabled controls whether nmf updates platform IME candidate anchors.
func SetEnabled(on bool) {
	enabled.Store(on)
}

// Enabled reports whether platform IME candidate anchor updates are enabled.
func Enabled() bool {
	return enabled.Load()
}

// SetAnchor asks the platform IME to show composition and candidate UI near
// object.Position()+local. size describes the visual text/caret area to avoid.
func SetAnchor(window fyne.Window, object fyne.CanvasObject, local fyne.Position, size fyne.Size) bool {
	if !Enabled() {
		return false
	}
	return setAnchor(window, object, local, size)
}
