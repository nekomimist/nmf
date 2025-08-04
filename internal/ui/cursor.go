package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"

	"nmf/internal/config"
)

// CursorRenderer interface for different cursor display styles
type CursorRenderer interface {
	RenderCursor(bounds fyne.Size, textBounds fyne.Position, config config.CursorStyleConfig) fyne.CanvasObject
}

// UnderlineCursorRenderer renders cursor as an underline
type UnderlineCursorRenderer struct{}

func (r *UnderlineCursorRenderer) RenderCursor(bounds fyne.Size, textBounds fyne.Position, config config.CursorStyleConfig) fyne.CanvasObject {
	underline := canvas.NewRectangle(color.RGBA{
		R: config.Color[0],
		G: config.Color[1],
		B: config.Color[2],
		A: config.Color[3],
	})

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

func (r *BorderCursorRenderer) RenderCursor(bounds fyne.Size, textBounds fyne.Position, config config.CursorStyleConfig) fyne.CanvasObject {
	borderColor := color.RGBA{
		R: config.Color[0],
		G: config.Color[1],
		B: config.Color[2],
		A: config.Color[3],
	}

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

func (r *BackgroundCursorRenderer) RenderCursor(bounds fyne.Size, textBounds fyne.Position, config config.CursorStyleConfig) fyne.CanvasObject {
	background := canvas.NewRectangle(color.RGBA{
		R: config.Color[0],
		G: config.Color[1],
		B: config.Color[2],
		A: config.Color[3],
	})

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
