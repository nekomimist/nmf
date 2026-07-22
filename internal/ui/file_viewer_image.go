package ui

import (
	"fmt"
	"image"
	"image/draw"
	"math"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	xdraw "golang.org/x/image/draw"

	"nmf/internal/keymanager"
)

const (
	fileViewerImageMinZoom  = 0.10
	fileViewerImageMaxZoom  = 8.00
	fileViewerImageZoomStep = 1.25
	fileViewerImagePanStep  = 32
)

// fileViewerImageView renders only the visible viewport. Keeping the generated
// raster at viewport size avoids creating a GPU texture as large as the source
// image while still making 100% mean one source pixel per physical pixel.
type fileViewerImageView struct {
	widget.BaseWidget

	mu     sync.RWMutex
	source image.Image
	raster *canvas.Raster
	km     *keymanager.KeyManager

	fit     bool
	zoom    float64
	centerX float64
	centerY float64

	onChanged  func()
	debugPrint func(format string, args ...interface{})
}

func newFileViewerImageView(source image.Image, km *keymanager.KeyManager, onChanged func(), debugPrint func(format string, args ...interface{})) *fileViewerImageView {
	v := &fileViewerImageView{
		source:     source,
		km:         km,
		fit:        true,
		zoom:       1,
		onChanged:  onChanged,
		debugPrint: debugPrint,
	}
	if source != nil {
		bounds := source.Bounds()
		v.centerX = float64(bounds.Dx()) / 2
		v.centerY = float64(bounds.Dy()) / 2
	}
	v.raster = canvas.NewRaster(v.render)
	v.raster.ScaleMode = canvas.ImageScaleSmooth
	v.ExtendBaseWidget(v)
	return v
}

func (v *fileViewerImageView) CreateRenderer() fyne.WidgetRenderer {
	return &fileViewerImageRenderer{view: v}
}

func (v *fileViewerImageView) Resize(size fyne.Size) {
	old := v.Size()
	v.BaseWidget.Resize(size)
	if v.raster != nil {
		v.raster.Resize(size)
	}
	if old != size {
		v.clampCenter()
		v.notifyChanged()
	}
}

func (v *fileViewerImageView) FocusGained() {}

func (v *fileViewerImageView) FocusLost() {}

func (v *fileViewerImageView) TypedKey(ev *fyne.KeyEvent) {
	if v.km != nil {
		v.km.HandleTypedKey(ev)
	}
}

func (v *fileViewerImageView) TypedRune(r rune) {
	if v.km != nil {
		v.km.HandleTypedRune(r)
	}
}

func (v *fileViewerImageView) TypedShortcut(shortcut fyne.Shortcut) {
	if v.km != nil {
		v.km.HandleShortcut(shortcut)
	}
}

func (v *fileViewerImageView) KeyDown(ev *fyne.KeyEvent) {
	if v.km != nil {
		v.km.HandleKeyDown(ev)
	}
}

func (v *fileViewerImageView) KeyUp(ev *fyne.KeyEvent) {
	if v.km != nil {
		v.km.HandleKeyUp(ev)
	}
}

func (v *fileViewerImageView) AcceptsTab() bool { return true }

func (v *fileViewerImageView) Tapped(*fyne.PointEvent) {}

func (v *fileViewerImageView) Dragged(ev *fyne.DragEvent) {
	if ev == nil {
		return
	}
	// Grab-style panning: dragging the bitmap right reveals content to the left.
	v.PanByDisplay(-float64(ev.Dragged.DX), -float64(ev.Dragged.DY))
}

func (v *fileViewerImageView) DragEnd() {}

func (v *fileViewerImageView) Cursor() desktop.Cursor {
	if v.Fit() {
		return desktop.DefaultCursor
	}
	return desktop.CrosshairCursor
}

func (v *fileViewerImageView) ToggleFit() {
	v.mu.Lock()
	v.fit = !v.fit
	leavingFit := !v.fit
	v.mu.Unlock()
	if leavingFit {
		v.clampCenter()
	}
	v.refresh("toggle-fit")
}

func (v *fileViewerImageView) ZoomIn() bool {
	return v.changeZoom(fileViewerImageZoomStep)
}

func (v *fileViewerImageView) ZoomOut() bool {
	return v.changeZoom(1 / fileViewerImageZoomStep)
}

func (v *fileViewerImageView) changeZoom(factor float64) bool {
	v.mu.Lock()
	if v.fit {
		v.mu.Unlock()
		return false
	}
	old := v.zoom
	v.zoom = math.Max(fileViewerImageMinZoom, math.Min(fileViewerImageMaxZoom, v.zoom*factor))
	changed := math.Abs(v.zoom-old) > 0.000001
	v.mu.Unlock()
	if !changed {
		return false
	}
	v.clampCenter()
	v.refresh("zoom")
	return true
}

// PanByDisplay moves the visible range by logical canvas units. Positive X/Y
// moves the viewport right/down over the source image.
func (v *fileViewerImageView) PanByDisplay(dx, dy float64) bool {
	v.mu.Lock()
	if v.fit || v.source == nil {
		v.mu.Unlock()
		return false
	}
	scale := v.canvasScale()
	oldX, oldY := v.centerX, v.centerY
	v.centerX += dx * scale / v.zoom
	v.centerY += dy * scale / v.zoom
	v.clampCenterLocked(v.viewportPixelSize())
	changed := math.Abs(v.centerX-oldX) > 0.000001 || math.Abs(v.centerY-oldY) > 0.000001
	v.mu.Unlock()
	if !changed {
		return false
	}
	v.refresh("pan")
	return true
}

