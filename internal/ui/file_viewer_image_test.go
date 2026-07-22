package ui

import (
	"image"
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestFileViewerImageViewFitAndZoomState(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	view := newFileViewerImageView(image.NewNRGBA(image.Rect(0, 0, 400, 100)), nil, nil, nil)
	view.Resize(fyne.NewSize(200, 100))

	if !view.Fit() {
		t.Fatal("initial mode should be fit")
	}
	if got := view.EffectiveZoom(); math.Abs(got-0.5) > 0.001 {
		t.Fatalf("fit zoom = %f, want 0.5", got)
	}

	view.ToggleFit()
	if view.Fit() || view.Zoom() != 1 {
		t.Fatalf("after toggle fit=%t zoom=%f, want 100%% mode", view.Fit(), view.Zoom())
	}
	if !view.PanByDisplay(32, 0) {
		t.Fatal("pan should move an overflowing 100% image")
	}
	center := view.centerX
	view.ZoomIn()
	if got := view.Zoom(); math.Abs(got-1.25) > 0.001 {
		t.Fatalf("zoom after + = %f, want 1.25", got)
	}

	view.ToggleFit()
	view.Resize(fyne.NewSize(160, 80))
	if view.centerX != center {
		t.Fatalf("fit mode changed saved center from %f to %f", center, view.centerX)
	}
	view.ToggleFit()
	if got := view.Zoom(); math.Abs(got-1.25) > 0.001 {
		t.Fatalf("restored zoom = %f, want 1.25", got)
	}
	if view.centerX != center {
		t.Fatalf("restored center = %f, want %f", view.centerX, center)
	}
}

func TestFileViewerImageViewFitDoesNotUpscale(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	view := newFileViewerImageView(image.NewNRGBA(image.Rect(0, 0, 40, 20)), nil, nil, nil)
	view.Resize(fyne.NewSize(200, 100))
	if got := view.EffectiveZoom(); got != 1 {
		t.Fatalf("fit zoom = %f, want no upscale", got)
	}
	if view.PanByDisplay(32, 32) {
		t.Fatal("fit mode should not pan")
	}
}

func TestFileViewerImageViewZoomLimitsAndPanClamp(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	view := newFileViewerImageView(image.NewNRGBA(image.Rect(0, 0, 400, 400)), nil, nil, nil)
	view.Resize(fyne.NewSize(100, 100))
	view.ToggleFit()
	for range 100 {
		view.ZoomIn()
	}
	if got := view.Zoom(); got != fileViewerImageMaxZoom {
		t.Fatalf("maximum zoom = %f, want %f", got, fileViewerImageMaxZoom)
	}
	view.PanByDisplay(100000, 100000)
	wantMax := 400 - 100/(2*fileViewerImageMaxZoom)
	if math.Abs(view.centerX-wantMax) > 0.001 || math.Abs(view.centerY-wantMax) > 0.001 {
		t.Fatalf("clamped center = %.3f,%.3f, want %.3f", view.centerX, view.centerY, wantMax)
	}
	for range 200 {
		view.ZoomOut()
	}
	if got := view.Zoom(); got != fileViewerImageMinZoom {
		t.Fatalf("minimum zoom = %f, want %f", got, fileViewerImageMinZoom)
	}
}

func TestFileViewerImageViewRenderUsesViewportSize(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 1000, 500))
	view := newFileViewerImageView(source, nil, nil, nil)
	rendered := view.render(120, 80)
	if got := rendered.Bounds().Size(); got.X != 120 || got.Y != 80 {
		t.Fatalf("render size = %v, want 120x80 viewport", got)
	}
}
