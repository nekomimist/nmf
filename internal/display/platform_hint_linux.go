//go:build linux

package display

import (
	"os"

	"github.com/go-gl/glfw/v3.4/glfw"
)

const platformEnvKey = "FYNE_PLATFORM"

// platformHintFromEnv mirrors Fyne's forcePlatform decision (see
// fyne.io/fyne/v2/internal/driver/glfw/wayland_csd_linux.go): a FYNE_PLATFORM
// value of "x11" or "wayland" forces the corresponding GLFW platform; any other
// value, including unset, leaves the choice to GLFW's own auto-detection. This
// is a pure function so the decision can be table-tested without calling into
// GLFW.
func platformHintFromEnv(value string) (hint glfw.Platform, ok bool) {
	switch value {
	case "x11":
		return glfw.PlatformX11, true
	case "wayland":
		return glfw.PlatformWayland, true
	default:
		return 0, false
	}
}

// applyPlatformHint applies the same GLFW platform hint Fyne's own initGLFW()
// would apply, based on FYNE_PLATFORM, before glfw.Init() locks in a platform
// choice for the process.
func applyPlatformHint(debugPrint func(format string, args ...interface{})) {
	if debugPrint == nil {
		debugPrint = func(string, ...interface{}) {}
	}

	value := os.Getenv(platformEnvKey)
	hint, ok := platformHintFromEnv(value)
	if !ok {
		debugPrint("Display: platform hint none %s=%q", platformEnvKey, value)
		return
	}

	glfw.InitHint(glfw.PlatformHint, int(hint))
	debugPrint("Display: platform hint applied %s=%q", platformEnvKey, value)
}
