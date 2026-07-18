package main

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
)

type canvasLookupFunc func(fyne.CanvasObject) fyne.Canvas

func (f canvasLookupFunc) CanvasForObject(object fyne.CanvasObject) fyne.Canvas {
	return f(object)
}

func TestCanvasMappingState(t *testing.T) {
	object := canvas.NewRectangle(color.White)
	windowCanvas := test.NewCanvas()
	otherCanvas := test.NewCanvas()

	tests := []struct {
		name   string
		lookup canvasObjectLookup
		object fyne.CanvasObject
		want   string
	}{
		{
			name:   "object missing",
			lookup: canvasLookupFunc(func(fyne.CanvasObject) fyne.Canvas { return windowCanvas }),
			want:   "object-nil",
		},
		{
			name:   "lookup missing",
			object: object,
			want:   "lookup-nil",
		},
		{
			name:   "mapping missing",
			lookup: canvasLookupFunc(func(fyne.CanvasObject) fyne.Canvas { return nil }),
			object: object,
			want:   "nil",
		},
		{
			name:   "expected window",
			lookup: canvasLookupFunc(func(fyne.CanvasObject) fyne.Canvas { return windowCanvas }),
			object: object,
			want:   "window",
		},
		{
			name:   "different window",
			lookup: canvasLookupFunc(func(fyne.CanvasObject) fyne.Canvas { return otherCanvas }),
			object: object,
			want:   "other",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canvasMappingState(tt.lookup, tt.object, windowCanvas); got != tt.want {
				t.Fatalf("canvasMappingState() = %q, want %q", got, tt.want)
			}
		})
	}
}