func (v *fileViewerImageView) PanLine(dx, dy int) bool {
	return v.PanByDisplay(float64(dx*fileViewerImagePanStep), float64(dy*fileViewerImagePanStep))
}

func (v *fileViewerImageView) PanPage(direction int) bool {
	height := float64(v.Size().Height) * 0.9
	return v.PanByDisplay(0, float64(direction)*height)
}

func (v *fileViewerImageView) Fit() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.fit
}

func (v *fileViewerImageView) Zoom() float64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.zoom
}

func (v *fileViewerImageView) EffectiveZoom() float64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if !v.fit {
		return v.zoom
	}
	size := v.viewportPixelSize()
	return v.fitZoomLocked(float64(size.Width), float64(size.Height))
}

func (v *fileViewerImageView) ModeText() string {
	zoom := v.EffectiveZoom() * 100
	if v.Fit() {
		return fmt.Sprintf("fit %.0f%%", zoom)
	}
	return fmt.Sprintf("%.0f%%", zoom)
}

func (v *fileViewerImageView) render(width, height int) image.Image {
	if width < 1 || height < 1 {
		return image.NewNRGBA(image.Rect(0, 0, max(width, 1), max(height, 1)))
	}
	dst := image.NewNRGBA(image.Rect(0, 0, width, height))

	v.mu.RLock()
	source := v.source
	if source == nil {
		v.mu.RUnlock()
		return dst
	}
	zoom := v.zoom
	centerX, centerY := v.centerX, v.centerY
	fit := v.fit
	if fit {
		zoom = v.fitZoomLocked(float64(width), float64(height))
		bounds := source.Bounds()
		centerX = float64(bounds.Dx()) / 2
		centerY = float64(bounds.Dy()) / 2
	}
	v.mu.RUnlock()

	bounds := source.Bounds()
	scaledWidth := int(math.Round(float64(bounds.Dx()) * zoom))
	scaledHeight := int(math.Round(float64(bounds.Dy()) * zoom))
	left := int(math.Round(float64(width)/2 - centerX*zoom))
	top := int(math.Round(float64(height)/2 - centerY*zoom))
	target := image.Rect(left, top, left+scaledWidth, top+scaledHeight)
	if target.Empty() {
		return dst
	}
	xdraw.ApproxBiLinear.Scale(dst, target, source, bounds, draw.Over, nil)
	return dst
}

func (v *fileViewerImageView) clampCenter() {
	v.mu.Lock()
	v.clampCenterLocked(v.viewportPixelSize())
	v.mu.Unlock()
}

func (v *fileViewerImageView) clampCenterLocked(viewport fyne.Size) {
	if v.source == nil {
		return
	}
	bounds := v.source.Bounds()
	width := float64(bounds.Dx())
	height := float64(bounds.Dy())
	if v.fit {
		return
	}
	v.centerX = clampImageCenter(v.centerX, width, float64(viewport.Width)/v.zoom)
	v.centerY = clampImageCenter(v.centerY, height, float64(viewport.Height)/v.zoom)
}

func clampImageCenter(center, sourceLength, viewportLength float64) float64 {
	if sourceLength <= 0 || viewportLength >= sourceLength {
		return sourceLength / 2
	}
	half := viewportLength / 2
	return math.Max(half, math.Min(sourceLength-half, center))
}

func (v *fileViewerImageView) fitZoomLocked(viewportWidth, viewportHeight float64) float64 {
	if v.source == nil || viewportWidth <= 0 || viewportHeight <= 0 {
		return 1
	}
	bounds := v.source.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return 1
	}
	return math.Min(1, math.Min(viewportWidth/float64(bounds.Dx()), viewportHeight/float64(bounds.Dy())))
}

func (v *fileViewerImageView) viewportPixelSize() fyne.Size {
	scale := float32(v.canvasScale())
	return fyne.NewSize(v.Size().Width*scale, v.Size().Height*scale)
}

func (v *fileViewerImageView) canvasScale() float64 {
	app := fyne.CurrentApp()
	if app == nil || app.Driver() == nil {
		return 1
	}
	c := app.Driver().CanvasForObject(v)
	if c == nil || c.Scale() <= 0 {
		return 1
	}
	return float64(c.Scale())
}

func (v *fileViewerImageView) refresh(reason string) {
	if v.raster != nil {
		v.raster.Refresh()
	}
	if v.debugPrint != nil {
		v.mu.RLock()
		centerX, centerY := v.centerX, v.centerY
		v.mu.RUnlock()
		v.debugPrint("FileViewer: image-view action=%s mode=%s center=%.1f,%.1f", reason, v.ModeText(), centerX, centerY)
	}
	v.notifyChanged()
}

func (v *fileViewerImageView) notifyChanged() {
	if v.onChanged != nil {
		v.onChanged()
	}
}

type fileViewerImageRenderer struct {
	view *fileViewerImageView
}

func (r *fileViewerImageRenderer) Destroy() {}

func (r *fileViewerImageRenderer) Layout(size fyne.Size) {
	r.view.raster.Move(fyne.NewPos(0, 0))
	r.view.raster.Resize(size)
}

func (r *fileViewerImageRenderer) MinSize() fyne.Size {
	return fyne.NewSize(0, 0)
}

func (r *fileViewerImageRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.view.raster}
}

func (r *fileViewerImageRenderer) Refresh() {
	r.view.raster.Refresh()
}
