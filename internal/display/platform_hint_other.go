//go:build !linux

package display

// applyPlatformHint is a no-op on platforms where GLFW's platform hint does not
// apply (GLFW's PlatformHint only matters where more than one native backend is
// available, i.e. Linux's X11/Wayland split).
func applyPlatformHint(debugPrint func(format string, args ...interface{})) {
	if debugPrint == nil {
		debugPrint = func(string, ...interface{}) {}
	}
	debugPrint("Display: platform hint not applicable on this platform")
}
