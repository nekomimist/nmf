package ui

import (
	"image/color"
)

// ThemeColorProvider provides custom theme colors
type ThemeColorProvider interface {
	GetCustomColor(colorType string) color.RGBA
}
