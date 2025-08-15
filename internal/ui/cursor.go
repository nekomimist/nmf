package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"

	"nmf/internal/config"
)

// ThemeColorProvider provides custom theme colors
type ThemeColorProvider interface {
	GetCustomColor(colorType string) color.RGBA
}

// CursorRenderer interface for different cursor display styles
type CursorRenderer interface {
	RenderCursor(bounds fyne.Size, textBounds fyne.Position, config config.CursorStyleConfig, themeProvider ThemeColorProvider) fyne.CanvasObject
}

// UnderlineCursorRenderer renders cursor as an underline
type UnderlineCursorRenderer struct{}

func (r *UnderlineCursorRenderer) RenderCursor(bounds fyne.Size, textBounds fyne.Position, config config.CursorStyleConfig, themeProvider ThemeColorProvider) fyne.CanvasObject {
	cursorColor := themeProvider.GetCustomColor("cursor")
	underline := canvas.NewRectangle(cursorColor)

	thickness := float32(config.Thickness)
	if thickness <= 0 {
		thickness = 2
	}

	// Simple full-width underline at bottom edge
	underline.Resize(fyne.NewSize(bounds.Width, thickness))
	underline.Move(fyne.NewPos(0, bounds.Height-thickness))

	return underline
}

// BorderCursorRenderer renders cursor as a border
type BorderCursorRenderer struct{}

func (r *BorderCursorRenderer) RenderCursor(bounds fyne.Size, textBounds fyne.Position, config config.CursorStyleConfig, themeProvider ThemeColorProvider) fyne.CanvasObject {
	borderColor := themeProvider.GetCustomColor("cursor")

	thickness := float32(config.Thickness)
	if thickness <= 0 {
		thickness = 1
	}

	// Create border using multiple rectangles
	top := canvas.NewRectangle(borderColor)
	top.Resize(fyne.NewSize(bounds.Width, thickness))
	top.Move(fyne.NewPos(0, 0))

	bottom := canvas.NewRectangle(borderColor)
	bottom.Resize(fyne.NewSize(bounds.Width, thickness))
	bottom.Move(fyne.NewPos(0, bounds.Height-thickness))

	left := canvas.NewRectangle(borderColor)
	left.Resize(fyne.NewSize(thickness, bounds.Height))
	left.Move(fyne.NewPos(0, 0))

	right := canvas.NewRectangle(borderColor)
	right.Resize(fyne.NewSize(thickness, bounds.Height))
	right.Move(fyne.NewPos(bounds.Width-thickness, 0))

	return container.NewWithoutLayout(top, bottom, left, right)
}

// BackgroundCursorRenderer renders cursor as background highlight
type BackgroundCursorRenderer struct{}

func (r *BackgroundCursorRenderer) RenderCursor(bounds fyne.Size, textBounds fyne.Position, config config.CursorStyleConfig, themeProvider ThemeColorProvider) fyne.CanvasObject {
	cursorColor := themeProvider.GetCustomColor("cursor")
	background := canvas.NewRectangle(cursorColor)

	background.Resize(bounds)
	background.Move(fyne.NewPos(0, 0))

	return background
}

// NewCursorRenderer creates appropriate cursor renderer based on config
func NewCursorRenderer(config config.CursorStyleConfig) CursorRenderer {
	switch config.Type {
	case "underline", "":
		return &UnderlineCursorRenderer{}
	case "border":
		return &BorderCursorRenderer{}
	case "background":
		return &BackgroundCursorRenderer{}
	default:
		// Default to underline for unknown types
		return &UnderlineCursorRenderer{}
	}
}
