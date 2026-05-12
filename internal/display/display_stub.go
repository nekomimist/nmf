//go:build !linux && !windows

package display

// Primary returns unavailable display information on unsupported platforms.
func Primary(debugPrint func(format string, args ...interface{})) Info {
	if debugPrint != nil {
		debugPrint("Display: primary unavailable platform=unsupported")
	}
	return Info{}
}
