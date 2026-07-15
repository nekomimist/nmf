//go:build linux || windows

package display

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/go-gl/glfw/v3.4/glfw"
)

const disableDPIDetectionEnvKey = "FYNE_DISABLE_DPI_DETECTION"

// Primary returns the primary monitor information. It never returns an error to
// callers that would block startup; failures are represented as unavailable info.
func Primary(debugPrint func(format string, args ...interface{})) Info {
	if debugPrint == nil {
		debugPrint = func(string, ...interface{}) {}
	}

	info, err := primary()
	if err != nil {
		debugPrint("Display: primary unavailable err=%v", err)
		return Info{}
	}
	debugPrint("Display: primary name=%s work=%dx%d pixels=%dx%d scale=%.2f display_scale=%.2f user_scale=%.2f",
		info.Name,
		info.WorkWidth,
		info.WorkHeight,
		info.PixelWidth,
		info.PixelHeight,
		info.Scale,
		info.DisplayScale,
		info.UserScale,
	)
	return info
}

func primary() (info Info, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("glfw display query failed: %v", recovered)
		}
	}()

	if err := glfw.Init(); err != nil {
		return Info{}, err
	}

	monitor := glfw.GetPrimaryMonitor()
	if monitor == nil {
		return Info{}, fmt.Errorf("primary monitor not found")
	}

	mode := monitor.GetVideoMode()
	if mode == nil {
		return Info{}, fmt.Errorf("primary monitor video mode not found")
	}

	_, _, workWidthPx, workHeightPx := monitor.GetWorkarea()
	displayScale := monitorDisplayScale(monitor, mode.Width)
	userScale := userScaleFromEnv()
	scale := effectiveScale(userScale, systemScale(displayScale), displayScale)

	return Info{
		Available:    true,
		Name:         monitor.GetName(),
		Width:        scaledInt(mode.Width, scale),
		Height:       scaledInt(mode.Height, scale),
		WorkWidth:    scaledInt(workWidthPx, scale),
		WorkHeight:   scaledInt(workHeightPx, scale),
		PixelWidth:   mode.Width,
		PixelHeight:  mode.Height,
		Scale:        scale,
		DisplayScale: displayScale,
		UserScale:    userScale,
	}, nil
}

func monitorDisplayScale(monitor *glfw.Monitor, widthPx int) float32 {
	if runtime.GOOS == "windows" {
		xScale, _ := monitor.GetContentScale()
		if xScale > 0 {
			return xScale
		}
		return 1
	}

	if dpiDetectionDisabled() {
		return 1
	}

	widthMm, heightMm := monitor.GetPhysicalSize()
	if runtime.GOOS == "linux" && widthMm == 60 && heightMm == 60 {
		return 1
	}
	return detectedScale(widthMm, widthPx)
}

func systemScale(displayScale float32) float32 {
	if runtime.GOOS == "windows" {
		return displayScale
	}
	return scaleAuto
}

func dpiDetectionDisabled() bool {
	env := os.Getenv(disableDPIDetectionEnvKey)
	return strings.EqualFold(env, "true") || strings.EqualFold(env, "t") || env == "1"
}
