package display

import (
	"math"
	"os"
	"strconv"
)

func effectiveScale(user, system, detected float32) float32 {
	if user < 0 {
		user = 1
	}
	if system == scaleAuto {
		system = detected
	}
	raw := system * user
	if raw <= 0 {
		raw = 1
	}
	return float32(math.Round(float64(raw*10))) / 10
}

func userScaleFromEnv() float32 {
	env := os.Getenv(scaleEnvKey)
	if env != "" && env != "auto" {
		scale, err := strconv.ParseFloat(env, 32)
		if err == nil && scale != 0 {
			return float32(scale)
		}
	}
	return 1
}

func detectedScale(widthMm, widthPx int) float32 {
	dpi := float32(widthPx) / (float32(widthMm) / 25.4)
	if dpi > 1000 || dpi < 10 {
		dpi = baselineDPI
	}

	scale := float32(float64(dpi) / baselineDPI)
	if scale < 1 {
		return 1
	}
	return scale
}

func scaledInt(value int, scale float32) int {
	if scale <= 0 {
		return value
	}
	return int(math.Round(float64(value) / float64(scale)))
}
