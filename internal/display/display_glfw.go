//go:build linux || windows

package display

import (
	"fmt"
	"math"

	"github.com/go-gl/glfw/v3.3/glfw"
)

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
	debugPrint("Display: primary name=%s work=%dx%d pixels=%dx%d scale=%.2f",
		info.Name,
		info.WorkWidth,
		info.WorkHeight,
		info.PixelWidth,
		info.PixelHeight,
		info.Scale,
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

	_, _, workWidth, workHeight := monitor.GetWorkarea()
	scale, _ := monitor.GetContentScale()
	if scale <= 0 {
		scale = 1
	}

	return Info{
		Available:   true,
		Name:        monitor.GetName(),
		Width:       scaledInt(mode.Width, scale),
		Height:      scaledInt(mode.Height, scale),
		WorkWidth:   workWidth,
		WorkHeight:  workHeight,
		PixelWidth:  mode.Width,
		PixelHeight: mode.Height,
		Scale:       scale,
	}, nil
}

func scaledInt(value int, scale float32) int {
	if scale <= 0 {
		return value
	}
	return int(math.Round(float64(value) / float64(scale)))
}
